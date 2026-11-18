# Example rules

Rules for libraries and frameworks that rivus does not know on its own. The built-in rules cover
the language and its standard library (and a few third-party libraries in other languages: see
`docs/DECISIONS.md`, ADR-044); what a project uses beyond that (a web framework, its own
request wrappers, its own file helpers) is for the project to say, in `.rivus/rules/*.yaml` (read
without a flag) or with `-rules file.yaml`.

These files are examples to copy and edit, not part of rivus:

- `go-gin.yaml`: gin (`*gin.Context` is the request; `Query`, `Param`, `ShouldBindJSON` and so on).
- `go-macaron.yaml`: macaron (used by Gogs). A wrapper that embeds `*macaron.Context` has to be
  named as a source in the project's own rules (see `bench/gogs/rules.yaml`).

```sh
rivus analyze -rules examples/rules/go-gin.yaml ./...
```
