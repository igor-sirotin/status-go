package statusgo

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"unsafe"

	validator "gopkg.in/go-playground/validator.v9"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"

	"github.com/status-im/zxcvbn-go"
	"github.com/status-im/zxcvbn-go/scoring"

	abi_spec "github.com/status-im/status-go/abi-spec"
	"github.com/status-im/status-go/account"
	"github.com/status-im/status-go/api"
	"github.com/status-im/status-go/api/multiformat"
	"github.com/status-im/status-go/centralizedmetrics"
	"github.com/status-im/status-go/centralizedmetrics/providers"
	"github.com/status-im/status-go/eth-node/crypto"
	"github.com/status-im/status-go/eth-node/types"
	"github.com/status-im/status-go/exportlogs"
	"github.com/status-im/status-go/extkeys"
	"github.com/status-im/status-go/images"
	"github.com/status-im/status-go/logutils"
	"github.com/status-im/status-go/logutils/requestlog"
	"github.com/status-im/status-go/multiaccounts"
	"github.com/status-im/status-go/multiaccounts/accounts"
	"github.com/status-im/status-go/multiaccounts/settings"
	"github.com/status-im/status-go/params"
	"github.com/status-im/status-go/profiling"
	"github.com/status-im/status-go/protocol"
	"github.com/status-im/status-go/protocol/common"
	identityUtils "github.com/status-im/status-go/protocol/identity"
	"github.com/status-im/status-go/protocol/identity/alias"
	"github.com/status-im/status-go/protocol/identity/colorhash"
	"github.com/status-im/status-go/protocol/identity/emojihash"
	"github.com/status-im/status-go/protocol/requests"
	"github.com/status-im/status-go/server"
	"github.com/status-im/status-go/server/pairing"
	"github.com/status-im/status-go/server/pairing/preflight"
	"github.com/status-im/status-go/services/personal"
	"github.com/status-im/status-go/services/typeddata"
	"github.com/status-im/status-go/signal"
	"github.com/status-im/status-go/transactions"
)

type InitializeApplicationResponse struct {
	Accounts               []multiaccounts.Account         `json:"accounts"`
	CentralizedMetricsInfo *centralizedmetrics.MetricsInfo `json:"centralizedMetricsInfo"`
}

func InitializeApplication(requestJSON string) string {
	return logAndCallString(initializeApplication, requestJSON)
}

func initializeApplication(requestJSON string) string {
	var request requests.InitializeApplication
	err := json.Unmarshal([]byte(requestJSON), &request)
	if err != nil {
		return makeJSONResponse(err)
	}
	err = request.Validate()
	if err != nil {
		return makeJSONResponse(err)
	}

	// initialize metrics
	providers.MixpanelAppID = request.MixpanelAppID
	providers.MixpanelToken = request.MixpanelToken

	datadir := request.DataDir

	statusBackend.UpdateRootDataDir(datadir)
	err = statusBackend.OpenAccounts()
	if err != nil {
		return makeJSONResponse(err)
	}
	accs, err := statusBackend.GetAccounts()
	if err != nil {
		return makeJSONResponse(err)
	}
	centralizedMetricsInfo, err := statusBackend.CentralizedMetricsInfo()
	if err != nil {
		return makeJSONResponse(err)
	}
	response := &InitializeApplicationResponse{
		Accounts:               accs,
		CentralizedMetricsInfo: centralizedMetricsInfo,
	}
	data, err := json.Marshal(response)
	if err != nil {
		return makeJSONResponse(err)
	}
	return string(data)
}

func OpenAccounts(datadir string) string {
	return logAndCallString(openAccounts, datadir)
}

// DEPRECATED: use InitializeApplication
// openAccounts opens database and returns accounts list.
func openAccounts(datadir string) string {
	statusBackend.UpdateRootDataDir(datadir)
	err := statusBackend.OpenAccounts()
	if err != nil {
		return makeJSONResponse(err)
	}
	accs, err := statusBackend.GetAccounts()
	if err != nil {
		return makeJSONResponse(err)
	}
	data, err := json.Marshal(accs)
	if err != nil {
		return makeJSONResponse(err)
	}
	return string(data)
}

func ExtractGroupMembershipSignatures(signaturePairsStr string) string {
	return logAndCallString(extractGroupMembershipSignatures, signaturePairsStr)
}

// ExtractGroupMembershipSignatures extract public keys from tuples of content/signature.
func extractGroupMembershipSignatures(signaturePairsStr string) string {
	var signaturePairs [][2]string

	if err := json.Unmarshal([]byte(signaturePairsStr), &signaturePairs); err != nil {
		return makeJSONResponse(err)
	}

	identities, err := statusBackend.ExtractGroupMembershipSignatures(signaturePairs)
	if err != nil {
		return makeJSONResponse(err)
	}

	data, err := json.Marshal(struct {
		Identities []string `json:"identities"`
	}{Identities: identities})
	if err != nil {
		return makeJSONResponse(err)
	}

	return string(data)
}

func SignGroupMembership(content string) string {
	return logAndCallString(signGroupMembership, content)
}

// signGroupMembership signs a string containing group membership information.
func signGroupMembership(content string) string {
	signature, err := statusBackend.SignGroupMembership(content)
	if err != nil {
		return makeJSONResponse(err)
	}

	data, err := json.Marshal(struct {
		Signature string `json:"signature"`
	}{Signature: signature})
	if err != nil {
		return makeJSONResponse(err)
	}

	return string(data)
}

func GetNodeConfig() string {
	return logAndCallString(getNodeConfig)
}

// getNodeConfig returns the current config of the Status node
func getNodeConfig() string {
	conf, err := statusBackend.GetNodeConfig()
	if err != nil {
		return makeJSONResponse(err)
	}

	respJSON, err := json.Marshal(conf)
	if err != nil {
		return makeJSONResponse(err)
	}

	return string(respJSON)
}

func ValidateNodeConfig(configJSON string) string {
	return logAndCallString(validateNodeConfig, configJSON)
}

// validateNodeConfig validates config for the Status node.
func validateNodeConfig(configJSON string) string {
	var resp APIDetailedResponse

	_, err := params.NewConfigFromJSON(configJSON)

	// Convert errors to APIDetailedResponse
	switch err := err.(type) {
	case validator.ValidationErrors:
		resp = APIDetailedResponse{
			Message:     "validation: validation failed",
			FieldErrors: make([]APIFieldError, len(err)),
		}

		for i, ve := range err {
			resp.FieldErrors[i] = APIFieldError{
				Parameter: ve.Namespace(),
				Errors: []APIError{
					{
						Message: fmt.Sprintf("field validation failed on the '%s' tag", ve.Tag()),
					},
				},
			}
		}
	case error:
		resp = APIDetailedResponse{
			Message: fmt.Sprintf("validation: %s", err.Error()),
		}
	case nil:
		resp = APIDetailedResponse{
			Status: true,
		}
	}

	respJSON, err := json.Marshal(resp)
	if err != nil {
		return makeJSONResponse(err)
	}

	return string(respJSON)
}

func ResetChainData() string {
	return logAndCallString(resetChainData)
}

// resetChainData removes chain data from data directory.
func resetChainData() string {
	api.RunAsync(statusBackend.ResetChainData)
	return makeJSONResponse(nil)
}

