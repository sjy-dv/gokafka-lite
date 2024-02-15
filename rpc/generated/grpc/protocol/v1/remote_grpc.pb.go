// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v3.19.6
// source: v1/remote.proto

package protocol

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
	Remote_Receive_FullMethodName = "/gokafkalite.v1.Remote/Receive"
)

// RemoteClient is the client API for Remote service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type RemoteClient interface {
	Receive(ctx context.Context, opts ...grpc.CallOption) (Remote_ReceiveClient, error)
}

type remoteClient struct {
	cc grpc.ClientConnInterface
}

func NewRemoteClient(cc grpc.ClientConnInterface) RemoteClient {
	return &remoteClient{cc}
}

func (c *remoteClient) Receive(ctx context.Context, opts ...grpc.CallOption) (Remote_ReceiveClient, error) {
	stream, err := c.cc.NewStream(ctx, &Remote_ServiceDesc.Streams[0], Remote_Receive_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &remoteReceiveClient{stream}
	return x, nil
}

type Remote_ReceiveClient interface {
	Send(*Envelope) error
	Recv() (*Envelope, error)
	grpc.ClientStream
}

type remoteReceiveClient struct {
	grpc.ClientStream
}

func (x *remoteReceiveClient) Send(m *Envelope) error {
	return x.ClientStream.SendMsg(m)
}

func (x *remoteReceiveClient) Recv() (*Envelope, error) {
	m := new(Envelope)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// RemoteServer is the server API for Remote service.
// All implementations should embed UnimplementedRemoteServer
// for forward compatibility
type RemoteServer interface {
	Receive(Remote_ReceiveServer) error
}

// UnimplementedRemoteServer should be embedded to have forward compatible implementations.
type UnimplementedRemoteServer struct {
}

func (UnimplementedRemoteServer) Receive(Remote_ReceiveServer) error {
	return status.Errorf(codes.Unimplemented, "method Receive not implemented")
}

// UnsafeRemoteServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to RemoteServer will
// result in compilation errors.
type UnsafeRemoteServer interface {
	mustEmbedUnimplementedRemoteServer()
}

func RegisterRemoteServer(s grpc.ServiceRegistrar, srv RemoteServer) {
	s.RegisterService(&Remote_ServiceDesc, srv)
}

func _Remote_Receive_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(RemoteServer).Receive(&remoteReceiveServer{stream})
}

type Remote_ReceiveServer interface {
	Send(*Envelope) error
	Recv() (*Envelope, error)
	grpc.ServerStream
}

type remoteReceiveServer struct {
	grpc.ServerStream
}

func (x *remoteReceiveServer) Send(m *Envelope) error {
	return x.ServerStream.SendMsg(m)
}

func (x *remoteReceiveServer) Recv() (*Envelope, error) {
	m := new(Envelope)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Remote_ServiceDesc is the grpc.ServiceDesc for Remote service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Remote_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "gokafkalite.v1.Remote",
	HandlerType: (*RemoteServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Receive",
			Handler:       _Remote_Receive_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "v1/remote.proto",
}