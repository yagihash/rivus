package routers

import (
	"curated/gitea/context"
	"curated/gitea/openid"
)

const appURL = "https://git.example.com/"

// SignInOpenIDVerify takes the URL of the request that the provider sent the user
// back with, and has the library verify it.
func SignInOpenIDVerify(ctx *context.Context) { // @source(openid_verify)
	fullURL := appURL + ctx.Req.URL.String()[1:]
	id, err := openid.Verify(fullURL)
	if err != nil {
		ctx.ServerError(err)
		return
	}
	_ = id
}
