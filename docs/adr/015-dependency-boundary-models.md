# ADR-015: 依存パッケージの境界はモデルで扱う

- 状態: 一部覆った(→ADR-043, ADR-063)
- 関連: ARCHITECTURE 5.5, README
- 状態の詳細: 依存のソースを読む `-deps` が既定オン: ADR-043 追記。範囲: 044→063; 覆した先・注意: 043 追記 / 063
- 要旨: 解析は自モジュールのみ、依存は手書きモデル。`models coverage`

- **決定:** 解析するのは自モジュールのみ。依存（stdlib・第三者ライブラリ）との境界は手書きモデルで扱い、無ければ `unmodeled_library_call`。
  設定で選んだ依存だけ解析対象に含められる。
  **モデルカバレッジレポート**（`rivus models coverage ./...`）を Phase 1 に含める。
- **理由:** 依存まで全部解析すると遅くなり、ノイズも増える。
  一方、手書きモデル方式は「どこにモデルを書けば検出が改善するか」が分からないと機能しない。
  すでに `unmodeled_library_call` を記録しているので集計するだけで実装が軽く、効果が大きい。
- **帰結:** 組み込みモデルの最低ライン: Go は `database/sql`、`os/exec`、`net/http`、`html/template`、`text/template`、`strings` など。
  Dart は `dart:*` と Flutter SDK の主要部分。Rust は `std` と頻出クレート（`serde`、`tokio`、`reqwest` など）。
  すべてユーザー定義と同じ YAML 形式で書く（ドッグフーディング）。

