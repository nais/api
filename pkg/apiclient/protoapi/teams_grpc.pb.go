// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v5.29.3
// source: teams.proto

package protoapi

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	Teams_ListAuthorizedRepositories_FullMethodName           = "/nais.api.protobuf.Teams/ListAuthorizedRepositories"
	Teams_Get_FullMethodName                                  = "/nais.api.protobuf.Teams/Get"
	Teams_List_FullMethodName                                 = "/nais.api.protobuf.Teams/List"
	Teams_Members_FullMethodName                              = "/nais.api.protobuf.Teams/Members"
	Teams_Environments_FullMethodName                         = "/nais.api.protobuf.Teams/Environments"
	Teams_SetTeamExternalReferences_FullMethodName            = "/nais.api.protobuf.Teams/SetTeamExternalReferences"
	Teams_SetTeamEnvironmentExternalReferences_FullMethodName = "/nais.api.protobuf.Teams/SetTeamEnvironmentExternalReferences"
	Teams_Delete_FullMethodName                               = "/nais.api.protobuf.Teams/Delete"
	Teams_IsRepositoryAuthorized_FullMethodName               = "/nais.api.protobuf.Teams/IsRepositoryAuthorized"
)

// TeamsClient is the client API for Teams service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type TeamsClient interface {
	ListAuthorizedRepositories(ctx context.Context, in *ListAuthorizedRepositoriesRequest, opts ...grpc.CallOption) (*ListAuthorizedRepositoriesResponse, error)
	Get(ctx context.Context, in *GetTeamRequest, opts ...grpc.CallOption) (*GetTeamResponse, error)
	List(ctx context.Context, in *ListTeamsRequest, opts ...grpc.CallOption) (*ListTeamsResponse, error)
	Members(ctx context.Context, in *ListTeamMembersRequest, opts ...grpc.CallOption) (*ListTeamMembersResponse, error)
	Environments(ctx context.Context, in *ListTeamEnvironmentsRequest, opts ...grpc.CallOption) (*ListTeamEnvironmentsResponse, error)
	SetTeamExternalReferences(ctx context.Context, in *SetTeamExternalReferencesRequest, opts ...grpc.CallOption) (*SetTeamExternalReferencesResponse, error)
	SetTeamEnvironmentExternalReferences(ctx context.Context, in *SetTeamEnvironmentExternalReferencesRequest, opts ...grpc.CallOption) (*SetTeamEnvironmentExternalReferencesResponse, error)
	Delete(ctx context.Context, in *DeleteTeamRequest, opts ...grpc.CallOption) (*DeleteTeamResponse, error)
	IsRepositoryAuthorized(ctx context.Context, in *IsRepositoryAuthorizedRequest, opts ...grpc.CallOption) (*IsRepositoryAuthorizedResponse, error)
}

type teamsClient struct {
	cc grpc.ClientConnInterface
}

func NewTeamsClient(cc grpc.ClientConnInterface) TeamsClient {
	return &teamsClient{cc}
}

