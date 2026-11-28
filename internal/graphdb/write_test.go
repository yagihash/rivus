package graphdb_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yagihash/rivus/internal/graphdb"
	"github.com/yagihash/rivus/internal/sg"
)

// The path of the file goes into a URI: a # ? or % in a directory name must not cut it or be read as an escape. Before, the data
// went to a file named after the part before the # while the empty temporary file was moved into place (no error).
func TestWriteSpecialCharactersInPath(t *testing.T) {
	for _, name := range []string{"a#b", "a?b", "a%41b", "a b"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, name, "g.db")
			if _, err := graphdb.Write(path, &graphdb.Input{Graph: &sg.Program{}}); err != nil {
				t.Fatal(err)
			}
			fi, err := os.Stat(path)
			if err != nil || fi.Size() == 0 {
				t.Fatalf("the graph file is empty or missing: %v", err)
			}
			d, err := graphdb.Open(path)
			if err != nil {
				t.Fatalf("what was written cannot be opened: %v", err)
			}
			d.Close()
			ents, err := os.ReadDir(root)
			if err != nil || len(ents) != 1 || ents[0].Name() != name {
				t.Errorf("stray files next to the directory: %v (%v)", ents, err)
			}
		})
	}
}
