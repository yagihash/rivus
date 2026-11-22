// Package names is a cell of the package storage.
//
// @cell(F/pkg/storage/names)
// @shape: a value of a source is the name of a bucket and of an object of Cloud Storage; the call that names the resource is the sink
package names

import (
	"cloud.google.com/go/storage"
	"zoo/mark"
)

func init() {
	mark.Case("F/pkg/storage/names/vuln", vuln)
	mark.Case("F/pkg/storage/names/fixed", fixed)
	mark.Case("F/pkg/storage/names/decoy", decoy)
}

func vuln() { // @vuln
	c, _ := storage.NewClient(nil)
	b := mark.Src("pkg-storage-names-vuln-bucket") // @source(pkg-storage-names-vuln-bucket)
	o := mark.Src("pkg-storage-names-vuln-object") // @source(pkg-storage-names-vuln-object)
	h := c.Bucket(b)                               // @sink(pkg-storage-names-vuln-bucket)
	h.Object(o)                                    // @sink(pkg-storage-names-vuln-object)
}

func fixed() { // @fixed
	c, _ := storage.NewClient(nil)
	b := mark.CleanCmd(mark.Src("pkg-storage-names-fixed-bucket"))
	o := mark.CleanCmd(mark.Src("pkg-storage-names-fixed-object"))
	h := c.Bucket(b) // @sink(none)
	h.Object(o)      // @sink(none)
}

func decoy() { // @decoy
	c, _ := storage.NewClient(nil)
	t := mark.Src("pkg-storage-names-decoy") // @source(pkg-storage-names-decoy)
	h := c.Bucket("constant")                // @sink(none) @nosink(pkg-storage-names-decoy)
	h.Object("constant")                     // @sink(none) @nosink(pkg-storage-names-decoy)
	mark.SinkSQL(t)                          // @sink(pkg-storage-names-decoy)
}
