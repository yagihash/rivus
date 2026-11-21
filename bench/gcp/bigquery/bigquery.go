// Package bigquery holds the cases of the BigQuery model (internal/model/builtin/go-gcp.yaml).
// Nothing here connects to Google Cloud.
package bigquery

import (
	"net/http"

	"cloud.google.com/go/bigquery"
)

type Server struct {
	Client *bigquery.Client
}

func param(r *http.Request) string { return r.URL.Query().Get("name") }

func (s *Server) VRun(w http.ResponseWriter, r *http.Request) {
	q := s.Client.Query("SELECT id FROM d.t WHERE name = '" + param(r) + "'")
	_, _ = q.Run(r.Context())
}

func (s *Server) VRead(w http.ResponseWriter, r *http.Request) {
	q := s.Client.Query("SELECT id FROM d.t WHERE name = '" + param(r) + "'")
	_, _ = q.Read(r.Context())
}

func (s *Server) VField(w http.ResponseWriter, r *http.Request) {
	q := s.Client.Query("SELECT 1")
	q.Q = "SELECT id FROM d.t WHERE name = '" + param(r) + "'"
	_, _ = q.Run(r.Context())
}

func (s *Server) FRun(w http.ResponseWriter, r *http.Request) {
	q := s.Client.Query("SELECT id FROM d.t WHERE name = @name")
	q.Parameters = []bigquery.QueryParameter{{Name: "name", Value: param(r)}}
	_, _ = q.Run(r.Context())
}

func (s *Server) DRun(w http.ResponseWriter, r *http.Request) {
	q := s.Client.Query("SELECT 1")
	q.Parameters = []bigquery.QueryParameter{{Name: "name", Value: param(r)}}
	_, _ = q.Read(r.Context())
}

func (s *Server) DLabels(w http.ResponseWriter, r *http.Request) {
	q := s.Client.Query("SELECT 1")
	q.Labels = map[string]string{"who": param(r)}
	_, _ = q.Run(r.Context())
}
