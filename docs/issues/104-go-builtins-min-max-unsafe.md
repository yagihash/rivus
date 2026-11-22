# #104 Go: 組み込み関数(min / max / unsafe)の結果で、流れが落ちる

- 状態: **closed**
- 種類: サイレントな見逃し(直した)
- 言語: Go
- ROADMAP の節: 1
- 守るもの: zoo `F/conv/builtin-minmax`、`F/conv/unsafe-string`、`cmd/rivus/go_builtin_test.go`

## 今の状態(2026-11-22 の見直し)

直した: bb29727(組み込み関数の結果を引数から作る)、d112867(`unsafe` の組み込み関数を `builtin.unsafe.<名前>` で名指す。CORE-9)。

## 経緯

2026-11-22 に、直してから記録した(「直ってから記録する」)。再現の詳細は書かない。

