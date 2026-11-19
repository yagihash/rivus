package grpcsrv

import (
	"context"

	"example.com/cases/lib"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// What the client wrote in the headers of the call.
func FromMetadata(ctx context.Context) {
	md, _ := metadata.FromIncomingContext(ctx) // @source(grpc_md)
	lib.Sink(md.Get("x-file")[0])              // @sink(grpc_md)
}

func FromValue(ctx context.Context) {
	v := metadata.ValueFromIncomingContext(ctx, "x-file") // @source(grpc_val)
	lib.Sink(v[0])                                        // @sink(grpc_val)
}

// What the server sends back in its own metadata is not input.
func Outgoing(ctx context.Context) {
	md, _ := metadata.FromOutgoingContext(ctx)
	lib.Sink(md.Get("x")[0]) // @sink(none)
}

type fileRequest struct{ Path string }

// A stream of requests.
func Stream(stream grpc.ServerStream) {
	var req fileRequest
	_ = stream.RecvMsg(&req) // @source(grpc_stream)
	lib.Sink(req.Path)       // @sink(grpc_stream)
}

// A connection to an address that came from outside.
func Dial() {
	target := lib.Source()        // @source(grpc_dial)
	_, _ = grpc.NewClient(target) // @sink(grpc_dial)
}
