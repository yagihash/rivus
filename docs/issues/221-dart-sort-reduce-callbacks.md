# #221 Dart: `List.sort` の比較関数と `Iterable.reduce` の関数に、リストの要素が渡らない(`calls` の行が無い)

- 状態: **closed**(`dart-core.yaml` の `List.sort`、`Iterable.reduce`、`Iterable.fold` の行に `calls` を足して解消。セル `clos/to-sort` が守る)
- 優先度: 中
- 種類: サイレントな見逃し
- 言語: Dart
- ROADMAP の節: -
- 守るもの: zoo-dart `clos/to-sort`(`vuln`、`vuln2`、`vuln3`)。実行との照合は `bench/zoo-dart/callbacks/observe.dart`
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

## 直した内容

コミット 540ba342 と、その後の修正(fold のセルを `to_sort` に移した)。`dart-core.yaml` の次の 3 つの行に `calls` を足した。

- `List.sort`: `calls: [{fn: arg0, with: [recv, recv]}]`(比較関数に、リストの要素が 2 つ渡る)。
- `Iterable.reduce`: `calls: [{fn: arg0, with: [recv, recv], ret: result0}]`(戻り値が、次の累積になる)。
- `Iterable.fold`: `calls: [{fn: arg1, with: [arg0, recv], ret: result0}]`(初期値と要素が渡る。キャプチャした値は元から出ていたが、要素は渡っていなかった)。

セルは、`bench/zoo-dart/cells/lib/clos/to_sort.dart`(`sort` が `vuln`、`reduce` が `vuln2`、`fold` の要素が `vuln3`、`fixed`、`decoy`。Go の `clos/to-sort`、Rust の同名のセルと同じ名前)。fold も同じファイルに置いた(既存のセル `to_lib_call` に行を足すと、ゲートが緩めた印と見るため)。行を戻すと、`vuln`、`vuln2`、`vuln3` が検出されなくなることを確かめた。実行との照合は `callbacks/observe.dart` の 3 つの呼び出し(`TestZooCallbacksDart`)。`marks.d` のロックも足した。

`Iterable.singleWhere` と `Iterable.expand` は、同じ形に見えるが、行が無くても、セルの形(要素を受ける関数の中の sink)は、すでに検出される(確かめた)ので、行は足していない。
