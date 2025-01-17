// Code generated by MockGen. DO NOT EDIT.
// Source: rpc/network/network.go

// Package mock_network is a generated GoMock package.
package mock_network

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"

	params "github.com/status-im/status-go/params"
)

// MockManagerInterface is a mock of ManagerInterface interface.
type MockManagerInterface struct {
	ctrl     *gomock.Controller
	recorder *MockManagerInterfaceMockRecorder
}

// MockManagerInterfaceMockRecorder is the mock recorder for MockManagerInterface.
type MockManagerInterfaceMockRecorder struct {
	mock *MockManagerInterface
}

// NewMockManagerInterface creates a new mock instance.
func NewMockManagerInterface(ctrl *gomock.Controller) *MockManagerInterface {
	mock := &MockManagerInterface{ctrl: ctrl}
	mock.recorder = &MockManagerInterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockManagerInterface) EXPECT() *MockManagerInterfaceMockRecorder {
	return m.recorder
}

// Find mocks base method.
func (m *MockManagerInterface) Find(chainID uint64) *params.Network {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Find", chainID)
	ret0, _ := ret[0].(*params.Network)
	return ret0
}

// Find indicates an expected call of Find.
func (mr *MockManagerInterfaceMockRecorder) Find(chainID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Find", reflect.TypeOf((*MockManagerInterface)(nil).Find), chainID)
}

// Get mocks base method.
func (m *MockManagerInterface) Get(onlyEnabled bool) ([]*params.Network, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", onlyEnabled)
	ret0, _ := ret[0].([]*params.Network)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockManagerInterfaceMockRecorder) Get(onlyEnabled interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockManagerInterface)(nil).Get), onlyEnabled)
}

// GetAll mocks base method.
func (m *MockManagerInterface) GetAll() ([]*params.Network, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAll")
	ret0, _ := ret[0].([]*params.Network)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAll indicates an expected call of GetAll.
func (mr *MockManagerInterfaceMockRecorder) GetAll() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAll", reflect.TypeOf((*MockManagerInterface)(nil).GetAll))
}

// GetConfiguredNetworks mocks base method.
func (m *MockManagerInterface) GetConfiguredNetworks() []params.Network {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetConfiguredNetworks")
	ret0, _ := ret[0].([]params.Network)
	return ret0
}

// GetConfiguredNetworks indicates an expected call of GetConfiguredNetworks.
func (mr *MockManagerInterfaceMockRecorder) GetConfiguredNetworks() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetConfiguredNetworks", reflect.TypeOf((*MockManagerInterface)(nil).GetConfiguredNetworks))
}

// GetTestNetworksEnabled mocks base method.
func (m *MockManagerInterface) GetTestNetworksEnabled() (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetTestNetworksEnabled")
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTestNetworksEnabled indicates an expected call of GetTestNetworksEnabled.
func (mr *MockManagerInterfaceMockRecorder) GetTestNetworksEnabled() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTestNetworksEnabled", reflect.TypeOf((*MockManagerInterface)(nil).GetTestNetworksEnabled))
}
