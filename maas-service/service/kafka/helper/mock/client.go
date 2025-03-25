// Code generated by MockGen. DO NOT EDIT.
// Source: sarama.go

// Package mock_helper is a generated GoMock package.
package mock_helper

import (
	reflect "reflect"

	sarama "github.com/IBM/sarama"
	gomock "github.com/golang/mock/gomock"
)

// MockSaramaClient is a mock of SaramaClient interface.
type MockSaramaClient struct {
	ctrl     *gomock.Controller
	recorder *MockSaramaClientMockRecorder
}

// MockSaramaClientMockRecorder is the mock recorder for MockSaramaClient.
type MockSaramaClientMockRecorder struct {
	mock *MockSaramaClient
}

// NewMockSaramaClient creates a new mock instance.
func NewMockSaramaClient(ctrl *gomock.Controller) *MockSaramaClient {
	mock := &MockSaramaClient{ctrl: ctrl}
	mock.recorder = &MockSaramaClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSaramaClient) EXPECT() *MockSaramaClientMockRecorder {
	return m.recorder
}

// NewClusterAdmin mocks base method.
func (m *MockSaramaClient) NewClusterAdmin(addrs []string, conf *sarama.Config) (sarama.ClusterAdmin, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NewClusterAdmin", addrs, conf)
	ret0, _ := ret[0].(sarama.ClusterAdmin)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NewClusterAdmin indicates an expected call of NewClusterAdmin.
func (mr *MockSaramaClientMockRecorder) NewClusterAdmin(addrs, conf interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewClusterAdmin", reflect.TypeOf((*MockSaramaClient)(nil).NewClusterAdmin), addrs, conf)
}
