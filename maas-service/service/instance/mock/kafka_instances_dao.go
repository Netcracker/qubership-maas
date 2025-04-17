// Code generated by MockGen. DO NOT EDIT.
// Source: kafka_instances_dao.go

// Package mock_instance is a generated GoMock package.
package mock_instance

import (
	context "context"
	model "github.com/netcracker/qubership-maas/model"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// MockKafkaInstancesDao is a mock of KafkaInstancesDao interface.
type MockKafkaInstancesDao struct {
	ctrl     *gomock.Controller
	recorder *MockKafkaInstancesDaoMockRecorder
}

// MockKafkaInstancesDaoMockRecorder is the mock recorder for MockKafkaInstancesDao.
type MockKafkaInstancesDaoMockRecorder struct {
	mock *MockKafkaInstancesDao
}

// NewMockKafkaInstancesDao creates a new mock instance.
func NewMockKafkaInstancesDao(ctrl *gomock.Controller) *MockKafkaInstancesDao {
	mock := &MockKafkaInstancesDao{ctrl: ctrl}
	mock.recorder = &MockKafkaInstancesDaoMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockKafkaInstancesDao) EXPECT() *MockKafkaInstancesDaoMockRecorder {
	return m.recorder
}

// DeleteKafkaInstanceDesignatorByNamespace mocks base method.
func (m *MockKafkaInstancesDao) DeleteKafkaInstanceDesignatorByNamespace(ctx context.Context, namespace string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteKafkaInstanceDesignatorByNamespace", ctx, namespace)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteKafkaInstanceDesignatorByNamespace indicates an expected call of DeleteKafkaInstanceDesignatorByNamespace.
func (mr *MockKafkaInstancesDaoMockRecorder) DeleteKafkaInstanceDesignatorByNamespace(ctx, namespace interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteKafkaInstanceDesignatorByNamespace", reflect.TypeOf((*MockKafkaInstancesDao)(nil).DeleteKafkaInstanceDesignatorByNamespace), ctx, namespace)
}

// GetDefaultInstance mocks base method.
func (m *MockKafkaInstancesDao) GetDefaultInstance(arg0 context.Context) (*model.KafkaInstance, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetDefaultInstance", arg0)
	ret0, _ := ret[0].(*model.KafkaInstance)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetDefaultInstance indicates an expected call of GetDefaultInstance.
func (mr *MockKafkaInstancesDaoMockRecorder) GetDefaultInstance(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDefaultInstance", reflect.TypeOf((*MockKafkaInstancesDao)(nil).GetDefaultInstance), arg0)
}

// GetInstanceById mocks base method.
func (m *MockKafkaInstancesDao) GetInstanceById(arg0 context.Context, arg1 string) (*model.KafkaInstance, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetInstanceById", arg0, arg1)
	ret0, _ := ret[0].(*model.KafkaInstance)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetInstanceById indicates an expected call of GetInstanceById.
func (mr *MockKafkaInstancesDaoMockRecorder) GetInstanceById(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetInstanceById", reflect.TypeOf((*MockKafkaInstancesDao)(nil).GetInstanceById), arg0, arg1)
}

// GetKafkaInstanceDesignatorByNamespace mocks base method.
func (m *MockKafkaInstancesDao) GetKafkaInstanceDesignatorByNamespace(ctx context.Context, namespace string) (*model.InstanceDesignatorKafka, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetKafkaInstanceDesignatorByNamespace", ctx, namespace)
	ret0, _ := ret[0].(*model.InstanceDesignatorKafka)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetKafkaInstanceDesignatorByNamespace indicates an expected call of GetKafkaInstanceDesignatorByNamespace.
func (mr *MockKafkaInstancesDaoMockRecorder) GetKafkaInstanceDesignatorByNamespace(ctx, namespace interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetKafkaInstanceDesignatorByNamespace", reflect.TypeOf((*MockKafkaInstancesDao)(nil).GetKafkaInstanceDesignatorByNamespace), ctx, namespace)
}

// GetKafkaInstanceRegistrations mocks base method.
func (m *MockKafkaInstancesDao) GetKafkaInstanceRegistrations(arg0 context.Context) (*[]model.KafkaInstance, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetKafkaInstanceRegistrations", arg0)
	ret0, _ := ret[0].(*[]model.KafkaInstance)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetKafkaInstanceRegistrations indicates an expected call of GetKafkaInstanceRegistrations.
func (mr *MockKafkaInstancesDaoMockRecorder) GetKafkaInstanceRegistrations(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetKafkaInstanceRegistrations", reflect.TypeOf((*MockKafkaInstancesDao)(nil).GetKafkaInstanceRegistrations), arg0)
}

// InsertInstanceDesignatorKafka mocks base method.
func (m *MockKafkaInstancesDao) InsertInstanceDesignatorKafka(ctx context.Context, instanceDesignator *model.InstanceDesignatorKafka) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "InsertInstanceDesignatorKafka", ctx, instanceDesignator)
	ret0, _ := ret[0].(error)
	return ret0
}

// InsertInstanceDesignatorKafka indicates an expected call of InsertInstanceDesignatorKafka.
func (mr *MockKafkaInstancesDaoMockRecorder) InsertInstanceDesignatorKafka(ctx, instanceDesignator interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "InsertInstanceDesignatorKafka", reflect.TypeOf((*MockKafkaInstancesDao)(nil).InsertInstanceDesignatorKafka), ctx, instanceDesignator)
}

// InsertInstanceRegistration mocks base method.
func (m *MockKafkaInstancesDao) InsertInstanceRegistration(arg0 context.Context, arg1 *model.KafkaInstance) (*model.KafkaInstance, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "InsertInstanceRegistration", arg0, arg1)
	ret0, _ := ret[0].(*model.KafkaInstance)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// InsertInstanceRegistration indicates an expected call of InsertInstanceRegistration.
func (mr *MockKafkaInstancesDaoMockRecorder) InsertInstanceRegistration(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "InsertInstanceRegistration", reflect.TypeOf((*MockKafkaInstancesDao)(nil).InsertInstanceRegistration), arg0, arg1)
}

// RemoveInstanceRegistration mocks base method.
func (m *MockKafkaInstancesDao) RemoveInstanceRegistration(ctx context.Context, instanceId string) (*model.KafkaInstance, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoveInstanceRegistration", ctx, instanceId)
	ret0, _ := ret[0].(*model.KafkaInstance)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RemoveInstanceRegistration indicates an expected call of RemoveInstanceRegistration.
func (mr *MockKafkaInstancesDaoMockRecorder) RemoveInstanceRegistration(ctx, instanceId interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveInstanceRegistration", reflect.TypeOf((*MockKafkaInstancesDao)(nil).RemoveInstanceRegistration), ctx, instanceId)
}

// SetDefaultInstance mocks base method.
func (m *MockKafkaInstancesDao) SetDefaultInstance(arg0 context.Context, arg1 *model.KafkaInstance) (*model.KafkaInstance, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetDefaultInstance", arg0, arg1)
	ret0, _ := ret[0].(*model.KafkaInstance)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SetDefaultInstance indicates an expected call of SetDefaultInstance.
func (mr *MockKafkaInstancesDaoMockRecorder) SetDefaultInstance(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetDefaultInstance", reflect.TypeOf((*MockKafkaInstancesDao)(nil).SetDefaultInstance), arg0, arg1)
}

// UpdateInstanceRegistration mocks base method.
func (m *MockKafkaInstancesDao) UpdateInstanceRegistration(arg0 context.Context, arg1 *model.KafkaInstance) (*model.KafkaInstance, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateInstanceRegistration", arg0, arg1)
	ret0, _ := ret[0].(*model.KafkaInstance)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdateInstanceRegistration indicates an expected call of UpdateInstanceRegistration.
func (mr *MockKafkaInstancesDaoMockRecorder) UpdateInstanceRegistration(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateInstanceRegistration", reflect.TypeOf((*MockKafkaInstancesDao)(nil).UpdateInstanceRegistration), arg0, arg1)
}
