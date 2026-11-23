# 出力の widening(24 要素)が、出力に記録されない。セルの widening(96 要素)と `guard_limit` の、順序に依らないことの確認

- 状態: **open**
- 種類: サイレントな過大近似(注記のない、過大近似の側)
- 言語: 全
- ROADMAP の節: 2
- 守るもの: -
- 題: 出力の widening(24 要素)の記録がない。widening と `guard_limit` の、順序に依らないことの確認

[ADR-118](../adr/118-scale-mechanisms-three-conditions.md) の表の行。足りない条件: 出力の widening は (iii)。セルの widening(96 要素)と `guard_limit` は (ii) が未確認。

## 今、出力はどうなるか

- 出力の widening(`internal/taint/summary.go` の `widen`、`maxOutElems` = 24): 何も出ない。要約の中で、同じパラメータの要素が 1 つの全体にまとまり、その分、finding が多めに出うる(過大近似の側)。不確実性にも `stats` にも `summary` にも、残らない。
- セルの widening(`state.go` の `maxCellElems` = 96): 不確実性 `depth_limit`(detail `path_limit:`)に、切った関数の一覧が出る。
- `guard_limit`: 不確実性 `depth_limit`(detail `guard_limit:`)が、関数ごとに出る。

## すること

1. 出力の widening を、セルの widening と同じ形で記録する(`path_limit:` の関数の一覧に、widening した関数を入れる、または同じ形の記録を足す)。小さい変更。
2. テスト 1 つ: SCC の処理の順を入れ替えても、セルの widening と `guard_limit` を通る要約と finding が同じになること。

## 済んだこと(2026-11-23)

- すること 1: 出力の widening を記録した。不確実性 `depth_limit` の 1 件(detail は `out_limit:` で始まる。`candidates` に、最終の要約に畳んだ出力がある関数の ID。ソート済み)。`stats` に `out_widened_funcs`(関数の数)と `out_widened_outputs`(畳んだ出力の数)。数えるのは `Summary.WidenedOutputs`(内部。署名、出力、索引には入らない)で、最終の要約から集める(順序に依らないことの確認は、すること 2 に入れる: 下の「残り」)。ちょうど 24 要素では畳まず、数えない。結果(finding、要約、ID)は変わらない。期待値のファイルで変わったのは `depth_limit` の数だけ(`middleware` 10 から 11、`all` 62 から 63、`middleware18` を足した。`widen`、`registry`、`longlived` は 0 から 1)。
- テスト: `internal/taint/outlimit_test.go`(24 と 25 の境目、一覧のソート)、`TestScaleSynth` の `middleware18`(`-workers 1` と `4` で `candidates` が同じ)。
- 索引: 要約は索引に保存されない(保存されるのは IR)ので、扱うことはない。

## 残り

- すること 2 のテスト(SCC の処理の順を入れ替えても、セルの widening と `guard_limit` を通る要約と finding が同じ)。
- finding からの対応: 今の記録は、プログラム全体の一覧。ある finding の経路が、畳まれた出力を通ったかは分からない(`Step.Func` を `candidates` に当てることはできるが、その関数の畳まれていない出力を通った場合と区別できない)。分かるようにするには、`OutElem` に「畳んだ」のマークを持たせ、要約を適用するステップの `Notes` に付ける(`access_path_truncated` と同じ形)必要がある。要素の同一性、合成、署名に触れるので、この課題では実装しない。

- 記録されるのは、畳んだ関数だけ。その要約を使った呼び出し元は、一覧に出ない(畳んだ結果を受け取っているのに)。finding の経路のステップに付ける設計は、これも解く必要がある。
- 順序に依らないか: マークは合成で OR なので、SCC の中の処理の順で、途中の要約が畳まれるかどうかが変わるなら、最終の一覧も変わりうる(未確認)。すること 2 のテスト(順序を入れ替える)で、`out_limit` の一覧も、比べる対象に入れる。
- 数字(2026-11-23、memos 00fcd170、1 回、GOGC=25): 畳まれた出力のある関数 1098(関数の約 2.7%)、畳まれた出力 4550、記録は 1 件。

## 守るもの

2 のテスト。1 は、widening した関数が一覧に出ること。

## 済んだこと(2026-11-23、ステップの単位の注記)

ステップの単位の注記を入れた(ADR-118 の追記)。要約を適用するステップで、畳んだ出力の、パラメータ全体になった要素から作るステップに、`depth_limit` の注記(detail は `out_widened:`)が付く。そのステップを通る finding の `uncertainties` にも入り、`confidence` は `requires_review` になる(`no_uncertainty_on_path` だった finding が変わることがある。ID と `stats` は変わらない)。順序(宣言、ファイル、`-workers`)を入れ替えても、付くステップの集合は同じ(テストあり)。

残り: (1) 畳んだ要素だけに付ける(今は、もとから全体だった要素にも、畳んだ出力の中なら付く)。`OutElem` にマークが要る。(2) セルの widening(96 要素)と `guard_limit` の、順序に依らないことの確認(すること 2 のうち、出力の widening 以外)。(3) `out_limit` の記録の、`candidates` の一覧が、順序に依らないかの確認。
