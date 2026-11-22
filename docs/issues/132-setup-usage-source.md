# 132: `rivus setup` の usage が `[-source …]` を任意に見せている(実際は必須)

- 状態: **closed**
- 種類: 文書(重さ: 低)
- 言語: Rust
- 見つけた経緯: 利用ガイド(docs/AGENT-GUIDE.md)を書くときに、help とコードを読んで見つかった食い違い。v0.0.4 には入れない。

usage の `[-source …]` は、角括弧で任意のように見えるが、実際は必須。usage の書き方を直す。

## 直した内容

文を、今の動作に合わせた(`cmd/rivus/setup.go`(usage))。
