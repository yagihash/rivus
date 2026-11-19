package grpcunary

import (
	"context"

	"example.com/cases/lib"
	"google.golang.org/grpc"
)

type FileRequest struct{ Path string }
type FileReply struct{ Data string }

// What protoc-gen-go-grpc writes for a service.
type FilesServer interface {
	Read(context.Context, *FileRequest) (*FileReply, error)
	Watch(*FileRequest, grpc.ServerStream) error
	mustEmbedUnimplementedFilesServer()
}

type UnimplementedFilesServer struct{}

func (UnimplementedFilesServer) Read(context.Context, *FileRequest) (*FileReply, error) {
	return nil, nil
}
func (UnimplementedFilesServer) Watch(*FileRequest, grpc.ServerStream) error { return nil }
func (UnimplementedFilesServer) mustEmbedUnimplementedFilesServer()          {}

// The implementation of the project, registered the way the generated code does
// it (a conversion to the interface: it is what makes the compiler build the
// methods).
type files struct{ UnimplementedFilesServer }

func Register() FilesServer { return &files{} }

func (f *files) Read(ctx context.Context, req *FileRequest) (*FileReply, error) { // @source(grpc_unary)
	lib.Sink(req.Path) // @sink(grpc_unary)
	return nil, nil
}

func (f *files) Watch(req *FileRequest, s grpc.ServerStream) error { // @source(grpc_watch)
	lib.Sink(req.Path) // @sink(grpc_watch)
	return nil
}

// An interface without that method is not a service: the same shape, no input.
type Helper interface{ Do(*FileRequest) }

type helper struct{}

func NewHelper() Helper { return helper{} }

func (helper) Do(req *FileRequest) {
	lib.Sink(req.Path) // @sink(none)
}
