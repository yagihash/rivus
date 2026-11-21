// Package spanner holds the cases of the Cloud Spanner model (internal/model/builtin/go-gcp.yaml).
// Nothing here connects to Google Cloud: the code only builds the calls, and cmd/run sends them
// to the fake of the library (spanner/spannertest) where it can.
//
// Each case is a handler: V* is vulnerable (the query text is made from the request: the sink
// must report it), F* is the fixed form (the value goes in by Params: quiet), D* is a decoy (a
// value of the request is in another field of the same statement: quiet).
package spanner

import (
	"context"
	"fmt"
	"net/http"

	"cloud.google.com/go/spanner"
)

type Server struct {
	Client *spanner.Client
}

func param(r *http.Request) string { return r.URL.Query().Get("name") }

// Single().Query: the Statement is the argument.

func (s *Server) VQuery(w http.ResponseWriter, r *http.Request) {
	stmt := spanner.Statement{SQL: "SELECT id FROM users WHERE name = '" + param(r) + "'"}
	it := s.Client.Single().Query(r.Context(), stmt)
	_ = it.Do(func(*spanner.Row) error { return nil })
}

func (s *Server) FQuery(w http.ResponseWriter, r *http.Request) {
	stmt := spanner.Statement{
		SQL:    "SELECT id FROM users WHERE name = @name",
		Params: map[string]any{"name": param(r)},
	}
	it := s.Client.Single().Query(r.Context(), stmt)
	_ = it.Do(func(*spanner.Row) error { return nil })
}

func (s *Server) DQuery(w http.ResponseWriter, r *http.Request) {
	stmt := spanner.Statement{SQL: "SELECT id FROM users", Params: map[string]any{"name": param(r)}}
	it := s.Client.Single().Query(r.Context(), stmt)
	_ = it.Do(func(*spanner.Row) error { return nil })
}

// NewStatement: a function of the library that builds the Statement.

func (s *Server) VNewStatement(w http.ResponseWriter, r *http.Request) {
	stmt := spanner.NewStatement("SELECT id FROM users WHERE name = '" + param(r) + "'")
	it := s.Client.Single().Query(r.Context(), stmt)
	_ = it.Do(func(*spanner.Row) error { return nil })
}

func (s *Server) FNewStatement(w http.ResponseWriter, r *http.Request) {
	stmt := spanner.NewStatement("SELECT id FROM users WHERE name = @name")
	stmt.Params["name"] = param(r)
	it := s.Client.Single().Query(r.Context(), stmt)
	_ = it.Do(func(*spanner.Row) error { return nil })
}

// QueryWithOptions, ReadOnlyTransaction, AnalyzeQuery.

func (s *Server) VQueryWithOptions(w http.ResponseWriter, r *http.Request) {
	stmt := spanner.Statement{SQL: "SELECT 1 FROM t WHERE c = '" + param(r) + "'"}
	it := s.Client.Single().QueryWithOptions(r.Context(), stmt, spanner.QueryOptions{})
	_ = it.Do(func(*spanner.Row) error { return nil })
}

func (s *Server) VReadOnlyTransaction(w http.ResponseWriter, r *http.Request) {
	tx := s.Client.ReadOnlyTransaction()
	defer tx.Close()
	it := tx.Query(r.Context(), spanner.Statement{SQL: "SELECT 1 FROM t WHERE c = '" + param(r) + "'"})
	_ = it.Do(func(*spanner.Row) error { return nil })
}

func (s *Server) VAnalyze(w http.ResponseWriter, r *http.Request) {
	_, _ = s.Client.Single().AnalyzeQuery(r.Context(), spanner.Statement{SQL: "SELECT 1 FROM t WHERE c = '" + param(r) + "'"})
}

// ReadWriteTransaction: Update, BatchUpdate, Query.

func (s *Server) VUpdate(w http.ResponseWriter, r *http.Request) {
	_, _ = s.Client.ReadWriteTransaction(r.Context(), func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		_, err := tx.Update(ctx, spanner.Statement{SQL: "UPDATE t SET c = 1 WHERE n = '" + param(r) + "'"})
		return err
	})
}

func (s *Server) FUpdate(w http.ResponseWriter, r *http.Request) {
	_, _ = s.Client.ReadWriteTransaction(r.Context(), func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		_, err := tx.Update(ctx, spanner.Statement{SQL: "UPDATE t SET c = 1 WHERE n = @n", Params: map[string]any{"n": param(r)}})
		return err
	})
}

func (s *Server) DUpdate(w http.ResponseWriter, r *http.Request) {
	_, _ = s.Client.ReadWriteTransaction(r.Context(), func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		_, err := tx.Update(ctx, spanner.Statement{SQL: "UPDATE t SET c = 1", Params: map[string]any{"n": param(r)}})
		return err
	})
}

func (s *Server) VTxQuery(w http.ResponseWriter, r *http.Request) {
	_, _ = s.Client.ReadWriteTransaction(r.Context(), func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		it := tx.Query(ctx, spanner.Statement{SQL: "SELECT 1 FROM t WHERE n = '" + param(r) + "'"})
		_ = it.Do(func(*spanner.Row) error { return nil })
		return nil
	})
}

func (s *Server) VBatchUpdate(w http.ResponseWriter, r *http.Request) {
	_, _ = s.Client.ReadWriteTransaction(r.Context(), func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		_, err := tx.BatchUpdate(ctx, []spanner.Statement{
			{SQL: "UPDATE t SET c = 1"},
			{SQL: "DELETE FROM t WHERE n = '" + param(r) + "'"},
		})
		return err
	})
}

