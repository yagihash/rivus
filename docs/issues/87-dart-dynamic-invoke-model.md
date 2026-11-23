# #87 Dart: dynamic なメソッド呼び出しが dart:core#dynamic.invoke のモデルで覆われ、本体の source と sink の見逃し・誤検出が注記なしに起きる

- 状態: **partial**
- 種類: 注記なしの見逃し・誤検出(一部)
- 言語: Dart
- ROADMAP の節: 2
- 守るもの: `graph` の `exhaustive`(GRAPH-3 まで)
- 題: Dart: dynamic なメソッド呼び出しが dart:core#dynamic.invoke のモデルで覆われ、本体の sourc…
- GitHub: #87(移行前の番号は同じ。2026-11-21 に開いた。移行時の状態: OPEN)

## 今の状態(2026-11-22 の見直し)

**残っている形: taint の側の、本体の source と sink の見逃しと誤検出**。グラフの `exhaustive` は、候補 0 件の呼び出しを見て止めるようにした(28a6395、9c315cc)。

## 本文(GitHub から移した写し)

## 確度
コード読解 + フィクスチャで実行確認(78c3bdb。`@miss` 2、`@fp` 1)。モデルを外す対照はしていない。

## 場所
- `extractor/dart/lib/exprs.dart:347` `_methodInvocation`: `dynamic` なレシーバのメソッド呼び出しを `dart:core#dynamic.invoke` の静的呼び出しにする
- `internal/model/builtin/dart-core.yaml:8`: そのモデルは `flows: [{from: args, to: result}]`

## 再現
`bench/dart-curated/lib/dynamic_calls.dart`
- `dynamic_arg`: メソッドの中の sink に届かない(`@miss`)
- `dynamic_body`: メソッドの本体の source が届かない(`@miss`)
- `dynamic_drop`: 実装が引数を捨てる(`@sink(none) @fp`)

## 期待と実際
期待: `dynamic` のメソッド呼び出しは、候補の本体を読むか、近似であることが不確実性に出る。
実際: モデルで覆われて不確実性の注記が残らない。サイレントな見逃しと誤検出。

## 影響
Dart の `dynamic` を使うコード(JSON を扱う部分など)で、何も報告せずに流れが失われる、または混ざる。モデルが正体を隠す型(RUST-14 と同じ型。バックログ C-8 の属性案と関連)。

## 修正の方向
`dynamic.invoke` を、レシーバの動的な型の候補(メソッド名で絞る)への呼び出しとして扱うか、モデルに「近似」の不確実性を付ける。案 2(ADR-047)では直らない。

## 関連
bench/MISSES.md「未解決のレビュー指摘」節、#30(レシーバの式が落ちる件。閉じた)

別セッションのレビューの識別子: DART-12

