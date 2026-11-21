//go:build gcpexec

package opaque

import "cloud.google.com/go/spanner"

func Move(s *spanner.Statement) {
	if v, ok := s.Params["name"].(string); ok {
		s.SQL = v
	}
}

func Rewrite(s spanner.Statement) spanner.Statement {
	Move(&s)
	return s
}
