// Package goth is the shape of the OAuth2 library's user: what the provider says
// about the user is in its fields.
package goth

// User is the user the provider told us about.
type User struct {
	Provider  string
	Name      string
	AvatarURL string
	RawData   map[string]any
}
