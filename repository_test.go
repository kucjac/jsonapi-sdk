// Code generated by mockery v1.0.0
package jsonapisdk

import jsonapi "github.com/kucjac/jsonapi"
import mock "github.com/stretchr/testify/mock"
import unidb "github.com/kucjac/uni-db"

// MockRepository is an autogenerated mock type for the Repository type
type MockRepository struct {
	mock.Mock
}

// Create provides a mock function with given fields: scope
func (_m *MockRepository) Create(scope *jsonapi.Scope) *unidb.Error {
	ret := _m.Called(scope)

	var r0 *unidb.Error
	if rf, ok := ret.Get(0).(func(*jsonapi.Scope) *unidb.Error); ok {
		r0 = rf(scope)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*unidb.Error)
		}
	}

	return r0
}

// Delete provides a mock function with given fields: scope
func (_m *MockRepository) Delete(scope *jsonapi.Scope) *unidb.Error {
	ret := _m.Called(scope)

	var r0 *unidb.Error
	if rf, ok := ret.Get(0).(func(*jsonapi.Scope) *unidb.Error); ok {
		r0 = rf(scope)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*unidb.Error)
		}
	}

	return r0
}

// Get provides a mock function with given fields: scope
func (_m *MockRepository) Get(scope *jsonapi.Scope) *unidb.Error {
	ret := _m.Called(scope)

	var r0 *unidb.Error
	if rf, ok := ret.Get(0).(func(*jsonapi.Scope) *unidb.Error); ok {
		r0 = rf(scope)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*unidb.Error)
		}
	}

	return r0
}

// List provides a mock function with given fields: scope
func (_m *MockRepository) List(scope *jsonapi.Scope) *unidb.Error {
	ret := _m.Called(scope)

	var r0 *unidb.Error
	if rf, ok := ret.Get(0).(func(*jsonapi.Scope) *unidb.Error); ok {
		r0 = rf(scope)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*unidb.Error)
		}
	}

	return r0
}

// Patch provides a mock function with given fields: scope
func (_m *MockRepository) Patch(scope *jsonapi.Scope) *unidb.Error {
	ret := _m.Called(scope)

	var r0 *unidb.Error
	if rf, ok := ret.Get(0).(func(*jsonapi.Scope) *unidb.Error); ok {
		r0 = rf(scope)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*unidb.Error)
		}
	}

	return r0
}
