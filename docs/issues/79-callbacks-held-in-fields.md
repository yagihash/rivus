# #79 Dart / Rust: List やフィールドに持たれたコールバック(エミッタ、リスナー、遅延評価)の中の sink に、値が届かない

- 状態: **open**
- 種類: サイレントな見逃し
- 言語: Dart / Rust
- ROADMAP の節: 2
- 守るもの: zoo `F/clos/stored`、`F/dyn/func-field`
- GitHub: #79(移行前の番号は同じ。2026-11-20 に開いた。移行時の状態: OPEN)

## 今の状態(2026-11-22 の見直し)

関数値の要素(段階 4)で閉じる見込み。

## 本文(GitHub から移した写し)

## 現象
コールバックを **List / フィールドに持ち、あとで呼ぶ**(イベントエミッタ、リスナー、遅延評価、ハブ)形で、登録したコールバックの中の sink に、`emit` した値が届かない。Dart では `dynamic_dispatch`(候補 1)が出るだけ、Rust では**サイレントな見逃し**(ヘルパーの `f(s)` に `unmodeled_library_call`)。

## 再現(Dart)
```dart
class Bus {
  final List<void Function(String)> _l = [];
  void on(void Function(String) f) => _l.add(f);
  void emit(String s) { for (final f in _l) { f(s); } }
}
void s() {
  final b = Bus();
  b.on((s) => sink(s));
  b.emit(source());          // 期待: source → sink   実際: 出ない
}
```
`Lazy(this._make)`(フィールドの関数を getter で呼ぶ)も同じ。
## 再現(Rust)
`Vec<Box<dyn Fn(&str)>>` に `Box::new(|s| sink(s))` を `push` し、`emit` で `for f in &self.subs { f(s) }`。

## `@miss`
`bench/dart-curated`: `emit` `emit-two` `lazy` `across-hub`。`bench/rust-curated`: `emit` `emit-two` `across-hub`(何も報告しなかった)。`bench/MISSES.md` の型 `STORE`(7 件)。

## 落ちる場所の推定
`internal/valuecalls/valuecalls.go` は、メモリ(フィールド、リスト)をたどらない(ファイル冒頭のコメント)。`iter.next` / `field_load` の結果は `refOther` で、呼び出しは候補なしになる。ADR-047(案 2)では直らない(条件 1: 保存されて、別の関数から呼ばれる)。フィールド / 要素を通る関数値の追跡(点の解析)が要る。

