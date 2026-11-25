package scripts

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot is the root of the repository (the folder above scripts/).
func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Dir(wd)
}

// sourceTarball runs `build-release-archives.sh --source-only 9.9.9 HEAD` in a scratch copy of the layout (dist/ goes to
// the folder it runs in, so it runs inside a temporary clone of HEAD) and returns the sha256 and the member names.
func sourceTarball(t *testing.T, clone string) (string, []string) {
	t.Helper()
	cmd := exec.Command("bash", filepath.Join(repoRoot(t), "scripts", "build-release-archives.sh"), "--source-only", "9.9.9", "HEAD")
	cmd.Dir = clone
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("source-only: %v\n%s", err, out)
	}
	path := filepath.Join(clone, "dist", "rivus_9.9.9_source.tar.gz")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	zr, err := gzip.NewReader(strings.NewReader(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	tr := tar.NewReader(zr)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeXGlobalHeader { // git archive's header holding the commit id
			continue
		}
		names = append(names, h.Name)
	}
	return hex.EncodeToString(sum[:]), names
}

// The source tarball of a ref is the same bytes every time, has rivus-<version>/ as its top folder, and holds what the
// formula's install needs (scripts/extractor-hash, internal/extractorhash, extractor/, cmd/rivus).
func TestSourceTarballReproducible(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	clone := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = clone
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("clone", "-q", "--no-local", "--depth", "1", "file://"+repoRoot(t), ".")
	// a clone of the committed HEAD only: the working tree of a PR under test does not matter
	sum1, names := sourceTarball(t, clone)
	sum2, _ := sourceTarball(t, clone)
	if sum1 != sum2 {
		t.Fatalf("the sha256 differs between two builds: %s %s", sum1, sum2)
	}
	for _, n := range names {
		if !strings.HasPrefix(n, "rivus-9.9.9/") {
			t.Fatalf("member %q is not under rivus-9.9.9/", n)
		}
	}
	for _, want := range []string{"rivus-9.9.9/scripts/extractor-hash/", "rivus-9.9.9/internal/extractorhash/", "rivus-9.9.9/extractor/", "rivus-9.9.9/cmd/rivus/"} {
		found := false
		for _, n := range names {
			if strings.HasPrefix(n, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("the source tarball has nothing under %s", want)
		}
	}
}
