# The commands to build, check and test rivus. `make` or `make help` lists them.
#
#   make setup   on a new machine: Go, the Dart SDK and `dart pub get` (once; not Rust)
#   make check   before a push (about a minute): build, gofmt, go vet and the short tests (TestDogfood skips itself unless RIVUS_DOGFOOD=1: rivus analysing itself runs in the weekly CI run when something changed, ADR-112)
#   make test    every test, with the Dart and Rust extractors (what CI runs on main); a few minutes
#
# CI runs the same tests (a pull request with -short, main and the weekly run without): .github/workflows/ci.yml.

.DEFAULT_GOAL := help
.PHONY: help setup build check test test-docs merge daily-stats zoo-loosened shard-check test-shard go-warm zoo-dart-gen model-diff model-diff-changed model docs-gen

# the extractors that `make test` needs (a language whose extractor is not there FAILS, it is not skipped)
RIVUS_DART ?= dart
RIVUS_RUST ?= $(CURDIR)/extractor/rust/target/debug/rivus-rust

help: ## list the commands
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | sed -E 's/^([a-z-]+):.*## (.*)/  make \1\t\2/' | expand -t 24

setup: ## install Go and Dart if they are missing, and run dart pub get (Linux; not Rust). Run `make check` after it
	git config core.hooksPath .githooks
	bash scripts/dev-setup.sh

build: ## build every package
	go build ./...

check: ## before a push (about a minute): build, gofmt, go vet, the short tests (TestDogfood is skipped without RIVUS_DOGFOOD=1)
	go build ./...
	@bad="$$(gofmt -l cmd internal bench)"; if [ -n "$$bad" ]; then echo "gofmt: not formatted:"; echo "$$bad"; exit 1; fi
	go vet ./...
	go test -short -count=1 ./...

test: ## every test, with the Dart and Rust extractors (what CI runs on main)
	@command -v "$(firstword $(RIVUS_DART))" >/dev/null || { echo "make test: no dart ($(RIVUS_DART))"; exit 1; }
	@[ -x "$(RIVUS_RUST)" ] || { echo "make test: no Rust extractor at $(RIVUS_RUST) (cd extractor/rust && cargo build)"; exit 1; }
	go vet ./...
	RIVUS_DART="$(RIVUS_DART)" RIVUS_RUST="$(RIVUS_RUST)" RIVUS_REQUIRE_DART=1 RIVUS_REQUIRE_RUST=1 go test -count=1 -timeout 40m ./...

test-docs: ## only the tests that read the documents (links, docs/issues, docs/adr, the ADR references, no executables or files over 1 MB); CI runs it for a change of documents only
	go test ./cmd/rivus -run '^(TestDocLinks|TestIssuesFiles|TestAdrFiles|TestAdrReferences|TestNoBinariesOrLargeFiles)$$' -count=1

zoo-loosened: ## check that what the commits change in the zoo marks (bench/zoo/marks.d, against the merge base) is not a loosening without a "Zoo-Loosened:" line (BASE=<commit> HEAD=<commit>)
	@[ -n "$(BASE)" ] && [ -n "$(HEAD)" ] || { echo "usage: make zoo-loosened BASE=<commit> HEAD=<commit>"; exit 1; }
	bash scripts/zoo-loosened-check.sh "$(BASE)" "$(HEAD)"

shard-check: ## check that the shards of the tests of cmd/rivus (scripts/shard.sh) run every test exactly once (N=<number of shards>, 4 as in CI)
	@set -e; n=$(or $(N),4); \
	list=$$(go test -list '.' ./cmd/rivus | grep -E '^(Test|Example)' | LC_ALL=C sort); \
	got=$$(for i in $$(seq 0 $$((n - 1))); do c=$$(ALL="$$list" bash scripts/shard.sh $$i $$n); \
	  [ -n "$$c" ] || { echo "shard $$i has no test" >&2; exit 1; }; echo "shard $$i/$$n: $$(echo "$$c" | wc -l | tr -d ' ') tests" >&2; echo "$$c"; done); \
	dup=$$(echo "$$got" | LC_ALL=C sort | uniq -d); [ -z "$$dup" ] || { echo "run more than once:"; echo "$$dup"; exit 1; }; \
	[ "$$(echo "$$got" | LC_ALL=C sort)" = "$$list" ] || { echo "the union of the shards is not the list of the tests"; exit 1; }; \
	echo "ok: $$(echo "$$list" | wc -l | tr -d ' ') tests, each in exactly one of $$n shards"

test-shard: ## run one shard of the tests of cmd/rivus, as CI does: I=<index> N=<number of shards> (SHORT=-short, RIVUS_DART, RIVUS_RUST, RIVUS_REQUIRE_* from the environment)
	@set -e; [ -n "$(I)" ] && [ -n "$(N)" ] || { echo "usage: make test-shard I=0 N=4"; exit 1; }; \
	names=$$(bash scripts/shard.sh "$(I)" "$(N)"); \
	[ -n "$$names" ] || { echo "shard $(I)/$(N) has no test" >&2; exit 1; }; \
	echo "shard $(I)/$(N): $$(echo "$$names" | wc -l | tr -d ' ') tests"; \
	regex="^($$(echo "$$names" | paste -sd'|' -))\$$"; \
	go test $${SHORT:+"$$SHORT"} -timeout 30m -run "$$regex" ./cmd/rivus

