# 172: Dart: for-in で取った要素への書き込みが、元のコレクションから読むときに見つからない

- 状態: **open**
- 種類: サイレントな見逃し
- 言語: Dart
- ROADMAP の節: -
- 守るもの: zoo `F/cont/ptr-range-slice`(Dart)

## 何が起きるか

`for (final b in boxes) { b.v = 汚れた値; }` のあと、元のコレクションから同じ要素を読んで(`boxes[0].v`)sink に渡す流れが見つからない。Go では、結果が 2 つ以上の命令で得たポインタ経由の書き込みを追うので、同じ形が見つかる(zoo `F/cont/ptr-range-map` など)。

## 原因

Dart の抽出器は、`for-in` の反復の 1 歩(`iter.next`)の結果のローカルに型を付けない(`extractor/dart/lib/lower.dart` の `forEachLoop`)。エンジンは、型の種類がポインタかコンテナのものだけを別名として追うので、要素が参照として扱われず、書き込みは要素のコピーに入る。

## 案

- `iter.next` の値のローカルに、要素の型(反復する値の型引数)を付ける。
- そのうえで、エンジンの別名の扱い(`internal/taint/locs.go` の `computeLocs`)が、Dart でも同じに働くことを、zoo のセルで確かめる。
