// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v4.25.2
// source: reconciler_resources.proto

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
	ReconcilerResources_Delete_FullMethodName = "/ReconcilerResources/Delete"
	ReconcilerResources_Save_FullMethodName   = "/ReconcilerResources/Save"
	ReconcilerResources_List_FullMethodName   = "/ReconcilerResources/List"
)

// ReconcilerResourcesClient is the client API for ReconcilerResources service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ReconcilerResourcesClient interface {
	Delete(ctx context.Context, in *DeleteReconcilerResourcesRequest, opts ...grpc.CallOption) (*DeleteReconcilerResourcesResponse, error)
	Save(ctx context.Context, in *SaveReconcilerResourceRequest, opts ...grpc.CallOption) (*SaveReconcilerResourceResponse, error)
	List(ctx context.Context, in *ListReconcilerResourcesRequest, opts ...grpc.CallOption) (*ListReconcilerResourcesResponse, error)
}

type reconcilerResourcesClient struct {
	cc grpc.ClientConnInterface
}

func NewReconcilerResourcesClient(cc grpc.ClientConnInterface) ReconcilerResourcesClient {
	return &reconcilerResourcesClient{cc}
}

func (c *reconcilerResourcesClient) Delete(ctx context.Context, in *DeleteReconcilerResourcesRequest, opts ...grpc.CallOption) (*DeleteReconcilerResourcesResponse, error) {
	out := new(DeleteReconcilerResourcesResponse)
	err := c.cc.Invoke(ctx, ReconcilerResources_Delete_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *reconcilerResourcesClient) Save(ctx context.Context, in *SaveReconcilerResourceRequest, opts ...grpc.CallOption) (*SaveReconcilerResourceResponse, error) {
	out := new(SaveReconcilerResourceResponse)
	err := c.cc.Invoke(ctx, ReconcilerResources_Save_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *reconcilerResourcesClient) List(ctx context.Context, in *ListReconcilerResourcesRequest, opts ...grpc.CallOption) (*ListReconcilerResourcesResponse, error) {
	out := new(ListReconcilerResourcesResponse)
	err := c.cc.Invoke(ctx, ReconcilerResources_List_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ReconcilerResourcesServer is the server API for ReconcilerResources service.
// All implementations must embed UnimplementedReconcilerResourcesServer
// for forward compatibility
type ReconcilerResourcesServer interface {
	Delete(context.Context, *DeleteReconcilerResourcesRequest) (*DeleteReconcilerResourcesResponse, error)
	Save(context.Context, *SaveReconcilerResourceRequest) (*SaveReconcilerResourceResponse, error)
	List(context.Context, *ListReconcilerResourcesRequest) (*ListReconcilerResourcesResponse, error)
	mustEmbedUnimplementedReconcilerResourcesServer()
}

// UnimplementedReconcilerResourcesServer must be embedded to have forward compatible implementations.
type UnimplementedReconcilerResourcesServer struct {
}

func (UnimplementedReconcilerResourcesServer) Delete(context.Context, *DeleteReconcilerResourcesRequest) (*DeleteReconcilerResourcesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Delete not implemented")
}
func (UnimplementedReconcilerResourcesServer) Save(context.Context, *SaveReconcilerResourceRequest) (*SaveReconcilerResourceResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Save not implemented")
}
func (UnimplementedReconcilerResourcesServer) List(context.Context, *ListReconcilerResourcesRequest) (*ListReconcilerResourcesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method List not implemented")
}
func (UnimplementedReconcilerResourcesServer) mustEmbedUnimplementedReconcilerResourcesServer() {}

// UnsafeReconcilerResourcesServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ReconcilerResourcesServer will
// result in compilation errors.
type UnsafeReconcilerResourcesServer interface {
	mustEmbedUnimplementedReconcilerResourcesServer()
}

func RegisterReconcilerResourcesServer(s grpc.ServiceRegistrar, srv ReconcilerResourcesServer) {
	s.RegisterService(&ReconcilerResources_ServiceDesc, srv)
}

func _ReconcilerResources_Delete_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteReconcilerResourcesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ReconcilerResourcesServer).Delete(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ReconcilerResources_Delete_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ReconcilerResourcesServer).Delete(ctx, req.(*DeleteReconcilerResourcesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ReconcilerResources_Save_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SaveReconcilerResourceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ReconcilerResourcesServer).Save(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ReconcilerResources_Save_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ReconcilerResourcesServer).Save(ctx, req.(*SaveReconcilerResourceRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ReconcilerResources_List_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListReconcilerResourcesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ReconcilerResourcesServer).List(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ReconcilerResources_List_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ReconcilerResourcesServer).List(ctx, req.(*ListReconcilerResourcesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// ReconcilerResources_ServiceDesc is the grpc.ServiceDesc for ReconcilerResources service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var ReconcilerResources_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "ReconcilerResources",
	HandlerType: (*ReconcilerResourcesServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Delete",
			Handler:    _ReconcilerResources_Delete_Handler,
		},
		{
			MethodName: "Save",
			Handler:    _ReconcilerResources_Save_Handler,
		},
		{
			MethodName: "List",
			Handler:    _ReconcilerResources_List_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "reconciler_resources.proto",
}