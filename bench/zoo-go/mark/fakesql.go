package mark

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
)

// FakeDB is a *sql.DB whose driver has no database: it connects, and every statement it is asked to prepare is handed to SinkSQL and
// refused ("no database"). It is for the cells that call a sink of the standard library (database/sql: Conn, Tx, DB), which the
// harness cannot record itself: the query string reaches the driver, SinkSQL records it, and the place of the record is the first frame of the
// stack that is in cells/ (see record), that is the call of the sink in the cell, where the analysis has its finding. Nothing is
// registered, nothing listens.
func FakeDB() *sql.DB { return sql.OpenDB(fakeConnector{}) }

var errNoDB = errors.New("no database")

type fakeConnector struct{}

func (fakeConnector) Connect(context.Context) (driver.Conn, error) { return fakeConn{}, nil }
func (fakeConnector) Driver() driver.Driver                        { return nil }

type fakeConn struct{}

func (fakeConn) Prepare(query string) (driver.Stmt, error) {
	SinkSQL(query)
	return nil, errNoDB
}
func (fakeConn) Close() error              { return nil }
func (fakeConn) Begin() (driver.Tx, error) { return fakeTx{}, nil }

type fakeTx struct{}

func (fakeTx) Commit() error   { return nil }
func (fakeTx) Rollback() error { return nil }
