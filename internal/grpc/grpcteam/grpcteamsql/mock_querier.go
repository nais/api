// Code generated by mockery. DO NOT EDIT.

package grpcteamsql

import (
	context "context"

	slug "github.com/nais/api/internal/slug"
	mock "github.com/stretchr/testify/mock"
)

// MockQuerier is an autogenerated mock type for the Querier type
type MockQuerier struct {
	mock.Mock
}

type MockQuerier_Expecter struct {
	mock *mock.Mock
}

func (_m *MockQuerier) EXPECT() *MockQuerier_Expecter {
	return &MockQuerier_Expecter{mock: &_m.Mock}
}

// Count provides a mock function with given fields: ctx
func (_m *MockQuerier) Count(ctx context.Context) (int64, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Count")
	}

	var r0 int64
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (int64, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) int64); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Get(0).(int64)
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockQuerier_Count_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Count'
type MockQuerier_Count_Call struct {
	*mock.Call
}

// Count is a helper method to define mock.On call
//   - ctx context.Context
func (_e *MockQuerier_Expecter) Count(ctx interface{}) *MockQuerier_Count_Call {
	return &MockQuerier_Count_Call{Call: _e.mock.On("Count", ctx)}
}

func (_c *MockQuerier_Count_Call) Run(run func(ctx context.Context)) *MockQuerier_Count_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context))
	})
	return _c
}

func (_c *MockQuerier_Count_Call) Return(_a0 int64, _a1 error) *MockQuerier_Count_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockQuerier_Count_Call) RunAndReturn(run func(context.Context) (int64, error)) *MockQuerier_Count_Call {
	_c.Call.Return(run)
	return _c
}

// CountEnvironments provides a mock function with given fields: ctx, teamSlug
func (_m *MockQuerier) CountEnvironments(ctx context.Context, teamSlug slug.Slug) (int64, error) {
	ret := _m.Called(ctx, teamSlug)

	if len(ret) == 0 {
		panic("no return value specified for CountEnvironments")
	}

	var r0 int64
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, slug.Slug) (int64, error)); ok {
		return rf(ctx, teamSlug)
	}
	if rf, ok := ret.Get(0).(func(context.Context, slug.Slug) int64); ok {
		r0 = rf(ctx, teamSlug)
	} else {
		r0 = ret.Get(0).(int64)
	}

	if rf, ok := ret.Get(1).(func(context.Context, slug.Slug) error); ok {
		r1 = rf(ctx, teamSlug)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockQuerier_CountEnvironments_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CountEnvironments'
type MockQuerier_CountEnvironments_Call struct {
	*mock.Call
}

// CountEnvironments is a helper method to define mock.On call
//   - ctx context.Context
//   - teamSlug slug.Slug
func (_e *MockQuerier_Expecter) CountEnvironments(ctx interface{}, teamSlug interface{}) *MockQuerier_CountEnvironments_Call {
	return &MockQuerier_CountEnvironments_Call{Call: _e.mock.On("CountEnvironments", ctx, teamSlug)}
}

func (_c *MockQuerier_CountEnvironments_Call) Run(run func(ctx context.Context, teamSlug slug.Slug)) *MockQuerier_CountEnvironments_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(slug.Slug))
	})
	return _c
}

func (_c *MockQuerier_CountEnvironments_Call) Return(_a0 int64, _a1 error) *MockQuerier_CountEnvironments_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockQuerier_CountEnvironments_Call) RunAndReturn(run func(context.Context, slug.Slug) (int64, error)) *MockQuerier_CountEnvironments_Call {
	_c.Call.Return(run)
	return _c
}

// CountMembers provides a mock function with given fields: ctx, teamSlug
func (_m *MockQuerier) CountMembers(ctx context.Context, teamSlug slug.Slug) (int64, error) {
	ret := _m.Called(ctx, teamSlug)

	if len(ret) == 0 {
		panic("no return value specified for CountMembers")
	}

	var r0 int64
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, slug.Slug) (int64, error)); ok {
		return rf(ctx, teamSlug)
	}
	if rf, ok := ret.Get(0).(func(context.Context, slug.Slug) int64); ok {
		r0 = rf(ctx, teamSlug)
	} else {
		r0 = ret.Get(0).(int64)
	}

	if rf, ok := ret.Get(1).(func(context.Context, slug.Slug) error); ok {
		r1 = rf(ctx, teamSlug)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockQuerier_CountMembers_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CountMembers'
type MockQuerier_CountMembers_Call struct {
	*mock.Call
}

// CountMembers is a helper method to define mock.On call
//   - ctx context.Context
//   - teamSlug slug.Slug
func (_e *MockQuerier_Expecter) CountMembers(ctx interface{}, teamSlug interface{}) *MockQuerier_CountMembers_Call {
	return &MockQuerier_CountMembers_Call{Call: _e.mock.On("CountMembers", ctx, teamSlug)}
}

func (_c *MockQuerier_CountMembers_Call) Run(run func(ctx context.Context, teamSlug slug.Slug)) *MockQuerier_CountMembers_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(slug.Slug))
	})
	return _c
}