func (c *teamsClient) ListAuthorizedRepositories(ctx context.Context, in *ListAuthorizedRepositoriesRequest, opts ...grpc.CallOption) (*ListAuthorizedRepositoriesResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ListAuthorizedRepositoriesResponse)
	err := c.cc.Invoke(ctx, Teams_ListAuthorizedRepositories_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *teamsClient) Get(ctx context.Context, in *GetTeamRequest, opts ...grpc.CallOption) (*GetTeamResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetTeamResponse)
	err := c.cc.Invoke(ctx, Teams_Get_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *teamsClient) List(ctx context.Context, in *ListTeamsRequest, opts ...grpc.CallOption) (*ListTeamsResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ListTeamsResponse)
	err := c.cc.Invoke(ctx, Teams_List_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *teamsClient) Members(ctx context.Context, in *ListTeamMembersRequest, opts ...grpc.CallOption) (*ListTeamMembersResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ListTeamMembersResponse)
	err := c.cc.Invoke(ctx, Teams_Members_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *teamsClient) Environments(ctx context.Context, in *ListTeamEnvironmentsRequest, opts ...grpc.CallOption) (*ListTeamEnvironmentsResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ListTeamEnvironmentsResponse)
	err := c.cc.Invoke(ctx, Teams_Environments_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *teamsClient) SetTeamExternalReferences(ctx context.Context, in *SetTeamExternalReferencesRequest, opts ...grpc.CallOption) (*SetTeamExternalReferencesResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(SetTeamExternalReferencesResponse)
	err := c.cc.Invoke(ctx, Teams_SetTeamExternalReferences_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *teamsClient) SetTeamEnvironmentExternalReferences(ctx context.Context, in *SetTeamEnvironmentExternalReferencesRequest, opts ...grpc.CallOption) (*SetTeamEnvironmentExternalReferencesResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(SetTeamEnvironmentExternalReferencesResponse)
	err := c.cc.Invoke(ctx, Teams_SetTeamEnvironmentExternalReferences_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *teamsClient) Delete(ctx context.Context, in *DeleteTeamRequest, opts ...grpc.CallOption) (*DeleteTeamResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DeleteTeamResponse)
	err := c.cc.Invoke(ctx, Teams_Delete_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *teamsClient) IsRepositoryAuthorized(ctx context.Context, in *IsRepositoryAuthorizedRequest, opts ...grpc.CallOption) (*IsRepositoryAuthorizedResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(IsRepositoryAuthorizedResponse)
	err := c.cc.Invoke(ctx, Teams_IsRepositoryAuthorized_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// TeamsServer is the server API for Teams service.
// All implementations must embed UnimplementedTeamsServer
// for forward compatibility.
type TeamsServer interface {
	ListAuthorizedRepositories(context.Context, *ListAuthorizedRepositoriesRequest) (*ListAuthorizedRepositoriesResponse, error)
	Get(context.Context, *GetTeamRequest) (*GetTeamResponse, error)
	List(context.Context, *ListTeamsRequest) (*ListTeamsResponse, error)
	Members(context.Context, *ListTeamMembersRequest) (*ListTeamMembersResponse, error)
	Environments(context.Context, *ListTeamEnvironmentsRequest) (*ListTeamEnvironmentsResponse, error)
	SetTeamExternalReferences(context.Context, *SetTeamExternalReferencesRequest) (*SetTeamExternalReferencesResponse, error)
	SetTeamEnvironmentExternalReferences(context.Context, *SetTeamEnvironmentExternalReferencesRequest) (*SetTeamEnvironmentExternalReferencesResponse, error)
	Delete(context.Context, *DeleteTeamRequest) (*DeleteTeamResponse, error)
	IsRepositoryAuthorized(context.Context, *IsRepositoryAuthorizedRequest) (*IsRepositoryAuthorizedResponse, error)
	mustEmbedUnimplementedTeamsServer()
}

// UnimplementedTeamsServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedTeamsServer struct{}

func (UnimplementedTeamsServer) ListAuthorizedRepositories(context.Context, *ListAuthorizedRepositoriesRequest) (*ListAuthorizedRepositoriesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListAuthorizedRepositories not implemented")
}
func (UnimplementedTeamsServer) Get(context.Context, *GetTeamRequest) (*GetTeamResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Get not implemented")
}
func (UnimplementedTeamsServer) List(context.Context, *ListTeamsRequest) (*ListTeamsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method List not implemented")
}
func (UnimplementedTeamsServer) Members(context.Context, *ListTeamMembersRequest) (*ListTeamMembersResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Members not implemented")
}
func (UnimplementedTeamsServer) Environments(context.Context, *ListTeamEnvironmentsRequest) (*ListTeamEnvironmentsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Environments not implemented")
}
func (UnimplementedTeamsServer) SetTeamExternalReferences(context.Context, *SetTeamExternalReferencesRequest) (*SetTeamExternalReferencesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetTeamExternalReferences not implemented")
}
func (UnimplementedTeamsServer) SetTeamEnvironmentExternalReferences(context.Context, *SetTeamEnvironmentExternalReferencesRequest) (*SetTeamEnvironmentExternalReferencesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetTeamEnvironmentExternalReferences not implemented")
}
func (UnimplementedTeamsServer) Delete(context.Context, *DeleteTeamRequest) (*DeleteTeamResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Delete not implemented")
}
func (UnimplementedTeamsServer) IsRepositoryAuthorized(context.Context, *IsRepositoryAuthorizedRequest) (*IsRepositoryAuthorizedResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method IsRepositoryAuthorized not implemented")
}
func (UnimplementedTeamsServer) mustEmbedUnimplementedTeamsServer() {}
func (UnimplementedTeamsServer) testEmbeddedByValue()               {}

// UnsafeTeamsServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to TeamsServer will
// result in compilation errors.
type UnsafeTeamsServer interface {
	mustEmbedUnimplementedTeamsServer()
}

func RegisterTeamsServer(s grpc.ServiceRegistrar, srv TeamsServer) {
	// If the following call pancis, it indicates UnimplementedTeamsServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&Teams_ServiceDesc, srv)
}

func _Teams_ListAuthorizedRepositories_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListAuthorizedRepositoriesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TeamsServer).ListAuthorizedRepositories(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Teams_ListAuthorizedRepositories_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TeamsServer).ListAuthorizedRepositories(ctx, req.(*ListAuthorizedRepositoriesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Teams_Get_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetTeamRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TeamsServer).Get(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Teams_Get_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TeamsServer).Get(ctx, req.(*GetTeamRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Teams_List_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListTeamsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TeamsServer).List(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Teams_List_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TeamsServer).List(ctx, req.(*ListTeamsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Teams_Members_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListTeamMembersRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TeamsServer).Members(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Teams_Members_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TeamsServer).Members(ctx, req.(*ListTeamMembersRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Teams_Environments_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListTeamEnvironmentsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TeamsServer).Environments(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Teams_Environments_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TeamsServer).Environments(ctx, req.(*ListTeamEnvironmentsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Teams_SetTeamExternalReferences_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SetTeamExternalReferencesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TeamsServer).SetTeamExternalReferences(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Teams_SetTeamExternalReferences_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TeamsServer).SetTeamExternalReferences(ctx, req.(*SetTeamExternalReferencesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Teams_SetTeamEnvironmentExternalReferences_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SetTeamEnvironmentExternalReferencesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TeamsServer).SetTeamEnvironmentExternalReferences(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Teams_SetTeamEnvironmentExternalReferences_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TeamsServer).SetTeamEnvironmentExternalReferences(ctx, req.(*SetTeamEnvironmentExternalReferencesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Teams_Delete_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteTeamRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TeamsServer).Delete(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Teams_Delete_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TeamsServer).Delete(ctx, req.(*DeleteTeamRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Teams_IsRepositoryAuthorized_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(IsRepositoryAuthorizedRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TeamsServer).IsRepositoryAuthorized(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Teams_IsRepositoryAuthorized_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TeamsServer).IsRepositoryAuthorized(ctx, req.(*IsRepositoryAuthorizedRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Teams_ServiceDesc is the grpc.ServiceDesc for Teams service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Teams_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "nais.api.protobuf.Teams",
	HandlerType: (*TeamsServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ListAuthorizedRepositories",
			Handler:    _Teams_ListAuthorizedRepositories_Handler,
		},
		{
			MethodName: "Get",
			Handler:    _Teams_Get_Handler,
		},
		{
			MethodName: "List",
			Handler:    _Teams_List_Handler,
		},
		{
			MethodName: "Members",
			Handler:    _Teams_Members_Handler,
		},
		{
			MethodName: "Environments",
			Handler:    _Teams_Environments_Handler,
		},
		{
			MethodName: "SetTeamExternalReferences",
			Handler:    _Teams_SetTeamExternalReferences_Handler,
		},
		{
			MethodName: "SetTeamEnvironmentExternalReferences",
			Handler:    _Teams_SetTeamEnvironmentExternalReferences_Handler,
		},
		{
			MethodName: "Delete",
			Handler:    _Teams_Delete_Handler,
		},
		{
			MethodName: "IsRepositoryAuthorized",
			Handler:    _Teams_IsRepositoryAuthorized_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "teams.proto",
}
