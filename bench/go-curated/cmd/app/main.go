// Command app wires the pieces and serves.
package main

import (
	"curated/api"
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
	e.Serve("GET", "/users/direct", &web.Request{})
}
