// Package orm is a small ORM in the shape of xorm and gorm.
package orm

// DB is a connection.
type DB struct{ log []string }

// Exec and Where take SQL text (sinks, see rules.yaml).
func (d *DB) Exec(query string, args ...any) error { d.log = append(d.log, query); return nil }
func (d *DB) Where(query string, args ...any) *DB  { d.log = append(d.log, query); return d }

// Find fills dst with rows. Data that comes from the store is not a source here.
func (d *DB) Find(dst any) error { return nil }

// Create stores a row.
func (d *DB) Create(row any) error { return nil }