func CallRPC(inputJSON string) string {
	return logAndCallString(callRPC, inputJSON)
}

// callRPC calls public APIs via RPC.
func callRPC(inputJSON string) string {
	resp, err := statusBackend.CallRPC(inputJSON)
	if err != nil {
		return makeJSONResponse(err)
	}
	return resp
}

func CallPrivateRPC(inputJSON string) string {
	return logAndCallString(callPrivateRPC, inputJSON)
}

// callPrivateRPC calls both public and private APIs via RPC.
func callPrivateRPC(inputJSON string) string {
	resp, err := statusBackend.CallPrivateRPC(inputJSON)
	if err != nil {
		return makeJSONResponse(err)
	}
	return resp
}

func VerifyAccountPassword(keyStoreDir, address, password string) string {
	return logAndCallString(verifyAccountPassword, keyStoreDir, address, password)
}

// verifyAccountPassword verifies account password.
func verifyAccountPassword(keyStoreDir, address, password string) string {
	_, err := statusBackend.AccountManager().VerifyAccountPassword(keyStoreDir, address, password)
	return makeJSONResponse(err)
}

func VerifyDatabasePassword(keyUID, password string) string {
	return logAndCallString(verifyDatabasePassword, keyUID, password)
}

// verifyDatabasePassword verifies database password.
func verifyDatabasePassword(keyUID, password string) string {
	err := statusBackend.VerifyDatabasePassword(keyUID, password)
	return makeJSONResponse(err)
}

func MigrateKeyStoreDir(accountData, password, oldDir, newDir string) string {
	return logAndCallString(migrateKeyStoreDir, accountData, password, oldDir, newDir)
}

// migrateKeyStoreDir migrates key files to a new directory
func migrateKeyStoreDir(accountData, password, oldDir, newDir string) string {
	var account multiaccounts.Account
	err := json.Unmarshal([]byte(accountData), &account)
	if err != nil {
		return makeJSONResponse(err)
	}

	err = statusBackend.MigrateKeyStoreDir(account, password, oldDir, newDir)
	return makeJSONResponse(err)
}

// login deprecated as Login and LoginWithConfig are deprecated
func login(accountData, password, configJSON string) error {
	var account multiaccounts.Account
	err := json.Unmarshal([]byte(accountData), &account)
	if err != nil {
		return err
	}

	var conf params.NodeConfig
	if configJSON != "" {
		err = json.Unmarshal([]byte(configJSON), &conf)
		if err != nil {
			return err
		}
	}

	api.RunAsync(func() error {
		log.Debug("start a node with account", "key-uid", account.KeyUID)
		err := statusBackend.UpdateNodeConfigFleet(account, password, &conf)
		if err != nil {
			log.Error("failed to update node config fleet", "key-uid", account.KeyUID, "error", err)
			return statusBackend.LoggedIn(account.KeyUID, err)
		}

		err = statusBackend.StartNodeWithAccount(account, password, &conf, nil)
		if err != nil {
			log.Error("failed to start a node", "key-uid", account.KeyUID, "error", err)
			return err
		}
		log.Debug("started a node with", "key-uid", account.KeyUID)
		return nil
	})

	return nil
}

// Login loads a key file (for a given address), tries to decrypt it using the password,
// to verify ownership if verified, purges all the previous identities from Whisper,
// and injects verified key as shh identity.
//
// Deprecated: Use LoginAccount instead.
func Login(accountData, password string) string {
	err := login(accountData, password, "")
	if err != nil {
		return makeJSONResponse(err)
	}
	return makeJSONResponse(nil)
}

// LoginWithConfig loads a key file (for a given address), tries to decrypt it using the password,
// to verify ownership if verified, purges all the previous identities from Whisper,
// and injects verified key as shh identity. It then updates the accounts node db configuration
// mergin the values received in the configJSON parameter
//
// Deprecated: Use LoginAccount instead.
func LoginWithConfig(accountData, password, configJSON string) string {
	err := login(accountData, password, configJSON)
	if err != nil {
		return makeJSONResponse(err)
	}
	return makeJSONResponse(nil)
}

func CreateAccountAndLogin(requestJSON string) string {
	return logAndCallString(createAccountAndLogin, requestJSON)
}

func createAccountAndLogin(requestJSON string) string {
	var request requests.CreateAccount
	err := json.Unmarshal([]byte(requestJSON), &request)
	if err != nil {
		return makeJSONResponse(err)
	}

	err = request.Validate(&requests.CreateAccountValidation{
		AllowEmptyDisplayName: false,
	})
	if err != nil {
		return makeJSONResponse(err)
	}

	api.RunAsync(func() error {
		log.Debug("starting a node and creating config")
		_, err := statusBackend.CreateAccountAndLogin(&request)
		if err != nil {
			log.Error("failed to create account", "error", err)
			return err
		}
		log.Debug("started a node, and created account")
		return nil
	})
	return makeJSONResponse(nil)
}

func LoginAccount(requestJSON string) string {
	return logAndCallString(loginAccount, requestJSON)
}

func loginAccount(requestJSON string) string {
	var request requests.Login
	err := json.Unmarshal([]byte(requestJSON), &request)
	if err != nil {
		return makeJSONResponse(err)
	}

	err = request.Validate()
	if err != nil {
		return makeJSONResponse(err)
	}

	api.RunAsync(func() error {
		err := statusBackend.LoginAccount(&request)
		if err != nil {
			log.Error("loginAccount failed", "error", err)
			return err
		}
		log.Debug("loginAccount started node")
		return nil
	})
	return makeJSONResponse(nil)
}

func RestoreAccountAndLogin(requestJSON string) string {
	return logAndCallString(restoreAccountAndLogin, requestJSON)
}

func restoreAccountAndLogin(requestJSON string) string {
	var request requests.RestoreAccount
	err := json.Unmarshal([]byte(requestJSON), &request)
	if err != nil {
		return makeJSONResponse(err)
	}

	err = request.Validate()
	if err != nil {
		return makeJSONResponse(err)
	}

	api.RunAsync(func() error {
		log.Debug("starting a node and restoring account")

		if request.Keycard != nil {
			_, err = statusBackend.RestoreKeycardAccountAndLogin(&request)
		} else {
			_, err = statusBackend.RestoreAccountAndLogin(&request)
		}

		if err != nil {
			log.Error("failed to restore account", "error", err)
			return err
		}
		log.Debug("started a node, and restored account")
		return nil
	})

	return makeJSONResponse(nil)
}