func (_c *MockQuerier_CountMembers_Call) Return(_a0 int64, _a1 error) *MockQuerier_CountMembers_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockQuerier_CountMembers_Call) RunAndReturn(run func(context.Context, slug.Slug) (int64, error)) *MockQuerier_CountMembers_Call {
	_c.Call.Return(run)
	return _c
}

// Delete provides a mock function with given fields: ctx, argSlug
func (_m *MockQuerier) Delete(ctx context.Context, argSlug slug.Slug) error {
	ret := _m.Called(ctx, argSlug)

	if len(ret) == 0 {
		panic("no return value specified for Delete")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, slug.Slug) error); ok {
		r0 = rf(ctx, argSlug)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockQuerier_Delete_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Delete'
type MockQuerier_Delete_Call struct {
	*mock.Call
}

// Delete is a helper method to define mock.On call
//   - ctx context.Context
//   - argSlug slug.Slug
func (_e *MockQuerier_Expecter) Delete(ctx interface{}, argSlug interface{}) *MockQuerier_Delete_Call {
	return &MockQuerier_Delete_Call{Call: _e.mock.On("Delete", ctx, argSlug)}
}

func (_c *MockQuerier_Delete_Call) Run(run func(ctx context.Context, argSlug slug.Slug)) *MockQuerier_Delete_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(slug.Slug))
	})
	return _c
}

func (_c *MockQuerier_Delete_Call) Return(_a0 error) *MockQuerier_Delete_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockQuerier_Delete_Call) RunAndReturn(run func(context.Context, slug.Slug) error) *MockQuerier_Delete_Call {
	_c.Call.Return(run)
	return _c
}

// Get provides a mock function with given fields: ctx, argSlug
func (_m *MockQuerier) Get(ctx context.Context, argSlug slug.Slug) (*Team, error) {
	ret := _m.Called(ctx, argSlug)

	if len(ret) == 0 {
		panic("no return value specified for Get")
	}

	var r0 *Team
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, slug.Slug) (*Team, error)); ok {
		return rf(ctx, argSlug)
	}
	if rf, ok := ret.Get(0).(func(context.Context, slug.Slug) *Team); ok {
		r0 = rf(ctx, argSlug)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*Team)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, slug.Slug) error); ok {
		r1 = rf(ctx, argSlug)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockQuerier_Get_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Get'
type MockQuerier_Get_Call struct {
	*mock.Call
}

// Get is a helper method to define mock.On call
//   - ctx context.Context
//   - argSlug slug.Slug
func (_e *MockQuerier_Expecter) Get(ctx interface{}, argSlug interface{}) *MockQuerier_Get_Call {
	return &MockQuerier_Get_Call{Call: _e.mock.On("Get", ctx, argSlug)}
}

func (_c *MockQuerier_Get_Call) Run(run func(ctx context.Context, argSlug slug.Slug)) *MockQuerier_Get_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(slug.Slug))
	})
	return _c
}

func (_c *MockQuerier_Get_Call) Return(_a0 *Team, _a1 error) *MockQuerier_Get_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockQuerier_Get_Call) RunAndReturn(run func(context.Context, slug.Slug) (*Team, error)) *MockQuerier_Get_Call {
	_c.Call.Return(run)
	return _c
}

