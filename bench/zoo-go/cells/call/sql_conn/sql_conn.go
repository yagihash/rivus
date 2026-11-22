// Package sql_conn is a cell of the zoo.
//
// @cell(F/call/sql-conn)
// @shape: the query string given to a connection taken out of the pool (sql.Conn) or to Tx.Prepare is a sink, as it is for sql.DB; the driver is a fake that has no database.
// The cell has no vuln case: a sink of the standard library cannot be recorded at run time, and a manual mark in a new cell needs the approval of the user. That the rows
// find the sink is checked by `make model-diff PKG=database/sql` (the source in the query string gives a finding, in a bound argument it does not). This cell is what calls the rows
// (TestBuiltinRowsAreHitByTheZoo) with a cleaned string and with a constant.
package sql_conn

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"

	"zoo/mark"
)

func init() {
	mark.Case("F/call/sql-conn/vuln", vuln)
	mark.Case("F/call/sql-conn/fixed", fixed)
	mark.Case("F/call/sql-conn/decoy", decoy)
}

var errNoDB = errors.New("no database")

// the connector hands out a connection that takes no statement: the case builds the calls, and the driver says "no database"
type fakeConnector struct{}

func (fakeConnector) Connect(context.Context) (driver.Conn, error) { return fakeConn{}, nil }
func (fakeConnector) Driver() driver.Driver                        { return nil }

type fakeConn struct{}

func (fakeConn) Prepare(string) (driver.Stmt, error) { return nil, errNoDB }
func (fakeConn) Close() error                        { return nil }
func (fakeConn) Begin() (driver.Tx, error)           { return fakeTx{}, nil }

type fakeTx struct{}

func (fakeTx) Commit() error   { return nil }
func (fakeTx) Rollback() error { return nil }

func conn() (*sql.Conn, *sql.Tx) {
	db := sql.OpenDB(fakeConnector{})
	c, _ := db.Conn(context.Background())
	tx, _ := db.BeginTx(context.Background(), nil)
	return c, tx
}

// vuln: the query string is a constant and the source is a bound argument, so the calls of the connection are no sink here; the sink is the mark's, after them
// (the case needs a sink that the harness can record, and the standard library's cannot be).
func vuln() { // @vuln
	c, _ := conn()
	v := mark.Src("call-sql-conn-vuln") // @source(call-sql-conn-vuln)
	_, _ = c.QueryContext(context.Background(), "select ?", v)
	mark.SinkSQL(v) // @sink(call-sql-conn-vuln)
}

func fixed() { // @fixed
	c, tx := conn()
	ctx := context.Background()
	q := mark.CleanSQL(mark.Src("call-sql-conn-fixed"))
	_, _ = c.QueryContext(ctx, q)
	_ = c.QueryRowContext(ctx, q).Err()
	_, _ = c.ExecContext(ctx, q)
	_, _ = c.PrepareContext(ctx, q)
	_, _ = tx.Prepare(q)
	_, _ = tx.PrepareContext(ctx, q)
}

func decoy() { // @decoy
	c, tx := conn()
	ctx := context.Background()
	v := mark.Src("call-sql-conn-decoy") // @source(call-sql-conn-decoy)
	// the query string is a constant: the source goes in the bound args only
	_, _ = c.QueryContext(ctx, "select ?", v)
	_ = c.QueryRowContext(ctx, "select ?", v)
	_, _ = c.ExecContext(ctx, "update t set a = ?", v)
	_, _ = c.PrepareContext(ctx, "select 1")
	_, _ = tx.Prepare("select 1")
	_, _ = tx.PrepareContext(ctx, "select 1")
	mark.SinkCmd(v) // @sink(call-sql-conn-decoy)
}
