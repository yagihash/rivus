# #99 Dart: 値として持たれた関数・クロージャ(フィールド、map、返したクロージャ、変数)の呼び出しが、候補 0 件のグラフの辺になる

- 状態: **open**
- 種類: 道具・ベンチ(グラフの辺)
- 言語: Dart
- ROADMAP の節: 1
- 守るもの: 動物園 `F/clos/*`、`TestZeroCandidateCalls`、`TestFuncValuesDart`
- GitHub: #99(移行前の番号は同じ。2026-11-21 に開いた。移行時の状態: OPEN)

## 今の状態(2026-11-22 の見直し)

関数値の要素(段階 4)の第 2 歩と捕獲セルで閉じる見込み。第 1 歩(`-func-values`、診断)は 88c4cf2。

## 本文(GitHub から移した写し)

## 現象
`h.a!()`(フィールドに入れたクロージャを呼ぶ)、map から取り出した関数の呼び出し、返したクロージャをその場で呼ぶ形、変数に入れたクロージャの呼び出しは、グラフ(`rivus index`)の呼び出し点が `kind: value` で、**候補が 0 件**になり、実際に呼ばれる関数への辺がない。taint の側は、多くの形で流れを見つけている(動物園の `clos/stored`、`clos/returned`、`dyn/func_field`、`dyn/func_map` ほか)。

## 再現(`bench/zoo-dart/cells/lib/clos/stored.dart`)
```dart
class Holder { void Function()? a; }
void callA(Holder h) => h.a!();
void vuln() {
  final v = src('t');
  final h = Holder();
  h.a = () { sinkCmd(v); };
  callA(h);
}
```
`callA` の `h.a!()` の呼び出し点は候補 0 件。実行すると `vuln` のクロージャが呼ばれる。

## どう見つけたか
動物園の Dart を実行して観測した呼び出しと、グラフの辺の突き合わせ(`docs/DESIGN-observed-calls.md`)。辺がない 39 件のうち、この形が約 14 件(stored、partial_callers、func_field、func_map、returned、tearoff の一部)。ほかに super / mixin(`super_mixin_impl` 4 件)と `Navigator.pushNamed` のルート(3 件)が、同じく辺がない。

## 関係
#77、#81、#97(呼び出しが未解決になる形)と関連する。参照の段(`rivus graph path` の 4 段目)では届くものと届かないものがある。

