// Package query is a cell of the package sqlx.
//
// @cell(F/pkg/sqlx/query)
// @shape: a query string that comes from the source reaches the query methods of sqlx (DB and Tx methods, package functions)
package query

import (
	"context"

	"github.com/jmoiron/sqlx"
	"zoo/mark"
)

func init() {
	mark.Case("F/pkg/sqlx/query/vuln", vuln)
	mark.Case("F/pkg/sqlx/query/fixed", fixed)
	mark.Case("F/pkg/sqlx/query/decoy", decoy)
}

func vuln() { // @vuln
	q := mark.Src("pkg-sqlx-query-vuln") // @source(pkg-sqlx-query-vuln)
	db, _ := sqlx.Open("postgres", "dsn")
	var dest []string
	db.Select(&dest, q)                           // @sink(pkg-sqlx-query-vuln)
	db.Get(&dest, q)                              // @sink(pkg-sqlx-query-vuln)
	db.NamedExec(q, nil)                          // @sink(pkg-sqlx-query-vuln)
	db.Queryx(q)                                  // @sink(pkg-sqlx-query-vuln)
	db.MustExecContext(context.Background(), q)   // @sink(pkg-sqlx-query-vuln)
	db.GetContext(context.Background(), &dest, q) // @sink(pkg-sqlx-query-vuln)
	tx, _ := db.Beginx()
	tx.Select(&dest, q)                                 // @sink(pkg-sqlx-query-vuln)
	tx.NamedExec(q, nil)                                // @sink(pkg-sqlx-query-vuln)
	sqlx.Select(db, &dest, q)                           // @sink(pkg-sqlx-query-vuln)
	sqlx.NamedExec(db, q, nil)                          // @sink(pkg-sqlx-query-vuln)
	sqlx.GetContext(context.Background(), db, &dest, q) // @sink(pkg-sqlx-query-vuln)
}

func fixed() { // @fixed
	q := mark.CleanSQL(mark.Src("pkg-sqlx-query-fixed"))
	db, _ := sqlx.Open("postgres", "dsn")
	var dest []string
	db.Select(&dest, q)  // @sink(none)
	db.NamedExec(q, nil) // @sink(none)
}

func decoy() { // @decoy
	t := mark.Src("pkg-sqlx-query-decoy") // @source(pkg-sqlx-query-decoy)
	db, _ := sqlx.Open("postgres", "dsn")
	var dest []string
	db.Select(&dest, "SELECT 1", t) // @sink(none) @nosink(pkg-sqlx-query-decoy)
	db.Get(&dest, t)                // @sink(pkg-sqlx-query-decoy)
}
