package routers

import (
	"io"
	"net/http"
	"time"

	"curated/gitea/context"
	"curated/gitea/goth"
	"curated/gitea/models"
)

func avatarHTTPClient() *http.Client { return &http.Client{Timeout: 30 * time.Second} }

// oauth2SignInSync syncs the user with what the provider says. The user of the
// provider comes in by value (a struct that the library built from the claims).
func oauth2SignInSync(ctx *context.Context, authSourceID int64, u *models.User, gothUser goth.User) { // @source(oauth_avatar)
	oauth2UpdateAvatarIfNeed(ctx, gothUser.AvatarURL, u)
	if gothUser.Name == "" {
		ctx.ServerError(nil)
	}
}

// oauth2UpdateAvatarIfNeed fetches the avatar at the URL of the provider.
func oauth2UpdateAvatarIfNeed(ctx *context.Context, avatarURL string, u *models.User) {
	if len(avatarURL) == 0 {
		return
	}
	req, err := http.NewRequestWithContext(ctx.Req.Context(), http.MethodGet, avatarURL, nil) // @sink(oauth_avatar)
	if err != nil {
		return
	}
	resp, err := avatarHTTPClient().Do(req)
	if err != nil {
		return
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return
	}
	_ = data
}

// HandleSignInSync is the callback after the provider has said who the user is.
func HandleSignInSync(ctx *context.Context, gothUser goth.User) {
	oauth2SignInSync(ctx, 1, ctx.Doer, gothUser)
}
