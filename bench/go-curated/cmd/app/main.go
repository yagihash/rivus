// Command app wires the pieces and serves.
package main

import (
	"curated/api"
	"curated/gitea/context"
	"curated/gitea/goth"
	grouters "curated/gitea/routers"
	"curated/gitea/storage"
	gogsroutes "curated/gogs/routes"
	gvaapi "curated/gva/api"
	"curated/orm"
	"curated/routers"
	"curated/services"
	"curated/web"
)

func main() {
	db := &orm.DB{}
	searchers := services.NewSearchers(db)
	svc := services.NewService(searchers[0], &services.SQLUsers{DB: db})
	h := &api.Handlers{DB: db, Svc: svc, Searcher: searchers}
	e := web.New()
	routers.Register(e, h, &api.Base{DB: db}, &api.Cache{})
	storage.Init("local")
	gctx := &context.Context{}
	grouters.EditReleasePost(gctx)
	grouters.SignInOpenIDVerify(gctx)
	grouters.HandleSignInSync(gctx, goth.User{})
	grouters.DeleteInTx(gctx)
	grouters.RunInTx(gctx)
	var ac gvaapi.AutoCodeAPI
	ac.AutoPlug(&web.Context{})
	gogsroutes.PutContents(&web.Context{}, &gogsroutes.RepoCtx{}, nil)
	gogsroutes.SettingsGitHooksEdit(&web.Context{}, &gogsroutes.RepoCtx{})
	e.Serve("GET", "/users/direct", &web.Request{})
}
