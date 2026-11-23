// Package signed is a cell of the package storage.
//
// @cell(F/pkg/storage/signed)
// @shape: a value of a source is the bucket or the object of a signed URL, or of a signed post policy; the call that signs is the sink
package signed

import (
	"cloud.google.com/go/storage"
	"zoo/mark"
)

func init() {
	mark.Case("F/pkg/storage/signed/vuln", vuln)
	mark.Case("F/pkg/storage/signed/fixed", fixed)
	mark.Case("F/pkg/storage/signed/decoy", decoy)
}

func vuln() { // @vuln
	c, _ := storage.NewClient(nil)
	b := mark.Src("pkg-storage-signed-vuln-bucket") // @source(pkg-storage-signed-vuln-bucket)
	o := mark.Src("pkg-storage-signed-vuln-object") // @source(pkg-storage-signed-vuln-object)
	p := mark.Src("pkg-storage-signed-vuln-policy") // @source(pkg-storage-signed-vuln-policy)
	m := mark.Src("pkg-storage-signed-vuln-method") // @source(pkg-storage-signed-vuln-method)
	opts := &storage.SignedURLOptions{Method: "GET"}
	storage.SignedURL(b, "constant", opts)                  // @sink(pkg-storage-signed-vuln-bucket)
	storage.SignedURL("constant", o, opts)                  // @sink(pkg-storage-signed-vuln-object)
	c.Bucket("constant").SignedURL(m, opts)                 // @sink(pkg-storage-signed-vuln-method)
	c.Bucket("constant").GenerateSignedPostPolicyV4(p, nil) // @sink(pkg-storage-signed-vuln-policy)
}

func fixed() { // @fixed
	c, _ := storage.NewClient(nil)
	o := mark.CleanResource(mark.Src("pkg-storage-signed-fixed"))
	opts := &storage.SignedURLOptions{Method: "GET"}
	storage.SignedURL("constant", o, opts)                  // @sink(none)
	c.Bucket("constant").SignedURL(o, opts)                 // @sink(none)
	c.Bucket("constant").GenerateSignedPostPolicyV4(o, nil) // @sink(none)
}

func decoy() { // @decoy
	c, _ := storage.NewClient(nil)
	t := mark.Src("pkg-storage-signed-decoy") // @source(pkg-storage-signed-decoy)
	opts := &storage.SignedURLOptions{GoogleAccessID: t, Method: "GET"}
	storage.SignedURL("constant", "constant", opts)                  // @sink(none) @nosink(pkg-storage-signed-decoy)
	c.Bucket("constant").SignedURL("constant", opts)                 // @sink(none) @nosink(pkg-storage-signed-decoy)
	c.Bucket("constant").GenerateSignedPostPolicyV4("constant", nil) // @sink(none) @nosink(pkg-storage-signed-decoy)
	mark.SinkSQL(t)                                                  // @sink(pkg-storage-signed-decoy)
}
