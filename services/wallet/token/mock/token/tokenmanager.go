// Code generated by MockGen. DO NOT EDIT.
// Source: services/wallet/token/token.go

// Package mock_token is a generated GoMock package.
package mock_token

import (
	context "context"
	big "math/big"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"

	common "github.com/ethereum/go-ethereum/common"
	hexutil "github.com/ethereum/go-ethereum/common/hexutil"
	chain "github.com/status-im/status-go/rpc/chain"
	token "github.com/status-im/status-go/services/wallet/token"
)

// MockManagerInterface is a mock of ManagerInterface interface
type MockManagerInterface struct {
	ctrl     *gomock.Controller
	recorder *MockManagerInterfaceMockRecorder
}

// MockManagerInterfaceMockRecorder is the mock recorder for MockManagerInterface
type MockManagerInterfaceMockRecorder struct {
	mock *MockManagerInterface
}

// NewMockManagerInterface creates a new mock instance
func NewMockManagerInterface(ctrl *gomock.Controller) *MockManagerInterface {
	mock := &MockManagerInterface{ctrl: ctrl}
	mock.recorder = &MockManagerInterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockManagerInterface) EXPECT() *MockManagerInterfaceMockRecorder {
	return m.recorder
}

// LookupTokenIdentity mocks base method
func (m *MockManagerInterface) LookupTokenIdentity(chainID uint64, address common.Address, native bool) *token.Token {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LookupTokenIdentity", chainID, address, native)
	ret0, _ := ret[0].(*token.Token)
	return ret0
}

// LookupTokenIdentity indicates an expected call of LookupTokenIdentity
func (mr *MockManagerInterfaceMockRecorder) LookupTokenIdentity(chainID, address, native interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LookupTokenIdentity", reflect.TypeOf((*MockManagerInterface)(nil).LookupTokenIdentity), chainID, address, native)
}

// LookupToken mocks base method
func (m *MockManagerInterface) LookupToken(chainID *uint64, tokenSymbol string) (*token.Token, bool) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LookupToken", chainID, tokenSymbol)
	ret0, _ := ret[0].(*token.Token)
	ret1, _ := ret[1].(bool)
	return ret0, ret1
}

// LookupToken indicates an expected call of LookupToken
func (mr *MockManagerInterfaceMockRecorder) LookupToken(chainID, tokenSymbol interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LookupToken", reflect.TypeOf((*MockManagerInterface)(nil).LookupToken), chainID, tokenSymbol)
}

// GetTokensByChainIDs mocks base method
func (m *MockManagerInterface) GetTokensByChainIDs(chainIDs []uint64) ([]*token.Token, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetTokensByChainIDs", chainIDs)
	ret0, _ := ret[0].([]*token.Token)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTokensByChainIDs indicates an expected call of GetTokensByChainIDs
func (mr *MockManagerInterfaceMockRecorder) GetTokensByChainIDs(chainIDs interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTokensByChainIDs", reflect.TypeOf((*MockManagerInterface)(nil).GetTokensByChainIDs), chainIDs)
}

// GetBalancesByChain mocks base method
func (m *MockManagerInterface) GetBalancesByChain(parent context.Context, clients map[uint64]chain.ClientInterface, accounts, tokens []common.Address) (map[uint64]map[common.Address]map[common.Address]*hexutil.Big, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBalancesByChain", parent, clients, accounts, tokens)
	ret0, _ := ret[0].(map[uint64]map[common.Address]map[common.Address]*hexutil.Big)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBalancesByChain indicates an expected call of GetBalancesByChain
func (mr *MockManagerInterfaceMockRecorder) GetBalancesByChain(parent, clients, accounts, tokens interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBalancesByChain", reflect.TypeOf((*MockManagerInterface)(nil).GetBalancesByChain), parent, clients, accounts, tokens)
}

// GetTokenHistoricalBalance mocks base method
func (m *MockManagerInterface) GetTokenHistoricalBalance(account common.Address, chainID uint64, symbol string, timestamp int64) (*big.Int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetTokenHistoricalBalance", account, chainID, symbol, timestamp)
	ret0, _ := ret[0].(*big.Int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTokenHistoricalBalance indicates an expected call of GetTokenHistoricalBalance
func (mr *MockManagerInterfaceMockRecorder) GetTokenHistoricalBalance(account, chainID, symbol, timestamp interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTokenHistoricalBalance", reflect.TypeOf((*MockManagerInterface)(nil).GetTokenHistoricalBalance), account, chainID, symbol, timestamp)
}