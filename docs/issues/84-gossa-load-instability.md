# #84 go/ssa のロードの揺れ: 同じ入力で、関数の数が実行ごとに数件違う(stats.functions)

- 状態: **open**
- 種類: 道具・ベンチ
- 言語: Go
- ROADMAP の節: 2(経路の欠け・揺れ)
- 守るもの: -
- GitHub: #84(移行前の番号は同じ。2026-11-21 に開いた。移行時の状態: OPEN)

## 本文(GitHub から移した写し)

## 問題

同じバイナリで、何も変えずに、rivus 自身(`dogfood/rules.yaml`)を繰り返し解析すると、`stats.functions` が 6676〜6678 と、実行ごとに揺れる(`reanalyses` と `widest_level` も 1 ずつ動く)。finding は同じ。`docs/DESIGN-graph.md` の「既知の揺れ」(依存つきの Go のロードで、標準ライブラリの関数の本体の有無が、実行ごとに数件違う。x/tools で 9,542〜9,549。`(*sync.Mutex).Lock` や `slices.Repeat` など)と、同じものと見られる(`ssautil.AllFunctions` を含む、go/ssa のロード)。

## 影響

finding が同じでも、**入力(関数の集合)が、実行ごとに違う**ので、原理的には、結果が変わりうる。「rivus 単体は決定的」という方針に反する状態。常設の決定性のテスト(`cmd/rivus/determinism_test.go` の `TestDeterministicDogfood`)は、この揺れのために、finding だけを比べている(`stats` は比べない)。**揺れを、テストが隠している**ので、この issue で、原因を調べ、直したら、`stats.functions` も比べるようにする。

## 調べること

揺れる関数の名前(本体が、ある実行ではあり、別の実行ではないもの)を、1 度出す(`rivus debug extract -all` を繰り返して、差を取る)。ロードのどこで決まるか。

## 優先度

低(急ぎではない)。

