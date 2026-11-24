# #107 zoo: recursion_approximation の判定が、無関係な uncertainty を数える

- 状態: **closed**
- 種類: 道具・ベンチ(直した)
- 言語: 全
- ROADMAP の節: 1
- 守るもの: zoo のテスト(`cmd/rivus/zoo_*_test.go`)
- 題: zoo: recursion_approximation の判定

## 今の状態(2026-11-22 の見直し)

直した: zoo の `@miss` の uncertainty の判定を、大域の `recursion_approximation` では数えず、注釈が指定するときだけ数える形にした(`missTraces`)。

## 経緯

2026-11-22 に、直してから記録した(「直ってから記録する」)。再現の詳細は書かない。

