// Package routers has the handlers of the Gitea-shaped web UI.
package routers

import (
	"strings"

	"curated/gitea/context"
	"curated/gitea/models"
	"curated/gitea/service"
)

// EditReleasePost edits a release: the keys of the form that name an attachment
// to delete ("attachment-del-<uuid>") are collected, raw, in a slice, in a loop.
func EditReleasePost(ctx *context.Context) { // @source(release_del)
	const delPrefix = "attachment-del-"
	const editPrefix = "attachment-edit-"
	var addAttachmentUUIDs, delAttachmentUUIDs []string
	editAttachments := make(map[string]string) // uuid -> new name
	for k, v := range ctx.Req.Form {
		if strings.HasPrefix(k, delPrefix) && v[0] == "true" {
			delAttachmentUUIDs = append(delAttachmentUUIDs, k[len(delPrefix):])
		} else if strings.HasPrefix(k, editPrefix) {
			editAttachments[k[len(editPrefix):]] = v[0]
		}
	}
	rel := &models.Release{ID: 1}
	if err := service.UpdateRelease(ctx.Req.Context(), ctx.Doer, rel, addAttachmentUUIDs, delAttachmentUUIDs, editAttachments); err != nil {
		ctx.JSONErrorAuto(err)
		return
	}
	ctx.JSONRedirect("/releases")
}
