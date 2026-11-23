package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A tracked file must not be a built executable or larger than binMaxSize: two accidents committed a built program (a Go
// executable of 9 MB, one of 15 MB). binAllow is the list of the files that are allowed, each with its reason (it starts empty).
const binMaxSize = 1 << 20

var binAllow = map[string]string{}

// binSkipped: for the walk without git only; build output and the work of other sessions are not part of the repository.
func binSkipped(rel string) bool {
	for _, part := range strings.Split(rel, "/") {
		switch part {
		case ".git", "node_modules", "target", ".dart_tool":
			return true
		}
	}
	return rel == ".claude"
}

// binProblem: why the file (its first bytes head and its size) is refused, or "".
func binProblem(head []byte, size int64) string {
	for _, m := range [][]byte{
		{0xfe, 0xed, 0xfa, 0xce}, {0xfe, 0xed, 0xfa, 0xcf}, {0xca, 0xfe, 0xba, 0xbe},
		{0xcf, 0xfa, 0xed, 0xfe}, {0xce, 0xfa, 0xed, 0xfe}, {0x7f, 'E', 'L', 'F'},
	} {
		if bytes.HasPrefix(head, m) {
			return "starts like an executable (Mach-O or ELF)"
		}
	}
	// PE: "MZ", and the offset at 0x3c points to "PE\0\0".
	if len(head) >= 0x40 && head[0] == 'M' && head[1] == 'Z' {
		off := int(head[0x3c]) | int(head[0x3d])<<8 | int(head[0x3e])<<16 | int(head[0x3f])<<24
		if off >= 0 && off+4 <= len(head) && bytes.Equal(head[off:off+4], []byte("PE\x00\x00")) {
			return "starts like an executable (PE)"
		}
	}
	if size > binMaxSize {
		return "is larger than 1 MB"
	}
	return ""
}

func TestNoBinariesOrLargeFiles(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	rels, err := repoFiles(root, binSkipped)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range rels {
		if _, ok := binAllow[rel]; ok {
			continue
		}
		fi, err := os.Lstat(filepath.Join(root, rel))
		if err != nil || !fi.Mode().IsRegular() {
			continue // deleted in the work tree, a symlink or a submodule
		}
		f, err := os.Open(filepath.Join(root, rel))
		if err != nil {
			continue
		}
		head := make([]byte, 4096)
		n, _ := f.Read(head)
		f.Close()
		if why := binProblem(head[:n], fi.Size()); why != "" {
			t.Errorf("%s %s (%d bytes). Add it to .gitignore and `git rm --cached` it, or add it to binAllow in cmd/rivus/binguard_test.go with a reason", rel, why, fi.Size())
		}
	}
}

func TestBinProblem(t *testing.T) {
	t.Parallel()
	pe := make([]byte, 0x80)
	pe[0], pe[1], pe[0x3c] = 'M', 'Z', 0x40
	copy(pe[0x40:], "PE\x00\x00")
	mzOnly := make([]byte, 0x80)
	mzOnly[0], mzOnly[1] = 'M', 'Z'
	for _, c := range []struct {
		name string
		head []byte
		size int64
		bad  bool
	}{
		{"text", []byte("package main\n"), 100, false},
		{"empty", nil, 0, false},
		{"macho64", []byte{0xcf, 0xfa, 0xed, 0xfe, 7, 0}, 100, true},
		{"macho32", []byte{0xfe, 0xed, 0xfa, 0xce}, 100, true},
		{"universal", []byte{0xca, 0xfe, 0xba, 0xbe}, 100, true},
		{"elf", []byte("\x7fELF\x02\x01"), 100, true},
		{"pe", pe, 100, true},
		{"mz without PE header", mzOnly, 100, false},
		{"exactly 1 MB", []byte("x"), binMaxSize, false},
		{"over 1 MB", []byte("x"), binMaxSize + 1, true},
	} {
		if got := binProblem(c.head, c.size) != ""; got != c.bad {
			t.Errorf("%s: refused = %v, want %v", c.name, got, c.bad)
		}
	}
}
