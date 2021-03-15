// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package network

import (
	context "context"

	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// NetworkServiceClient is the client API for NetworkService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type NetworkServiceClient interface {
	Routes(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*RoutesResponse, error)
	Interfaces(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*InterfacesResponse, error)
}

type networkServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewNetworkServiceClient(cc grpc.ClientConnInterface) NetworkServiceClient {
	return &networkServiceClient{cc}
}

func (c *networkServiceClient) Routes(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*RoutesResponse, error) {
	out := new(RoutesResponse)
	err := c.cc.Invoke(ctx, "/network.NetworkService/Routes", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *networkServiceClient) Interfaces(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*InterfacesResponse, error) {
	out := new(InterfacesResponse)
	err := c.cc.Invoke(ctx, "/network.NetworkService/Interfaces", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// NetworkServiceServer is the server API for NetworkService service.
// All implementations must embed UnimplementedNetworkServiceServer
// for forward compatibility
type NetworkServiceServer interface {
	Routes(context.Context, *emptypb.Empty) (*RoutesResponse, error)
	Interfaces(context.Context, *emptypb.Empty) (*InterfacesResponse, error)
	mustEmbedUnimplementedNetworkServiceServer()
}

// UnimplementedNetworkServiceServer must be embedded to have forward compatible implementations.
type UnimplementedNetworkServiceServer struct {
}

func (UnimplementedNetworkServiceServer) Routes(context.Context, *emptypb.Empty) (*RoutesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Routes not implemented")
}

func (UnimplementedNetworkServiceServer) Interfaces(context.Context, *emptypb.Empty) (*InterfacesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Interfaces not implemented")
}
func (UnimplementedNetworkServiceServer) mustEmbedUnimplementedNetworkServiceServer() {}

// UnsafeNetworkServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to NetworkServiceServer will
// result in compilation errors.
type UnsafeNetworkServiceServer interface {
	mustEmbedUnimplementedNetworkServiceServer()
}

func RegisterNetworkServiceServer(s grpc.ServiceRegistrar, srv NetworkServiceServer) {
	s.RegisterService(&NetworkService_ServiceDesc, srv)
}

func _NetworkService_Routes_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetworkServiceServer).Routes(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/network.NetworkService/Routes",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetworkServiceServer).Routes(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _NetworkService_Interfaces_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetworkServiceServer).Interfaces(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/network.NetworkService/Interfaces",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetworkServiceServer).Interfaces(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

// NetworkService_ServiceDesc is the grpc.ServiceDesc for NetworkService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var NetworkService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "network.NetworkService",
	HandlerType: (*NetworkServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Routes",
			Handler:    _NetworkService_Routes_Handler,
		},
		{
			MethodName: "Interfaces",
			Handler:    _NetworkService_Interfaces_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "network/network.proto",
}
