package router

import (
	"context"
	"errors"
	"math"
	"math/big"
	"sort"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/status-im/status-go/contracts"
	gaspriceoracle "github.com/status-im/status-go/contracts/gas-price-oracle"
	"github.com/status-im/status-go/contracts/ierc20"
	"github.com/status-im/status-go/params"
	"github.com/status-im/status-go/rpc"
	"github.com/status-im/status-go/services/ens"
	"github.com/status-im/status-go/services/stickers"
	"github.com/status-im/status-go/services/wallet/async"
	"github.com/status-im/status-go/services/wallet/bigint"
	"github.com/status-im/status-go/services/wallet/bridge"
	"github.com/status-im/status-go/services/wallet/collectibles"
	walletCommon "github.com/status-im/status-go/services/wallet/common"
	"github.com/status-im/status-go/services/wallet/market"
	"github.com/status-im/status-go/services/wallet/token"
	walletToken "github.com/status-im/status-go/services/wallet/token"
	"github.com/status-im/status-go/transactions"
)

// //////////////////////////////////////////////////////////////////////////////
// TODO: once new router is in place, remove this `router.go` file,
// rename and make `router_v2.go` file the main and only file
// //////////////////////////////////////////////////////////////////////////////

const EstimateUsername = "RandomUsername"
const EstimatePubKey = "0x04bb2024ce5d72e45d4a4f8589ae657ef9745855006996115a23a1af88d536cf02c0524a585fce7bfa79d6a9669af735eda6205d6c7e5b3cdc2b8ff7b2fa1f0b56"
const ERC721TransferString = "ERC721Transfer"
const ERC1155TransferString = "ERC1155Transfer"

var zero = big.NewInt(0)

type Path struct {
	BridgeName              string
	From                    *params.Network
	To                      *params.Network
	MaxAmountIn             *hexutil.Big
	AmountIn                *hexutil.Big
	AmountInLocked          bool
	AmountOut               *hexutil.Big
	GasAmount               uint64
	GasFees                 *SuggestedFees
	BonderFees              *hexutil.Big
	TokenFees               *big.Float
	Cost                    *big.Float
	EstimatedTime           TransactionEstimation
	ApprovalRequired        bool
	ApprovalGasFees         *big.Float
	ApprovalAmountRequired  *hexutil.Big
	ApprovalContractAddress *common.Address
}

func (p *Path) Equal(o *Path) bool {
	return p.From.ChainID == o.From.ChainID && p.To.ChainID == o.To.ChainID
}

type Graph = []*Node

type Node struct {
	Path     *Path
	Children Graph
}

func newNode(path *Path) *Node {
	return &Node{Path: path, Children: make(Graph, 0)}
}

func buildGraph(AmountIn *big.Int, routes []*Path, level int, sourceChainIDs []uint64) Graph {
	graph := make(Graph, 0)
	for _, route := range routes {
		found := false
		for _, chainID := range sourceChainIDs {
			if chainID == route.From.ChainID {
				found = true
				break
			}
		}
		if found {
			continue
		}
		node := newNode(route)

		newRoutes := make([]*Path, 0)
		for _, r := range routes {
			if route.Equal(r) {
				continue
			}
			newRoutes = append(newRoutes, r)
		}

		newAmountIn := new(big.Int).Sub(AmountIn, route.MaxAmountIn.ToInt())
		if newAmountIn.Sign() > 0 {
			newSourceChainIDs := make([]uint64, len(sourceChainIDs))
			copy(newSourceChainIDs, sourceChainIDs)
			newSourceChainIDs = append(newSourceChainIDs, route.From.ChainID)
			node.Children = buildGraph(newAmountIn, newRoutes, level+1, newSourceChainIDs)

			if len(node.Children) == 0 {
				continue
			}
		}

		graph = append(graph, node)
	}

	return graph
}

func (n Node) buildAllRoutes() [][]*Path {
	res := make([][]*Path, 0)

	if len(n.Children) == 0 && n.Path != nil {
		res = append(res, []*Path{n.Path})
	}

	for _, node := range n.Children {
		for _, route := range node.buildAllRoutes() {
			extendedRoute := route
			if n.Path != nil {
				extendedRoute = append([]*Path{n.Path}, route...)
			}
			res = append(res, extendedRoute)
		}
	}

	return res
}

