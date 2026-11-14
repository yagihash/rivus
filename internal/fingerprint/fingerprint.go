// Package fingerprint defines the fingerprint keys of sites and findings and
// the hash used to compute them. See docs/ARCHITECTURE.md section 4.6.
package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// Keys. The suffix after @ is the version of the algorithm.
const (
	Structural = "structural@1" // identity: function + symbol + ordinal
	Content    = "content@1"    // identity: function + symbol + expression hash
	Expr       = "expr@1"       // change detection: normalized expression
	Body       = "body@1"       // change detection: normalized function body
	Path       = "path@1"       // change detection (findings): the call chain
)

// IDPrefix is prepended to the structural fingerprint to form a site ID.
const IDPrefix = "s1:"

// Hash returns a short stable hash of the parts. The parts are separated
// unambiguously, so ("a","bc") and ("ab","c") hash differently.
func Hash(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)[:10])
}

// Combine merges two fingerprint maps key by key, as done for findings:
// Combined[k] = Hash(a[k], b[k]) for every key present in both.
func Combine(a, b map[string]string) map[string]string {
	out := map[string]string{}
	for k, av := range a {
		if bv, ok := b[k]; ok {
			out[k] = Hash(av, bv)
		}
	}
	return out
}

// SortedKeys returns the keys of m in order.
func SortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Join is a convenience for hashing a sequence of identifiers.
func Join(parts ...string) string { return strings.Join(parts, "|") }
