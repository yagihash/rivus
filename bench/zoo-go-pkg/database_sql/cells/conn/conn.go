// Package conn is a cell of the package database/sql.
//
// @cell(F/pkg/database-sql/conn)
// @shape: the query string given to a connection taken out of the pool (sql.Conn) or to Tx.Prepare is a sink, as it is for sql.DB; the driver (mark.FakeDB) has no database and hands the string it is asked to prepare to the recording sink, so the run sees the standard library's sink at the line of its call in the cell
package conn

import (
	"context"
	"database/sql"

	"zoo/mark"
)

func init() {
	mark.Case("F/pkg/database-sql/conn/vuln", vuln)
	mark.Case("F/pkg/database-sql/conn/fixed", fixed)
	mark.Case("F/pkg/database-sql/conn/decoy", decoy)
}

func conn() (*sql.Conn, *sql.Tx) {
	db := mark.FakeDB()
	c, _ := db.Conn(context.Background())
	tx, _ := db.BeginTx(context.Background(), nil)
	return c, tx
}

func vuln() { // @vuln
	c, tx := conn()
	ctx := context.Background()
	_, _ = c.QueryContext(ctx, mark.Src("pkg-database-sql-conn-vuln-q"))     // @source(pkg-database-sql-conn-vuln-q) @sink(pkg-database-sql-conn-vuln-q)
	_ = c.QueryRowContext(ctx, mark.Src("pkg-database-sql-conn-vuln-r"))     // @source(pkg-database-sql-conn-vuln-r) @sink(pkg-database-sql-conn-vuln-r)
	_, _ = c.ExecContext(ctx, mark.Src("pkg-database-sql-conn-vuln-e"))      // @source(pkg-database-sql-conn-vuln-e) @sink(pkg-database-sql-conn-vuln-e)
	_, _ = c.PrepareContext(ctx, mark.Src("pkg-database-sql-conn-vuln-p"))   // @source(pkg-database-sql-conn-vuln-p) @sink(pkg-database-sql-conn-vuln-p)
	_, _ = tx.Prepare(mark.Src("pkg-database-sql-conn-vuln-tp"))             // @source(pkg-database-sql-conn-vuln-tp) @sink(pkg-database-sql-conn-vuln-tp)
	_, _ = tx.PrepareContext(ctx, mark.Src("pkg-database-sql-conn-vuln-tc")) // @source(pkg-database-sql-conn-vuln-tc) @sink(pkg-database-sql-conn-vuln-tc)
}

func fixed() { // @fixed
	c, tx := conn()
	ctx := context.Background()
	q := mark.CleanSQL(mark.Src("pkg-database-sql-conn-fixed"))
	_, _ = c.QueryContext(ctx, q)       // @sink(none)
	_ = c.QueryRowContext(ctx, q).Err() // @sink(none)
	_, _ = c.ExecContext(ctx, q)        // @sink(none)
	_, _ = c.PrepareContext(ctx, q)     // @sink(none)
	_, _ = tx.Prepare(q)                // @sink(none)
	_, _ = tx.PrepareContext(ctx, q)    // @sink(none)
}

func decoy() { // @decoy
	c, tx := conn()
	ctx := context.Background()
	v := mark.Src("pkg-database-sql-conn-decoy") // @source(pkg-database-sql-conn-decoy)
	// the query string is a constant: the source goes in the bound args only
	_, _ = c.QueryContext(ctx, "select ?", v)
	_ = c.QueryRowContext(ctx, "select ?", v)
	_, _ = c.ExecContext(ctx, "update t set a = ?", v)
	_, _ = c.PrepareContext(ctx, "select 1")
	_, _ = tx.Prepare("select 1")
	_, _ = tx.PrepareContext(ctx, "select 1")
	mark.SinkCmd(v) // @sink(pkg-database-sql-conn-decoy)
}
