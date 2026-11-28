package graphdb_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yagihash/rivus/internal/graphdb"
)

func hashOf(t *testing.T, path string) string {
	t.Helper()
	recs, err := graphdb.HashFiles([]graphdb.Entry{{Path: "p", Abs: path, Kind: "tree"}}, time.Time{})
	if err != nil || len(recs) != 1 {
		t.Fatalf("%v, %v", recs, err)
	}
	return recs[0].SHA256
}

// A link is hashed with what it leads to: the extractor reads the file through the link, and the listing of the tree does not follow
// it, so that a change of the target (a file outside the tree) was not a change of the key, and an old index was taken as fresh.
func TestHashFilesFollowsALinkToItsContent(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "outside", "shared.dart")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "tree", "lib", "shared.dart")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skip("no symbolic links here:", err)
	}
	before := hashOf(t, link)
	if err := os.WriteFile(target, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	if after := hashOf(t, link); after == before {
		t.Error("the content of the file that the link leads to changed, and the hash of the link did not")
	}
	// a link to nothing is a hash too, and it is not the one of a link that leads to a file
	os.Remove(target)
	gone := hashOf(t, link)
	if err := os.WriteFile(target, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	if hashOf(t, link) == gone {
		t.Error("a link to nothing and the link to a file have the same hash")
	}
}