func (s *Server) DBatchUpdate(w http.ResponseWriter, r *http.Request) {
	_, _ = s.Client.ReadWriteTransaction(r.Context(), func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		_, err := tx.BatchUpdate(ctx, []spanner.Statement{
			{SQL: "UPDATE t SET c = 1", Params: map[string]any{"n": param(r)}},
		})
		return err
	})
}

func (s *Server) VPartitionedUpdate(w http.ResponseWriter, r *http.Request) {
	_, _ = s.Client.PartitionedUpdate(r.Context(), spanner.Statement{SQL: "DELETE FROM t WHERE n = '" + param(r) + "'"})
}

func (s *Server) DPartitionedUpdate(w http.ResponseWriter, r *http.Request) {
	_, _ = s.Client.PartitionedUpdate(r.Context(), spanner.Statement{SQL: "DELETE FROM t WHERE true", Params: map[string]any{"n": param(r)}})
}

// The variants of the calls above that take options, a statistic or a partition.

func (s *Server) VQueryWithStats(w http.ResponseWriter, r *http.Request) {
	it := s.Client.Single().QueryWithStats(r.Context(), spanner.Statement{SQL: "SELECT 1 FROM t WHERE c = '" + param(r) + "'"})
	_ = it.Do(func(*spanner.Row) error { return nil })
}

func (s *Server) VUpdateWithOptions(w http.ResponseWriter, r *http.Request) {
	_, _ = s.Client.ReadWriteTransaction(r.Context(), func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		_, err := tx.UpdateWithOptions(ctx, spanner.Statement{SQL: "UPDATE t SET c = 1 WHERE n = '" + param(r) + "'"}, spanner.QueryOptions{})
		return err
	})
}

func (s *Server) VBatchUpdateWithOptions(w http.ResponseWriter, r *http.Request) {
	_, _ = s.Client.ReadWriteTransaction(r.Context(), func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		_, err := tx.BatchUpdateWithOptions(ctx, []spanner.Statement{{SQL: "DELETE FROM t WHERE n = '" + param(r) + "'"}}, spanner.QueryOptions{})
		return err
	})
}

func (s *Server) VPartitionedUpdateWithOptions(w http.ResponseWriter, r *http.Request) {
	_, _ = s.Client.PartitionedUpdateWithOptions(r.Context(), spanner.Statement{SQL: "DELETE FROM t WHERE n = '" + param(r) + "'"}, spanner.QueryOptions{})
}

func (s *Server) VPartitionQuery(w http.ResponseWriter, r *http.Request) {
	tx, err := s.Client.BatchReadOnlyTransaction(r.Context(), spanner.StrongRead())
	if err != nil {
		return
	}
	defer tx.Close()
	_, _ = tx.PartitionQuery(r.Context(), spanner.Statement{SQL: "SELECT 1 FROM t WHERE c = '" + param(r) + "'"}, spanner.PartitionOptions{})
}

func (s *Server) DPartitionQuery(w http.ResponseWriter, r *http.Request) {
	tx, err := s.Client.BatchReadOnlyTransaction(r.Context(), spanner.StrongRead())
	if err != nil {
		return
	}
	defer tx.Close()
	_, _ = tx.PartitionQuery(r.Context(), spanner.Statement{SQL: "SELECT 1 FROM t", Params: map[string]any{"c": param(r)}}, spanner.PartitionOptions{})
}

func (s *Server) VPartitionQueryWithOptions(w http.ResponseWriter, r *http.Request) {
	tx, err := s.Client.BatchReadOnlyTransaction(r.Context(), spanner.StrongRead())
	if err != nil {
		return
	}
	defer tx.Close()
	_, _ = tx.PartitionQueryWithOptions(r.Context(), spanner.Statement{SQL: "SELECT 1 FROM t WHERE c = '" + param(r) + "'"}, spanner.PartitionOptions{}, spanner.QueryOptions{})
}

// Reading a row: the stored data that comes back goes to the output arguments. It is not a source until the
// user says that the data of the database is not to be trusted: TestGCPRows reads these with a rule for it
// (rules-stored.yaml).

type user struct {
	Name string `spanner:"name"`
}

func (s *Server) ReadColumns(w http.ResponseWriter, r *http.Request) {
	it := s.Client.Single().Query(r.Context(), spanner.NewStatement("SELECT name FROM users"))
	defer it.Stop()
	row, err := it.Next()
	if err != nil {
		return
	}
	var name string
	if row.Columns(&name) == nil {
		fmt.Fprint(w, name)
	}
}

func (s *Server) ReadColumnByName(w http.ResponseWriter, r *http.Request) {
	it := s.Client.Single().Query(r.Context(), spanner.NewStatement("SELECT name FROM users"))
	defer it.Stop()
	row, err := it.Next()
	if err != nil {
		return
	}
	var name string
	if row.ColumnByName("name", &name) == nil {
		fmt.Fprint(w, name)
	}
}

func (s *Server) ReadToStruct(w http.ResponseWriter, r *http.Request) {
	it := s.Client.Single().Query(r.Context(), spanner.NewStatement("SELECT name FROM users"))
	defer it.Stop()
	row, err := it.Next()
	if err != nil {
		return
	}
	var u user
	if row.ToStruct(&u) == nil {
		fmt.Fprint(w, u.Name)
	}
}
