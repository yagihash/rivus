# ADR-008: 不確実性は閉じた terminal kind で分類し、`attributable_to` を持たせる

- 状態: 一部覆った(→ADR-043)
- 関連: ARCHITECTURE 4.4
- 状態の詳細: 「追加はバージョンを上げる」は運用で逆。ADR-043 追記 CORE-1、ARCH 4.4; 訂正・追記あり(書き方を保つ)。追記あり。覆した先・注意: バージョン運用
- 要旨: 10 種 + 4 種、`target`/`rivus`/`mixed`。追記: ADR-059 で `assumed_call`

- **状況:** 探索を打ち切ったとき、理由を分類して残したい。ブリーフの 10 種に不足がある。
- **決定:**
  - ブリーフの 10 種に 4 種を追加する: `ffi_boundary`、`unresolved_callee`、`missing_source`（サブ理由を文字列で持つ。proc-macro 展開結果や未生成コードを含む）、`recursion_approximation`
  - 集合は**閉じる**。追加はスキーマのバージョンを上げて行う
  - 各 kind に `attributable_to: target | rivus | mixed` を持たせる。
    `mixed` は「言語機能由来だが rivus の精度向上でも減らせる」もの。集計では `rivus` と別々に出す
- **理由:** これらを `depth_limit` に押し込むと「rivus の都合」と「対象の性質」が混ざる。
  `attributable_to: rivus` の比率がそのまま「rivus の未熟さの指標」になり、どこに投資すべきかが数字で出る。既製 SAST では得られないフィードバックループ。
- **追記(ADR-059、C-7):** `assumed_call`(`attributable_to: mixed`)を足した。使う条件: **呼ばれるところが見えないクロージャ(渡す、保存する、返す)を、作った場所で、呼ばれるものとして適用した**ときだけ。クロージャの要約が、キャプチャした値に依る効果(sink への到達、書き込み、throw、結果)を持つものにだけ付く。`unresolved_callee` は、呼び出し先の関数が分からないとき(候補を増やせば直る)で、こちらは、呼び出し先は分かっていて、呼ぶ側が見えないとき(メモリをたどる、モデルを足す、で直る)。修正方法が違うので、集計と `attributable_to` の指標を濁らせないために、別の kind にした。
- **追記(ADR-093、auto で Rust を読む):** フィールド `skipped_languages`(`{"lang","why"}` の並び。`summary.unread` と index の `meta.unread`)を足した。読まなかった言語を残すフィールドで、失敗した package ではない。スキーマのバージョンは上げない(フィールドの追加)。
- **帰結:** kind ごとの割り当て初版は ARCHITECTURE.md 4.4。`dynamic_dispatch` / `unresolved_callee` / `concurrency` は `mixed`。
  コールグラフの精度を実装前に見積もれないため、2 値に決め打ちせず 3 値で様子を見る。Phase 1 のステップ 1.12 の実測後に見直す（レビューで決定）。
  `container_element_imprecision` は結論に影響した場合のみ記録する（ノイズで集計が無意味にならないように）。
  ガード上限の超過は専用 kind にせず、サブ理由 `guard_limit` として別の記録に残す。
  `depth_limit` の比率は上限チューニングの指標として監視する。

