// Package sink_behind_own_interface is a cell of the zoo.
//
// @cell(F/dyn/sink-behind-own-interface)
// @shape: the project calls its own interface (a sqlc DBTX, a Repository), the call site is the interface's method, and the sink is in an implementation (mark's sink in the body; vuln2: the standard library's (*sql.DB).QueryContext, which satisfies the interface itself)
package sink_behind_own_interface

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"

	"zoo/mark"
)

func init() {
	mark.Case("F/dyn/sink-behind-own-interface/vuln", vuln)
	mark.Case("F/dyn/sink-behind-own-interface/vuln2", vuln2)
	mark.Case("F/dyn/sink-behind-own-interface/fixed", fixed)
	mark.Case("F/dyn/sink-behind-own-interface/decoy", decoy)
}

type querier interface {
	Query(ctx context.Context, sql string)
}

type pgQuerier struct{}

func (pgQuerier) Query(ctx context.Context, sql string) {
	mark.SinkSQL(sql) // @sink(dyn-sink-behind-own-interface-vuln)
}

func vuln() { // @vuln
	var q querier = pgQuerier{}
	q.Query(context.Background(), mark.Src("dyn-sink-behind-own-interface-vuln")) // @source(dyn-sink-behind-own-interface-vuln)
}

// dbtx is the shape of what sqlc generates: *sql.DB, *sql.Tx and *sql.Conn all satisfy it,
// and the code is written against it so that a pool and a transaction are both accepted.
type dbtx interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// noDB is a connector that never connects: the case builds the call and the driver says
// "no database" (nothing is registered, nothing listens).
type noDB struct{}

var errNoDB = errors.New("no database")

func (noDB) Connect(context.Context) (driver.Conn, error) { return nil, errNoDB }
func (noDB) Driver() driver.Driver                        { return nil }

func newDB() dbtx {
	db := sql.OpenDB(noDB{})
	_ = db.Driver()
	return db
}

func vuln2() { // @vuln
	q := newDB()
	_, _ = q.QueryContext(context.Background(), mark.Src("dyn-sink-behind-own-interface-vuln2")) // @source(dyn-sink-behind-own-interface-vuln2) @miss(unmodeled_library_call) @ref(R-11) @sink(dyn-sink-behind-own-interface-vuln2) @manual(the sink is the standard library's (*sql.DB).QueryContext, which the case cannot record; it never reaches a database)
}

type querierFixed interface {
	Query(ctx context.Context, sql string)
}

type pgQuerierFixed struct{}

func (pgQuerierFixed) Query(ctx context.Context, sql string) {
	mark.SinkSQL(sql) // @sink(none)
}

func fixed() { // @fixed
	var q querierFixed = pgQuerierFixed{}
	q.Query(context.Background(), mark.CleanSQL(mark.Src("dyn-sink-behind-own-interface-fixed")))
}

// quietQuerier is the only implementation, and it has no sink for what it is given.
type querierQuiet interface {
	Query(ctx context.Context, sql string)
}

type quietQuerier struct{}

func (quietQuerier) Query(ctx context.Context, sql string) {
	mark.SinkSQL("select 1") // @sink(none) @nosink(dyn-sink-behind-own-interface-decoy)
}

func decoy() { // @decoy
	var q querierQuiet = quietQuerier{}
	v := mark.Src("dyn-sink-behind-own-interface-decoy") // @source(dyn-sink-behind-own-interface-decoy)
	q.Query(context.Background(), v)
	mark.SinkCmd(v) // @sink(dyn-sink-behind-own-interface-decoy)
}
