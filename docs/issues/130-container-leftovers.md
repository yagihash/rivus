# 130: `-allow-fetch-host`、`-no-fetch` の help と `lang.go` のコメントに、廃止済みのコンテナの記述が残っている

- 状態: **closed**
- 種類: 文書(重さ: 低)
- 言語: Rust
- 見つけた経緯: 利用ガイド(docs/AGENT-GUIDE.md)を書くときに、help とコードを読んで見つかった食い違い。v0.0.4 には入れない。

help とコメントに、"in the container" / "in the volume" が残っている。Rust の隔離は、Docker から macOS のサンドボックスに替えた(ADR-093)ので、今の動作に合わせて直す。

## 直した内容

文を、今の動作に合わせた(`cmd/rivus/analyze.go`、`lang.go`(help とコメント))。
