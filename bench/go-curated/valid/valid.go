// Package valid checks and cleans text.
package valid

import "strings"

// Ident makes a name safe to use as an SQL identifier (a sanitizer, see rules.yaml).
func Ident(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r == '_' {
			return r
		}
		return -1
	}, s)
}

// Quote makes text safe to put in a statement or in a command (a sanitizer).
func Quote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