func filterRoutes(routes [][]*Path, amountIn *big.Int, fromLockedAmount map[uint64]*hexutil.Big) [][]*Path {
	if len(fromLockedAmount) == 0 {
		return routes
	}

	filteredRoutesLevel1 := make([][]*Path, 0)
	for _, route := range routes {
		routeOk := true
		fromIncluded := make(map[uint64]bool)
		fromExcluded := make(map[uint64]bool)
		for chainID, amount := range fromLockedAmount {
			if amount.ToInt().Cmp(zero) == 0 {
				fromExcluded[chainID] = false
			} else {
				fromIncluded[chainID] = false
			}

		}
		for _, path := range route {
			if _, ok := fromExcluded[path.From.ChainID]; ok {
				routeOk = false
				break
			}
			if _, ok := fromIncluded[path.From.ChainID]; ok {
				fromIncluded[path.From.ChainID] = true
			}
		}
		for _, value := range fromIncluded {
			if !value {
				routeOk = false
				break
			}
		}

		if routeOk {
			filteredRoutesLevel1 = append(filteredRoutesLevel1, route)
		}
	}

	filteredRoutesLevel2 := make([][]*Path, 0)
	for _, route := range filteredRoutesLevel1 {
		routeOk := true
		for _, path := range route {
			if amount, ok := fromLockedAmount[path.From.ChainID]; ok {
				requiredAmountIn := new(big.Int).Sub(amountIn, amount.ToInt())
				restAmountIn := big.NewInt(0)

				for _, otherPath := range route {
					if path.Equal(otherPath) {
						continue
					}
					restAmountIn = new(big.Int).Add(otherPath.MaxAmountIn.ToInt(), restAmountIn)
				}
				if restAmountIn.Cmp(requiredAmountIn) >= 0 {
					path.AmountIn = amount
					path.AmountInLocked = true
				} else {
					routeOk = false
					break
				}
			}
		}
		if routeOk {
			filteredRoutesLevel2 = append(filteredRoutesLevel2, route)
		}
	}

	return filteredRoutesLevel2
}

func findBest(routes [][]*Path) []*Path {
	var best []*Path
	bestCost := big.NewFloat(math.Inf(1))
	for _, route := range routes {
		currentCost := big.NewFloat(0)
		for _, path := range route {
			currentCost = new(big.Float).Add(currentCost, path.Cost)
		}

		if currentCost.Cmp(bestCost) == -1 {
			best = route
			bestCost = currentCost
		}
	}

	return best
}

type SuggestedRoutes struct {
	Best                  []*Path
	Candidates            []*Path
	TokenPrice            float64
	NativeChainTokenPrice float64
}

func newSuggestedRoutes(
	amountIn *big.Int,
	candidates []*Path,
	fromLockedAmount map[uint64]*hexutil.Big,
) *SuggestedRoutes {
	if len(candidates) == 0 {
		return &SuggestedRoutes{
			Candidates: candidates,
			Best:       candidates,
		}
	}

	node := &Node{
		Path:     nil,
		Children: buildGraph(amountIn, candidates, 0, []uint64{}),
	}
	routes := node.buildAllRoutes()
	routes = filterRoutes(routes, amountIn, fromLockedAmount)
	best := findBest(routes)

	if len(best) > 0 {
		sort.Slice(best, func(i, j int) bool {
			return best[i].AmountInLocked
		})
		rest := new(big.Int).Set(amountIn)
		for _, path := range best {
			diff := new(big.Int).Sub(rest, path.MaxAmountIn.ToInt())
			if diff.Cmp(zero) >= 0 {
				path.AmountIn = (*hexutil.Big)(path.MaxAmountIn.ToInt())
			} else {
				path.AmountIn = (*hexutil.Big)(new(big.Int).Set(rest))
			}
			rest.Sub(rest, path.AmountIn.ToInt())
		}
	}

	return &SuggestedRoutes{
		Candidates: candidates,
		Best:       best,
	}
}

func NewRouter(rpcClient *rpc.Client, transactor *transactions.Transactor, tokenManager *token.Manager, marketManager *market.Manager,
	collectibles *collectibles.Service, collectiblesManager *collectibles.Manager, ensService *ens.Service, stickersService *stickers.Service) *Router {
	bridges := make(map[string]bridge.Bridge)
	transfer := bridge.NewTransferBridge(rpcClient, transactor)
	erc721Transfer := bridge.NewERC721TransferBridge(rpcClient, transactor)
	erc1155Transfer := bridge.NewERC1155TransferBridge(rpcClient, transactor)
	cbridge := bridge.NewCbridge(rpcClient, transactor, tokenManager)
	hop := bridge.NewHopBridge(rpcClient, transactor, tokenManager)
	paraswap := bridge.NewSwapParaswap(rpcClient, transactor, tokenManager)
	bridges[transfer.Name()] = transfer
	bridges[erc721Transfer.Name()] = erc721Transfer
	bridges[hop.Name()] = hop
	bridges[cbridge.Name()] = cbridge
	bridges[erc1155Transfer.Name()] = erc1155Transfer
	bridges[paraswap.Name()] = paraswap

	return &Router{
		rpcClient:           rpcClient,
		tokenManager:        tokenManager,
		marketManager:       marketManager,
		collectiblesService: collectibles,
		collectiblesManager: collectiblesManager,
		ensService:          ensService,
		stickersService:     stickersService,
		feesManager:         &FeeManager{rpcClient},
		bridges:             bridges,
	}
}

