# #85 Rust: Fn / FnMut を &self で呼ぶと、受け手がセルでクロージャ候補にガードが付かず、呼び元ごとに絞れない(誤検出)

- 状態: **open**
- 種類: 過検出
- 言語: Rust
- ROADMAP の節: 後回し
- 守るもの: 動物園(Rust)`F/clos/two-callers`
- GitHub: #85(移行前の番号は同じ。2026-11-21 に開いた。移行時の状態: OPEN)

## 本文(GitHub から移した写し)

## 確度
コード読解 + フィクスチャで実行確認(78c3bdb。`@fp` のフィクスチャが落ちる)。

## 場所
- `internal/taint/calls.go:357` `dispatchGuards`: `recv.Kind != sg.KParam` のとき nil を返す
- `makeDispatchGuards`: 候補が method でない(`!fn.Recv`)と、ガードを付けない。クロージャは全部これに当たる

## 再現
`bench/rust-curated/src/fnself.rs`
- 誤検出(`@fp`): `fn_ret_clean` / `fn_mut_ret_clean`(`@sink(none)`)、`fn_sink_a/b`、`fn_mut_sink_a/b`(`@nosink`)の 6 か所
- 対照: `fn_ref_clean`(`&impl Fn`。受け手がパラメータそのもの)も落ちる

## 期待と実際
期待: `Fn` / `FnMut` を `&self` で呼ぶとき、呼び元ごとに候補を絞る(ガードが効く)ので、別の呼び元のクロージャの流れが混ざらない。
実際: 受け手がセル(パラメータそのものでない)だとガードが付かない。さらに、候補がクロージャ(`!fn.Recv`)だとガード自体が付かない。受け手がセルであることだけが原因ではない(対照も落ちる)。6 か所とも `@fp`。

## 影響
ヘルパーが `f(..)` で呼ぶ `impl Fn` の形で、別の呼び元のクロージャの値が混ざる誤検出。`bench/rust-curated` の XTALK の `@fp`(12)と `@noisy`(16 か所)が同じ形(1 件ずつ、受け手がセルかは未確認)。

## 修正の方向
ADR-047(案 2)で直る見込み(ヘルパーの要約にコールバックの出力を持たせ、呼び元が渡したクロージャで具体化するので `dispatchGuards` を通らない)。これは下書きからの推測で、実装して確かめたものではない。案 2 の受け入れは、この 7 か所の `@fp` が全部消えること。

## 関連
bench/MISSES.md「未解決のレビュー指摘」節、ADR-047、#66、#81

別セッションのレビューの識別子: RUST-13