// SaveAccountAndLogin saves account in status-go database.
// Deprecated: Use CreateAccountAndLogin instead.
func SaveAccountAndLogin(accountData, password, settingsJSON, configJSON, subaccountData string) string {
	var account multiaccounts.Account
	err := json.Unmarshal([]byte(accountData), &account)
	if err != nil {
		return makeJSONResponse(err)
	}
	var settings settings.Settings
	err = json.Unmarshal([]byte(settingsJSON), &settings)
	if err != nil {
		return makeJSONResponse(err)
	}

	if *settings.Mnemonic != "" {
		settings.MnemonicWasNotShown = true
	}

	var conf params.NodeConfig
	err = json.Unmarshal([]byte(configJSON), &conf)
	if err != nil {
		return makeJSONResponse(err)
	}
	var subaccs []*accounts.Account
	err = json.Unmarshal([]byte(subaccountData), &subaccs)
	if err != nil {
		return makeJSONResponse(err)
	}

	api.RunAsync(func() error {
		log.Debug("starting a node, and saving account with configuration", "key-uid", account.KeyUID)
		err := statusBackend.StartNodeWithAccountAndInitialConfig(account, password, settings, &conf, subaccs, nil)
		if err != nil {
			log.Error("failed to start node and save account", "key-uid", account.KeyUID, "error", err)
			return err
		}
		log.Debug("started a node, and saved account", "key-uid", account.KeyUID)
		return nil
	})
	return makeJSONResponse(nil)
}

func DeleteMultiaccount(keyUID, keyStoreDir string) string {
	return logAndCallString(deleteMultiaccount, keyUID, keyStoreDir)
}

// deleteMultiaccount
func deleteMultiaccount(keyUID, keyStoreDir string) string {
	err := statusBackend.DeleteMultiaccount(keyUID, keyStoreDir)
	return makeJSONResponse(err)
}

func DeleteImportedKey(address, password, keyStoreDir string) string {
	return logAndCallString(deleteImportedKey, address, password, keyStoreDir)
}

// deleteImportedKey
func deleteImportedKey(address, password, keyStoreDir string) string {
	err := statusBackend.DeleteImportedKey(address, password, keyStoreDir)
	return makeJSONResponse(err)
}

func InitKeystore(keydir string) string {
	return logAndCallString(initKeystore, keydir)
}

// initKeystore initialize keystore before doing any operations with keys.
func initKeystore(keydir string) string {
	err := statusBackend.AccountManager().InitKeystore(keydir)
	return makeJSONResponse(err)
}

// SaveAccountAndLoginWithKeycard saves account in status-go database.
// Deprecated: Use CreateAndAccountAndLogin with required keycard properties.
func SaveAccountAndLoginWithKeycard(accountData, password, settingsJSON, configJSON, subaccountData string, keyHex string) string {
	var account multiaccounts.Account
	err := json.Unmarshal([]byte(accountData), &account)
	if err != nil {
		return makeJSONResponse(err)
	}
	var settings settings.Settings
	err = json.Unmarshal([]byte(settingsJSON), &settings)
	if err != nil {
		return makeJSONResponse(err)
	}
	var conf params.NodeConfig
	err = json.Unmarshal([]byte(configJSON), &conf)
	if err != nil {
		return makeJSONResponse(err)
	}
	var subaccs []*accounts.Account
	err = json.Unmarshal([]byte(subaccountData), &subaccs)
	if err != nil {
		return makeJSONResponse(err)
	}

	api.RunAsync(func() error {
		log.Debug("starting a node, and saving account with configuration", "key-uid", account.KeyUID)
		err := statusBackend.SaveAccountAndStartNodeWithKey(account, password, settings, &conf, subaccs, keyHex)
		if err != nil {
			log.Error("failed to start node and save account", "key-uid", account.KeyUID, "error", err)
			return err
		}
		log.Debug("started a node, and saved account", "key-uid", account.KeyUID)
		return nil
	})
	return makeJSONResponse(nil)
}

// LoginWithKeycard initializes an account with a chat key and encryption key used for PFS.
// It purges all the previous identities from Whisper, and injects the key as shh identity.
// Deprecated: Use LoginAccount instead.
func LoginWithKeycard(accountData, password, keyHex string, configJSON string) string {
	var account multiaccounts.Account
	err := json.Unmarshal([]byte(accountData), &account)
	if err != nil {
		return makeJSONResponse(err)
	}
	var conf params.NodeConfig
	err = json.Unmarshal([]byte(configJSON), &conf)
	if err != nil {
		return makeJSONResponse(err)
	}
	api.RunAsync(func() error {
		log.Debug("start a node with account", "key-uid", account.KeyUID)
		err := statusBackend.StartNodeWithKey(account, password, keyHex, &conf)
		if err != nil {
			log.Error("failed to start a node", "key-uid", account.KeyUID, "error", err)
			return err
		}
		log.Debug("started a node with", "key-uid", account.KeyUID)
		return nil
	})
	return makeJSONResponse(nil)
}

func Logout() string {
	return logAndCallString(logout)
}

// logout is equivalent to clearing whisper identities.
func logout() string {
	return makeJSONResponse(statusBackend.Logout())
}

func SignMessage(rpcParams string) string {
	return logAndCallString(signMessage, rpcParams)
}

// signMessage unmarshals rpc params {data, address, password} and
// passes them onto backend.SignMessage.
func signMessage(rpcParams string) string {
	var params personal.SignParams
	err := json.Unmarshal([]byte(rpcParams), &params)
	if err != nil {
		return prepareJSONResponseWithCode(nil, err, codeFailedParseParams)
	}
	result, err := statusBackend.SignMessage(params)
	return prepareJSONResponse(result.String(), err)
}

// SignTypedData unmarshall data into TypedData, validate it and signs with selected account,
// if password matches selected account.
//
//export SignTypedData
func SignTypedData(data, address, password string) string {
	return logAndCallString(signTypedData, data, address, password)
}

func signTypedData(data, address, password string) string {
	var typed typeddata.TypedData
	err := json.Unmarshal([]byte(data), &typed)
	if err != nil {
		return prepareJSONResponseWithCode(nil, err, codeFailedParseParams)
	}
	if err := typed.Validate(); err != nil {
		return prepareJSONResponseWithCode(nil, err, codeFailedParseParams)
	}
	result, err := statusBackend.SignTypedData(typed, address, password)
	return prepareJSONResponse(result.String(), err)
}

// HashTypedData unmarshalls data into TypedData, validates it and hashes it.
//
//export HashTypedData
func HashTypedData(data string) string {
	return logAndCallString(hashTypedData, data)
}

func hashTypedData(data string) string {
	var typed typeddata.TypedData
	err := json.Unmarshal([]byte(data), &typed)
	if err != nil {
		return prepareJSONResponseWithCode(nil, err, codeFailedParseParams)
	}
	if err := typed.Validate(); err != nil {
		return prepareJSONResponseWithCode(nil, err, codeFailedParseParams)
	}
	result, err := statusBackend.HashTypedData(typed)
	return prepareJSONResponse(result.String(), err)
}

// SignTypedDataV4 unmarshall data into TypedData, validate it and signs with selected account,
// if password matches selected account.
//
//export SignTypedDataV4
func SignTypedDataV4(data, address, password string) string {
	return logAndCallString(signTypedDataV4, data, address, password)
}

func signTypedDataV4(data, address, password string) string {
	var typed apitypes.TypedData
	err := json.Unmarshal([]byte(data), &typed)
	if err != nil {
		return prepareJSONResponseWithCode(nil, err, codeFailedParseParams)
	}
	result, err := statusBackend.SignTypedDataV4(typed, address, password)
	return prepareJSONResponse(result.String(), err)
}

// HashTypedDataV4 unmarshalls data into TypedData, validates it and hashes it.
//
//export HashTypedDataV4
func HashTypedDataV4(data string) string {
	return logAndCallString(hashTypedDataV4, data)
}

