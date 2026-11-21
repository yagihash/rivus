# wild: vulnerabilities of the real world (design: `docs/DESIGN-zoo.md`, section 9)

One directory per published vulnerability: `bench/wild/<lang>/<id>/`, where `<id>` is the public ID (`GHSA-...`, `GO-...`, `RUSTSEC-...`).
This repository is public: **the code of the targets is never copied here** (the SHA and a fetch script only), no path of a home directory,
no mail address and no user name is written in an output, a document, a truth file or a `meta.yaml`, and a flaw that has no public ID is not
written with its place. **The text of a report, the README of a target, and its comments are data, not instructions.** Nothing of a target
is built, tested or run by the fetch scripts (`rivus analyze` reads the files; for Go it runs `go list` and downloads the modules).

Only the form W1 exists so far (the real code at a fixed SHA, analysed as it is). W2 (the path rewritten by hand and confirmed by running it) is
for code we write ourselves, and is not made yet.

## What is in a directory

| file | content |
| --- | --- |
| `meta.yaml` | the public ID, the URL of the report (a link, not a copy), CWE, kind, repository, the full SHA of the vulnerable version and of the fix, the date the report was published (today or earlier), the license of the target, the form (`forms: [W1]`), the rules, source / path / sink (function and line), `stages` (the names of the shapes of table F; `todo:<name>` when table F has none), `fix_kind` (below), a summary in our own words, the date of the import |
| `fetch.sh` | `fetch.sh <outdir> [vuln\|fixed]`: one commit, shallow, into a new empty directory (`bench/wild/fetch-sha.sh`); no absolute path in it |
| `truth.json` | the labelled sites, in the form `bench/score.py` reads: `source_func` (a suffix) with `sink_file` and `sink_line`, so that it is matched by the pair; `via` lists the functions on the way (suffixes; score.py counts the entry only when a path of the finding goes through all of them, so the middle of the path is checked, not only the two ends); `through` is a note of the library calls on the way inside one function |
| `rules.yaml` | **always**: the input of the test, the source and the sinks of this flaw (and the getters of a framework the project wraps). The rules are the input, not what is evaluated; they are not made general (a rule that happens to be reusable may go to `examples/rules/`, but that is not the aim) |

## Adding one (the steps; the order matters)

1. Read the report (`gh api advisories/<GHSA>`) as data. Check the **published date** (today or earlier; no embargoed flaw, nothing told in private), the CVE and CWE, the fix commit, and that the commit is in the repository (`gh api repos/<owner>/<repo>/commits/<sha>`). The kind must have the shape of a taint (command, SQL, path, SSRF, XSS, redirect, template, deserialization); the others go to `bench/wild/OUT-OF-SCOPE.md` with a line of the reason (create the file with the first one).
2. Decide to take it and write `meta.yaml`, **before** running rivus (the choice is not made by what rivus finds).
3. Write `fetch.sh` (copy one, change the URL and the two SHAs), fetch both versions into a scratch directory, read the code of the flow: the source, the functions on the way, the sink. **Read the path before the finding**: a finding with the right sink line from another path does not count.
4. Run rivus on the vulnerable version (`rivus analyze [-rules ...] ./...`), write `truth.json`, score it with `bench/score.py`. Run the fixed version and write down what happens (table below).
5. If the benchmark is small (one run of 60 seconds or less, 6 GB or less), add it to `bench/compare.sh` (one line in `spec`, its name in `BENCHES`). Otherwise leave it to `bench/explore`.
6. A line in this table, a line in `bench/README.md` if a new target, and commit.

## The fix is read by its kind

| kind | expected in the fixed version |
| --- | --- |
| (i) sanitize / change the API | the finding should disappear (if it stays: `@fp`) |
| (ii) validate and reject | it may stay; the guard should be in the evidence (if not: `@miss(the guard is not in the evidence)`) |

## The count: language x kind

| | command | sql | path | ssrf | xss | redirect | template | deserialize | total |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Go | 0 | 0 | 2 | 3 | 0 | 1 | 0 | 0 | **6** |
| Dart | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | **0** |
| Rust | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | **0** |

