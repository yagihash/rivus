# #241 同じ入力で `stats.adds_total` と `stats.reanalyses` が、回によって +11 と +1 だけ揺れる(rustypaste で見つけた。原因は未特定)

- 状態: **open**
- 優先度: 中(出力に現れない内部の計数の揺れで、finding と uncertainty は変わらない。ただし ADR-118 の (ii)「順序とマシンに依らない」に関わるので、原因が分かるまで中にする。原因が分かって出力に影響しないと確かめたら、低に下げる)
- 種類: 決定性
- 言語: Rust で見つけた(解析器の `internal/taint` の話なので、他の言語でも起きる可能性がある。確かめていない)
- ROADMAP の節: -
- 守るもの: -(`stats` の計数の同一性は、どのテストも守っていない)
- 進める完了条件: -

関連: docs/adr/118-scale-mechanisms-three-conditions.md の (ii)、`--graph-check` の `same`(finding と uncertainty の順序つきハッシュ)。#642(`summary` の `dispatch_candidates_applied`)は別の数(呼びごとの適用候補の位置の集合)で、この揺れとは別。

## 何が起きるか(事実)

rustypaste を `bench/deps_bench.py -no-ledger`、`bench/heavylock.py` の直列で解析すると、同じ入力で次の 2 つが回によって揺れる。

- `stats.adds_total`: 4244618 か 4244629(+11)
- `stats.reanalyses`: 79682 か 79683(+1)

2 つは必ず一緒に動く(+11 の回は必ず +1)。Rust の抽出器は全部の回で同じ(`88c49307…`)。測定の出力は `~/tmp/rivus-developer/narrow-out/{b1..b5,n1..n5}/` の `row.json` と `rustypaste-prefix.json`。b は origin/main のビルド(base)、n は PR #641 のビルド(new)で、各 5 回。

- base は 5 回中 1 回(b5)、new は 5 回中 3 回(n2、n4、n5)が +11 / +1 の値だった。
- 10 回とも同じ: `summary_elems`、`stats.functions`、`summary_steps`、finding の ID の集合と順序(12 件)、uncertainty の kind ごとの数。`func_timeouts` と `budget_exhausted` は 0。
- 揺れの出方は load と対応しない。+11 の回は load の低い静かな回が多いが、load の高い回も 4 回とも +11 ではなかった(つまり、load が高いと +11 になる、という形でもない)。

したがって `--graph-check` の `same` の根拠(finding と uncertainty の順序つきハッシュ)には出ていない。内部の計数だけの揺れで、出力は変わっていない。

## 見当(未確認)

見当: `internal/taint/engine.go` は、1 つの level の SCC 同士を `runLevel` で、SCC の中を `runParallel` で並列に動かす。並列の結果は順序どおりに公開する設計だが、`reanalyses`(`atomic.Int64`)と dirty の判定(`takeDirty`)の数え方に、並列の順序で 1 回の再解析が増減する余地があるのではないか。+1 と +11 が 1 組で動くので、1 つの関数の再解析が 1 回増え、その分の追加が 11 件増える形に見える。どの関数かは特定していない。

## 完了条件(案)

1. 揺れが出る回と出ない回で `-v` のログを照合するか、`stats` の `adds_per_function` を比べて、どの関数が増えるかを特定する。
2. 決定的にできるなら直す(並列の順序に依らない数え方にする)。出力(finding、uncertainty、要約)が変わらないことを `--graph-check` で確かめる。
3. 直せない、または直す価値がないなら、「内部の計数(`stats` の作業量)の揺れは仕様」と、ADR-118 の (ii) の適用範囲(出力と要約が対象で、作業量の計数は対象外)に書く。その場合は、`stats` のどの値が揺れてよいかを一覧にする。

## 限界

出力が変わる揺れではないので、利用者には見えない。ただし、作業量の `stats` を使う測定(`bench/deps_bench.py`、`-no-ledger` なしの台帳)で、同じ入力の差を「変更の効果」と読み違える。また原因が、順序に依る再解析の増減なら、要約の到達に時間が関わる別の揺れの入口かもしれない(見当で、確かめていない)。

## 追記(2026-11-27): `summary.dispatch_candidates_applied` は揺れない

#642(`rivus.result/6`)の `summary.dispatch_candidates_applied` は、この揺れとは別の数で、揺れない。事実だけを書く。

- dart-pub: `-workers 1`、`-workers 8`(各 2 回)、既定、`-pass-stats` ありの run のすべてで 288156。
- rustypaste: 2 回とも 357303。
- `value_candidates` も、全回で同じ。dart-pub は `{0: 449, 1: 199, 2-5: 108, 6-20: 56, 21-32: 6, 33+: 15}`。
- 前後(`e5275f92` と #646 を載せた木)で、finding(30 件)の ID 列のハッシュ `9653068dd46983ef` と、uncertainty(7579 件)の順序つきハッシュ `b8d5b3a18a0daf7a` は同じ。

理由(コードを読んで確認した。実験で確かめたのは上の数が同じことだけ): 呼びの候補の数は、`Run()` の前に決まる。

- dispatch の候補は、`internal/taint/calls.go` の `withBody`(295〜299 行)が `ci.Candidates` から作る(`callees` は 566〜575 行)。
- `ci.Candidates` の書き込みは `internal/sg/build.go:104`、`:435` と、`internal/valuecalls/valuecalls.go` の 262〜323 行(`Resolve`)、`internal/valuecalls/narrow.go:246`。値の候補は `valuecalls.Resolve` が `Run()` の前に決める。
- `SetCellCandidates`(`cmd/rivus/analyze.go:317`)は `Run()` の後。
- 反復で増える cell の候補(`internal/taint/funccells.go` の `applyFuncCells`)は、数えない。

したがって `adds_total` と `reanalyses` の揺れ(この課題の主題)とは別の数で、この課題の原因の手がかりにはならない。

## 訂正(2026-11-27): #642 の本文の「`dispatch_candidates` と桁が違う」

#642 の本文にあった「`dispatch_candidates` と桁が違う」は、`summary.dispatch_candidates` が uncertainty を数えている点では正しい。ただし、IR の候補の合計との比較としては成り立たない。

dart-pub の `-pass-stats` の `stats.dispatch_candidates`(呼びの数)は `{one: 38369, two_to_8: 14245, 9_to_16: 1979, over_16: 1966, none: 8101, largest: 357}`。区切りの下限と上限で足した IR の候補の合計は、およそ 118,092〜885,000 件の間になる。applied の 288156 はこの範囲の中で、桁は違わない。

IR の候補の合計そのものは、未計算。
