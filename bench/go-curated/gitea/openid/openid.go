// Package openid is the wrapper of the library, with the cache and the store of
// the nonces that the program keeps.
package openid

import "curated/ext/openidgo"

type discoveryCache struct{}
type nonceStore struct{}

var (
	cache = &discoveryCache{}
	store = &nonceStore{}
)

// Verify checks the assertion that came back from the provider.
func Verify(fullURL string) (id string, err error) {
	return openidgo.Verify(fullURL, cache, store) // @sink(openid_verify)
}
