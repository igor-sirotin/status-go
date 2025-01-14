// Code generated by MockGen. DO NOT EDIT.
// Source: services/wallet/token/balance_persistence.go

// Package mock_balance_persistence is a generated GoMock package.
package mock_balance_persistence

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"

	common "github.com/ethereum/go-ethereum/common"
	token "github.com/status-im/status-go/services/wallet/token"
)

// MockTokenBalancesStorage is a mock of TokenBalancesStorage interface
type MockTokenBalancesStorage struct {
	ctrl     *gomock.Controller
	recorder *MockTokenBalancesStorageMockRecorder
}

// MockTokenBalancesStorageMockRecorder is the mock recorder for MockTokenBalancesStorage
type MockTokenBalancesStorageMockRecorder struct {
	mock *MockTokenBalancesStorage
}

// NewMockTokenBalancesStorage creates a new mock instance
func NewMockTokenBalancesStorage(ctrl *gomock.Controller) *MockTokenBalancesStorage {
	mock := &MockTokenBalancesStorage{ctrl: ctrl}
	mock.recorder = &MockTokenBalancesStorageMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockTokenBalancesStorage) EXPECT() *MockTokenBalancesStorageMockRecorder {
	return m.recorder
}

// SaveTokens mocks base method
func (m *MockTokenBalancesStorage) SaveTokens(tokens map[common.Address][]token.StorageToken) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SaveTokens", tokens)
	ret0, _ := ret[0].(error)
	return ret0
}

// SaveTokens indicates an expected call of SaveTokens
func (mr *MockTokenBalancesStorageMockRecorder) SaveTokens(tokens interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SaveTokens", reflect.TypeOf((*MockTokenBalancesStorage)(nil).SaveTokens), tokens)
}

// GetTokens mocks base method
func (m *MockTokenBalancesStorage) GetTokens() (map[common.Address][]token.StorageToken, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetTokens")
	ret0, _ := ret[0].(map[common.Address][]token.StorageToken)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTokens indicates an expected call of GetTokens
func (mr *MockTokenBalancesStorageMockRecorder) GetTokens() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTokens", reflect.TypeOf((*MockTokenBalancesStorage)(nil).GetTokens))
}
