# #202 Go: 要約のフェーズの間、すべての関数の `sg` の本体(約 1 GB)が持たれたまま

- 状態: **open**
- 種類: 道具・ベンチ
- 言語: 全
- ROADMAP の節: 3
- 守るもの: -
- 題: 要約のフェーズでは、一度解析した関数の `sg` の本体も、最後まで持たれる(memos で約 1.07 GB)

## 何が起きているか

memos の要約のフェーズで、GC のマーク時点の heap profile を取ると、`sg` の本体(`internal/sg` の `build` が作る `Func.Blocks`、`Instr`、`Value`)は約 1.07 GB で、フェーズの間ずっと一定。大きな SCC の結果の `state` を commit 前に縮める修正のあと(`perf/scc-release-states`)は、マーク時点の最大(約 2.3 GB)の約半分になる。

## なぜそのまま外せないか

「確定した SCC は、呼び出し元が呼び出し先の要約だけを使うので、本体を手放せる」は、そのままは成り立たない。

- 1 つのパスで確定しても、あとのパスで呼び出し先の要約が変わると、呼び出し元が再解析される。そのとき呼び出し元自身の本体が要る(memos は 4 パス、再解析 65250 回、関数 40830)。「最後のパスで二度と解析されない関数」は、最後のパスまで分からない。
- 全関数の `Blocks` を読む所がある: `Run` の最初の `order` / `levelsOf` / `callees`、`selfCalls`、`writersOf`、`callbackFuncs`(`internal/taint/engine.go`)、`provenance.go`、`summary.go`(呼び出し元や home の本体を読む所)、`funcvalues.go`、`fieldsink.go`、`diagstats.go`、`internal/sg/env.go`、`internal/sg/sg.go`。
- `Instr` が他の関数の `*Func` や `*Instr` を指す: `ci.Static`、`ci.CandidateFns`、`Program.EnvClosures`、値のポインタ(`Value`)が関数をまたぐ。

## 作業の形(案)

本体を手放し、要るとき IR(`internal/irspill`)から `sg.buildFunc` で作り直す。

- `sg.build` は一括で、`infos` の共有と、関数をまたぐポインタがある。関数単位で呼べる形にし、他の関数の `Instr` を指すポインタを ID で引く形に替える必要がある。広い変更。
- 作り直しの CPU が増える。`buildInstrs` と `defineValues` が `sg` の約 0.8 GB を作る。再解析 65250 回に掛かる。
- 効果の見込みは最大約 1 GB(推測)。山の大部分は、上の `state` の修正で下がるので、A(`-max-memory 4GiB`)には要らない。余裕を増やしたいときの候補。

## 測定

heap profile の取り方: `/gc/heap/live:bytes` が前の最大を超えたときに `runtime/pprof` の `heap` を書く(測定用の一時のコード。コミットしない)。`-workers` を 1、2、4、既定で変えても、`sg` の本体は 1.05〜1.09 GB で一定。
