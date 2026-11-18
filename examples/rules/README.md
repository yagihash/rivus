# Example rules

Rules for libraries and frameworks that rivus does not know on its own. The built-in rules cover
the language, its standard library, and the very widely used third-party libraries (the policy is
in `docs/DECISIONS.md`, and the list grows); what a project uses beyond that (a smaller framework,
its own request wrappers, its own file helpers) is for the project to say, in `.rivus/rules/*.yaml` (read
without a flag) or with `-rules file.yaml`.

These files are examples to copy and edit, not part of rivus:

- `go-macaron.yaml`: macaron (used by Gogs). A wrapper that embeds `*macaron.Context` has to be
  named as a source in the project's own rules (see `bench/gogs/rules.yaml`).

```sh
rivus analyze -rules examples/rules/go-macaron.yaml ./...
```
