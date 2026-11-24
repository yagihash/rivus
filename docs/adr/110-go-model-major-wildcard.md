# ADR-110: Go のモデルの行は、module の path の major を `/v*` と書いて、どの major にも当てる(2026-11-23)

- 状態: 有効
- 関連: DESIGN-package-models.md 6.9、MODEL-RUNBOOK.md 4 節
- 状態の詳細: 解析の読み込みで major ごとの写しを足す(`Rules.ExpandMajors`)。範囲の外の major の uncertainty は別の変更
- 要旨: major ごとにファイルや行を写さない

**背景:** Go の module は、major が 2 以上になると、path に `/vN` を持ち、モデルの行の鍵(シンボル)にも入る。`google/go-github` は major が頻繁に上がり(v84 から v93 まで)、`pgx/v5`、`go-redis/v9`、`jwt/v5` なども、`docs/MODEL-QUEUE.md` にある。major ごとにファイルや行を写すと、行の数、`.dead-allow`、zoo のセルが、major の数だけ増える。

**決めたこと:**
1. **書式で解決する。major ごとにファイルや行を写さない。** シンボル、`kind: field` の `field`、`kind: param_type` の `type`、`verified_versions` の `package` で、major の要素を `/v*` と書ける(`github.com/google/go-github/v*/github.Client`)。
2. **`/v*` が当たるのは、path の 1 要素で、`/v` と数字(2 以上、先頭が 0 でない)だけ。** 任意の glob にしない。`/vendor`、`/v2x`、major のない path には当たらない。1 回だけ、要素の全体として書く。Go だけ。違反は、モデルの読み込みで失敗する。
3. **解析の読み込みで、プログラムが持つ major ごとに、行の写しを具体的なシンボルで足す**(`Rules.ExpandMajors`。`match: implementations` の展開と同じ位置)。索引の鍵の形、ファイルの行、`/v*` を書かない行の動作は変わらない。
4. **`verified_versions` は、`package: <module>/v*`、`versions` は、実際に差分テストを通した範囲**(その major だけ)。差分テストは、`-version` で major を決める。範囲の外の major が解析の対象のとき、`summary` に出すこと(設計 6.7 の 3)は、別の変更。
5. 設計は DESIGN-package-models.md 6.9、手順書は MODEL-RUNBOOK.md 4 節の末尾。

**しないこと(この ADR では):** `body_replaced_packages` の `package` の `/v*`、Dart と Rust の同様の書式、path のほかの形(`gopkg.in/x.v3`、`aws-sdk-go-v2` のように、`/vN` の要素でない major の表し方)。