func hashTypedDataV4(data string) string {
	var typed apitypes.TypedData
	err := json.Unmarshal([]byte(data), &typed)
	if err != nil {
		return prepareJSONResponseWithCode(nil, err, codeFailedParseParams)
	}
	result, err := statusBackend.HashTypedDataV4(typed)
	return prepareJSONResponse(result.String(), err)
}

func Recover(rpcParams string) string {
	return logAndCallString(recoverWithRPCParams, rpcParams)
}

// recoverWithRPCParams unmarshals rpc params {signDataString, signedData} and passes
// them onto backend.
func recoverWithRPCParams(rpcParams string) string {
	var params personal.RecoverParams
	err := json.Unmarshal([]byte(rpcParams), &params)
	if err != nil {
		return prepareJSONResponseWithCode(nil, err, codeFailedParseParams)
	}
	addr, err := statusBackend.Recover(params)
	return prepareJSONResponse(addr.String(), err)
}

func SendTransactionWithChainID(chainID int, txArgsJSON, password string) string {
	return logAndCallString(sendTransactionWithChainID, chainID, txArgsJSON, password)
}

// sendTransactionWithChainID converts RPC args and calls backend.SendTransactionWithChainID.
func sendTransactionWithChainID(chainID int, txArgsJSON, password string) string {
	var params transactions.SendTxArgs
	err := json.Unmarshal([]byte(txArgsJSON), &params)
	if err != nil {
		return prepareJSONResponseWithCode(nil, err, codeFailedParseParams)
	}
	hash, err := statusBackend.SendTransactionWithChainID(uint64(chainID), params, password)
	code := codeUnknown
	if c, ok := errToCodeMap[err]; ok {
		code = c
	}
	return prepareJSONResponseWithCode(hash.String(), err, code)
}

func SendTransaction(txArgsJSON, password string) string {
	return logAndCallString(sendTransaction, txArgsJSON, password)
}

// sendTransaction converts RPC args and calls backend.SendTransaction.
func sendTransaction(txArgsJSON, password string) string {
	var params transactions.SendTxArgs
	err := json.Unmarshal([]byte(txArgsJSON), &params)
	if err != nil {
		return prepareJSONResponseWithCode(nil, err, codeFailedParseParams)
	}
	hash, err := statusBackend.SendTransaction(params, password)
	code := codeUnknown
	if c, ok := errToCodeMap[err]; ok {
		code = c
	}
	return prepareJSONResponseWithCode(hash.String(), err, code)
}

func SendTransactionWithSignature(txArgsJSON, sigString string) string {
	return logAndCallString(sendTransactionWithSignature, txArgsJSON, sigString)
}

// sendTransactionWithSignature converts RPC args and calls backend.SendTransactionWithSignature
func sendTransactionWithSignature(txArgsJSON, sigString string) string {
	var params transactions.SendTxArgs
	err := json.Unmarshal([]byte(txArgsJSON), &params)
	if err != nil {
		return prepareJSONResponseWithCode(nil, err, codeFailedParseParams)
	}

	sig, err := hex.DecodeString(sigString)
	if err != nil {
		return prepareJSONResponseWithCode(nil, err, codeFailedParseParams)
	}

	hash, err := statusBackend.SendTransactionWithSignature(params, sig)
	code := codeUnknown
	if c, ok := errToCodeMap[err]; ok {
		code = c
	}
	return prepareJSONResponseWithCode(hash.String(), err, code)
}

func HashTransaction(txArgsJSON string) string {
	return logAndCallString(hashTransaction, txArgsJSON)
}

// hashTransaction validate the transaction and returns new txArgs and the transaction hash.
func hashTransaction(txArgsJSON string) string {
	var params transactions.SendTxArgs
	err := json.Unmarshal([]byte(txArgsJSON), &params)
	if err != nil {
		return prepareJSONResponseWithCode(nil, err, codeFailedParseParams)
	}

	newTxArgs, hash, err := statusBackend.HashTransaction(params)
	code := codeUnknown
	if c, ok := errToCodeMap[err]; ok {
		code = c
	}

	result := struct {
		Transaction transactions.SendTxArgs `json:"transaction"`
		Hash        types.Hash              `json:"hash"`
	}{
		Transaction: newTxArgs,
		Hash:        hash,
	}

	return prepareJSONResponseWithCode(result, err, code)
}

func HashMessage(message string) string {
	return logAndCallString(hashMessage, message)
}

// hashMessage calculates the hash of a message to be safely signed by the keycard
// The hash is calulcated as
//
//	keccak256("\x19Ethereum Signed Message:\n"${message length}${message}).
//
// This gives context to the signed message and prevents signing of transactions.
func hashMessage(message string) string {
	hash, err := api.HashMessage(message)
	code := codeUnknown
	if c, ok := errToCodeMap[err]; ok {
		code = c
	}
	return prepareJSONResponseWithCode(fmt.Sprintf("0x%x", hash), err, code)
}

func StartCPUProfile(dataDir string) string {
	return logAndCallString(startCPUProfile, dataDir)
}

// startCPUProfile runs pprof for CPU.
func startCPUProfile(dataDir string) string {
	err := profiling.StartCPUProfile(dataDir)
	return makeJSONResponse(err)
}

func StopCPUProfiling() string {
	return logAndCallString(stopCPUProfiling)
}

// stopCPUProfiling stops pprof for cpu.
func stopCPUProfiling() string { //nolint: deadcode
	err := profiling.StopCPUProfile()
	return makeJSONResponse(err)
}

func WriteHeapProfile(dataDir string) string {
	return logAndCallString(writeHeapProfile, dataDir)
}

// writeHeapProfile starts pprof for heap
func writeHeapProfile(dataDir string) string { //nolint: deadcode
	err := profiling.WriteHeapFile(dataDir)
	return makeJSONResponse(err)
}

func makeJSONResponse(err error) string {
	errString := ""
	if err != nil {
		log.Error("error in makeJSONResponse", "error", err)
		errString = err.Error()
	}

	out := APIResponse{
		Error: errString,
	}
	outBytes, _ := json.Marshal(out)

	return string(outBytes)
}

func AddPeer(enode string) string {
	return logAndCallString(addPeer, enode)
}

// addPeer adds an enode as a peer.
func addPeer(enode string) string {
	err := statusBackend.StatusNode().AddPeer(enode)
	return makeJSONResponse(err)
}

func ConnectionChange(typ string, expensive int) {
	logAndCall(connectionChange, typ, expensive)
}

// connectionChange handles network state changes as reported
// by ReactNative (see https://facebook.github.io/react-native/docs/netinfo.html)
func connectionChange(typ string, expensive int) {
	statusBackend.ConnectionChange(typ, expensive == 1)
}

func AppStateChange(state string) {
	logAndCall(appStateChange, state)
}

// appStateChange handles app state changes (background/foreground).
func appStateChange(state string) {
	statusBackend.AppStateChange(state)
}

func StartLocalNotifications() string {
	return logAndCallString(startLocalNotifications)
}

// startLocalNotifications
func startLocalNotifications() string {
	err := statusBackend.StartLocalNotifications()
	return makeJSONResponse(err)
}

func StopLocalNotifications() string {
	return logAndCallString(stopLocalNotifications)
}

