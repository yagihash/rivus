# 174: zoo の実行の確認は、本物のエスケープの `@sink(none)` を裏づけられない

- 状態: **open**
- 種類: 検査の穴
- 言語: 全(zoo)
- ROADMAP の節: 1
- 守るもの: -

zoo の実行の確認(truth)は、掃除の関数が危険な文字を `_` に替えるものだと決めている。本物のエスケープ(`html.EscapeString`、Dart の `HtmlEscape.convert`)は、2 つの変種(`<>&` と `<&>`)で違う文字列を返す。sink の側では「届かなかった」でも「運ばれた」(carry)と数えるため、`@sink(none)` と `@nosink` が、実行の確認で落ちる。そのため、組み込みの sanitizer の行(ADR-115)は zoo のセルにできず、`cmd/rivus/sanitizer_scope_test.go` の実コードのテストで確認している。

直す方向: 「運ばれたが危険は届かなかった」を、`@sink(none)` と両立する判定にする(危険の部分が壊れていれば、文字列が違っても carry と数えない)。
