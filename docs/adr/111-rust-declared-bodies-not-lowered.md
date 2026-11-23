# ADR-111: Rust の extractor は、宣言のある依存クレートの本体を、IR に落とさない(ADR-105 の Rust の分。2026-11-23)

- 状態: 有効
- 関連: ADR-105、DESIGN-package-models 7a

**背景:** ADR-105 は、モデルの `body_replaced_packages` が宣言したパッケージの本体を、extractor が読まない(IR に出さない)形を、Go と Dart で決めた。Rust には、まだなかった(`cmd/rivus/lang.go` が Dart のときだけ宣言を渡していた)。

**決めたこと:**
1. Rust の driver は、宣言のある依存クレートの本体を、MIR に落とさない。出すのは `body_not_read`(`unit`、本体を持つ関数の数)だけ。数は MIR を作らずに数える。トレイトの呼び出しと `Drop` のリンクのために、関数の名前は `merge` に渡して、渡したあとは出力から落とす。
2. 宣言の `actix-web` と `actix_web` は、同じクレートを指す。照合は、`/`、`.`、`:` を含まない名前(Rust のクレートと Go の標準ライブラリ)だけ、`-` と `_` を同じに扱う。Go の import path と Dart の `package:` の名前は、今までどおり文字どおりに照合する。
3. `cargo check` は、宣言したクレートも全部コンパイルする。なので、Rust の段階 4 で小さくなるのは、IR と、解析の入力で、extractor の時間とメモリは減らない(測定: IR が 9% と 29% 減り、時間と最大メモリは誤差の範囲)。
4. 宣言を extractor で行った場合と、フィルタ(`-drop-after-read`)で行った場合の違いは、`functions_declared` と `functions`、`fn_skipped`(飛ばしたクレートでは減る)、宣言したクレートの中の「同じ名前の関数が 2 つのクレートにある」注記だけ。findings と `summary.program` は同じ(テストで確認)。

**しないこと(この ADR では):** 解析側の効果の測定(測定の対象に、依存の関数に届く経路が少なかった)。サンドボックスが `DEVELOPER_DIR` を自分で設定すること。
