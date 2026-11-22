# 129: `-tests` の help が「`_test.go` を含める」だけ(Dart と Rust にも効く)

- 状態: **closed**
- 種類: 文書(重さ: 低)
- 言語: 全
- 見つけた経緯: 利用ガイド(docs/AGENT-GUIDE.md)を書くときに、help とコードを読んで見つかった食い違い。v0.0.4 には入れない。

`-tests` の help の文が `_test.go` だけを言っている。実際は、Dart の `test/` と Rust のテスト・例・ベンチにも効く(README にはそう書いてある)。help の文を、README に合わせる。

## 直した内容

文を、今の動作に合わせた(`cmd/rivus/common.go`(`-tests` の help))。
