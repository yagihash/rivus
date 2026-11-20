// Package gitmodule is a dependency (like gogs/git-module): it reads and writes the
// hook files of a repository. Sinks inside dependencies are not looked for, so the
// rules name the call.
package gitmodule

// Repository is a git repository.
type Repository struct{ Path string }

// Hook reads the hook of the name in the directory of the hooks.
func (r *Repository) Hook(dir, name string) (string, error) { return r.Path + dir + name, nil }
