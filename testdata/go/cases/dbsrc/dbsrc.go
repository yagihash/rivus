// What comes back from a data store is a source of its own kind.
package dbsrc

import (
	"database/sql"

	"example.com/cases/lib"
)

type Row struct {
	Name string
	Note string
}

type Other struct{ Name string }

// out: the call fills the pointee of its argument.
func FillString() {
	var s string
	_ = lib.Fill(&s) // @source(fill)
	lib.Sink(s)      // @sink(fill)
}

func FillStruct() {
	var r Other
	_ = lib.FillAny(&r) // @source(fillany)
	lib.Sink(r.Name)    // @sink(fillany)
}

func FillSlice() {
	var rows []Other
	_ = lib.FillAny(&rows) // @source(fillslice)
	for _, r := range rows {
		lib.Sink(r.Name) // @sink(fillslice)
	}
}

func FillLeavesOthersAlone() {
	var s, t string
	_ = lib.Fill(&s) // @source(fill2)
	lib.Sink(s)      // @sink(fill2)
	lib.Sink(t)      // @sink(none)
}

// any field of a type.
func TypeFields(r *Row) {
	name := r.Name // @source(typefield)
	lib.Sink(name) // @sink(typefield)
	note := r.Note // @source(typefield2)
	lib.Sink(note) // @sink(typefield2)
}

func OtherTypeIsNotASource(o *Other) {
	lib.Sink(o.Name) // @sink(none)
}

// a pure call that is a boundary of the data store: no flow, and it is recorded.
func Boundary() {
	s := lib.StoreRead()
	lib.Sink(s) // @sink(none)
}

// the real thing: the destinations of Scan are the elements of its variadic slice.
func ScanRows(rows *sql.Rows) {
	var name string
	var n int
	_ = rows.Scan(&name, &n) // @source(scan)
	lib.Sink(name)           // @sink(scan)
}
