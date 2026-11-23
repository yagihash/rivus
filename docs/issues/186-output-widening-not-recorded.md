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

## 守るもの

2 のテスト。1 は、widening した関数が一覧に出ること。