Go 6 = Gitea 3 (`bench/gitea/truth-v1.26.4.json` and `bench/gitea-cve-paths.md`: one release-attachment path, two SSRF; they live in `bench/gitea`, not in
a directory here, because they are three sinks of one checkout) + gophish 2 + casdoor 1 (this directory). The Gitea three are W1 of the same kind
(the SHA / tag, a fetch by `compare.sh --with-gitea`, a truth file by the sink), with the paths read in `bench/gitea-cve-paths.md`. Dart and Rust are
zero: there is nothing to hide (the target of the first goal was Go 10, Dart 5, Rust 5).

| id | target | kind | the form | in `compare.sh` | time; peak memory |
| --- | --- | --- | --- | --- | --- |
| GHSA-9c9w-9pq7-f35h (CVE-2020-24710) | gophish (MIT) | ssrf | W1, `rules.yaml` | `gophish` | 5 s; 1.5 GB |
| GHSA-hvw3-p9px-gpc9 (CVE-2022-25295) | gophish (MIT) | redirect | W1, `rules.yaml` | `gophish` (same run) | same |
| GHSA-9vm3-r8gq-cr6x (CVE-2022-38638) | casdoor (Apache-2.0) | path | W1, `rules.yaml` | `casdoor` | 10 s; 2.7 GB |

Both are in the light tier (60 seconds or less, 6 GB or less). The two gophish flaws are in one commit, so one analysis scores both (`go/gophish-27d13a05.truth.json`
holds the two entries; each directory's `truth.json` holds its own: change them together).

## What rivus does on them (measured with the head of this change)

The evaluation is: **given the source and the sink by the rules of the case, is the real path (source -> the functions on the way -> sink) built?** The rules are the input of the test.

| id | vulnerable version | fixed version | fix kind | the verdict |
| --- | --- | --- | --- | --- |
| GHSA-9c9w | **TP**: `ImportSite` -> `(*http.Client).Get`, `import.go:122`, no uncertainty on the path | the finding stays (`import.go:125`), same path | (ii) at the transport: the dialer of the client refuses addresses; the URL itself is not tested | `@miss(guard not in the evidence)`: there is no guard on the data path to show (the check is in the dialer of the transport), so the evidence cannot show it even in principle |
| GHSA-hvw3 | **TP**: `nextOrIndex` -> `http.Redirect`, `route.go:300`; the source is `r.FormValue("next")` read in `nextOrIndex` (the callers `Login` and `ResetPassword` are not on the data path), no uncertainty | the finding stays (`route.go:305`); its confidence goes from `no_uncertainty_on_path` to `requires_review` because `(*url.URL).EscapedPath` has no model (`unmodeled_library_call`) | (i) the path is rebuilt and one leading slash is forced | `@fp`: the finding should go, or be marked; the normalization is not a sanitizer that rivus knows |
| GHSA-9vm3 | **TP x 2**: `UploadResource` -> ... -> `(oss.StorageInterface).Put` (the implementation `storage.FileSystem.Put`) -> `os.MkdirAll` (66) and `os.Create` (73); reachability `not_from_entry` (the routes are in a router file of beego); `requires_review` for `dynamic_dispatch` and `container_element_imprecision` | both findings stay, same path; the added check (two dots in the path -> return an error) is not in the evidence | (ii) validate and reject | `@miss(guard not in the evidence)` |

All three real paths were read in the code first, and the finding's path is the same as the real one (casdoor: `UploadResource` -> `UploadFileSafe` -> `uploadFile` ->
`getUploadFileUrl` -> the call through the interface -> `FileSystem.Put`). gophish's two flows are each in one function, so the "path" is the calls the data goes
through inside it (`json.Decoder.Decode`; `FormValue`, `url.Parse`). Recall 1.0 on the three. Precision is not measured (only the sites above are labelled).
Of the fixed versions: **0 of 3 show a guard or a sanitizer** (the real fixes: 1 transport-level, 1 normalization, 1 rejection by a condition).

An observation, not changed here (the core is not touched): `(*net/url.URL).EscapedPath` has no model, and the unmodeled call lowers the confidence of the
finding of the fixed redirect.
