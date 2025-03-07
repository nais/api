// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v5.29.3
// source: reconcilers.proto

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
	Reconcilers_Register_FullMethodName                     = "/nais.api.protobuf.Reconcilers/Register"
	Reconcilers_Get_FullMethodName                          = "/nais.api.protobuf.Reconcilers/Get"
	Reconcilers_List_FullMethodName                         = "/nais.api.protobuf.Reconcilers/List"
	Reconcilers_Config_FullMethodName                       = "/nais.api.protobuf.Reconcilers/Config"
	Reconcilers_SetReconcilerErrorForTeam_FullMethodName    = "/nais.api.protobuf.Reconcilers/SetReconcilerErrorForTeam"
	Reconcilers_RemoveReconcilerErrorForTeam_FullMethodName = "/nais.api.protobuf.Reconcilers/RemoveReconcilerErrorForTeam"
	Reconcilers_SuccessfulTeamSync_FullMethodName           = "/nais.api.protobuf.Reconcilers/SuccessfulTeamSync"
	Reconcilers_SaveState_FullMethodName                    = "/nais.api.protobuf.Reconcilers/SaveState"
	Reconcilers_State_FullMethodName                        = "/nais.api.protobuf.Reconcilers/State"
	Reconcilers_DeleteState_FullMethodName                  = "/nais.api.protobuf.Reconcilers/DeleteState"
)

// ReconcilersClient is the client API for Reconcilers service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ReconcilersClient interface {
	Register(ctx context.Context, in *RegisterReconcilerRequest, opts ...grpc.CallOption) (*RegisterReconcilerResponse, error)
	Get(ctx context.Context, in *GetReconcilerRequest, opts ...grpc.CallOption) (*GetReconcilerResponse, error)
	List(ctx context.Context, in *ListReconcilersRequest, opts ...grpc.CallOption) (*ListReconcilersResponse, error)
	Config(ctx context.Context, in *ConfigReconcilerRequest, opts ...grpc.CallOption) (*ConfigReconcilerResponse, error)
	SetReconcilerErrorForTeam(ctx context.Context, in *SetReconcilerErrorForTeamRequest, opts ...grpc.CallOption) (*SetReconcilerErrorForTeamResponse, error)
	RemoveReconcilerErrorForTeam(ctx context.Context, in *RemoveReconcilerErrorForTeamRequest, opts ...grpc.CallOption) (*RemoveReconcilerErrorForTeamResponse, error)
	SuccessfulTeamSync(ctx context.Context, in *SuccessfulTeamSyncRequest, opts ...grpc.CallOption) (*SuccessfulTeamSyncResponse, error)
	SaveState(ctx context.Context, in *SaveReconcilerStateRequest, opts ...grpc.CallOption) (*SaveReconcilerStateResponse, error)
	State(ctx context.Context, in *GetReconcilerStateRequest, opts ...grpc.CallOption) (*GetReconcilerStateResponse, error)
	DeleteState(ctx context.Context, in *DeleteReconcilerStateRequest, opts ...grpc.CallOption) (*DeleteReconcilerStateResponse, error)
}

type reconcilersClient struct {
	cc grpc.ClientConnInterface
}

func NewReconcilersClient(cc grpc.ClientConnInterface) ReconcilersClient {
	return &reconcilersClient{cc}
}

