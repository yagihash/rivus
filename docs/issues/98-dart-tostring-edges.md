# #98 Dart: 文字列補間と sinkAny が呼ぶ toString が、呼び出しのエッジ(グラフ)にならない

- 状態: **open**
- 種類: 道具・ベンチ(グラフのエッジ)
- 言語: Dart
- ROADMAP の節: 1
- 守るもの: -
- GitHub: #98(移行前の番号は同じ。2026-11-21 に開いた。移行時の状態: OPEN)

## 本文(GitHub から移した写し)

## 現象
`"run $w"`(文字列補間)や、値を文字列にして渡す SDK の呼び出しは、実行時にその型の `toString` を呼ぶ。`rivus index` のグラフには、この呼び出しのエッジも、`toString` への参照もない。taint の側(`rivus analyze`)は、zoo の `dart/interp_tostring`、`cont/struct_whole_to_sink` で、この流れを見つけている(`@miss` なし)。つまり、**グラフだけが欠けている**。

## 再現(`bench/zoo-dart/cells/lib/dart/interp_tostring.dart`)
```dart
class Wrapper {
  Wrapper(this.v);
  final String v;
  @override
  String toString() => v;
}
void vuln() {
  final w = Wrapper(src('t'));
  sinkCmd('run $w');
}
```
`rivus index` のあと `rivus graph path --from vuln --to Wrapper.toString`: 実際に起きる呼び出しなのに、経路なし(2026-11-22 の修正以前は `exhaustive: true` だった。今は `false`)。

## どう見つけたか
zoo の Dart を実行して観測した呼び出しと、グラフのエッジの照合(`docs/DESIGN-observed-calls.md`、`bench/zoo-dart/observed/`)。`no_ref` の 6 件がこの形。

## 修正の方向
文字列補間の `toString` 呼び出しは言語の意味(脱糖は extractor の責務)なので、extractor が呼び出しとして出す(候補を広げる修正。否定のフィクスチャが必要: 補間する値の型の `toString` だけに絞る)。

