// Package ownif holds calls through an interface of the project whose
// implementation is a type of the standard library with a sink rule: the call
// site is the project's method, the sink is the implementation's.
package ownif

import (
	"context"
	"database/sql"

	"example.com/cases/lib"
)

// dbtx is the shape sqlc generates: *sql.DB satisfies it.
type dbtx interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func run(q dbtx, query string) {
	_, _ = q.QueryContext(context.Background(), query) // @sink(ownif_pool)
}

func Pool(db *sql.DB) {
	q := lib.Source() // @source(ownif_pool)
	run(db, q)
}

func Direct(db *sql.DB) {
	var t dbtx = db
	q := lib.Source()                              // @source(ownif_direct)
	_, _ = t.QueryContext(context.Background(), q) // @sink(ownif_direct)
}

// two implementations: the standard library's and one of the project's that
// has nothing dangerous. The call may reach the first (an uncertainty stays).
type fake struct{}

func (fake) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return nil, nil
}

func Two(db *sql.DB, useFake bool) {
	var t dbtx = db
	if useFake {
		t = fake{}
	}
	q := lib.Source()                              // @source(ownif_two)
	_, _ = t.QueryContext(context.Background(), q) // @sink(ownif_two)
}

// the tainted value is not the query: not a sink argument
func NotQuery(db *sql.DB) {
	var t dbtx = db
	q := lib.Source()
	_, _ = t.QueryContext(context.Background(), "select 1", q)
}

// only an implementation without a sink
type onlyFake interface {
	Do(ctx context.Context, query string)
}

type fake2 struct{}

func (fake2) Do(ctx context.Context, query string) {}

func Quiet() {
	var t onlyFake = fake2{}
	q := lib.Source()
	t.Do(context.Background(), q)
}
