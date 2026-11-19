package grpcnogen

import "google.golang.org/grpc"

// A server whose generated code is not there: no interface has the marker.
func Run() *grpc.Server {
	return grpc.NewServer()
}
