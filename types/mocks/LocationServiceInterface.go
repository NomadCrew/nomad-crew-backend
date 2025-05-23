// Code generated by mockery v2.53.3. DO NOT EDIT.

package mocks

import (
	types "github.com/NomadCrew/nomad-crew-backend/types"
	mock "github.com/stretchr/testify/mock"
)

// LocationServiceInterface is an autogenerated mock type for the LocationServiceInterface type
type LocationServiceInterface struct {
	mock.Mock
}

// GetTripMemberLocations provides a mock function with given fields: ctx, tripID
func (_m *LocationServiceInterface) GetTripMemberLocations(ctx interface{}, tripID string) ([]types.MemberLocation, error) {
	ret := _m.Called(ctx, tripID)

	if len(ret) == 0 {
		panic("no return value specified for GetTripMemberLocations")
	}

	var r0 []types.MemberLocation
	var r1 error
	if rf, ok := ret.Get(0).(func(interface{}, string) ([]types.MemberLocation, error)); ok {
		return rf(ctx, tripID)
	}
	if rf, ok := ret.Get(0).(func(interface{}, string) []types.MemberLocation); ok {
		r0 = rf(ctx, tripID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]types.MemberLocation)
		}
	}

	if rf, ok := ret.Get(1).(func(interface{}, string) error); ok {
		r1 = rf(ctx, tripID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UpdateLocation provides a mock function with given fields: ctx, userID, update
func (_m *LocationServiceInterface) UpdateLocation(ctx interface{}, userID string, update types.LocationUpdate) (*types.Location, error) {
	ret := _m.Called(ctx, userID, update)

	if len(ret) == 0 {
		panic("no return value specified for UpdateLocation")
	}

	var r0 *types.Location
	var r1 error
	if rf, ok := ret.Get(0).(func(interface{}, string, types.LocationUpdate) (*types.Location, error)); ok {
		return rf(ctx, userID, update)
	}
	if rf, ok := ret.Get(0).(func(interface{}, string, types.LocationUpdate) *types.Location); ok {
		r0 = rf(ctx, userID, update)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.Location)
		}
	}

	if rf, ok := ret.Get(1).(func(interface{}, string, types.LocationUpdate) error); ok {
		r1 = rf(ctx, userID, update)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewLocationServiceInterface creates a new instance of LocationServiceInterface. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewLocationServiceInterface(t interface {
	mock.TestingT
	Cleanup(func())
}) *LocationServiceInterface {
	mock := &LocationServiceInterface{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
