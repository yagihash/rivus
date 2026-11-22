# #97 Dart: call メソッドを持つクラスのインスタンスを関数のように呼ぶと(r(x))、呼び先が未解決になる

- 状態: **closed**
- 種類: 黙った見逃し
- 言語: Dart
- ROADMAP の節: 1
- 守るもの: 動物園 `F/dart/callable-class`(`@miss` が外れた)、`cmd/rivus/dart_callable_test.go`
- GitHub: #97(移行前の番号は同じ。2026-11-21 に開いた。移行時の状態: OPEN)

## 今の状態(2026-11-22 の見直し)

直した: 537c6b9。確認: 2026-11-22。

## 本文(GitHub から移した写し)

動物園(bench/zoo-dart)の作り直しの前に見つかった見逃し。`call` メソッドを持つクラスのインスタンスを、関数のように `r(x)` と呼ぶと、呼び先が解決されず `unresolved_callee` になる(抽出器が、`call` メソッドへの暗黙の呼び出しを作っていない)。結果、`call` の中の source / sink の流れが落ちる。

期待: `r(x)` を `r.call(x)` として扱い、静的に解決する(Dart の言語の規則)。

確認方法: bench/zoo-dart に、脆弱 / 直した / おとりの 3 ケースのマス(`dyn/callable-class` 相当)を足し、今は見逃し(痕跡あり)として記録、直ったら外す。

