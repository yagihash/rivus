package main

import (
	"fmt"
	"net/url"
	"strings"
)

// The only place that the fetch stage gets from without a flag: the registry of crates.io. Where the fetch goes is chosen
// by Cargo.lock of the project, and the stage has a network. A git dependency is not allowed by default, on any host
// (github.com included): the repository can name more places that Cargo.lock does not show (the submodules in its
// .gitmodules, which cargo fetches too), and a public host can redirect (30x) to an address inside a network.
var cratesIOSources = map[string]bool{
	"registry+https://github.com/rust-lang/crates.io-index": true,
	"registry+sparse+https://index.crates.io/":              true,
	"sparse+https://index.crates.io/":                       true,
}

// lockPackage is a [[package]] of Cargo.lock, the parts that say where it comes from.
type lockPackage struct{ name, version, source string }

// lockPackages are the packages of Cargo.lock that have a source, in order, without a repeated (name, version, source).
func lockPackages(lock string) []lockPackage {
	var out []lockPackage
	seen := map[lockPackage]bool{}
	var cur lockPackage
	flush := func() {
		if cur.source != "" && !seen[cur] {
			seen[cur] = true
			out = append(out, cur)
		}
		cur = lockPackage{}
	}
	for _, l := range strings.Split(lock, "\n") {
		l = strings.TrimSpace(l)
		if l == "[[package]]" {
			flush()
			continue
		}
		k, v, ok := strings.Cut(l, "=")
		if !ok {
			continue
		}
		v = strings.Trim(strings.TrimSpace(v), "\"'")
		switch strings.TrimSpace(k) {
		case "name":
			cur.name = v
		case "version":
			cur.version = v
		case "source":
			cur.source = v
		}
	}
	flush()
	return out
}

// sourceHost is the host a source of Cargo.lock is on ("" when it has none).
func sourceHost(src string) string {
	rest, isRepo := strings.CutPrefix(src, "git+")
	if !isRepo {
		rest = strings.TrimPrefix(strings.TrimPrefix(src, "registry+"), "sparse+")
	}
	u, err := url.Parse(rest)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// describeSource says, for a refused package, which repository at which commit (or which registry) it comes from.
func describeSource(p lockPackage) string {
	who := p.name + " " + p.version + ": "
	rest, isRepo := strings.CutPrefix(p.source, "git+")
	if !isRepo {
		return who + p.source
	}
	repo, commit, _ := strings.Cut(rest, "#")
	if commit == "" {
		return who + repo
	}
	return who + repo + " at commit " + commit
}

// checkFetchSources says which of the sources of Cargo.lock the fetch stage would get from although they are not allowed:
// everything that is not the registry of crates.io, unless the host is named in allow (-allow-fetch-host). The ones that
// are refused are listed with the repository and the commit, and how to allow them.
func checkFetchSources(lock string, allow []string) error {
	var refused []string
	hosts := map[string]bool{}
	var hostList []string
	for _, p := range lockPackages(lock) {
		if cratesIOSources[p.source] {
			continue
		}
		host := sourceHost(p.source)
		allowed := false
		for _, a := range allow {
			if host != "" && strings.EqualFold(strings.Trim(a, "[]"), host) {
				allowed = true
			}
		}
		if allowed {
			continue
		}
		refused = append(refused, describeSource(p))
		if host != "" && !hosts[host] {
			hosts[host] = true
			hostList = append(hostList, "-allow-fetch-host "+host)
		}
	}
	if len(refused) == 0 {
		return nil
	}
	return fmt.Errorf("Cargo.lock has dependencies that do not come from crates.io. The fetch stage has a network and where it goes is chosen by the project, "+
		"so by default it gets from crates.io only (git dependencies on any host, github.com included, are not fetched). Not allowed:\n  %s\n"+
		"To fetch them, after you have looked at them: %s (or fetch the dependencies yourself and use -no-fetch).\n"+
		"Note that allowing a host lets the repository send the fetch to other places: the submodules it names (.gitmodules) and redirects from the host. "+
		"That is not prevented until the network of the fetch stage is narrowed (docs/DESIGN-fetch-network.md)",
		strings.Join(refused, "\n  "), strings.Join(hostList, " "))
}
