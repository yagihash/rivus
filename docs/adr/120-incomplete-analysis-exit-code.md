# ADR-120: 完全でない解析は、結果を出したうえで、専用の終了コード 4 で終わる(2026-11-23)

- 状態: 有効
- 関連: ADR-054(時計に依る結果)、`stats.timed_out`、`summary.incomplete`
- 要旨: 全体の時間切れ(`-timeout`)と未収束(`-max-passes`)は、結果を全部出したあと、終了コード 4 で終わる。理由は `summary.incomplete_reasons`(`timeout`、`not_converged`)。`-max-memory` の停止は、同じコードの別の理由として足す(この PR にはない)。

**背景:** 今まで、時間切れも未収束も終了コード 0 で、`stats` を見ないと完全かどうか分からなかった。利用者(ユーザーの決定)は、完全でない解析は 0 にしないことを選んだ。

**決めたこと:**
1. コードは 4(0 成功、1 エラー、2 使い方、3 Rust を断った、130 中断。extractor の 10/20/50/70 は `make model` の中のもので、`rivus analyze` の終了コードではない)。理由が複数でも 1 つのコード(呼ぶ側が知りたいのは、完全かどうか)。
2. 出力は全部出してから終わる。`summary.incomplete`(bool)と `summary.incomplete_reasons`(配列)は、いつも出る。`stats` の項目は変えない。
3. 関数ごとの切り(`stats.func_timeouts`、`budget_exhausted`)は、実行が収束していれば、今はこの PR では不完全に数えない(`summary` と `stats` に出るだけ)。段階を踏む: 先に全体の限界だけ。関数ごとの切りも数える案は、頻度を測ってから(下の数字)。
4. `analyze`、`index -taint`、`query`、`resolve`、`models coverage` が同じ。保存した index を読むだけの `-index` は、今の解析ではないので 0。

**頻度(関数ごとの切り):** `.claude/work/bench-out` の結果では、収束した実行(cve、memos-aws)に `func_timeouts`、`budget_exhausted` は無かった(出力に項目なし)。全体の時間切れは、memos と dartpad のパスラインの実行で起きている。軽い対象での測定は未完。

**呼ぶ側と道具への影響:** 時間切れ(`-timeout`)で、今まで 0 で終わっていた実行が 4 になる。終了コードが 0 でないことを失敗として扱うスクリプトは、4 を許す形にする。`bench/deps_bench.py`、`bench/compare.py`(`rc` は 0 のまま、`exit_code` に 4 を記録し、完全でない実行はキャッシュに入れない)、`bench/go-scale`、`bench/go-synth`、`bench/model-diff` は 4 を許す。

**zoo:** zoo の `F/state/global-chain`(13 段と 30 段の鎖)は、周回の上限で止まるように作ってある(`recursion_approximation` の `@miss`。Go、Dart、Rust に同じ)。それを含む実行(zoo 全体、`TestScaleSynth` の chain13 と chain30)は 4 で終わるのが正しく、テストはその実行だけ 4 を許す(`notFailed`、`notFailedIf`)。ほかの実行は、4 なら失敗で、zoo の残りのセルが収束することは `TestZoo*` の `zooSplit` が確かめる。
