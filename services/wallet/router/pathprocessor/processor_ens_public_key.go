package pathprocessor

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/status-im/status-go/account"
	"github.com/status-im/status-go/contracts"
	"github.com/status-im/status-go/contracts/resolver"
	"github.com/status-im/status-go/eth-node/types"
	"github.com/status-im/status-go/rpc"
	"github.com/status-im/status-go/services/ens"
	walletCommon "github.com/status-im/status-go/services/wallet/common"
	"github.com/status-im/status-go/transactions"
)

type ENSPublicKeyProcessor struct {
	contractMaker *contracts.ContractMaker
	transactor    transactions.TransactorIface
	ensService    *ens.Service
}

func NewENSPublicKeyProcessor(rpcClient *rpc.Client, transactor transactions.TransactorIface, ensService *ens.Service) *ENSPublicKeyProcessor {
	return &ENSPublicKeyProcessor{
		contractMaker: &contracts.ContractMaker{
			RPCClient: rpcClient,
		},
		transactor: transactor,
		ensService: ensService,
	}
}

func (s *ENSPublicKeyProcessor) Name() string {
	return ProcessorENSPublicKeyName
}

func (s *ENSPublicKeyProcessor) AvailableFor(params ProcessorInputParams) (bool, error) {
	return params.FromChain.ChainID == walletCommon.EthereumMainnet || params.FromChain.ChainID == walletCommon.EthereumSepolia, nil
}

func (s *ENSPublicKeyProcessor) CalculateFees(params ProcessorInputParams) (*big.Int, *big.Int, error) {
	return ZeroBigIntValue, ZeroBigIntValue, nil
}

func (s *ENSPublicKeyProcessor) PackTxInputData(params ProcessorInputParams) ([]byte, error) {
	resolverABI, err := abi.JSON(strings.NewReader(resolver.PublicResolverABI))
	if err != nil {
		return []byte{}, err
	}

	x, y := ens.ExtractCoordinates(params.PublicKey)
	return resolverABI.Pack("setPubkey", ens.NameHash(params.Username), x, y)
}

func (s *ENSPublicKeyProcessor) EstimateGas(params ProcessorInputParams) (uint64, error) {
	contractAddress, err := s.GetContractAddress(params)
	if err != nil {
		return 0, err
	}

	input, err := s.PackTxInputData(params)
	if err != nil {
		return 0, err
	}

	ethClient, err := s.contractMaker.RPCClient.EthClient(params.FromChain.ChainID)
	if err != nil {
		return 0, err
	}

	msg := ethereum.CallMsg{
		From:  params.FromAddr,
		To:    &contractAddress,
		Value: ZeroBigIntValue,
		Data:  input,
	}

	estimation, err := ethClient.EstimateGas(context.Background(), msg)
	if err != nil {
		return 0, err
	}

	increasedEstimation := float64(estimation) * IncreaseEstimatedGasFactor

	return uint64(increasedEstimation), nil
}

func (s *ENSPublicKeyProcessor) BuildTx(params ProcessorInputParams) (*ethTypes.Transaction, error) {
	toAddr := types.Address(params.ToAddr)
	inputData, err := s.PackTxInputData(params)
	if err != nil {
		return nil, err
	}

	sendArgs := &MultipathProcessorTxArgs{
		TransferTx: &transactions.SendTxArgs{
			From:  types.Address(params.FromAddr),
			To:    &toAddr,
			Value: (*hexutil.Big)(ZeroBigIntValue),
			Data:  inputData,
		},
		ChainID: params.FromChain.ChainID,
	}

	return s.BuildTransaction(sendArgs)
}

func (s *ENSPublicKeyProcessor) Send(sendArgs *MultipathProcessorTxArgs, verifiedAccount *account.SelectedExtKey) (hash types.Hash, err error) {
	return s.transactor.SendTransactionWithChainID(sendArgs.ChainID, *sendArgs.TransferTx, verifiedAccount)
}

func (s *ENSPublicKeyProcessor) BuildTransaction(sendArgs *MultipathProcessorTxArgs) (*ethTypes.Transaction, error) {
	return s.transactor.ValidateAndBuildTransaction(sendArgs.ChainID, *sendArgs.TransferTx)
}

func (s *ENSPublicKeyProcessor) CalculateAmountOut(params ProcessorInputParams) (*big.Int, error) {
	return params.AmountIn, nil
}

func (s *ENSPublicKeyProcessor) GetContractAddress(params ProcessorInputParams) (common.Address, error) {
	addr, err := s.ensService.API().Resolver(context.Background(), params.FromChain.ChainID, params.Username)
	if err != nil {
		return common.Address{}, err
	}
	if *addr == ZeroAddress {
		return common.Address{}, errors.New("ENS resolver not found")
	}
	return *addr, nil
}