// stopLocalNotifications
func stopLocalNotifications() string {
	err := statusBackend.StopLocalNotifications()
	return makeJSONResponse(err)
}

func SetMobileSignalHandler(handler SignalHandler) {
	logAndCall(setMobileSignalHandler, handler)
}

// setMobileSignalHandler setup geth callback to notify about new signal
// used for gomobile builds
func setMobileSignalHandler(handler SignalHandler) {
	signal.SetMobileSignalHandler(func(data []byte) {
		if len(data) > 0 {
			handler.HandleSignal(string(data))
		}
	})
}

func SetSignalEventCallback(cb unsafe.Pointer) {
	logAndCall(setSignalEventCallback, cb)
}

// setSignalEventCallback setup geth callback to notify about new signal
func setSignalEventCallback(cb unsafe.Pointer) {
	signal.SetSignalEventCallback(cb)
}

// ExportNodeLogs reads current node log and returns content to a caller.
//
//export ExportNodeLogs
func ExportNodeLogs() string {
	return logAndCallString(exportNodeLogs)
}

func exportNodeLogs() string {
	node := statusBackend.StatusNode()
	if node == nil {
		return makeJSONResponse(errors.New("node is not running"))
	}
	config := node.Config()
	if config == nil {
		return makeJSONResponse(errors.New("config and log file are not available"))
	}
	data, err := json.Marshal(exportlogs.ExportFromBaseFile(config.LogFile))
	if err != nil {
		return makeJSONResponse(fmt.Errorf("error marshalling to json: %v", err))
	}
	return string(data)
}

func SignHash(hexEncodedHash string) string {
	return logAndCallString(signHash, hexEncodedHash)
}

// signHash exposes vanilla ECDSA signing required for Swarm messages
func signHash(hexEncodedHash string) string {
	hexEncodedSignature, err := statusBackend.SignHash(hexEncodedHash)
	if err != nil {
		return makeJSONResponse(err)
	}
	return hexEncodedSignature
}

func GenerateAlias(pk string) string {
	return logAndCallString(generateAlias, pk)
}

func generateAlias(pk string) string {
	// We ignore any error, empty string is considered an error
	name, _ := protocol.GenerateAlias(pk)
	return name
}

func IsAlias(value string) string {
	return logAndCallString(isAlias, value)
}

func isAlias(value string) string {
	return prepareJSONResponse(alias.IsAlias(value), nil)
}

func Identicon(pk string) string {
	return logAndCallString(identicon, pk)
}

func identicon(pk string) string {
	// We ignore any error, empty string is considered an error
	identicon, _ := protocol.Identicon(pk)
	return identicon
}

func EmojiHash(pk string) string {
	return logAndCallString(emojiHash, pk)
}

func emojiHash(pk string) string {
	return prepareJSONResponse(emojihash.GenerateFor(pk))
}

func ColorHash(pk string) string {
	return logAndCallString(colorHash, pk)
}

func colorHash(pk string) string {
	return prepareJSONResponse(colorhash.GenerateFor(pk))
}

func ColorID(pk string) string {
	return logAndCallString(colorID, pk)
}

func colorID(pk string) string {
	return prepareJSONResponse(identityUtils.ToColorID(pk))
}

func ValidateMnemonic(mnemonic string) string {
	return logAndCallString(validateMnemonic, mnemonic)
}

func validateMnemonic(mnemonic string) string {
	m := extkeys.NewMnemonic()
	err := m.ValidateMnemonic(mnemonic, extkeys.Language(0))
	if err != nil {
		return makeJSONResponse(err)
	}

	keyUID, err := statusBackend.GetKeyUIDByMnemonic(mnemonic)
	if err != nil {
		return makeJSONResponse(err)
	}

	response := &APIKeyUIDResponse{KeyUID: keyUID}
	data, err := json.Marshal(response)
	if err != nil {
		return makeJSONResponse(err)
	}
	return string(data)
}

func DecompressPublicKey(key string) string {
	return logAndCallString(decompressPublicKey, key)
}

// decompressPublicKey decompresses 33-byte compressed format to uncompressed 65-byte format.
func decompressPublicKey(key string) string {
	decoded, err := types.DecodeHex(key)
	if err != nil {
		return makeJSONResponse(err)
	}
	const compressionBytesNumber = 33
	if len(decoded) != compressionBytesNumber {
		return makeJSONResponse(errors.New("key is not 33 bytes long"))
	}
	pubKey, err := crypto.DecompressPubkey(decoded)
	if err != nil {
		return makeJSONResponse(err)
	}
	return types.EncodeHex(crypto.FromECDSAPub(pubKey))
}

func CompressPublicKey(key string) string {
	return logAndCallString(compressPublicKey, key)
}

// compressPublicKey compresses uncompressed 65-byte format to 33-byte compressed format.
func compressPublicKey(key string) string {
	pubKey, err := common.HexToPubkey(key)
	if err != nil {
		return makeJSONResponse(err)
	}
	return types.EncodeHex(crypto.CompressPubkey(pubKey))
}

func SerializeLegacyKey(key string) string {
	return logAndCallString(serializeLegacyKey, key)
}

// serializeLegacyKey compresses an old format public key (0x04...) to the new one zQ...
func serializeLegacyKey(key string) string {
	cpk, err := multiformat.SerializeLegacyKey(key)
	if err != nil {
		return makeJSONResponse(err)
	}
	return cpk
}

func MultiformatSerializePublicKey(key, outBase string) string {
	return logAndCallString(multiformatSerializePublicKey, key, outBase)
}

// SerializePublicKey compresses an uncompressed multibase encoded multicodec identified EC public key
// For details on usage see specs https://specs.status.im/spec/2#public-key-serialization
func multiformatSerializePublicKey(key, outBase string) string {
	cpk, err := multiformat.SerializePublicKey(key, outBase)
	if err != nil {
		return makeJSONResponse(err)
	}
	return cpk
}

func MultiformatDeserializePublicKey(key, outBase string) string {
	return logAndCallString(multiformatDeserializePublicKey, key, outBase)
}

// DeserializePublicKey decompresses a compressed multibase encoded multicodec identified EC public key
// For details on usage see specs https://specs.status.im/spec/2#public-key-serialization
func multiformatDeserializePublicKey(key, outBase string) string {
	pk, err := multiformat.DeserializePublicKey(key, outBase)
	if err != nil {
		return makeJSONResponse(err)
	}
	return pk
}

func ExportUnencryptedDatabase(accountData, password, databasePath string) string {
	return logAndCallString(exportUnencryptedDatabase, accountData, password, databasePath)
}

// exportUnencryptedDatabase exports the database unencrypted to the given path
func exportUnencryptedDatabase(accountData, password, databasePath string) string {
	var account multiaccounts.Account
	err := json.Unmarshal([]byte(accountData), &account)
	if err != nil {
		return makeJSONResponse(err)
	}
	err = statusBackend.ExportUnencryptedDatabase(account, password, databasePath)
	return makeJSONResponse(err)
}

func ImportUnencryptedDatabase(accountData, password, databasePath string) string {
	return logAndCallString(importUnencryptedDatabase, accountData, password, databasePath)
}