func (r *Router) GetFeesManager() *FeeManager {
	return r.feesManager
}

func (r *Router) GetBridges() map[string]bridge.Bridge {
	return r.bridges
}

func containsNetworkChainID(network *params.Network, chainIDs []uint64) bool {
	for _, chainID := range chainIDs {
		if chainID == network.ChainID {
			return true
		}
	}

	return false
}

type Router struct {
	rpcClient           *rpc.Client
	tokenManager        *token.Manager
	marketManager       *market.Manager
	collectiblesService *collectibles.Service
	collectiblesManager *collectibles.Manager
	ensService          *ens.Service
	stickersService     *stickers.Service
	feesManager         *FeeManager
	bridges             map[string]bridge.Bridge
}

func (r *Router) requireApproval(ctx context.Context, sendType SendType, approvalContractAddress *common.Address, account common.Address, network *params.Network, token *token.Token, amountIn *big.Int) (
	bool, *big.Int, uint64, uint64, error) {
	if sendType.IsCollectiblesTransfer() {
		return false, nil, 0, 0, nil
	}

	if token.IsNative() {
		return false, nil, 0, 0, nil
	}
	contractMaker, err := contracts.NewContractMaker(r.rpcClient)
	if err != nil {
		return false, nil, 0, 0, err
	}

	contract, err := contractMaker.NewERC20(network.ChainID, token.Address)
	if err != nil {
		return false, nil, 0, 0, err
	}

	if approvalContractAddress == nil || *approvalContractAddress == bridge.ZeroAddress {
		return false, nil, 0, 0, nil
	}

	allowance, err := contract.Allowance(&bind.CallOpts{
		Context: ctx,
	}, account, *approvalContractAddress)

	if err != nil {
		return false, nil, 0, 0, err
	}

	if allowance.Cmp(amountIn) >= 0 {
		return false, nil, 0, 0, nil
	}

	ethClient, err := r.rpcClient.EthClient(network.ChainID)
	if err != nil {
		return false, nil, 0, 0, err
	}

	erc20ABI, err := abi.JSON(strings.NewReader(ierc20.IERC20ABI))
	if err != nil {
		return false, nil, 0, 0, err
	}

	data, err := erc20ABI.Pack("approve", approvalContractAddress, amountIn)
	if err != nil {
		return false, nil, 0, 0, err
	}

	estimate, err := ethClient.EstimateGas(context.Background(), ethereum.CallMsg{
		From:  account,
		To:    &token.Address,
		Value: big.NewInt(0),
		Data:  data,
	})
	if err != nil {
		return false, nil, 0, 0, err
	}

	// fetching l1 fee
	var l1Fee uint64
	oracleContractAddress, err := gaspriceoracle.ContractAddress(network.ChainID)
	if err == nil {
		oracleContract, err := gaspriceoracle.NewGaspriceoracleCaller(oracleContractAddress, ethClient)
		if err != nil {
			return false, nil, 0, 0, err
		}

		callOpt := &bind.CallOpts{}

		l1FeeResult, _ := oracleContract.GetL1Fee(callOpt, data)
		l1Fee = l1FeeResult.Uint64()
	}

	return true, amountIn, estimate, l1Fee, nil
}

func (r *Router) getBalance(ctx context.Context, network *params.Network, token *token.Token, account common.Address) (*big.Int, error) {
	client, err := r.rpcClient.EthClient(network.ChainID)
	if err != nil {
		return nil, err
	}

	return r.tokenManager.GetBalance(ctx, client, account, token.Address)
}