func (c *reconcilersClient) Register(ctx context.Context, in *RegisterReconcilerRequest, opts ...grpc.CallOption) (*RegisterReconcilerResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(RegisterReconcilerResponse)
	err := c.cc.Invoke(ctx, Reconcilers_Register_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *reconcilersClient) Get(ctx context.Context, in *GetReconcilerRequest, opts ...grpc.CallOption) (*GetReconcilerResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetReconcilerResponse)
	err := c.cc.Invoke(ctx, Reconcilers_Get_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *reconcilersClient) List(ctx context.Context, in *ListReconcilersRequest, opts ...grpc.CallOption) (*ListReconcilersResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ListReconcilersResponse)
	err := c.cc.Invoke(ctx, Reconcilers_List_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *reconcilersClient) Config(ctx context.Context, in *ConfigReconcilerRequest, opts ...grpc.CallOption) (*ConfigReconcilerResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ConfigReconcilerResponse)
	err := c.cc.Invoke(ctx, Reconcilers_Config_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *reconcilersClient) SetReconcilerErrorForTeam(ctx context.Context, in *SetReconcilerErrorForTeamRequest, opts ...grpc.CallOption) (*SetReconcilerErrorForTeamResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(SetReconcilerErrorForTeamResponse)
	err := c.cc.Invoke(ctx, Reconcilers_SetReconcilerErrorForTeam_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *reconcilersClient) RemoveReconcilerErrorForTeam(ctx context.Context, in *RemoveReconcilerErrorForTeamRequest, opts ...grpc.CallOption) (*RemoveReconcilerErrorForTeamResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(RemoveReconcilerErrorForTeamResponse)
	err := c.cc.Invoke(ctx, Reconcilers_RemoveReconcilerErrorForTeam_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *reconcilersClient) SuccessfulTeamSync(ctx context.Context, in *SuccessfulTeamSyncRequest, opts ...grpc.CallOption) (*SuccessfulTeamSyncResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(SuccessfulTeamSyncResponse)
	err := c.cc.Invoke(ctx, Reconcilers_SuccessfulTeamSync_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *reconcilersClient) SaveState(ctx context.Context, in *SaveReconcilerStateRequest, opts ...grpc.CallOption) (*SaveReconcilerStateResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(SaveReconcilerStateResponse)
	err := c.cc.Invoke(ctx, Reconcilers_SaveState_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *reconcilersClient) State(ctx context.Context, in *GetReconcilerStateRequest, opts ...grpc.CallOption) (*GetReconcilerStateResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetReconcilerStateResponse)
	err := c.cc.Invoke(ctx, Reconcilers_State_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *reconcilersClient) DeleteState(ctx context.Context, in *DeleteReconcilerStateRequest, opts ...grpc.CallOption) (*DeleteReconcilerStateResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DeleteReconcilerStateResponse)
	err := c.cc.Invoke(ctx, Reconcilers_DeleteState_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ReconcilersServer is the server API for Reconcilers service.
// All implementations must embed UnimplementedReconcilersServer
// for forward compatibility.
type ReconcilersServer interface {
	Register(context.Context, *RegisterReconcilerRequest) (*RegisterReconcilerResponse, error)
	Get(context.Context, *GetReconcilerRequest) (*GetReconcilerResponse, error)
	List(context.Context, *ListReconcilersRequest) (*ListReconcilersResponse, error)
	Config(context.Context, *ConfigReconcilerRequest) (*ConfigReconcilerResponse, error)
	SetReconcilerErrorForTeam(context.Context, *SetReconcilerErrorForTeamRequest) (*SetReconcilerErrorForTeamResponse, error)
	RemoveReconcilerErrorForTeam(context.Context, *RemoveReconcilerErrorForTeamRequest) (*RemoveReconcilerErrorForTeamResponse, error)
	SuccessfulTeamSync(context.Context, *SuccessfulTeamSyncRequest) (*SuccessfulTeamSyncResponse, error)
	SaveState(context.Context, *SaveReconcilerStateRequest) (*SaveReconcilerStateResponse, error)
	State(context.Context, *GetReconcilerStateRequest) (*GetReconcilerStateResponse, error)
	DeleteState(context.Context, *DeleteReconcilerStateRequest) (*DeleteReconcilerStateResponse, error)
	mustEmbedUnimplementedReconcilersServer()
}

// UnimplementedReconcilersServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedReconcilersServer struct{}

func (UnimplementedReconcilersServer) Register(context.Context, *RegisterReconcilerRequest) (*RegisterReconcilerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Register not implemented")
}
func (UnimplementedReconcilersServer) Get(context.Context, *GetReconcilerRequest) (*GetReconcilerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Get not implemented")
}
func (UnimplementedReconcilersServer) List(context.Context, *ListReconcilersRequest) (*ListReconcilersResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method List not implemented")
}
func (UnimplementedReconcilersServer) Config(context.Context, *ConfigReconcilerRequest) (*ConfigReconcilerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Config not implemented")
}
func (UnimplementedReconcilersServer) SetReconcilerErrorForTeam(context.Context, *SetReconcilerErrorForTeamRequest) (*SetReconcilerErrorForTeamResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetReconcilerErrorForTeam not implemented")
}
func (UnimplementedReconcilersServer) RemoveReconcilerErrorForTeam(context.Context, *RemoveReconcilerErrorForTeamRequest) (*RemoveReconcilerErrorForTeamResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RemoveReconcilerErrorForTeam not implemented")
}
func (UnimplementedReconcilersServer) SuccessfulTeamSync(context.Context, *SuccessfulTeamSyncRequest) (*SuccessfulTeamSyncResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SuccessfulTeamSync not implemented")
}
func (UnimplementedReconcilersServer) SaveState(context.Context, *SaveReconcilerStateRequest) (*SaveReconcilerStateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SaveState not implemented")
}
func (UnimplementedReconcilersServer) State(context.Context, *GetReconcilerStateRequest) (*GetReconcilerStateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method State not implemented")
}
func (UnimplementedReconcilersServer) DeleteState(context.Context, *DeleteReconcilerStateRequest) (*DeleteReconcilerStateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteState not implemented")
}
func (UnimplementedReconcilersServer) mustEmbedUnimplementedReconcilersServer() {}
func (UnimplementedReconcilersServer) testEmbeddedByValue()                     {}

// UnsafeReconcilersServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ReconcilersServer will
// result in compilation errors.
type UnsafeReconcilersServer interface {
	mustEmbedUnimplementedReconcilersServer()
}

func RegisterReconcilersServer(s grpc.ServiceRegistrar, srv ReconcilersServer) {
	// If the following call pancis, it indicates UnimplementedReconcilersServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&Reconcilers_ServiceDesc, srv)
}

func _Reconcilers_Register_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RegisterReconcilerRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ReconcilersServer).Register(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Reconcilers_Register_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ReconcilersServer).Register(ctx, req.(*RegisterReconcilerRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Reconcilers_Get_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetReconcilerRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ReconcilersServer).Get(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Reconcilers_Get_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ReconcilersServer).Get(ctx, req.(*GetReconcilerRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Reconcilers_List_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListReconcilersRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ReconcilersServer).List(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Reconcilers_List_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ReconcilersServer).List(ctx, req.(*ListReconcilersRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Reconcilers_Config_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ConfigReconcilerRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ReconcilersServer).Config(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Reconcilers_Config_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ReconcilersServer).Config(ctx, req.(*ConfigReconcilerRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Reconcilers_SetReconcilerErrorForTeam_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SetReconcilerErrorForTeamRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ReconcilersServer).SetReconcilerErrorForTeam(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Reconcilers_SetReconcilerErrorForTeam_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ReconcilersServer).SetReconcilerErrorForTeam(ctx, req.(*SetReconcilerErrorForTeamRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Reconcilers_RemoveReconcilerErrorForTeam_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RemoveReconcilerErrorForTeamRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ReconcilersServer).RemoveReconcilerErrorForTeam(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Reconcilers_RemoveReconcilerErrorForTeam_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ReconcilersServer).RemoveReconcilerErrorForTeam(ctx, req.(*RemoveReconcilerErrorForTeamRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Reconcilers_SuccessfulTeamSync_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SuccessfulTeamSyncRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ReconcilersServer).SuccessfulTeamSync(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Reconcilers_SuccessfulTeamSync_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ReconcilersServer).SuccessfulTeamSync(ctx, req.(*SuccessfulTeamSyncRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Reconcilers_SaveState_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SaveReconcilerStateRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ReconcilersServer).SaveState(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Reconcilers_SaveState_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ReconcilersServer).SaveState(ctx, req.(*SaveReconcilerStateRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Reconcilers_State_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetReconcilerStateRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ReconcilersServer).State(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Reconcilers_State_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ReconcilersServer).State(ctx, req.(*GetReconcilerStateRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Reconcilers_DeleteState_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteReconcilerStateRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ReconcilersServer).DeleteState(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Reconcilers_DeleteState_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ReconcilersServer).DeleteState(ctx, req.(*DeleteReconcilerStateRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Reconcilers_ServiceDesc is the grpc.ServiceDesc for Reconcilers service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Reconcilers_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "nais.api.protobuf.Reconcilers",
	HandlerType: (*ReconcilersServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Register",
			Handler:    _Reconcilers_Register_Handler,
		},
		{
			MethodName: "Get",
			Handler:    _Reconcilers_Get_Handler,
		},
		{
			MethodName: "List",
			Handler:    _Reconcilers_List_Handler,
		},
		{
			MethodName: "Config",
			Handler:    _Reconcilers_Config_Handler,
		},
		{
			MethodName: "SetReconcilerErrorForTeam",
			Handler:    _Reconcilers_SetReconcilerErrorForTeam_Handler,
		},
		{
			MethodName: "RemoveReconcilerErrorForTeam",
			Handler:    _Reconcilers_RemoveReconcilerErrorForTeam_Handler,
		},
		{
			MethodName: "SuccessfulTeamSync",
			Handler:    _Reconcilers_SuccessfulTeamSync_Handler,
		},
		{
			MethodName: "SaveState",
			Handler:    _Reconcilers_SaveState_Handler,
		},
		{
			MethodName: "State",
			Handler:    _Reconcilers_State_Handler,
		},
		{
			MethodName: "DeleteState",
			Handler:    _Reconcilers_DeleteState_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "reconcilers.proto",
}
