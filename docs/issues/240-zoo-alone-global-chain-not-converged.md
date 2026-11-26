# #240 zoo の `ZOO_ALONE=1` が、周回の上限で止まるセル `state/global-chain` を exit 4 で失敗させる(Rust。Dart も同じ形)

- 状態: **open**
- 優先度: 低
- 種類: 道具・ベンチ
- 言語: Rust(Dart も同じ書き方。Go は直してある)
- ROADMAP の節: -
- 守るもの: -(`ZOO_ALONE=1` の実行は、どのテストも守っていない)
- 進める完了条件: -

関連: #217(`--graph-check` の zoo の 4)、docs/adr/120-incomplete-analysis-exit-code.md(zoo の鎖は 4 で終わるのが正しい)。

## 何が起きるか(事実)

zoo-rust のセル `F/state/global-chain` を `ZOO_ALONE=1`(1 つのセルだけ解析する実行)で解析すると、`analyze: exit status 4`(`not_converged`)になり、`TestZooRust/alone/state/global-chain` が失敗する。zoo 全体の実行(`ZOO_ALONE` なし)は通る。

2026-11-27 に、origin/main(#638 のマージ後)と抽出器 `traitpath-out/rivus-rust-new` で、次の 1 回だけ実行して確かめた(`-run 'TestZooRust$/alone/state/global-chain'`、`GOMAXPROCS=6`)。

```
zoo_rust_test.go:335: analyze: exit status 4
rivus: the analysis is incomplete (not_converged): the result is a part of what a complete run would give; ...
--- FAIL: TestZooRust/alone/state/global-chain
```

`ir.ValueCandidateMost` を 1000 にした試作でも同じだった(これは別の上限で、このセルには関係しない)。最近の変更とは無関係。

## このセルの形(事実)

`bench/zoo-rust/cells/src/state/global_chain.rs` は、値が static の鎖を 1 階層ずつ関数で渡る形で、3 階層、13 階層、30 階層の 3 つ(`vuln`、`vuln13`、`vuln30`)に、`fixed` と `decoy` がある。13 階層と 30 階層の source には `@miss(recursion_approximation)` が付いている。つまり、周回の上限(`taint.DefaultOptions().MaxPasses` は 12)で止めるために作ったセルで、13 階層と 30 階層は 12 周では最後まで届かない。ADR 120 も、zoo のこのセルは 4 で終わるのが正しいと書いている。Go と Dart にも同名のセルがある(`bench/zoo-go/cells/state/global_chain/`、`bench/zoo-dart/cells/lib/state/global_chain.dart`)。

## なぜ全体では PASS で、単独では失敗するか(事実と、読んだ範囲での見当)

収束するかどうかの差ではなく、テストの書き方の差。

- 事実(読んだ): `TestZooRust` は、`zooSplit` で `recursion_approximation` の `@miss` を持つセルを別の木(`recCells`)に分け、その木だけ `annotOpts{allowUnconverged: true, programWide: true}` で解析する(`cmd/rivus/zoo_rust_test.go:360` から `:367`)。残りのセルは `annotOpts{}` で、収束しなければ失敗にする。つまり全体の実行でも、`state/global-chain` の木は収束していない(exit 4)。4 を許しているので失敗しない。
- 事実(読んだ): `ZOO_ALONE=1` の `zooRustAlone`(`:329` から `:338`)は、セルごとに `checkCuratedIR(t, one, ...)` を呼ぶ。これは `annotOpts{}`(4 を許さない)の呼び出しで、`zooSplit` の区別を持たない。
- 事実(読んだ): Go の同じ処理(`cmd/rivus/zoo_iso_test.go:103`)は、周回の上限で止めるセルだけ `allowUnconverged: rec, programWide: rec` にしている。Dart の `zooDartAlone`(`zoo_dart_test.go:256` から `:265`)は Rust と同じで、`checkCuratedIR` を使う(Dart は、読んだだけで、実行していない)。
- 見当(実行して確かめていない): 他のセルがあると変わるのは、4 を許すかどうかだけで、`not_converged` になるのは単独でも全体の `recCells` の木でも同じ(13 階層と 30 階層の鎖が 12 周より長いため)。`stats` の周回数や最大 SCC は、この調査では取っていない(`rivus analyze` を直接動かすと抽出器の出力が要るため、テストの形だけで動かした)。

## 完了条件

次のどちらか。

1. `zooRustAlone` と `zooDartAlone` が、`zooSplit` と同じ基準(`recursion_approximation` の `@miss` を持つセル)で `allowUnconverged: true, programWide: true` を渡す(Go の `zoo_iso_test.go:103` と同じ)。`ZOO_ALONE=1` の実行で `alone/state/global-chain` が通る。
2. 単独の実行では、そのセルを対象から外す、と決めて、`bench/zoo/README.md` の `ZOO_ALONE` の説明(68 行目)に書く。

どちらでも、鎖が 12 周で止まること自体(`@miss` と ADR 120)は変えない。

## 限界

このセルは意図して 4(`not_converged`)で終わる。単独の診断(`ZOO_ALONE=1`)の書き方の非対称で、解析結果は不完全ではない。13 階層と 30 階層の不完全は意図したもので、`@miss` で期待値に書いてある。ただし、診断で失敗が常に出ると、本当に収束しなくなったセルの失敗が見分けにくくなるので、直す価値はある(優先度は低)。
