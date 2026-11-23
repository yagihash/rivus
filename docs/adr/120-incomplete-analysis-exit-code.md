# ADR-120: 完全でない解析は、結果を出したうえで、専用の終了コード 4 で終わる(2026-11-23)

- 状態: 有効(WIP: 段階の根拠の数字が未完)
- 関連: ADR-054(時計に依る結果)、`stats.timed_out`、`summary.incomplete`
- 要旨: 全体の時間切れ(`-timeout`)と未収束(`-max-passes`)は、結果を全部出したあと、終了コード 4 で終わる。理由は `summary.incomplete_reasons`(`timeout`、`not_converged`)。`-max-memory` の停止は、同じコードの別の理由として足す(この PR にはない)。

**背景:** 今まで、時間切れも未収束も終了コード 0 で、`stats` を見ないと完全かどうか分からなかった。利用者(ユーザーの決定)は、完全でない解析は 0 にしないことを選んだ。

**決めたこと:**
1. コードは 4(0 成功、1 エラー、2 使い方、3 Rust を断った、130 中断。extractor の 10/20/50/70 は `make model` の中のもので、`rivus analyze` の終了コードではない)。理由が複数でも 1 つのコード(呼ぶ側が知りたいのは、完全かどうか)。
2. 出力は全部出してから終わる。`summary.incomplete`(bool)と `summary.incomplete_reasons`(配列)は、いつも出る。`stats` の旗は変えない。
3. 関数ごとの切り(`stats.func_timeouts`、`budget_exhausted`)は、実行が収束していれば、今はこの PR では不完全に数えない(`summary` と `stats` に出るだけ)。段階を踏む: 先に全体の限界だけ。関数ごとの切りも数える案は、頻度を測ってから(下の数字)。
4. `analyze`、`index -taint`、`query`、`resolve`、`models coverage` が同じ。保存した index を読むだけの `-index` は、今の解析ではないので 0。

**頻度(関数ごとの切り):** `.claude/work/bench-out` の結果では、収束した実行(cve、memos-aws)に `func_timeouts`、`budget_exhausted` は無かった(出力に項目なし)。全体の時間切れは、memos と dartpad のパスラインの実行で起きている。軽い対象での測定は未完。
