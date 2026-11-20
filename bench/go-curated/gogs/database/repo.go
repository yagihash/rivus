// Package database is the data layer of the Gogs-shaped project: the repository
// and the edits of its files.
package database

import (
	"os"
	"path"
)

// Repository is a repository.
type Repository struct{ LocalPath string }

// User is the signed-in user.
type User struct{ Name string }

// UpdateRepoFileOptions is what an edit of a file takes.
type UpdateRepoFileOptions struct {
	OldTreeName string
	NewTreeName string
	Message     string
	Content     string
}

// UpdateRepoFile writes the file of the tree path in the local copy of the repository.
func (r *Repository) UpdateRepoFile(doer *User, opts UpdateRepoFileOptions) (err error) {
	localPath := r.LocalPath
	oldFilePath := path.Join(localPath, opts.OldTreeName)
	filePath := path.Join(localPath, opts.NewTreeName)
	_ = oldFilePath
	if err = os.MkdirAll(path.Dir(filePath), os.ModePerm); err != nil { // @sink(gogs_put)
		return err
	}
	return os.WriteFile(filePath, []byte(opts.Content), 0600) // @sink(gogs_put)
}