// GetTeamRepositories provides a mock function with given fields: ctx, teamSlug
func (_m *MockQuerier) GetTeamRepositories(ctx context.Context, teamSlug slug.Slug) ([]string, error) {
	ret := _m.Called(ctx, teamSlug)

	if len(ret) == 0 {
		panic("no return value specified for GetTeamRepositories")
	}

	var r0 []string
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, slug.Slug) ([]string, error)); ok {
		return rf(ctx, teamSlug)
	}
	if rf, ok := ret.Get(0).(func(context.Context, slug.Slug) []string); ok {
		r0 = rf(ctx, teamSlug)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, slug.Slug) error); ok {
		r1 = rf(ctx, teamSlug)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockQuerier_GetTeamRepositories_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetTeamRepositories'
type MockQuerier_GetTeamRepositories_Call struct {
	*mock.Call
}

// GetTeamRepositories is a helper method to define mock.On call
//   - ctx context.Context
//   - teamSlug slug.Slug
func (_e *MockQuerier_Expecter) GetTeamRepositories(ctx interface{}, teamSlug interface{}) *MockQuerier_GetTeamRepositories_Call {
	return &MockQuerier_GetTeamRepositories_Call{Call: _e.mock.On("GetTeamRepositories", ctx, teamSlug)}
}

func (_c *MockQuerier_GetTeamRepositories_Call) Run(run func(ctx context.Context, teamSlug slug.Slug)) *MockQuerier_GetTeamRepositories_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(slug.Slug))
	})
	return _c
}

func (_c *MockQuerier_GetTeamRepositories_Call) Return(_a0 []string, _a1 error) *MockQuerier_GetTeamRepositories_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockQuerier_GetTeamRepositories_Call) RunAndReturn(run func(context.Context, slug.Slug) ([]string, error)) *MockQuerier_GetTeamRepositories_Call {
	_c.Call.Return(run)
	return _c
}

// IsTeamRepository provides a mock function with given fields: ctx, arg
func (_m *MockQuerier) IsTeamRepository(ctx context.Context, arg IsTeamRepositoryParams) (bool, error) {
	ret := _m.Called(ctx, arg)

	if len(ret) == 0 {
		panic("no return value specified for IsTeamRepository")
	}

	var r0 bool
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, IsTeamRepositoryParams) (bool, error)); ok {
		return rf(ctx, arg)
	}
	if rf, ok := ret.Get(0).(func(context.Context, IsTeamRepositoryParams) bool); ok {
		r0 = rf(ctx, arg)
	} else {
		r0 = ret.Get(0).(bool)
	}

	if rf, ok := ret.Get(1).(func(context.Context, IsTeamRepositoryParams) error); ok {
		r1 = rf(ctx, arg)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockQuerier_IsTeamRepository_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'IsTeamRepository'
type MockQuerier_IsTeamRepository_Call struct {
	*mock.Call
}

// IsTeamRepository is a helper method to define mock.On call
//   - ctx context.Context
//   - arg IsTeamRepositoryParams
func (_e *MockQuerier_Expecter) IsTeamRepository(ctx interface{}, arg interface{}) *MockQuerier_IsTeamRepository_Call {
	return &MockQuerier_IsTeamRepository_Call{Call: _e.mock.On("IsTeamRepository", ctx, arg)}
}

func (_c *MockQuerier_IsTeamRepository_Call) Run(run func(ctx context.Context, arg IsTeamRepositoryParams)) *MockQuerier_IsTeamRepository_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(IsTeamRepositoryParams))
	})
	return _c
}

func (_c *MockQuerier_IsTeamRepository_Call) Return(_a0 bool, _a1 error) *MockQuerier_IsTeamRepository_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockQuerier_IsTeamRepository_Call) RunAndReturn(run func(context.Context, IsTeamRepositoryParams) (bool, error)) *MockQuerier_IsTeamRepository_Call {
	_c.Call.Return(run)
	return _c
}

// List provides a mock function with given fields: ctx, arg
func (_m *MockQuerier) List(ctx context.Context, arg ListParams) ([]*Team, error) {
	ret := _m.Called(ctx, arg)

	if len(ret) == 0 {
		panic("no return value specified for List")
	}

	var r0 []*Team
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, ListParams) ([]*Team, error)); ok {
		return rf(ctx, arg)
	}
	if rf, ok := ret.Get(0).(func(context.Context, ListParams) []*Team); ok {
		r0 = rf(ctx, arg)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*Team)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, ListParams) error); ok {
		r1 = rf(ctx, arg)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockQuerier_List_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'List'
type MockQuerier_List_Call struct {
	*mock.Call
}

// List is a helper method to define mock.On call
//   - ctx context.Context
//   - arg ListParams
func (_e *MockQuerier_Expecter) List(ctx interface{}, arg interface{}) *MockQuerier_List_Call {
	return &MockQuerier_List_Call{Call: _e.mock.On("List", ctx, arg)}
}

func (_c *MockQuerier_List_Call) Run(run func(ctx context.Context, arg ListParams)) *MockQuerier_List_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(ListParams))
	})
	return _c
}

