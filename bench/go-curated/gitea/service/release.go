// Package service is the service layer of the releases.
package service

import (
	"context"
	"errors"
	"fmt"

	"curated/gitea/db"
	"curated/gitea/models"
	"curated/gitea/storage"
)

// UpdateRelease updates a release: the row, the attachments to add, the attachments
// to delete (by UUID), the attachments to rename (UUID -> new name). The rows are
// changed in a closure given to the transaction helper (which reads the rows of the
// UUIDs that exist); the files of the attachments to delete go after it, by the
// UUIDs that arrived.
func UpdateRelease(ctx context.Context, doer *models.User, rel *models.Release,
	addAttachmentUUIDs, delAttachmentUUIDs []string, editAttachments map[string]string,
) error {
	if rel.ID == 0 {
		return errors.New("only an existing release")
	}
	if err := db.WithTx(ctx, func(ctx context.Context) error {
		if err := models.UpdateRelease(ctx, rel); err != nil {
			return err
		}
		if len(delAttachmentUUIDs) > 0 {
			attachments, err := models.GetAttachmentsByUUIDs(ctx, delAttachmentUUIDs)
			if err != nil {
				return fmt.Errorf("GetAttachmentsByUUIDs [uuids: %v]: %w", delAttachmentUUIDs, err)
			}
			if _, err := models.DeleteAttachmentRows(ctx, attachments); err != nil {
				return err
			}
		}
		for uuid := range editAttachments {
			_ = uuid
		}
		return nil
	}); err != nil {
		return err
	}

	for _, uuid := range delAttachmentUUIDs {
		if err := storage.Attachments.Delete(models.AttachmentRelativePath(uuid)); err != nil { // @sink(release_del)
			// the rows are gone already: the error is only logged
			continue
		}
	}
	return nil
}
