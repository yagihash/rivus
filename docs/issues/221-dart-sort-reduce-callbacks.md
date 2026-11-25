# #221 Dart: `List.sort` の比較関数と `Iterable.reduce` の関数に、リストの要素が渡らない(`calls` の行が無い)

- 状態: **open**
- 優先度: 中
- 種類: サイレントな見逃し
- 言語: Dart
- ROADMAP の節: -
- 守るもの: `bench/zoo-go/cells/clos/to_sort`(Go の対応。Dart のセルは、効かない形なので、まだ置いていない)
- 題: Dart: `List.sort` と `Iterable.reduce` の関数が、要素を引数に受けても、sink に届かない

## 何が起きるか

関数値を受けて呼ぶ API の調査(zoo のセル `clos/to_later`、`clos/to_sort`、`clos/to_lib_call`)で、Dart の次の 2 つの形だけ、コールバックの引数に、レシーバーの要素が届かなかった。

```dart
final s = <String>[src(), src()];
s.sort((a, b) { sinkCmd(a); return a.compareTo(b); });       // 出ない(b も同じ)
[v, v].reduce((a, b) { sinkCmd(a); return b; });             // 出ない
```

同じ調査で、`Future.whenComplete`、`Future.catchError`、`Stream.map`、`Stream.listen`(関数を直接渡す形)、`Iterable.fold`(キャプチャした値)は、既定で出た。`Future.then`、`Iterable.forEach` / `any` / `where` の要素は、既存のセルが見ている。

## 原因の推定

`internal/model/builtin/dart-core.yaml` の `dart:core#List.sort` と `dart:core#Iterable.reduce` は、`flows` だけで、`calls` の行が無い。ほかの同じ形(`Iterable.any`、`every`、`skipWhile`、`Map.forEach` など)は、`calls: [{fn: arg0, with: [recv]}]` で、要素を関数の引数に渡している。`extractor/dart/lib/exprs.dart` の `_callbackMode` の表(決め打ち)にも、この 2 つは無い。

モデルで書ける形。決め打ちを増やす必要は無い。

## 対処の案

`List.sort` に `calls: [{fn: arg0, with: [recv, recv]}]`、`Iterable.reduce` に同じ `with: [recv, recv]` と `ret: result0` を足す。`List.sort` は、引数なしの呼び出し(関数が無い)でも、読み込みが壊れないことを確かめる。足したら、Dart の `clos/to_sort` のセルを、Go と同じ形で置く(`@miss` は付けない)。`calls` の行の `params` が、関数の引数の数と合うかは、`CallWarnings` の表の対象ではない(Go の標準ライブラリだけ)ので、セルで見る。

## 見直す条件

次のどちらかのとき。

- Dart の extractor が、コールバックを呼ぶ API の表を、モデルに移したとき。
- `List.sort` の比較関数が、実コードの検出に必要になったとき。
