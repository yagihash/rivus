# #86 Rust: モデルを適用した動的呼び出しは read_impls でないと実装の本体を読まず、本体の source の見逃しと、捨てる値の誤検出が何も報告せずに起きる

- 状態: **open**
- 種類: サイレントな見逃し
- 言語: Rust
- ROADMAP の節: 2
- 守るもの: -
- 題: Rust: モデルを適用した動的呼び出しは read_impls でないと実装の本体を読まず、本体の source の見逃しと、捨てる値の…
- GitHub: #86(移行前の番号は同じ。2026-11-21 に開いた。移行時の状態: OPEN)

## 本文(GitHub から移した写し)

## 確度
コード読解 + フィクスチャで実行確認(78c3bdb。`@miss` と `@fp` が出る)。モデルを外して流すと、`@miss` の流れは見つかり `@fp` は検出されなくなる(実験)。

## 場所
`internal/taint/calls.go:182-196`: モデルを適用した(`modelled`)あと、`!rules.ReadsImpls(cc.sym)` なら return し、動的呼び出しの候補の本体を読まない。`read_impls: true` は `rust-std.yaml` と `rust-misc.yaml` だけ。

## 再現
`bench/rust-curated/src/modelled.rs`(rules.yaml に、プロジェクトの trait のメソッド `Render::render`、`Mask::mask` の propagator(recv→result)を足した)
- `model_body`: source が実装の本体の中にある(`@miss`)
- `model_drop`: 実装が引数を捨てる(`@sink(none) @fp`)

## 期待と実際
期待: trait のメソッドにモデルを書いても、本体が読める候補なら、本体の流れ(source、捨てる)が効く。または、モデルが本体を隠していることが不確実性として出る。
実際: モデルが本体を隠し、本体の source は見逃し、本体が捨てる値は流れ続ける。どちらも何も報告しない(uncertainty なし)。

## 影響
プロジェクトの trait にモデルを書いた利用者が、サイレントな見逃しと誤検出を受ける。モデルが正体を隠す型の指摘(バックログの C-8: モデルに「近似であり、適用したらこの kind の不確実性を出す」属性を足す案と関連)。

## 修正の方向
モデルを適用しても、`read_impls` でない動的呼び出しの本体を読む(または、読まなかったことを不確実性に出す)。案 2(ADR-047)では直らない(別の修正)。

## 関連
bench/MISSES.md「未解決のレビュー指摘」節

別セッションのレビューの識別子: RUST-14