// importUnencryptedDatabase imports the database unencrypted to the given directory
func importUnencryptedDatabase(accountData, password, databasePath string) string {
	var account multiaccounts.Account
	err := json.Unmarshal([]byte(accountData), &account)
	if err != nil {
		return makeJSONResponse(err)
	}
	err = statusBackend.ImportUnencryptedDatabase(account, password, databasePath)
	return makeJSONResponse(err)
}

func ChangeDatabasePassword(KeyUID, password, newPassword string) string {
	return logAndCallString(changeDatabasePassword, KeyUID, password, newPassword)
}

// changeDatabasePassword changes the password of the database
func changeDatabasePassword(KeyUID, password, newPassword string) string {
	err := statusBackend.ChangeDatabasePassword(KeyUID, password, newPassword)
	return makeJSONResponse(err)
}

func ConvertToKeycardAccount(accountData, settingsJSON, keycardUID, password, newPassword string) string {
	return logAndCallString(convertToKeycardAccount, accountData, settingsJSON, keycardUID, password, newPassword)
}

// convertToKeycardAccount converts the account to a keycard account
func convertToKeycardAccount(accountData, settingsJSON, keycardUID, password, newPassword string) string {
	var account multiaccounts.Account
	err := json.Unmarshal([]byte(accountData), &account)
	if err != nil {
		return makeJSONResponse(err)
	}
	var settings settings.Settings
	err = json.Unmarshal([]byte(settingsJSON), &settings)
	if err != nil {
		return makeJSONResponse(err)
	}

	err = statusBackend.ConvertToKeycardAccount(account, settings, keycardUID, password, newPassword)
	return makeJSONResponse(err)
}

func ConvertToRegularAccount(mnemonic, currPassword, newPassword string) string {
	return logAndCallString(convertToRegularAccount, mnemonic, currPassword, newPassword)
}

// convertToRegularAccount converts the account to a regular account
func convertToRegularAccount(mnemonic, currPassword, newPassword string) string {
	err := statusBackend.ConvertToRegularAccount(mnemonic, currPassword, newPassword)
	return makeJSONResponse(err)
}

func ImageServerTLSCert() string {
	cert, err := server.PublicMediaTLSCert()
	if err != nil {
		return makeJSONResponse(err)
	}
	return cert
}

type GetPasswordStrengthRequest struct {
	Password   string   `json:"password"`
	UserInputs []string `json:"userInputs"`
}

type PasswordScoreResponse struct {
	Score int `json:"score"`
}

// GetPasswordStrength uses zxcvbn module and generates a JSON containing information about the quality of the given password
// (Entropy, CrackTime, CrackTimeDisplay, Score, MatchSequence and CalcTime).
// userInputs argument can be whatever list of strings like user's personal info or site-specific vocabulary that zxcvbn will
// make use to determine the result.
// For more details on usage see https://github.com/status-im/zxcvbn-go
func GetPasswordStrength(paramsJSON string) string {
	var requestParams GetPasswordStrengthRequest

	err := json.Unmarshal([]byte(paramsJSON), &requestParams)
	if err != nil {
		return makeJSONResponse(err)
	}

	data, err := json.Marshal(zxcvbn.PasswordStrength(requestParams.Password, requestParams.UserInputs))
	if err != nil {
		return makeJSONResponse(fmt.Errorf("Error marshalling to json: %v", err))
	}
	return string(data)
}

// GetPasswordStrengthScore uses zxcvbn module and gets the score information about the given password.
// userInputs argument can be whatever list of strings like user's personal info or site-specific vocabulary that zxcvbn will
// make use to determine the result.
// For more details on usage see https://github.com/status-im/zxcvbn-go
func GetPasswordStrengthScore(paramsJSON string) string {
	var requestParams GetPasswordStrengthRequest
	var quality scoring.MinEntropyMatch

	err := json.Unmarshal([]byte(paramsJSON), &requestParams)
	if err != nil {
		return makeJSONResponse(err)
	}

	quality = zxcvbn.PasswordStrength(requestParams.Password, requestParams.UserInputs)

	data, err := json.Marshal(PasswordScoreResponse{
		Score: quality.Score,
	})
	if err != nil {
		return makeJSONResponse(fmt.Errorf("Error marshalling to json: %v", err))
	}
	return string(data)
}

type FleetDescription struct {
	DefaultFleet string                         `json:"defaultFleet"`
	Fleets       map[string]map[string][]string `json:"fleets"`
}

func Fleets() string {
	return logAndCallString(fleets)
}

func fleets() string {
	fleets := FleetDescription{
		DefaultFleet: api.DefaultFleet,
		Fleets:       params.GetSupportedFleets(),
	}

	data, err := json.Marshal(fleets)
	if err != nil {
		return makeJSONResponse(fmt.Errorf("Error marshalling to json: %v", err))
	}
	return string(data)
}

func SwitchFleet(fleet string, configJSON string) string {
	return logAndCallString(switchFleet, fleet, configJSON)
}

func switchFleet(fleet string, configJSON string) string {
	var conf params.NodeConfig
	if configJSON != "" {
		err := json.Unmarshal([]byte(configJSON), &conf)
		if err != nil {
			return makeJSONResponse(err)
		}
	}

	clusterConfig, err := params.LoadClusterConfigFromFleet(fleet)
	if err != nil {
		return makeJSONResponse(err)
	}

	conf.ClusterConfig.Fleet = fleet
	conf.ClusterConfig.ClusterID = clusterConfig.ClusterID

	err = statusBackend.SwitchFleet(fleet, &conf)

	return makeJSONResponse(err)
}

func GenerateImages(filepath string, aX, aY, bX, bY int) string {
	iis, err := images.GenerateIdentityImages(filepath, aX, aY, bX, bY)
	if err != nil {
		return makeJSONResponse(err)
	}

	data, err := json.Marshal(iis)
	if err != nil {
		return makeJSONResponse(fmt.Errorf("Error marshalling to json: %v", err))
	}
	return string(data)
}

func LocalPairingPreflightOutboundCheck() string {
	return logAndCallString(localPairingPreflightOutboundCheck)
}

// localPairingPreflightOutboundCheck creates a local tls server accessible via an outbound network address.
// The function creates a client and makes an outbound network call to the local server. This function should be
// triggered to ensure that the device has permissions to access its LAN or to make outbound network calls.
//
// In addition, the functionality attempts to address an issue with iOS devices https://stackoverflow.com/a/64242745
func localPairingPreflightOutboundCheck() string {
	err := preflight.CheckOutbound()
	return makeJSONResponse(err)
}

func StartSearchForLocalPairingPeers() string {
	return logAndCallString(startSearchForLocalPairingPeers)
}

// startSearchForLocalPairingPeers starts a UDP multicast beacon that both listens for and broadcasts to LAN peers
// on discovery the beacon will emit a signal with the details of the discovered peer.
//
// Currently, beacons are configured to search for 2 minutes pinging the network every 500 ms;
//   - If no peer discovery is made before this time elapses the operation will terminate.
//   - If a peer is discovered the pairing.PeerNotifier will terminate operation after 5 seconds, giving the peer
//     reasonable time to discover this device.
//
// Peer details are represented by a json.Marshal peers.LocalPairingPeerHello
func startSearchForLocalPairingPeers() string {
	pn := pairing.NewPeerNotifier()
	err := pn.Search()
	return makeJSONResponse(err)
}

