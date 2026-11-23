# 128: `rivus sites` が Go だけを読む: Dart と Rust のディレクトリで、エラーなしの空の結果になる

- 状態: **closed**
- 種類: サイレントな見逃し(重さ: 高)
- 言語: 全
- ROADMAP の節: 0
- 守るもの: `cmd/rivus/dart_tests_default_test.go`(`TestSitesReadsTheLanguagesOfTheDirectory`)
- 題: `rivus sites` が Go だけを読む: Dart と Rust のディレクトリで、エラーなしの空の結果になる
- 見つけた経緯: 利用ガイド(docs/AGENT-GUIDE.md)を書くときに、help とコードを読んで見つかった食い違い。v0.0.4 には入れない。

`rivus sites` には `-lang` がなく、Go のローダだけで読む。Dart や Rust だけのディレクトリを指すと、エラーにならず `"sites": []`、終了コード 0 で終わる。利用者は「ルールが足りない」と読み違える(読まなかった言語の報告もない)。

修正の案: `sites` も、`analyze` と同じ言語の選択(`-lang`、auto)を通す。少なくとも、読まなかった言語があることを、`summary.unread.skipped_languages` と標準エラーに出す。

## 直した内容

`sites` が、`analyze` と同じ言語の選択(`-lang`、auto: ディレクトリにある言語)を通すようにした。読まなかった言語は、出力の `skipped_languages` に出る(`summary.unread.skipped_languages` と同じ形)。読める言語がない(Dart だけで extractor がない、など)ときは、`analyze` と同じ「読むものがない」というエラーで止まる。守るもの: `TestSitesReadsTheLanguagesOfTheDirectory`。
