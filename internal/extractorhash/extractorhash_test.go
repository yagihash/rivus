package extractorhash

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tree writes a Rust extractor (the files of the table) under a temporary directory, and returns it.
func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	base := map[string]string{
		"build.rs": "fn main() {}\n", "Cargo.toml": "[package]\n", "Cargo.lock": "# lock\n", "rust-toolchain.toml": "channel = \"x\"\n",
		"src/main.rs": "fn main() {}\n", "src/a/b.rs": "// b\n",
	}
	for k, v := range files {
		base[k] = v
	}
	for k, v := range base {
		p := filepath.Join(dir, filepath.FromSlash(k))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(v), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func hash(t *testing.T, dir string) string {
	t.Helper()
	h, err := Compute("rust", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(h) != 64 {
		t.Fatalf("hash %q is not 64 digits", h)
	}
	return h
}

func TestSameTreeSameHash(t *testing.T) {
	if hash(t, tree(t, nil)) != hash(t, tree(t, nil)) {
		t.Error("the hash of two equal trees differs")
	}
}

func TestContentAndNameChangeTheHash(t *testing.T) {
	base := hash(t, tree(t, nil))
	for name, files := range map[string]map[string]string{
		"content of a source": {"src/main.rs": "fn main() { }\n"},
		"build.rs":            {"build.rs": "fn main() { }\n"},
		"the lock":            {"Cargo.lock": "# other\n"},
		"a new file in src":   {"src/c.rs": ""},
	} {
		if hash(t, tree(t, files)) == base {
			t.Errorf("%s did not change the hash", name)
		}
	}
}

// a name and a content do not run together: moving a byte from a name to a content is another hash
func TestNameAndContentAreSeparated(t *testing.T) {
	a := hash(t, tree(t, map[string]string{"src/x.rs": "yz"}))
	b := hash(t, tree(t, map[string]string{"src/xy.rs": "z"}))
	if a == b {
		t.Error("src/x.rs=yz and src/xy.rs=z have the same hash")
	}
}

func TestLineEndingsDoNotChangeTheHash(t *testing.T) {
	lf := hash(t, tree(t, map[string]string{"src/main.rs": "a\nb\n"}))
	crlf := hash(t, tree(t, map[string]string{"src/main.rs": "a\r\nb\r\n"}))
	if lf != crlf {
		t.Error("CRLF and LF give different hashes")
	}
	if lf == hash(t, tree(t, map[string]string{"src/main.rs": "a\n\nb\n"})) {
		t.Error("an added empty line did not change the hash")
	}
}

func TestLeftOutIsLeftOut(t *testing.T) {
	base := hash(t, tree(t, nil))
	same := tree(t, map[string]string{
		"test/t.rs": "x", "target/debug/x": "x", "README.md": "x", "LICENSE": "x",
	})
	if hash(t, same) != base {
		t.Error("test/, target/, README.md or LICENSE changed the hash")
	}
}

// the order of the files is the order of the paths, not of the walk or of the table: a file in a directory and a file named like it
func TestOrderIsByPath(t *testing.T) {
	dir := tree(t, nil)
	// the hash is computed twice in a directory made in another order of writing
	other := tree(t, map[string]string{"src/zz.rs": "1", "src/aa.rs": "2"})
	if err := os.WriteFile(filepath.Join(dir, "src", "zz.rs"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "aa.rs"), []byte("2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if hash(t, dir) != hash(t, other) {
		t.Error("the order of writing changed the hash")
	}
}

func TestMissingFileAndUnknownLanguage(t *testing.T) {
	dir := tree(t, nil)
	if err := os.Remove(filepath.Join(dir, "build.rs")); err != nil {
		t.Fatal(err)
	}
	if _, err := Compute("rust", dir); err == nil {
		t.Error("a missing build.rs was not an error")
	}
	if _, err := Compute("go", dir); err == nil || !strings.Contains(err.Error(), "go") {
		t.Errorf("a language without an extractor: %v", err)
	}
}

// The version of the table is in what is hashed, and the table is the one that was fixed: this value is of the table `v1` for a fixed tree. A
// change of the table (a file added, the normalisation) fails here: raise TableVersion, and put the new value.
func TestTableV1Fixed(t *testing.T) {
	const want = "dca700068ec4128c6deabb0aba4930408890ef359db99fa478568673735b5a73"
	if got := hash(t, tree(t, nil)); got != want {
		t.Errorf("the hash of the fixed tree is %s, want %s (the table or the way of hashing changed: raise TableVersion)", got, want)
	}
}

// Every entry of the directory of an extractor is in the table or in the exclusions: a file that the build starts to read (an include!, a file
// that build.rs reads) must not stay out of the hash.
func TestExtractorHashTableCoversTheSource(t *testing.T) {
	for _, lang := range Langs() {
		dir := filepath.Join("..", "..", "extractor", lang)
		es, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range es {
			if !Listed(lang, e.Name()) {
				t.Errorf("extractor/%s/%s is in neither the table nor the exclusions of internal/extractorhash: add it to one of them (and raise TableVersion if it is an input of the build)", lang, e.Name())
			}
		}
		for _, f := range tables[lang].Files {
			if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
				t.Errorf("extractor/%s/%s is in the table and not there", lang, f)
			}
		}
		if _, err := Compute(lang, dir); err != nil {
			t.Errorf("%s: %v", lang, err)
		}
	}
}
