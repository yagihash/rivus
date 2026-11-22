# #80 Dart: 標準ライブラリのコールバック(catchError、whenComplete、putIfAbsent、Map.forEach、onError、Future.sync)と await した Future.error が、決め打ちのリストに無く何も報告せずに失われる

- 状態: **open**
- 種類: サイレントな見逃し
- 言語: Dart
- ROADMAP の節: 2 / 3
- 守るもの: -
- GitHub: #80(移行前の番号は同じ。2026-11-20 に開いた。移行時の状態: OPEN)

## 本文(GitHub から移した写し)

## 現象
標準ライブラリの高階関数のうち、extractor が「コールバックを呼ぶ」と知っているのは、決め打ちのリストだけ(`Future.then`、`Stream.listen`、`Iterable.forEach` / `map` / `where` など)。**リストに無い呼び出しの、コールバックの中の sink や、コールバックが返す値が、落ちる**。`Map.forEach` 以外は不確実性も出ない(**サイレントな見逃し**)。実コードで、とてもよく使う形。

## 再現(Dart)
```dart
Future<String>.error(source()).catchError((e) { sink(e); return ''; });   // 出ない
final m = <String, String>{}; final t = source();
sink(m.putIfAbsent('k', () => t));                                        // 出ない
final m2 = {'k': source()}; m2.forEach((k, v) => sink(v));                // 出ない(dynamic_dispatch/rivus は出る)
Future.value(1).whenComplete(() { out = t; }); sink(out);                  // 出ない
Future<String>.sync(() => t).then((v) => sink(v));                         // 出ない
Future<String>.error(a).then((v) => v, onError: (e) => sink(e));           // onError を呼ばない
```
同じ形の `xs.forEach((x) => sink(x))` と `xs.fold(...)` は見つかる(対照)。
`await Future<void>.error(AppError(source()))` を `on AppError catch (e)` で受けて `e.detail` を sink に渡す形も落ちる(失敗した Future の await が、throw として値を渡さない)。

## `@miss`
`bench/dart-curated`: `catch-error` `when-complete` `completer` `put-if-absent` `on-error` `map-entries` `future-error`。`bench/MISSES.md` の型 `CB-LIST`(6 件)と `AWAIT-ERR`(1 件)。

## 落ちる場所の推定
`extractor/dart/lib/exprs.dart` の `_callbackMode`(決め打ち)、`internal/model/builtin/dart-async.yaml` / `dart-core.yaml`(モデルは args → 結果、recv → 結果だけで、コールバックの戻り値と引数を繋がない)。ADR-047(案 2)では直らない。表(コールバックを呼ぶ API、引数、結果の繋ぎ方)をモデルの側に持つのが筋。

