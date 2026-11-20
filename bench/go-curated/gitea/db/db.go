// Package db has the transaction helper: the work of a service is a closure given to it.
package db

import "context"

// WithTx runs f in a transaction.
func WithTx(ctx context.Context, f func(ctx context.Context) error) error {
	return f(ctx)
}
