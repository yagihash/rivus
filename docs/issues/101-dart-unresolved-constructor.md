# #101 Dart: 解決できないコンストラクタ呼び出しの結果が、汚染のない定数になる

- 状態: **closed**
- 種類: 黙った見逃し(直した)
- 言語: Dart
- ROADMAP の節: 1
- 守るもの: `cmd/rivus/dart_syntax_test.go`(`TestDartUnresolvedConstructorKeepsTheFlow`)

## 今の状態(2026-11-22 の見直し)

直した: e4e6c5f、ADR-091(解決できない式が値を返すとき、`unsupported` が元の値と結果を持つ)。

## 経緯

2026-11-22 に、直してから記録した(「直ってから記録する」)。再現の詳細は書かない。

