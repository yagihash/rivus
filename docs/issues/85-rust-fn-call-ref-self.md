# #85 Rust: Fn / FnMut を &self で呼ぶと、レシーバがセルでクロージャ候補にガードが付かず、呼び出し元ごとに絞れない(誤検出)

- 状態: **partial**
- 残り: `rust-curated/src/fnself.rs` の `@fp` が、`fn_ref_clean` の 1 か所に残る(本文の 6 か所は、書いたときの数)。
- 種類: 過検出
- 言語: Rust
- ROADMAP の節: 後回し
- 守るもの: zoo(Rust)`F/clos/two-callers`
- GitHub: #85(移行前の番号は同じ。2026-11-21 に開いた。移行時の状態: OPEN)

## 本文(GitHub から移した写し)

## 確度
コード読解 + フィクスチャで実行確認(78c3bdb。`@fp` のフィクスチャが失敗する)。

## 場所
- `internal/taint/calls.go:357` `dispatchGuards`: `recv.Kind != sg.KParam` のとき nil を返す
- `makeDispatchGuards`: 候補が method でない(`!fn.Recv`)と、ガードを付けない。クロージャは全部これに該当する

## 再現
`bench/rust-curated/src/fnself.rs`
- 誤検出(`@fp`): `fn_ret_clean` / `fn_mut_ret_clean`(`@sink(none)`)、`fn_sink_a/b`、`fn_mut_sink_a/b`(`@nosink`)の 6 か所
- 対照: `fn_ref_clean`(`&impl Fn`。レシーバがパラメータそのもの)も落ちる

## 期待と実際
期待: `Fn` / `FnMut` を `&self` で呼ぶとき、呼び出し元ごとに候補を絞る(ガードが効く)ので、別の呼び出し元のクロージャの流れが混ざらない。
実際: レシーバがセル(パラメータそのものでない)だとガードが付かない。さらに、候補がクロージャ(`!fn.Recv`)だとガード自体が付かない。レシーバがセルであることだけが原因ではない(対照も落ちる)。6 か所とも `@fp`。

## 影響
ヘルパーが `f(..)` で呼ぶ `impl Fn` の形で、別の呼び出し元のクロージャの値が混ざる誤検出。`bench/rust-curated` の XTALK の `@fp`(12)と `@noisy`(16 か所)が同じ形(1 件ずつ、レシーバがセルかは未確認)。

## 修正の方向
ADR-047(案 2)で直る見込み(ヘルパーの要約にコールバックの出力を持たせ、呼び出し元が渡したクロージャで具体化するので `dispatchGuards` を通らない)。これは下書きからの推測で、実装して確かめたものではない。案 2 の受け入れは、この 7 か所の `@fp` が全部消えること。

## 関連
bench/MISSES.md「未解決のレビュー指摘」節、ADR-047、#66、#81

別セッションのレビューの識別子: RUST-13

