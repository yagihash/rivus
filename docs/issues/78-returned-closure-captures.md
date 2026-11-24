# #78 Dart / Go: 関数が返したクロージャを呼ぶと、キャプチャした値と中の source が落ちる(カリー化、ファクトリ、functional options)

- 状態: **open**
- 種類: サイレントな見逃し
- 言語: Dart / Go
- ROADMAP の節: 2
- 守るもの: zoo `F/clos/returned`
- 題: Dart / Go: 関数が返したクロージャを呼ぶと、キャプチャした値と中の source が落ちる(カリー化、ファクトリ、functiona…
- GitHub: #78(移行前の番号は同じ。2026-11-20 に開いた。移行時の状態: OPEN)

## 本文(GitHub から移した写し)

## 現象
**関数が返したクロージャ**を呼ぶと、そのクロージャがキャプチャした値や、中の source が落ちる。ファクトリ、カリー化、functional options(Go)、関数を返す getter でよくある書き方。

## 再現(Dart)
```dart
String Function() holder(String s) => () => s;

void a() {
  final get = holder(source());
  sink(get());               // 期待: source → sink   実際: 出ない(sink の行に unresolved_callee)
}

void curry() {
  final a = source();
  String Function(String) Function(String) add = (p) => (q) => p + q;
  sink(add(a)('x'));         // 実際: 出ない
}

class Source { String Function() get maker => () => source(); }
void g() { sink(Source().maker()); }   // 実際: 出ない
```
## 再現(Go、functional options)
```go
type Opt func(*Q)
func With(n string) Opt { return func(q *Q) { q.name = n } }
func build(opts ...Opt) *Q { q := &Q{}; for _, o := range opts { o(q) }; return q }
q := build(With(src()))
snk(q.name)                  // 期待: src → snk   実際: 出ない(黙った)
```
`o := With(src()); o(q)` と書いても落ちる(実験)。

## `@miss`
`bench/dart-curated`: `holder-a` `holder-b` `curry` `getter-fn`(uncertainty あり: `unresolved_callee` が sink の行)。`bench/go-curated`: `options`(**何も報告しなかった**。ヘルパーの `o(q)` に `dynamic_dispatch` だけ)。`bench/MISSES.md` の型 `RET-CLOSURE`(5 件)。

## 落ちる場所の推定
* Dart: `internal/valuecalls/valuecalls.go` の `classify`: 呼び出しの結果は `refOther`(候補が付かない)。
* Go: VTA の候補は付くが、束縛が無い(`internal/taint/calls.go` の `callees` は、静的なクロージャの `Bindings` だけ)。キャプチャした `n` が落ちる。
ADR-047(案 2)では直らない(別の原因: 関数の戻り値としてのクロージャの追跡)。


関連: 型が合う関数値を候補に補う案(今は入れない)は #207。