go-warm: ## compile the dependencies of every module of the bench into the build cache (the tests read them with `go list -export`: 14 s on a cold cache, 3 s on a warm one)
	@set -e; for gm in $$(find bench -name go.mod | sort); do (cd "$$(dirname "$$gm")" && go list -export -deps ./... > /dev/null); done

zoo-dart-gen: ## write bench/zoo-dart/exec/lib/cases.dart and the two pubspec.yaml of the Dart zoo (generated, not committed) from the directories of the cells and of the skeleton packages
	go test ./cmd/rivus -run '^TestZooDartGenerate$$' -count=1 -update

bench-deps: ## local, heavy: `rivus analyze` (no flags) on the benches with heavy dependencies (go-s3, gcp, dart-gcp) with a timeout; adds one row each to bench/heavy-ledger.tsv, unless NO_LEDGER=1 (TIMEOUT=600 BENCHES="go-s3 gcp"; bench/deps_bench.py)
	python3 bench/deps_bench.py $(if $(NO_LEDGER),-no-ledger) -timeout $(or $(TIMEOUT),600) $(BENCHES)

bench-passline: ## local, heavy: the pass line (one PASS/FAIL line per target: time, memory, convergence, detection; default flags) of memos, cocoon and dart_services; fetches the sources into $$BENCH_CACHE (bench/deps_bench.py, bench/deps-targets.json); adds one row per target to bench/heavy-ledger.tsv, unless NO_LEDGER=1
	python3 bench/deps_bench.py $(if $(NO_LEDGER),-no-ledger) $(or $(TARGETS),memos cocoon dart_services)

model-diff: ## the differential test of the model of a package: PKG=<Go import path | package:name> [MODEL=<file>] (bench/model-diff, docs/MODEL-RUNBOOK.md); fetches the package, does not run it
	@[ -n "$(PKG)" ] || { echo "usage: make model-diff PKG=strings   (or PKG=package:http, MODEL=internal/model/builtin/dart-http.yaml)"; exit 1; }
	@# built once and run, not `go run`: `go run` turns the exit status of the program into 1 ("exit status N")
	@d=$$(mktemp -d) && go build -o "$$d/model-diff" ./bench/model-diff && "$$d/model-diff" -pkg "$(PKG)" $(if $(MODEL),-model "$(MODEL)"); rc=$$?; rm -rf "$$d"; exit $$rc

model-diff-changed: ## the differential test of every package whose model rows differ from BASE=<commit> (what a pull request checks; LANGS=go by default; fetches the packages, does not run them)
	@[ -n "$(BASE)" ] || { echo "usage: make model-diff-changed BASE=origin/main"; exit 1; }
	go run ./bench/model-diff -run-changed "$(BASE)" $(if $(LANGS),-langs "$(LANGS)")

model: ## the steps of making the model of a Go or Dart package: PKG=<import path | dart package> [VERSION=v1.2.3] [LANG=go|dart] [ARGS="-diff -write-verified"] (bench/model-author, docs/MODEL-RUNBOOK.md); exit 70 = a candidate has no row and is not in rejected:; fetches the package, does not run it
	@[ -n "$(PKG)" ] || { echo "usage: make model PKG=github.com/jmoiron/sqlx VERSION=v1.4.0 LANG=go   (or PKG=dio VERSION=5.11.1 LANG=dart; ARGS=\"-recall\" for the recall of layer 1; ARGS=\"-gen-skeleton\" makes the skeleton and the cell template for the Go zoo)"; exit 1; }
	@# built once and run, not `go run`: `go run` turns the exit status of the program (70, 2) into 1 ("exit status N")
	@d=$$(mktemp -d) && go build -o "$$d/model-author" ./bench/model-author && "$$d/model-author" -pkg "$(PKG)" $(if $(VERSION),-version "$(VERSION)") -lang $(if $(filter command line,$(origin LANG)),$(LANG),go) $(ARGS); rc=$$?; rm -rf "$$d"; exit $$rc

docs-gen: ## print the tables of the issues and the ADRs (from docs/issues/*.md, docs/adr/*.md); WHAT=issues or WHAT=adr for one; OUT=docs/TABLE.md writes it (ignored by git)
	go run ./scripts/docs-gen $(if $(OUT),-o $(OUT)) $(WHAT)

daily-stats: ## count the pull requests merged into main since a date, by what they change: ENGINE/MODEL/EXTRACTOR or TOOLS/CI/DOCS (SINCE=2026-11-20; scripts/daily-stats, docs/WORKFLOW.md)
	@[ -n "$(SINCE)" ] || { echo "usage: make daily-stats SINCE=<date>"; exit 1; }
	go run ./scripts/daily-stats -since "$(SINCE)"

merge: ## merge a pull request with a merge commit, after checking the rules for what the developer may merge: PR=<number> (scripts/merge-pr.sh)
	@[ -n "$(PR)" ] || { echo "usage: make merge PR=<pull request number>"; exit 1; }
	bash scripts/merge-pr.sh "$(PR)"
