// Package storage is the storage layer: an interface with two implementations,
// and a package variable that holds the one that is configured.
package storage

import (
	"io"
	"os"
	"path/filepath"
)

// ObjectStorage is where the files of the attachments, the avatars and the
// packages are kept; the sinks of the rules are its methods.
type ObjectStorage interface {
	Open(path string) (io.ReadCloser, error)
	Save(path string, r io.Reader, size int64) (int64, error)
	Delete(path string) error
}

// Attachments is the storage of the attachments of the releases and the issues.
var Attachments ObjectStorage

// LocalStorage keeps the files in a directory.
type LocalStorage struct{ Root string }

func (l *LocalStorage) Open(p string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(l.Root, p))
}
func (l *LocalStorage) Save(p string, r io.Reader, size int64) (int64, error) {
	f, err := os.Create(filepath.Join(l.Root, p))
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return io.Copy(f, r)
}

// the local storage removes the file under its root (a sink of the built-in rules too)
func (l *LocalStorage) Delete(p string) error { return os.Remove(filepath.Join(l.Root, p)) } // @sink(release_del) @sink(tx_capture)

// MinioStorage keeps them in a bucket.
type MinioStorage struct{ Bucket string }

func (m *MinioStorage) Open(p string) (io.ReadCloser, error)                  { return nil, nil }
func (m *MinioStorage) Save(p string, r io.Reader, size int64) (int64, error) { return 0, nil }
func (m *MinioStorage) Delete(p string) error                                 { return nil }

// Init picks the implementation of the settings.
func Init(kind string) {
	if kind == "minio" {
		Attachments = &MinioStorage{Bucket: "gitea"}
		return
	}
	Attachments = &LocalStorage{Root: "data/attachments"}
}