func GetConnectionStringForBeingBootstrapped(configJSON string) string {
	return logAndCallString(getConnectionStringForBeingBootstrapped, configJSON)
}

// getConnectionStringForBeingBootstrapped starts a pairing.ReceiverServer
// then generates a pairing.ConnectionParams. Used when the device is Logged out or has no Account keys
// and the device has no camera to read a QR code with
//
// Example: A desktop device (device without camera) receiving account data from mobile (device with camera)
func getConnectionStringForBeingBootstrapped(configJSON string) string {
	if configJSON == "" {
		return makeJSONResponse(fmt.Errorf("no config given, PayloadSourceConfig is expected"))
	}

	statusBackend.LocalPairingStateManager.SetPairing(true)
	defer func() {
		statusBackend.LocalPairingStateManager.SetPairing(false)
	}()

	cs, err := pairing.StartUpReceiverServer(statusBackend, configJSON)
	if err != nil {
		return makeJSONResponse(err)
	}

	err = statusBackend.Logout()
	if err != nil {
		return makeJSONResponse(err)
	}

	return cs
}

func GetConnectionStringForBootstrappingAnotherDevice(configJSON string) string {
	return logAndCallString(getConnectionStringForBootstrappingAnotherDevice, configJSON)
}

// getConnectionStringForBootstrappingAnotherDevice starts a pairing.SenderServer
// then generates a pairing.ConnectionParams. Used when the device is Logged in and therefore has Account keys
// and the device might not have a camera
//
// Example: A mobile or desktop device (devices that MAY have a camera but MUST have a screen)
// sending account data to a mobile (device with camera)
func getConnectionStringForBootstrappingAnotherDevice(configJSON string) string {
	if configJSON == "" {
		return makeJSONResponse(fmt.Errorf("no config given, SendingServerConfig is expected"))
	}

	statusBackend.LocalPairingStateManager.SetPairing(true)
	defer func() {
		statusBackend.LocalPairingStateManager.SetPairing(false)
	}()

	cs, err := pairing.StartUpSenderServer(statusBackend, configJSON)
	if err != nil {
		return makeJSONResponse(err)
	}
	return cs
}

type inputConnectionStringForBootstrappingResponse struct {
	InstallationID string `json:"installationId"`
	KeyUID         string `json:"keyUID"`
	Error          error  `json:"error"`
}

func (i *inputConnectionStringForBootstrappingResponse) toJSON(err error) string {
	i.Error = err
	j, _ := json.Marshal(i)
	return string(j)
}

func InputConnectionStringForBootstrapping(cs, configJSON string) string {
	return logAndCallString(inputConnectionStringForBootstrapping, cs, configJSON)
}

// inputConnectionStringForBootstrapping starts a pairing.ReceiverClient
// The given server.ConnectionParams string will determine the server.Mode
//
// server.Mode = server.Sending
// Used when the device is Logged out or has no Account keys and has a camera to read a QR code
//
// Example: A mobile device (device with a camera) receiving account data from
// a device with a screen (mobile or desktop devices)
func inputConnectionStringForBootstrapping(cs, configJSON string) string {
	var err error
	if configJSON == "" {
		return makeJSONResponse(fmt.Errorf("no config given, ReceiverClientConfig is expected"))
	}

	params := &pairing.ConnectionParams{}
	err = params.FromString(cs)
	if err != nil {
		response := &inputConnectionStringForBootstrappingResponse{}
		return response.toJSON(fmt.Errorf("could not parse connection string"))
	}
	response := &inputConnectionStringForBootstrappingResponse{
		InstallationID: params.InstallationID(),
		KeyUID:         params.KeyUID(),
	}

	err = statusBackend.LocalPairingStateManager.StartPairing(cs)
	defer func() { statusBackend.LocalPairingStateManager.StopPairing(cs, err) }()
	if err != nil {
		return response.toJSON(err)
	}

	err = pairing.StartUpReceivingClient(statusBackend, cs, configJSON)
	if err != nil {
		return response.toJSON(err)
	}

	return response.toJSON(statusBackend.Logout())
}

func InputConnectionStringForBootstrappingAnotherDevice(cs, configJSON string) string {
	return logAndCallString(inputConnectionStringForBootstrappingAnotherDevice, cs, configJSON)
}

// inputConnectionStringForBootstrappingAnotherDevice starts a pairing.SendingClient
// The given server.ConnectionParams string will determine the server.Mode
//
// server.Mode = server.Receiving
// Used when the device is Logged in and therefore has Account keys and the has a camera to read a QR code
//
// Example: A mobile (device with camera) sending account data to a desktop device (device without camera)
func inputConnectionStringForBootstrappingAnotherDevice(cs, configJSON string) string {
	var err error
	if configJSON == "" {
		return makeJSONResponse(fmt.Errorf("no config given, SenderClientConfig is expected"))
	}

	err = statusBackend.LocalPairingStateManager.StartPairing(cs)
	defer func() { statusBackend.LocalPairingStateManager.StopPairing(cs, err) }()
	if err != nil {
		return makeJSONResponse(err)
	}

	err = pairing.StartUpSendingClient(statusBackend, cs, configJSON)
	return makeJSONResponse(err)
}

func GetConnectionStringForExportingKeypairsKeystores(configJSON string) string {
	return logAndCallString(getConnectionStringForExportingKeypairsKeystores, configJSON)
}

// getConnectionStringForExportingKeypairsKeystores starts a pairing.SenderServer
// then generates a pairing.ConnectionParams. Used when the device is Logged in and therefore has Account keys
// and the device might not have a camera, to transfer kestore files of provided key uids.
func getConnectionStringForExportingKeypairsKeystores(configJSON string) string {
	if configJSON == "" {
		return makeJSONResponse(fmt.Errorf("no config given, SendingServerConfig is expected"))
	}

	cs, err := pairing.StartUpKeystoreFilesSenderServer(statusBackend, configJSON)
	if err != nil {
		return makeJSONResponse(err)
	}
	return cs
}

func InputConnectionStringForImportingKeypairsKeystores(cs, configJSON string) string {
	return logAndCallString(inputConnectionStringForImportingKeypairsKeystores, cs, configJSON)
}

// inputConnectionStringForImportingKeypairsKeystores starts a pairing.ReceiverClient
// The given server.ConnectionParams string will determine the server.Mode
// Used when the device is Logged in and has Account keys and has a camera to read a QR code
//
// Example: A mobile device (device with a camera) receiving account data from
// a device with a screen (mobile or desktop devices)
func inputConnectionStringForImportingKeypairsKeystores(cs, configJSON string) string {
	if configJSON == "" {
		return makeJSONResponse(fmt.Errorf("no config given, ReceiverClientConfig is expected"))
	}

	err := pairing.StartUpKeystoreFilesReceivingClient(statusBackend, cs, configJSON)
	return makeJSONResponse(err)
}

func ValidateConnectionString(cs string) string {
	return logAndCallString(validateConnectionString, cs)
}

func validateConnectionString(cs string) string {
	err := pairing.ValidateConnectionString(cs)
	if err == nil {
		return ""
	}
	return err.Error()
}