func (_c *MockQuerier_List_Call) Return(_a0 []*Team, _a1 error) *MockQuerier_List_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockQuerier_List_Call) RunAndReturn(run func(context.Context, ListParams) ([]*Team, error)) *MockQuerier_List_Call {
	_c.Call.Return(run)
	return _c
}

// ListEnvironments provides a mock function with given fields: ctx, arg
func (_m *MockQuerier) ListEnvironments(ctx context.Context, arg ListEnvironmentsParams) ([]*TeamAllEnvironment, error) {
	ret := _m.Called(ctx, arg)

	if len(ret) == 0 {
		panic("no return value specified for ListEnvironments")
	}

	var r0 []*TeamAllEnvironment
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, ListEnvironmentsParams) ([]*TeamAllEnvironment, error)); ok {
		return rf(ctx, arg)
	}
	if rf, ok := ret.Get(0).(func(context.Context, ListEnvironmentsParams) []*TeamAllEnvironment); ok {
		r0 = rf(ctx, arg)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*TeamAllEnvironment)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, ListEnvironmentsParams) error); ok {
		r1 = rf(ctx, arg)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockQuerier_ListEnvironments_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ListEnvironments'
type MockQuerier_ListEnvironments_Call struct {
	*mock.Call
}

// ListEnvironments is a helper method to define mock.On call
//   - ctx context.Context
//   - arg ListEnvironmentsParams
func (_e *MockQuerier_Expecter) ListEnvironments(ctx interface{}, arg interface{}) *MockQuerier_ListEnvironments_Call {
	return &MockQuerier_ListEnvironments_Call{Call: _e.mock.On("ListEnvironments", ctx, arg)}
}

func (_c *MockQuerier_ListEnvironments_Call) Run(run func(ctx context.Context, arg ListEnvironmentsParams)) *MockQuerier_ListEnvironments_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(ListEnvironmentsParams))
	})
	return _c
}

func (_c *MockQuerier_ListEnvironments_Call) Return(_a0 []*TeamAllEnvironment, _a1 error) *MockQuerier_ListEnvironments_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockQuerier_ListEnvironments_Call) RunAndReturn(run func(context.Context, ListEnvironmentsParams) ([]*TeamAllEnvironment, error)) *MockQuerier_ListEnvironments_Call {
	_c.Call.Return(run)
	return _c
}

// ListMembers provides a mock function with given fields: ctx, arg
func (_m *MockQuerier) ListMembers(ctx context.Context, arg ListMembersParams) ([]*User, error) {
	ret := _m.Called(ctx, arg)

	if len(ret) == 0 {
		panic("no return value specified for ListMembers")
	}

	var r0 []*User
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, ListMembersParams) ([]*User, error)); ok {
		return rf(ctx, arg)
	}
	if rf, ok := ret.Get(0).(func(context.Context, ListMembersParams) []*User); ok {
		r0 = rf(ctx, arg)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*User)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, ListMembersParams) error); ok {
		r1 = rf(ctx, arg)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockQuerier_ListMembers_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ListMembers'
type MockQuerier_ListMembers_Call struct {
	*mock.Call
}

// ListMembers is a helper method to define mock.On call
//   - ctx context.Context
//   - arg ListMembersParams
func (_e *MockQuerier_Expecter) ListMembers(ctx interface{}, arg interface{}) *MockQuerier_ListMembers_Call {
	return &MockQuerier_ListMembers_Call{Call: _e.mock.On("ListMembers", ctx, arg)}
}

func (_c *MockQuerier_ListMembers_Call) Run(run func(ctx context.Context, arg ListMembersParams)) *MockQuerier_ListMembers_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(ListMembersParams))
	})
	return _c
}

func (_c *MockQuerier_ListMembers_Call) Return(_a0 []*User, _a1 error) *MockQuerier_ListMembers_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockQuerier_ListMembers_Call) RunAndReturn(run func(context.Context, ListMembersParams) ([]*User, error)) *MockQuerier_ListMembers_Call {
	_c.Call.Return(run)
	return _c
}

