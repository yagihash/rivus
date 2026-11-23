// Package entry is a cell of the packages archive/tar and archive/zip.
//
// @cell(F/pkg/archive/entry)
// @shape: the name of an entry of an archive (and the target of a link in a tar) is written by whoever made the archive, so it is a source; the name is the entry point of zip slip. A name that went through the path sanitizer is not a finding, and a field that is not a name (the size, the mode) is not a source
package entry

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"io"

	"zoo/mark"
)

func init() {
	mark.Case("F/pkg/archive/entry/vuln", vuln)
	mark.Case("F/pkg/archive/entry/fixed", fixed)
	mark.Case("F/pkg/archive/entry/decoy", decoy)
}

// the name that the archive was made with: the marker of the tag at run time (mark.NewIn is not a source for the analysis, as the
// file of an archive is not one: the model of archive/tar and archive/zip is what is under test)
func named(tag string) string {
	in := mark.NewIn(tag)
	return in.Value()
}

func tarReader(tag string) *tar.Reader {
	var b bytes.Buffer
	w := tar.NewWriter(&b)
	_ = w.WriteHeader(&tar.Header{Name: named(tag), Linkname: named(tag), Typeflag: tar.TypeSymlink, Mode: 0o644, Size: 0})
	_ = w.Close()
	return tar.NewReader(&b)
}

func zipReader(tag string) *zip.Reader {
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	f, _ := w.Create(named(tag))
	_, _ = io.WriteString(f, "x")
	_ = w.Close()
	r, _ := zip.NewReader(bytes.NewReader(b.Bytes()), int64(b.Len()))
	return r
}

func vuln() { // @vuln
	{
		h, _ := tarReader("pkg-archive-entry-vuln-tar-name").Next()
		mark.SinkPath("/tmp", h.Name) // @source(pkg-archive-entry-vuln-tar-name) @sink(pkg-archive-entry-vuln-tar-name)
	}
	{
		h, _ := tarReader("pkg-archive-entry-vuln-tar-linkname").Next()
		mark.SinkPath("/tmp", h.Linkname) // @source(pkg-archive-entry-vuln-tar-linkname) @sink(pkg-archive-entry-vuln-tar-linkname)
	}
	{
		for _, f := range zipReader("pkg-archive-entry-vuln-zip-file-name").File {
			mark.SinkPath("/tmp", f.Name) // @source(pkg-archive-entry-vuln-zip-file-name) @sink(pkg-archive-entry-vuln-zip-file-name)
		}
	}
	{
		for _, f := range zipReader("pkg-archive-entry-vuln-zip-header-name").File {
			h := f.FileHeader
			mark.SinkPath("/tmp", h.Name) // @source(pkg-archive-entry-vuln-zip-header-name) @sink(pkg-archive-entry-vuln-zip-header-name)
		}
	}
}

func fixed() { // @fixed
	{
		h, _ := tarReader("pkg-archive-entry-fixed").Next()
		mark.SinkPath("/tmp", mark.CleanPath(h.Name))     // @sink(none)
		mark.SinkPath("/tmp", mark.CleanPath(h.Linkname)) // @sink(none)
	}
	{
		for _, f := range zipReader("pkg-archive-entry-fixed").File {
			mark.SinkPath("/tmp", mark.CleanPath(f.Name)) // @sink(none)
		}
	}
}

func decoy() { // @decoy
	{
		h, _ := tarReader("pkg-archive-entry-decoy-tar").Next()
		name := h.Name                 // @source(pkg-archive-entry-decoy-tar)
		mark.SinkPath("/tmp", h.Uname) // @sink(none) @nosink(pkg-archive-entry-decoy-tar)
		mark.SinkPath("/tmp", name)    // @sink(pkg-archive-entry-decoy-tar)
	}
	{
		{
			h := zipReader("pkg-archive-entry-decoy-zip").File[0].FileHeader
			name := h.Name                   // @source(pkg-archive-entry-decoy-zip)
			mark.SinkPath("/tmp", h.Comment) // @sink(none) @nosink(pkg-archive-entry-decoy-zip)
			mark.SinkPath("/tmp", name)      // @sink(pkg-archive-entry-decoy-zip)
		}
	}
}
