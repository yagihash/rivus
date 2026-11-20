// Package routes has the handlers of the Gogs-shaped web UI and API.
package routes

import (
	"curated/ext/gitmodule"
	"curated/gogs/database"
	"curated/web"
)

// RepoCtx is what a handler has about the repository.
type RepoCtx struct {
	Repository *database.Repository
	GitRepo    *gitmodule.Repository
}

// PutContents updates a file by the tree path of the URL (the wildcard).
func PutContents(c *web.Context, repo *RepoCtx, doer *database.User) {
	treePath := c.Param("*") // @source(gogs_put)
	err := repo.Repository.UpdateRepoFile(doer, database.UpdateRepoFileOptions{
		OldTreeName: treePath,
		NewTreeName: treePath,
		Message:     c.PostForm("message"),
		Content:     "x",
	})
	if err != nil {
		c.JSON(500, "failed")
	}
}

// SettingsGitHooksEdit reads the hook of the name of the URL.
func SettingsGitHooksEdit(c *web.Context, repo *RepoCtx) {
	name := c.Param(":name")                      // @source(gogs_hook)
	hook, err := repo.GitRepo.Hook("hooks", name) // @sink(gogs_hook)
	if err != nil {
		c.JSON(404, "none")
		return
	}
	c.JSON(200, len(hook))
}
