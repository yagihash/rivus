package services

import (
	"reflect"

	"curated/orm"
	"curated/web"
)

// What is learned from the type of a value is not the data of the value: a library that
// keeps a cache of what it knows of each type (protobuf, encoding/json, gorm) is read by every
// function that uses the cache. The type of a value is not followed as taint.

// TypeNameInQuery puts the name of the type of a request value in a statement.
func TypeNameInQuery(c *web.Context, db *orm.DB) error {
	q := c.Query("q")                                                              // @source(typename)
	if err := db.Exec("SELECT '" + reflect.TypeOf(q).String() + "'"); err != nil { // @sink(none)
		return err
	}
	return db.Exec("SELECT '" + q + "'") // @sink(typename)
}

var keptTypes = map[string]reflect.Type{}

// KeepType remembers the type of a request value.
func KeepType(c *web.Context, db *orm.DB) error {
	v := c.Query("q") // @source(kept)
	keptTypes["last"] = reflect.TypeOf(v)
	return db.Exec("SELECT '" + v + "'") // @sink(kept)
}

// UseKeptType reads the cache of types in another function.
func UseKeptType(db *orm.DB) error {
	return db.Exec(keptTypes["last"].String()) // @sink(none)
}

// ThroughValue goes through a reflect.Value and back to the data: the data is followed.
func ThroughValue(c *web.Context, db *orm.DB) error {
	q := c.Query("q") // @source(reflectvalue)
	v := reflect.ValueOf(q)
	return db.Exec(v.Interface().(string)) // @sink(reflectvalue)
}