// SetLastSuccessfulSync provides a mock function with given fields: ctx, argSlug
func (_m *MockQuerier) SetLastSuccessfulSync(ctx context.Context, argSlug slug.Slug) error {
	ret := _m.Called(ctx, argSlug)

	if len(ret) == 0 {
		panic("no return value specified for SetLastSuccessfulSync")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, slug.Slug) error); ok {
		r0 = rf(ctx, argSlug)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockQuerier_SetLastSuccessfulSync_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SetLastSuccessfulSync'
type MockQuerier_SetLastSuccessfulSync_Call struct {
	*mock.Call
}

// SetLastSuccessfulSync is a helper method to define mock.On call
//   - ctx context.Context
//   - argSlug slug.Slug
func (_e *MockQuerier_Expecter) SetLastSuccessfulSync(ctx interface{}, argSlug interface{}) *MockQuerier_SetLastSuccessfulSync_Call {
	return &MockQuerier_SetLastSuccessfulSync_Call{Call: _e.mock.On("SetLastSuccessfulSync", ctx, argSlug)}
}

func (_c *MockQuerier_SetLastSuccessfulSync_Call) Run(run func(ctx context.Context, argSlug slug.Slug)) *MockQuerier_SetLastSuccessfulSync_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(slug.Slug))
	})
	return _c
}

func (_c *MockQuerier_SetLastSuccessfulSync_Call) Return(_a0 error) *MockQuerier_SetLastSuccessfulSync_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockQuerier_SetLastSuccessfulSync_Call) RunAndReturn(run func(context.Context, slug.Slug) error) *MockQuerier_SetLastSuccessfulSync_Call {
	_c.Call.Return(run)
	return _c
}

// UpdateExternalReferences provides a mock function with given fields: ctx, arg
func (_m *MockQuerier) UpdateExternalReferences(ctx context.Context, arg UpdateExternalReferencesParams) error {
	ret := _m.Called(ctx, arg)

	if len(ret) == 0 {
		panic("no return value specified for UpdateExternalReferences")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, UpdateExternalReferencesParams) error); ok {
		r0 = rf(ctx, arg)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockQuerier_UpdateExternalReferences_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'UpdateExternalReferences'
type MockQuerier_UpdateExternalReferences_Call struct {
	*mock.Call
}

// UpdateExternalReferences is a helper method to define mock.On call
//   - ctx context.Context
//   - arg UpdateExternalReferencesParams
func (_e *MockQuerier_Expecter) UpdateExternalReferences(ctx interface{}, arg interface{}) *MockQuerier_UpdateExternalReferences_Call {
	return &MockQuerier_UpdateExternalReferences_Call{Call: _e.mock.On("UpdateExternalReferences", ctx, arg)}
}

func (_c *MockQuerier_UpdateExternalReferences_Call) Run(run func(ctx context.Context, arg UpdateExternalReferencesParams)) *MockQuerier_UpdateExternalReferences_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(UpdateExternalReferencesParams))
	})
	return _c
}

func (_c *MockQuerier_UpdateExternalReferences_Call) Return(_a0 error) *MockQuerier_UpdateExternalReferences_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockQuerier_UpdateExternalReferences_Call) RunAndReturn(run func(context.Context, UpdateExternalReferencesParams) error) *MockQuerier_UpdateExternalReferences_Call {
	_c.Call.Return(run)
	return _c
}

// UpsertEnvironment provides a mock function with given fields: ctx, arg
func (_m *MockQuerier) UpsertEnvironment(ctx context.Context, arg UpsertEnvironmentParams) error {
	ret := _m.Called(ctx, arg)

	if len(ret) == 0 {
		panic("no return value specified for UpsertEnvironment")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, UpsertEnvironmentParams) error); ok {
		r0 = rf(ctx, arg)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockQuerier_UpsertEnvironment_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'UpsertEnvironment'
type MockQuerier_UpsertEnvironment_Call struct {
	*mock.Call
}

// UpsertEnvironment is a helper method to define mock.On call
//   - ctx context.Context
//   - arg UpsertEnvironmentParams
func (_e *MockQuerier_Expecter) UpsertEnvironment(ctx interface{}, arg interface{}) *MockQuerier_UpsertEnvironment_Call {
	return &MockQuerier_UpsertEnvironment_Call{Call: _e.mock.On("UpsertEnvironment", ctx, arg)}
}

func (_c *MockQuerier_UpsertEnvironment_Call) Run(run func(ctx context.Context, arg UpsertEnvironmentParams)) *MockQuerier_UpsertEnvironment_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(UpsertEnvironmentParams))
	})
	return _c
}

func (_c *MockQuerier_UpsertEnvironment_Call) Return(_a0 error) *MockQuerier_UpsertEnvironment_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockQuerier_UpsertEnvironment_Call) RunAndReturn(run func(context.Context, UpsertEnvironmentParams) error) *MockQuerier_UpsertEnvironment_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockQuerier creates a new instance of MockQuerier. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockQuerier(t interface {
	mock.TestingT
	Cleanup(func())
},
) *MockQuerier {
	mock := &MockQuerier{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}