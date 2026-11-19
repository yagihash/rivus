// Package grpc is a stand-in for google.golang.org/grpc with the few things the
// cases use, under the same import path and with the same names, so that the
// rules for the real package apply and the tests need no download.
package grpc

import "context"

type ServerStream interface {
	Context() context.Context
	RecvMsg(m any) error
	SendMsg(m any) error
}

type ClientConn struct{ target string }

func NewClient(target string, opts ...any) (*ClientConn, error) { return &ClientConn{target}, nil }
func Dial(target string, opts ...any) (*ClientConn, error)      { return &ClientConn{target}, nil }
func DialContext(ctx context.Context, target string, opts ...any) (*ClientConn, error) {
	return &ClientConn{target}, nil
}

type Server struct{}

func NewServer(opts ...any) *Server { return &Server{} }
