// Code generated by MockGen. DO NOT EDIT.
// Source: custom_resource.go

// Package v1 is a generated GoMock package.
package v1

import (
	context "context"
	cr "github.com/netcracker/qubership-maas/service/cr"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// MockCustomResourceProcessorService is a mock of CustomResourceProcessorService interface.
type MockCustomResourceProcessorService struct {
	ctrl     *gomock.Controller
	recorder *MockCustomResourceProcessorServiceMockRecorder
}

// MockCustomResourceProcessorServiceMockRecorder is the mock recorder for MockCustomResourceProcessorService.
type MockCustomResourceProcessorServiceMockRecorder struct {
	mock *MockCustomResourceProcessorService
}

// NewMockCustomResourceProcessorService creates a new mock instance.
func NewMockCustomResourceProcessorService(ctrl *gomock.Controller) *MockCustomResourceProcessorService {
	mock := &MockCustomResourceProcessorService{ctrl: ctrl}
	mock.recorder = &MockCustomResourceProcessorServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockCustomResourceProcessorService) EXPECT() *MockCustomResourceProcessorServiceMockRecorder {
	return m.recorder
}

// Apply mocks base method.
func (m *MockCustomResourceProcessorService) Apply(ctx context.Context, customResource *cr.CustomResourceRequest, action cr.Action) (*cr.CustomResourceWaitEntity, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Apply", ctx, customResource, action)
	ret0, _ := ret[0].(*cr.CustomResourceWaitEntity)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Apply indicates an expected call of Apply.
func (mr *MockCustomResourceProcessorServiceMockRecorder) Apply(ctx, customResource, action interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Apply", reflect.TypeOf((*MockCustomResourceProcessorService)(nil).Apply), ctx, customResource, action)
}

// GetStatus mocks base method.
func (m *MockCustomResourceProcessorService) GetStatus(ctx context.Context, trackingId int64) (*cr.CustomResourceWaitEntity, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetStatus", ctx, trackingId)
	ret0, _ := ret[0].(*cr.CustomResourceWaitEntity)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetStatus indicates an expected call of GetStatus.
func (mr *MockCustomResourceProcessorServiceMockRecorder) GetStatus(ctx, trackingId interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetStatus", reflect.TypeOf((*MockCustomResourceProcessorService)(nil).GetStatus), ctx, trackingId)
}

// Terminate mocks base method.
func (m *MockCustomResourceProcessorService) Terminate(ctx context.Context, trackingId int64) (*cr.CustomResourceWaitEntity, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Terminate", ctx, trackingId)
	ret0, _ := ret[0].(*cr.CustomResourceWaitEntity)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Terminate indicates an expected call of Terminate.
func (mr *MockCustomResourceProcessorServiceMockRecorder) Terminate(ctx, trackingId interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Terminate", reflect.TypeOf((*MockCustomResourceProcessorService)(nil).Terminate), ctx, trackingId)
}
