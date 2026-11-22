// Package move_list is a cell of the package storage.
//
// @cell(F/pkg/storage/move-list)
// @shape: a value of a source is the Object of the destination of a move, or the Prefix of a listing; the field of the value is the sink, the field beside it is not
package move_list

import (
	"context"

	"cloud.google.com/go/storage"
	"zoo/mark"
)

func init() {
	mark.Case("F/pkg/storage/move-list/vuln", vuln)
	mark.Case("F/pkg/storage/move-list/fixed", fixed)
	mark.Case("F/pkg/storage/move-list/decoy", decoy)
}

func vuln() { // @vuln
	ctx := context.Background()
	h := (&storage.Client{}).Bucket("constant").Object("constant")
	d := mark.Src("pkg-storage-move-list-vuln-dest")                               // @source(pkg-storage-move-list-vuln-dest)
	p := mark.Src("pkg-storage-move-list-vuln-prefix")                             // @source(pkg-storage-move-list-vuln-prefix)
	h.Move(ctx, storage.MoveObjectDestination{Object: d})                          // @sink(pkg-storage-move-list-vuln-dest)
	(&storage.Client{}).Bucket("constant").Objects(ctx, &storage.Query{Prefix: p}) // @sink(pkg-storage-move-list-vuln-prefix)
}

func fixed() { // @fixed
	ctx := context.Background()
	h := (&storage.Client{}).Bucket("constant").Object("constant")
	d := mark.CleanCmd(mark.Src("pkg-storage-move-list-fixed-dest"))
	p := mark.CleanCmd(mark.Src("pkg-storage-move-list-fixed-prefix"))
	h.Move(ctx, storage.MoveObjectDestination{Object: d})                          // @sink(none)
	(&storage.Client{}).Bucket("constant").Objects(ctx, &storage.Query{Prefix: p}) // @sink(none)
}

func decoy() { // @decoy
	ctx := context.Background()
	h := (&storage.Client{}).Bucket("constant").Object("constant")
	t := mark.Src("pkg-storage-move-list-decoy")                                                                          // @source(pkg-storage-move-list-decoy)
	h.Move(ctx, storage.MoveObjectDestination{Object: "constant", Conditions: &storage.Conditions{}})                     // @sink(none) @nosink(pkg-storage-move-list-decoy)
	(&storage.Client{}).Bucket("constant").Objects(ctx, &storage.Query{Prefix: "constant", StartOffset: t, EndOffset: t}) // @sink(none) @nosink(pkg-storage-move-list-decoy)
	mark.SinkSQL(t)                                                                                                       // @sink(pkg-storage-move-list-decoy)
}