func (r *Router) getERC1155Balance(ctx context.Context, network *params.Network, token *token.Token, account common.Address) (*big.Int, error) {
	tokenID, success := new(big.Int).SetString(token.Symbol, 10)
	if !success {
		return nil, errors.New("failed to convert token symbol to big.Int")
	}

	balances, err := r.collectiblesManager.FetchERC1155Balances(
		ctx,
		account,
		walletCommon.ChainID(network.ChainID),
		token.Address,
		[]*bigint.BigInt{&bigint.BigInt{Int: tokenID}},
	)
	if err != nil {
		return nil, err
	}

	if len(balances) != 1 || balances[0] == nil {
		return nil, errors.New("invalid ERC1155 balance fetch response")
	}

	return balances[0].Int, nil
}

func (r *Router) SuggestedRoutes(
	ctx context.Context,
	sendType SendType,
	addrFrom common.Address,
	addrTo common.Address,
	amountIn *big.Int,
	tokenID string,
	toTokenID string,
	disabledFromChainIDs,
	disabledToChaindIDs,
	preferedChainIDs []uint64,
	gasFeeMode GasFeeMode,
	fromLockedAmount map[uint64]*hexutil.Big,
	testnetMode bool,
) (*SuggestedRoutes, error) {

	networks, err := r.rpcClient.NetworkManager.Get(false)
	if err != nil {
		return nil, err
	}

	prices, err := sendType.FetchPrices(r.marketManager, tokenID)
	if err != nil {
		return nil, err
	}
	var (
		group      = async.NewAtomicGroup(ctx)
		mu         sync.Mutex
		candidates = make([]*Path, 0)
	)

	for networkIdx := range networks {
		network := networks[networkIdx]
		if network.IsTest != testnetMode {
			continue
		}

		if containsNetworkChainID(network, disabledFromChainIDs) {
			continue
		}

		if !sendType.isAvailableFor(network) {
			continue
		}

		token := sendType.FindToken(r.tokenManager, r.collectiblesService, addrFrom, network, tokenID)
		if token == nil {
			continue
		}

		var toToken *walletToken.Token
		if sendType == Swap {
			toToken = sendType.FindToken(r.tokenManager, r.collectiblesService, common.Address{}, network, toTokenID)
		}

		nativeToken := r.tokenManager.FindToken(network, network.NativeCurrencySymbol)
		if nativeToken == nil {
			continue
		}

		group.Add(func(c context.Context) error {
			gasFees, err := r.feesManager.SuggestedFees(ctx, network.ChainID)
			if err != nil {
				return err
			}

			// Default value is 1 as in case of erc721 as we built the token we are sure the account owns it
			balance := big.NewInt(1)
			if sendType == ERC1155Transfer {
				balance, err = r.getERC1155Balance(ctx, network, token, addrFrom)
				if err != nil {
					return err
				}
			} else if sendType != ERC721Transfer {
				balance, err = r.getBalance(ctx, network, token, addrFrom)
				if err != nil {
					return err
				}
			}

			maxAmountIn := (*hexutil.Big)(balance)
			if amount, ok := fromLockedAmount[network.ChainID]; ok {
				if amount.ToInt().Cmp(balance) == 1 {
					return errors.New("locked amount cannot be bigger than balance")
				}
				maxAmountIn = amount
			}

			nativeBalance, err := r.getBalance(ctx, network, nativeToken, addrFrom)
			if err != nil {
				return err
			}
			maxFees := gasFees.feeFor(gasFeeMode)

			estimatedTime := r.feesManager.TransactionEstimatedTime(ctx, network.ChainID, maxFees)
			for _, bridge := range r.bridges {
				if !sendType.canUseBridge(bridge) {
					continue
				}

				for _, dest := range networks {
					if dest.IsTest != testnetMode {
						continue
					}

					if !sendType.isAvailableFor(network) {
						continue
					}

					if !sendType.isAvailableBetween(network, dest) {
						continue
					}

					if len(preferedChainIDs) > 0 && !containsNetworkChainID(dest, preferedChainIDs) {
						continue
					}
					if containsNetworkChainID(dest, disabledToChaindIDs) {
						continue
					}

					can, err := bridge.AvailableFor(network, dest, token, toToken)
					if err != nil || !can {
						continue
					}
					if maxAmountIn.ToInt().Cmp(big.NewInt(0)) == 0 {
						continue
					}

					bonderFees, tokenFees, err := bridge.CalculateFees(network, dest, token, amountIn)
					if err != nil {
						continue
					}
					if bonderFees.Cmp(zero) != 0 {
						if maxAmountIn.ToInt().Cmp(amountIn) >= 0 {
							if bonderFees.Cmp(amountIn) >= 0 {
								continue
							}
						} else {
							if bonderFees.Cmp(maxAmountIn.ToInt()) >= 0 {
								continue
							}
						}
					}
					gasLimit := uint64(0)
					if sendType.isTransfer() {
						gasLimit, err = bridge.EstimateGas(network, dest, addrFrom, addrTo, token, toToken, amountIn)
						if err != nil {
							continue
						}
					} else {
						gasLimit = sendType.EstimateGas(r.ensService, r.stickersService, network, addrFrom, tokenID)
					}

					approvalContractAddress := bridge.GetContractAddress(network, token)
					approvalRequired, approvalAmountRequired, approvalGasLimit, l1ApprovalFee, err := r.requireApproval(ctx, sendType, approvalContractAddress, addrFrom, network, token, amountIn)
					if err != nil {
						continue
					}

					var l1GasFeeWei uint64
					if sendType.needL1Fee() {
						tx, err := bridge.BuildTx(network, dest, addrFrom, addrTo, token, amountIn, bonderFees)
						if err != nil {
							continue
						}

						l1GasFeeWei, _ = r.feesManager.GetL1Fee(ctx, network.ChainID, tx)
						l1GasFeeWei += l1ApprovalFee
					}

					gasFees.L1GasFee = weiToGwei(big.NewInt(int64(l1GasFeeWei)))

					requiredNativeBalance := new(big.Int).Mul(gweiToWei(maxFees), big.NewInt(int64(gasLimit)))
					requiredNativeBalance.Add(requiredNativeBalance, new(big.Int).Mul(gweiToWei(maxFees), big.NewInt(int64(approvalGasLimit))))
					requiredNativeBalance.Add(requiredNativeBalance, big.NewInt(int64(l1GasFeeWei))) // add l1Fee to requiredNativeBalance, in case of L1 chain l1Fee is 0

					if nativeBalance.Cmp(requiredNativeBalance) <= 0 {
						continue
					}

					// Removed the required fees from maxAMount in case of native token tx
					if token.IsNative() {
						maxAmountIn = (*hexutil.Big)(new(big.Int).Sub(maxAmountIn.ToInt(), requiredNativeBalance))
					}

					ethPrice := big.NewFloat(prices["ETH"])

					approvalGasFees := new(big.Float).Mul(gweiToEth(maxFees), big.NewFloat((float64(approvalGasLimit))))

					approvalGasCost := new(big.Float)
					approvalGasCost.Mul(approvalGasFees, ethPrice)

					l1GasCost := new(big.Float)
					l1GasCost.Mul(gasFees.L1GasFee, ethPrice)

					gasCost := new(big.Float)
					gasCost.Mul(new(big.Float).Mul(gweiToEth(maxFees), big.NewFloat(float64(gasLimit))), ethPrice)

					tokenFeesAsFloat := new(big.Float).Quo(
						new(big.Float).SetInt(tokenFees),
						big.NewFloat(math.Pow(10, float64(token.Decimals))),
					)
					tokenCost := new(big.Float)
					tokenCost.Mul(tokenFeesAsFloat, big.NewFloat(prices[tokenID]))

					cost := new(big.Float)
					cost.Add(tokenCost, gasCost)
					cost.Add(cost, approvalGasCost)
					cost.Add(cost, l1GasCost)
					mu.Lock()
					candidates = append(candidates, &Path{
						BridgeName:              bridge.Name(),
						From:                    network,
						To:                      dest,
						MaxAmountIn:             maxAmountIn,
						AmountIn:                (*hexutil.Big)(zero),
						AmountOut:               (*hexutil.Big)(zero),
						GasAmount:               gasLimit,
						GasFees:                 gasFees,
						BonderFees:              (*hexutil.Big)(bonderFees),
						TokenFees:               tokenFeesAsFloat,
						Cost:                    cost,
						EstimatedTime:           estimatedTime,
						ApprovalRequired:        approvalRequired,
						ApprovalGasFees:         approvalGasFees,
						ApprovalAmountRequired:  (*hexutil.Big)(approvalAmountRequired),
						ApprovalContractAddress: approvalContractAddress,
					})
					mu.Unlock()
				}
			}
			return nil
		})
	}

	group.Wait()

	suggestedRoutes := newSuggestedRoutes(amountIn, candidates, fromLockedAmount)
	suggestedRoutes.TokenPrice = prices[tokenID]
	suggestedRoutes.NativeChainTokenPrice = prices["ETH"]
	for _, path := range suggestedRoutes.Best {
		amountOut, err := r.bridges[path.BridgeName].CalculateAmountOut(path.From, path.To, (*big.Int)(path.AmountIn), tokenID)
		if err != nil {
			continue
		}
		path.AmountOut = (*hexutil.Big)(amountOut)
	}

	return suggestedRoutes, nil
}