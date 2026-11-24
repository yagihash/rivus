# 流れを落とす上限(作業量の予算、反復の上限、実時間)が、止まったとき、流れを落とす。最も粗い形に落とす

- 状態: **open**
- 種類: uncertainty つきの見逃し(上限に届いたとき。結果が負荷で変わるものがある)
- 言語: 全
- ROADMAP の節: 2 / 3
- 守るもの: -
- 題: 作業量の予算・反復の上限・実時間の上限が、止まった所の流れを落とす

[ADR-118](../adr/118-scale-mechanisms-three-conditions.md) の表の行。足りない条件: (i)。関数ごとの実時間は (ii) も。優先度: 高(サイレントではないが、見逃しになり、実時間のものは負荷で変わる)。

## 今、出力はどうなるか

- 作業量の予算(`Budget` 400000): 不確実性 `depth_limit`(detail `budget:`)が関数ごとに出る。`stats.budget_exhausted`。止まった関数の要約は、流れを欠きうる。
- 反復の上限(全体 12、SCC の中 8): 不確実性 `recursion_approximation`(「did not stabilise within the pass limit」、「did not reach a fixed point」)。`stats.converged: false`、`stats.unstable`、`unstable_count`、`not_converged`。
- 関数ごとの実時間(`-func-timeout`、既定 30 秒): 不確実性 `depth_limit`(detail `func_timeout:`)、`stats.func_timeouts`、`summary.load_dependent` と `load_dependent_note`。
- 全体の実時間(`-timeout`): 不確実性 `depth_limit`(「the time limit was reached」)、`stats.run_timeouts`、`load_dependent`。

## すること

1. 上の上限に届いたとき、その関数(SCC)を、最も粗い形(どの引数も、どの出力にも流れうる)にする。流れを落とさない。「予算で粗くする」安全網の設計の、対象の一覧になる。
2. 関数ごとの実時間は、(ii) も満たさない(時計)。要約の大きさのような、入力だけで決まる規則に置き換える。
3. 切ったときの結果が、SCC の順に依らないことをテストで確認する(反復の上限)。

## 守るもの

1 は、上限を小さくした実行が、元の finding を落とさないこと。2 は、`-workers` や負荷を変えても同じ結果になること。

## 追記(2026-11-24、ADR-124)

- 大きい SCC の入口を足した(`scc_coarsened`。既定は無効)。エンジンが解く SCC が N 関数以上で、依存の関数を含むなら、その依存の関数を最初から深さ 0 で解く。判定は、解く前に、コールグラフの構造だけで決める(時間切れや反復の上限に依らない)。
- 残り: N の値と既定。N 未満で収束しない SCC は守られない。プロジェクトの関数だけの SCC は、この仕組みで守られない。
