// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v4.25.2
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
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	Teams_Get_FullMethodName                 = "/Teams/Get"
	Teams_List_FullMethodName                = "/Teams/List"
	Teams_Members_FullMethodName             = "/Teams/Members"
	Teams_SlackAlertsChannels_FullMethodName = "/Teams/SlackAlertsChannels"
)

// TeamsClient is the client API for Teams service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type TeamsClient interface {
	Get(ctx context.Context, in *GetTeamRequest, opts ...grpc.CallOption) (*GetTeamResponse, error)
	List(ctx context.Context, in *ListTeamsRequest, opts ...grpc.CallOption) (*ListTeamsResponse, error)
	Members(ctx context.Context, in *ListTeamMembersRequest, opts ...grpc.CallOption) (*ListTeamMembersResponse, error)
	SlackAlertsChannels(ctx context.Context, in *SlackAlertsChannelsRequest, opts ...grpc.CallOption) (*SlackAlertsChannelsResponse, error)
}

type teamsClient struct {
	cc grpc.ClientConnInterface
}

func NewTeamsClient(cc grpc.ClientConnInterface) TeamsClient {
	return &teamsClient{cc}
}

func (c *teamsClient) Get(ctx context.Context, in *GetTeamRequest, opts ...grpc.CallOption) (*GetTeamResponse, error) {
	out := new(GetTeamResponse)
	err := c.cc.Invoke(ctx, Teams_Get_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *teamsClient) List(ctx context.Context, in *ListTeamsRequest, opts ...grpc.CallOption) (*ListTeamsResponse, error) {
	out := new(ListTeamsResponse)
	err := c.cc.Invoke(ctx, Teams_List_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *teamsClient) Members(ctx context.Context, in *ListTeamMembersRequest, opts ...grpc.CallOption) (*ListTeamMembersResponse, error) {
	out := new(ListTeamMembersResponse)
	err := c.cc.Invoke(ctx, Teams_Members_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *teamsClient) SlackAlertsChannels(ctx context.Context, in *SlackAlertsChannelsRequest, opts ...grpc.CallOption) (*SlackAlertsChannelsResponse, error) {
	out := new(SlackAlertsChannelsResponse)
	err := c.cc.Invoke(ctx, Teams_SlackAlertsChannels_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// TeamsServer is the server API for Teams service.
// All implementations must embed UnimplementedTeamsServer
// for forward compatibility
type TeamsServer interface {
	Get(context.Context, *GetTeamRequest) (*GetTeamResponse, error)
	List(context.Context, *ListTeamsRequest) (*ListTeamsResponse, error)
	Members(context.Context, *ListTeamMembersRequest) (*ListTeamMembersResponse, error)
	SlackAlertsChannels(context.Context, *SlackAlertsChannelsRequest) (*SlackAlertsChannelsResponse, error)
	mustEmbedUnimplementedTeamsServer()
}

// UnimplementedTeamsServer must be embedded to have forward compatible implementations.
type UnimplementedTeamsServer struct {
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
func (UnimplementedTeamsServer) SlackAlertsChannels(context.Context, *SlackAlertsChannelsRequest) (*SlackAlertsChannelsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SlackAlertsChannels not implemented")
}
func (UnimplementedTeamsServer) mustEmbedUnimplementedTeamsServer() {}

// UnsafeTeamsServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to TeamsServer will
// result in compilation errors.
type UnsafeTeamsServer interface {
	mustEmbedUnimplementedTeamsServer()
}

func RegisterTeamsServer(s grpc.ServiceRegistrar, srv TeamsServer) {
	s.RegisterService(&Teams_ServiceDesc, srv)
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

func _Teams_SlackAlertsChannels_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SlackAlertsChannelsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TeamsServer).SlackAlertsChannels(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Teams_SlackAlertsChannels_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TeamsServer).SlackAlertsChannels(ctx, req.(*SlackAlertsChannelsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Teams_ServiceDesc is the grpc.ServiceDesc for Teams service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Teams_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "Teams",
	HandlerType: (*TeamsServer)(nil),
	Methods: []grpc.MethodDesc{
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
			MethodName: "SlackAlertsChannels",
			Handler:    _Teams_SlackAlertsChannels_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "teams.proto",
}