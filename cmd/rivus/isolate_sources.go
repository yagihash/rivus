package main

import (
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// The sources are checked before the fetch stage: every Cargo.toml of the project (a symbolic link is refused) and Cargo.lock
// (read as TOML; one that cannot be read is refused). A link to a directory outside the project and a path dependency that leaves it are refused.
//
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
// Cargo.lock is read as TOML, the way cargo reads it. One that cannot be read is an error (refused): the sources are not known.
func lockPackages(lock string) ([]lockPackage, error) {
	var doc struct {
		Package []struct {
			Name    string `toml:"name"`
			Version string `toml:"version"`
			Source  string `toml:"source"`
		} `toml:"package"`
	}
	if _, err := toml.Decode(lock, &doc); err != nil {
		return nil, fmt.Errorf("Cargo.lock cannot be read as TOML, so where the dependencies come from is not known: %v", err)
	}
	var out []lockPackage
	seen := map[lockPackage]bool{}
	for _, p := range doc.Package {
		c := lockPackage{p.Name, p.Version, p.Source}
		if c.source != "" && !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	return out, nil
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
	pkgs, err := lockPackages(lock)
	if err != nil {
		return err
	}
	for _, p := range pkgs {
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

// manifestPlaces are the places that the manifests (Cargo.toml) of the project name for cargo to get from: the `git` of
// a dependency, the `registry-index` of a dependency, and the URL that names the source of a [patch] table. They are read
// from the manifests, not from Cargo.lock, because cargo asks the places a manifest names before it finds that Cargo.lock
// does not match (`--locked`): a manifest can send the fetch to a place that Cargo.lock never shows. The manifests are
// parsed as data (nothing is run). All of them are read, in every directory (target and .git too; the name is compared
// without regard to case). A manifest that cannot be parsed, or that is a symbolic link, is a place that is not known: it is refused. So is a
// symbolic link to a directory outside the project, and a path dependency that leaves the project (one that stays inside is walked).
type manifestPlace struct{ desc, url string }

func manifestPlaces(dir string) (places []manifestPlace, err error) {
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 && !strings.EqualFold(d.Name(), "Cargo.toml") {
			// A link to a directory is not followed by the walk, but cargo follows it. One that points inside the project is
			// walked at the place it points to; one that points outside is refused. (A link that points nowhere is no place.)
			if real, err := filepath.EvalSymlinks(path); err == nil {
				if fi, err := os.Stat(real); err == nil && fi.IsDir() && !within(root, real) {
					rel, _ := filepath.Rel(dir, path)
					return fmt.Errorf("%s is a symbolic link to a directory outside the project (%s), so the manifests in it are not known", rel, real)
				}
			}
			return nil
		}
		if d.IsDir() || !strings.EqualFold(d.Name(), "Cargo.toml") {
			return nil
		}
		// Every directory is walked (target and .git too): cargo reads a path dependency wherever it is. The name is compared
		// without regard to case (the file system may not tell the cases apart). A link is refused, not followed.
		if !d.Type().IsRegular() {
			rel, _ := filepath.Rel(dir, path)
			return fmt.Errorf("%s is not a regular file (a symbolic link, for example), so the places it names are not known", rel)
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	for _, f := range files {
		rel, _ := filepath.Rel(dir, f)
		var doc map[string]any
		if _, err := toml.DecodeFile(f, &doc); err != nil {
			return nil, fmt.Errorf("%s cannot be read as TOML, so the places it names are not known: %v", rel, err)
		}
		places = collectPlaces(places, doc, "", rel)
		for _, p := range collectPathDeps(nil, doc, "") {
			if strings.ContainsAny(p, "*?[") { // a glob of the members: its fixed part is what is checked
				p = filepath.Dir(p[:strings.IndexAny(p, "*?[")] + "x")
			}
			real, err := filepath.EvalSymlinks(filepath.Join(filepath.Dir(f), p))
			if err != nil {
				continue // it does not exist: cargo finds nothing there
			}
			if !within(root, real) {
				return nil, fmt.Errorf("%s: path = %q leaves the project (%s), so the manifests there are not known", rel, p, real)
			}
		}
	}
	return places, nil
}

// within says whether the (resolved) path p is root or inside it.
func within(root, p string) bool {
	r, err := filepath.Rel(root, p)
	return err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator))
}

// collectPathDeps adds to paths the directories that the table v (at path) names for cargo to read: the `path` of a
// dependency (in any table of dependencies, [patch] and [replace]) and the members of a workspace.
func collectPathDeps(paths []string, v any, path string) []string {
	t, ok := v.(map[string]any)
	if !ok {
		if arr, ok := v.([]map[string]any); ok {
			for _, e := range arr {
				paths = collectPathDeps(paths, e, path)
			}
		}
		return paths
	}
	inDeps := strings.HasPrefix(path, "patch") || strings.HasPrefix(path, "replace") ||
		strings.Contains(path, "dependencies")
	for k, e := range t {
		if path == "package.metadata" || path == "workspace.metadata" {
			return paths
		}
		switch {
		case k == "path" && inDeps && isString(e):
			paths = append(paths, e.(string))
		case path == "workspace" && (k == "members" || k == "exclude"):
			if arr, ok := e.([]any); ok {
				for _, m := range arr {
					if ms, ok := m.(string); ok && k == "members" {
						paths = append(paths, ms)
					}
				}
			}
		}
		paths = collectPathDeps(paths, e, strings.TrimPrefix(path+"."+k, "."))
	}
	return paths
}

// collectPlaces adds to places the places that the table v (at path, of the manifest rel) names, and those of the tables inside.
func collectPlaces(places []manifestPlace, v any, path, rel string) []manifestPlace {
	t, ok := v.(map[string]any)
	if !ok {
		if arr, ok := v.([]map[string]any); ok { // [[bin]] and the like
			for _, e := range arr {
				places = collectPlaces(places, e, path, rel)
			}
		}
		return places
	}
	for k, e := range t {
		switch {
		case path == "package.metadata" || path == "workspace.metadata":
			return places // inert: cargo does not read it
		case (k == "git" || k == "registry-index") && isString(e):
			places = append(places, manifestPlace{fmt.Sprintf("%s: %s = %q", rel, k, e), e.(string)})
		case (path == "patch" || path == "replace") && strings.Contains(k, "://"):
			places = append(places, manifestPlace{fmt.Sprintf("%s: [%s.%q]", rel, path, k), k})
		}
		places = collectPlaces(places, e, strings.TrimPrefix(path+"."+k, "."), rel)
	}
	return places
}

func isString(v any) bool { _, ok := v.(string); return ok }

// checkManifestSources refuses the places that the manifests name, unless the host is named in allow, as checkFetchSources
// does for Cargo.lock.
func checkManifestSources(dir string, allow []string) error {
	places, err := manifestPlaces(dir)
	if err != nil {
		return err
	}
	var refused []string
	for _, p := range places {
		host := ""
		if pu, err := url.Parse(strings.TrimSpace(p.url)); err == nil {
			host = pu.Hostname()
		}
		ok := false
		for _, a := range allow {
			if host != "" && strings.EqualFold(strings.Trim(a, "[]"), host) {
				ok = true
			}
		}
		if !ok {
			refused = append(refused, p.desc)
		}
	}
	if len(refused) == 0 {
		return nil
	}
	return fmt.Errorf("the manifests (Cargo.toml) of the project name places to fetch from that are not allowed. Cargo.lock does not have to show them: "+
		"cargo asks them before it finds that Cargo.lock does not match. By default the fetch stage gets from crates.io only. Not allowed:\n  %s\n"+
		"To fetch them, after you have looked at them, name their hosts with -allow-fetch-host HOST (or fetch the dependencies yourself and use -no-fetch)",
		strings.Join(refused, "\n  "))
}
