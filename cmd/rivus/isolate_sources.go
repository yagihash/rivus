package main

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// The places that the fetch stage gets from without a flag: the registry of crates.io, and repositories on https whose
// host is a public name (an IP address, localhost, and the names that are used inside a network are not). Where the fetch
// goes is chosen by Cargo.lock of the project, and the stage has a network.
var cratesIOSources = map[string]bool{
	"registry+https://github.com/rust-lang/crates.io-index": true,
	"registry+sparse+https://index.crates.io/":              true,
	"sparse+https://index.crates.io/":                       true,
}

// lockSources are the `source` values of Cargo.lock, in the order they appear, without the repeated ones.
func lockSources(lock string) []string {
	var out []string
	seen := map[string]bool{}
	for _, l := range strings.Split(lock, "\n") {
		l = strings.TrimSpace(l)
		v, ok := strings.CutPrefix(l, "source")
		if !ok {
			continue
		}
		v, ok = strings.CutPrefix(strings.TrimSpace(v), "=")
		if !ok {
			continue
		}
		v = strings.Trim(strings.TrimSpace(v), "\"'")
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// internalHost says whether a host names a place inside a network (or no name at all): an IP address in any of its
// spellings, localhost, a name without a dot, and the suffixes that are used for the inside of a network.
func internalHost(host string) bool {
	h := strings.ToLower(strings.TrimSuffix(strings.Trim(host, "[]"), "."))
	if h == "" || net.ParseIP(h) != nil || !strings.Contains(h, ".") {
		return true
	}
	last := h[strings.LastIndex(h, ".")+1:]
	if last == "" || last[0] >= '0' && last[0] <= '9' || strings.HasPrefix(last, "0x") { // 127.1, 0x7f.1, 2130706433
		return true
	}
	for _, suf := range []string{".localhost", ".local", ".localdomain", ".internal", ".intranet", ".lan", ".home", ".home.arpa", ".corp", ".private", ".test", ".invalid", ".example"} {
		if strings.HasSuffix(h, suf) {
			return true
		}
	}
	return false
}

// checkFetchSources says which of the sources of Cargo.lock the fetch stage would get from although they are not allowed:
// the ones that are not crates.io and not an https repository on a public name, unless the host is named in allow
// (-allow-fetch-host).
func checkFetchSources(lock string, allow []string) error {
	var refused []string
	for _, src := range lockSources(lock) {
		if cratesIOSources[src] {
			continue
		}
		rest, isRepo := strings.CutPrefix(src, "git+")
		if !isRepo {
			rest = strings.TrimPrefix(strings.TrimPrefix(src, "registry+"), "sparse+")
		}
		u, err := url.Parse(rest)
		host := ""
		if err == nil {
			host = u.Hostname()
		}
		allowed := false
		for _, a := range allow {
			if host != "" && strings.EqualFold(strings.Trim(a, "[]"), host) {
				allowed = true
			}
		}
		if allowed {
			continue
		}
		if isRepo && err == nil && u.Scheme == "https" && u.User == nil && !internalHost(host) {
			continue
		}
		refused = append(refused, src)
	}
	if len(refused) == 0 {
		return nil
	}
	return fmt.Errorf("Cargo.lock names places to fetch from that are not allowed (the fetch stage has a network, and where it goes is chosen by the project; "+
		"by default it gets from crates.io and from https repositories on public host names only). Not allowed:\n  %s\n"+
		"If you have looked at them and want them, name the hosts with -allow-fetch-host HOST (repeatable); or fetch the dependencies yourself and use -no-fetch",
		strings.Join(refused, "\n  "))
}
