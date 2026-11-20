// Package models holds the rows that the storage layer files are named after.
package models

import (
	"context"
	"errors"
)

// User is the signed-in user.
type User struct{ ID int64 }

// Release is a release of a repository.
type Release struct {
	ID    int64
	Title string
}

// Attachment is a file of a release; its path in the storage is made of its UUID.
type Attachment struct {
	ID        int64
	UUID      string
	ReleaseID int64
}

// AttachmentRelativePath is the path of the file under the storage root: the UUID
// is cut into two directory names, and what is left is the file name.
func AttachmentRelativePath(uuid string) string {
	if len(uuid) < 2 {
		return uuid
	}
	return uuid[0:1] + "/" + uuid[1:2] + "/" + uuid
}

// GetAttachmentsByUUIDs reads the rows of the UUIDs that exist (the database).
func GetAttachmentsByUUIDs(ctx context.Context, uuids []string) ([]*Attachment, error) {
	return nil, errors.New("not here")
}

// UpdateRelease writes the row of a release.
func UpdateRelease(ctx context.Context, r *Release) error { return nil }

// DeleteAttachmentRows removes the rows (the files are not touched).
func DeleteAttachmentRows(ctx context.Context, rows []*Attachment) (int, error) {
	return len(rows), nil
}
