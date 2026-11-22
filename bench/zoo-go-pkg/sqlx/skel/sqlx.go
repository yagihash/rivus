// Package sqlx is a skeleton of github.com/jmoiron/sqlx v1.4.0 for the zoo (docs/DESIGN-package-models.md, 6.8): the public
// signatures of the query methods that the built-in model (internal/model/builtin/go-sqlx.yaml) has rows for, and bodies that
// only record the query string (zoo/mark), so that the cells run. It is not sqlx: the bodies are not sqlx's, and the real sqlx is
// never fetched or run.
package sqlx

import (
	"context"
	"database/sql"

	"zoo/mark"
)

// DB, Tx and Conn embed the types of database/sql, as in sqlx: Query, Exec, Prepare and the rest are promoted from there (the rows
// of those are in go-stdlib.yaml). Only the methods that sqlx defines itself are written here.
type DB struct{ *sql.DB }
type Tx struct{ *sql.Tx }
type Conn struct{ *sql.Conn }
type Stmt struct{}
type NamedStmt struct{}
type Rows struct{}
type Row struct{}

// Ext and ExtContext stand for the interfaces of sqlx that DB, Tx and Conn satisfy (the skeleton gives them no methods).
type Ext interface{}
type ExtContext interface{}

func Open(driverName, dataSourceName string) (*DB, error) { return &DB{DB: &sql.DB{}}, nil }

func (db *DB) NamedQuery(query string, arg interface{}) (*Rows, error) {
	mark.SinkSQL(query)
	return nil, nil
}
func (db *DB) NamedExec(query string, arg interface{}) (sql.Result, error) {
	mark.SinkSQL(query)
	return nil, nil
}
func (db *DB) Select(dest interface{}, query string, args ...interface{}) error {
	mark.SinkSQL(query)
	return nil
}
func (db *DB) Get(dest interface{}, query string, args ...interface{}) error {
	mark.SinkSQL(query)
	return nil
}
func (db *DB) Queryx(query string, args ...interface{}) (*Rows, error) {
	mark.SinkSQL(query)
	return nil, nil
}
func (db *DB) QueryRowx(query string, args ...interface{}) *Row      { mark.SinkSQL(query); return nil }
func (db *DB) MustExec(query string, args ...interface{}) sql.Result { mark.SinkSQL(query); return nil }
func (db *DB) Preparex(query string) (*Stmt, error)                  { mark.SinkSQL(query); return nil, nil }
func (db *DB) PrepareNamed(query string) (*NamedStmt, error)         { mark.SinkSQL(query); return nil, nil }
func (db *DB) SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	mark.SinkSQL(query)
	return nil
}
func (db *DB) GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	mark.SinkSQL(query)
	return nil
}
func (db *DB) NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error) {
	mark.SinkSQL(query)
	return nil, nil
}
func (db *DB) NamedQueryContext(ctx context.Context, query string, arg interface{}) (*Rows, error) {
	mark.SinkSQL(query)
	return nil, nil
}
func (db *DB) QueryxContext(ctx context.Context, query string, args ...interface{}) (*Rows, error) {
	mark.SinkSQL(query)
	return nil, nil
}
func (db *DB) QueryRowxContext(ctx context.Context, query string, args ...interface{}) *Row {
	mark.SinkSQL(query)
	return nil
}
func (db *DB) MustExecContext(ctx context.Context, query string, args ...interface{}) sql.Result {
	mark.SinkSQL(query)
	return nil
}
func (db *DB) PreparexContext(ctx context.Context, query string) (*Stmt, error) {
	mark.SinkSQL(query)
	return nil, nil
}
func (db *DB) PrepareNamedContext(ctx context.Context, query string) (*NamedStmt, error) {
	mark.SinkSQL(query)
	return nil, nil
}

func (db *DB) Beginx() (*Tx, error) { return &Tx{Tx: &sql.Tx{}}, nil }

func (tx *Tx) Select(dest interface{}, query string, args ...interface{}) error {
	mark.SinkSQL(query)
	return nil
}
func (tx *Tx) Get(dest interface{}, query string, args ...interface{}) error {
	mark.SinkSQL(query)
	return nil
}
func (tx *Tx) NamedExec(query string, arg interface{}) (sql.Result, error) {
	mark.SinkSQL(query)
	return nil, nil
}
func (tx *Tx) MustExecContext(ctx context.Context, query string, args ...interface{}) sql.Result {
	mark.SinkSQL(query)
	return nil
}

func Select(q interface{}, dest interface{}, query string, args ...interface{}) error {
	mark.SinkSQL(query)
	return nil
}
func NamedExec(e Ext, query string, arg interface{}) (sql.Result, error) {
	mark.SinkSQL(query)
	return nil, nil
}
func GetContext(ctx context.Context, q interface{}, dest interface{}, query string, args ...interface{}) error {
	mark.SinkSQL(query)
	return nil
}
