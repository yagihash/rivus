// Package metadata is a stand-in for google.golang.org/grpc/metadata.
package metadata

import "context"

type MD map[string][]string

func (md MD) Get(k string) []string { return md[k] }

func FromIncomingContext(ctx context.Context) (MD, bool)               { return nil, false }
func ValueFromIncomingContext(ctx context.Context, key string) []string { return nil }
func FromOutgoingContext(ctx context.Context) (MD, bool)               { return nil, false }
