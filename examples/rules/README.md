# Example rules

Rules for libraries and frameworks that rivus does not know on its own. The built-in rules cover
the language and its standard library (and a few third-party libraries in other languages: see
`docs/DECISIONS.md`, ADR-044); what a project uses beyond that (a web framework, its own
request wrappers, its own file helpers) is for the project to say, in `.rivus/rules/*.yaml` (read
without a flag) or with `-rules file.yaml`.

These files are examples to copy and edit, not part of rivus:

- `go-gin.yaml`: gin (`*gin.Context` is the request; `Query`, `Param`, `ShouldBindJSON` and so on).
- `go-database-sql.yaml`: what database/sql reads back is a source (label `stored`): second order flows (stored XSS, stored path), with `out`, `field: Type.*` and the `stored_data_boundary` uncertainty.
- `go-macaron.yaml`: macaron (used by Gogs). A wrapper that embeds `*macaron.Context` has to be
  named as a source in the project's own rules (see `bench/gogs/rules.yaml`).

- `rust-actix-web.yaml`: actix-web (`HttpRequest` and the `web::Query` / `Path` / `Json` / `Form` / `Bytes` extractors are the request). It was built in before ADR-063; a specific framework is for the project to add. `rivus analyze` writes a line to stderr when the project calls actix-web and no rule names it.

What is built in: the standard library, the packages that have no replacement (`serde`, `tonic`, `tokio`, `futures`, `bytes`, `libc`, `http`, `url`...) and the SQL drivers (`sqlx`, `rusqlite`, `sqflite`). A specific framework is not: its model is here.

```sh
rivus analyze -rules examples/rules/go-gin.yaml ./...
```