func EncodeTransfer(to string, value string) string {
	return logAndCallString(encodeTransfer, to, value)
}

func encodeTransfer(to string, value string) string {
	result, err := abi_spec.EncodeTransfer(to, value)
	if err != nil {
		log.Error("failed to encode transfer", "to", to, "value", value, "error", err)
		return ""
	}
	return result
}

func EncodeFunctionCall(method string, paramsJSON string) string {
	return logAndCallString(encodeFunctionCall, method, paramsJSON)
}

func encodeFunctionCall(method string, paramsJSON string) string {
	result, err := abi_spec.Encode(method, paramsJSON)
	if err != nil {
		log.Error("failed to encode function call", "method", method, "paramsJSON", paramsJSON, "error", err)
		return ""
	}
	return result
}

func DecodeParameters(decodeParamJSON string) string {
	return decodeParameters(decodeParamJSON)
}

func decodeParameters(decodeParamJSON string) string {
	decodeParam := struct {
		BytesString string   `json:"bytesString"`
		Types       []string `json:"types"`
	}{}
	err := json.Unmarshal([]byte(decodeParamJSON), &decodeParam)
	if err != nil {
		log.Error("failed to unmarshal json when decoding parameters", "decodeParamJSON", decodeParamJSON, "error", err)
		return ""
	}
	result, err := abi_spec.Decode(decodeParam.BytesString, decodeParam.Types)
	if err != nil {
		log.Error("failed to decode parameters", "decodeParamJSON", decodeParamJSON, "error", err)
		return ""
	}
	bytes, err := json.Marshal(result)
	if err != nil {
		log.Error("failed to marshal result", "result", result, "decodeParamJSON", decodeParamJSON, "error", err)
		return ""
	}
	return string(bytes)
}

func HexToNumber(hex string) string {
	return logAndCallString(hexToNumber, hex)
}

func hexToNumber(hex string) string {
	return abi_spec.HexToNumber(hex)
}

func NumberToHex(numString string) string {
	return logAndCallString(numberToHex, numString)
}

func numberToHex(numString string) string {
	return abi_spec.NumberToHex(numString)
}

func Sha3(str string) string {
	return "0x" + abi_spec.Sha3(str)
}

func Utf8ToHex(str string) string {
	return logAndCallString(utf8ToHex, str)
}

func utf8ToHex(str string) string {
	hexString, err := abi_spec.Utf8ToHex(str)
	if err != nil {
		log.Error("failed to convert utf8 to hex", "str", str, "error", err)
	}
	return hexString
}

func HexToUtf8(hexString string) string {
	return logAndCallString(hexToUtf8, hexString)
}

func hexToUtf8(hexString string) string {
	str, err := abi_spec.HexToUtf8(hexString)
	if err != nil {
		log.Error("failed to convert hex to utf8", "hexString", hexString, "error", err)
	}
	return str
}

func CheckAddressChecksum(address string) string {
	return logAndCallString(checkAddressChecksum, address)
}

func checkAddressChecksum(address string) string {
	valid, err := abi_spec.CheckAddressChecksum(address)
	if err != nil {
		log.Error("failed to invoke check address checksum", "address", address, "error", err)
	}
	result, _ := json.Marshal(valid)
	return string(result)
}

func IsAddress(address string) string {
	return logAndCallString(isAddress, address)
}

func isAddress(address string) string {
	valid, err := abi_spec.IsAddress(address)
	if err != nil {
		log.Error("failed to invoke IsAddress", "address", address, "error", err)
	}
	result, _ := json.Marshal(valid)
	return string(result)
}

func ToChecksumAddress(address string) string {
	return logAndCallString(toChecksumAddress, address)
}

func toChecksumAddress(address string) string {
	address, err := abi_spec.ToChecksumAddress(address)
	if err != nil {
		log.Error("failed to convert to checksum address", "address", address, "error", err)
	}
	return address
}

func DeserializeAndCompressKey(DesktopKey string) string {
	return logAndCallString(deserializeAndCompressKey, DesktopKey)
}

func deserializeAndCompressKey(DesktopKey string) string {
	deserialisedKey := MultiformatDeserializePublicKey(DesktopKey, "f")
	sanitisedKey := "0x" + deserialisedKey[5:]
	return CompressPublicKey(sanitisedKey)
}

type InitLoggingRequest struct {
	logutils.LogSettings
	LogRequestGo   bool   `json:"LogRequestGo"`
	LogRequestFile string `json:"LogRequestFile"`
}

// InitLogging The InitLogging function should be called when the application starts.
// This ensures that we can capture logs before the user login. Subsequent calls will update the logger settings.
// Before this, we can only capture logs after user login since we will only configure the logging after the login process.
func InitLogging(logSettingsJSON string) string {
	var logSettings InitLoggingRequest
	var err error
	if err = json.Unmarshal([]byte(logSettingsJSON), &logSettings); err != nil {
		return makeJSONResponse(err)
	}

	if err = logutils.OverrideRootLogWithConfig(logSettings.LogSettings, false); err == nil {
		log.Info("logging initialised", "logSettings", logSettingsJSON)
	}

	if logSettings.LogRequestGo {
		err = requestlog.ConfigureAndEnableRequestLogging(logSettings.LogRequestFile)
		if err != nil {
			return makeJSONResponse(err)
		}
	}

	return makeJSONResponse(err)
}

func GetRandomMnemonic() string {
	mnemonic, err := account.GetRandomMnemonic()
	if err != nil {
		return makeJSONResponse(err)
	}
	return mnemonic
}

func ToggleCentralizedMetrics(requestJSON string) string {
	return logAndCallString(toggleCentralizedMetrics, requestJSON)
}

func toggleCentralizedMetrics(requestJSON string) string {
	var request requests.ToggleCentralizedMetrics
	err := json.Unmarshal([]byte(requestJSON), &request)
	if err != nil {
		return makeJSONResponse(err)
	}

	err = request.Validate()
	if err != nil {
		return makeJSONResponse(err)
	}

	err = statusBackend.ToggleCentralizedMetrics(request.Enabled)
	if err != nil {
		return makeJSONResponse(err)
	}

	return makeJSONResponse(nil)
}

func CentralizedMetricsInfo() string {
	return logAndCallString(centralizedMetricsInfo)
}

func centralizedMetricsInfo() string {
	metricsInfo, err := statusBackend.CentralizedMetricsInfo()
	if err != nil {
		return makeJSONResponse(err)
	}
	data, err := json.Marshal(metricsInfo)
	if err != nil {
		return makeJSONResponse(err)
	}
	return string(data)
}

func AddCentralizedMetric(requestJSON string) string {
	return logAndCallString(addCentralizedMetric, requestJSON)
}

func addCentralizedMetric(requestJSON string) string {
	var request requests.AddCentralizedMetric
	err := json.Unmarshal([]byte(requestJSON), &request)
	if err != nil {
		return makeJSONResponse(err)
	}

	err = request.Validate()
	if err != nil {
		return makeJSONResponse(err)
	}
	metric := request.Metric

	metric.EnsureID()
	err = statusBackend.AddCentralizedMetric(*metric)
	if err != nil {
		return makeJSONResponse(err)
	}

	return metric.ID
}
