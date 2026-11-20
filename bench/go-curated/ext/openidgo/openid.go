// Package openidgo is a dependency: a library that fetches the discovery document
// of the identity provider the assertion names. Sinks inside dependencies are not
// looked for, so the rules name its entry point (as for yohcop/openid-go).
package openidgo

// Verify checks the assertion at the URL (and posts to the endpoint it names).
func Verify(assertionURL string, cache any, nonces any) (string, error) {
	return assertionURL, nil
}
