# 観測した呼び出しとの突き合わせ(試行)

設計は [../../../docs/DESIGN-observed-calls.md](../../../docs/DESIGN-observed-calls.md)。**CI では走らせない**。動物園の Dart を 1 回通し、実際に起きた呼び出しのうち、rivus のグラフにない辺を出す。

| ファイル | 役目 |
|---|---|
| `compare.py` | 記録した stack と、`rivus index` が作ったグラフ(SQLite)を比べ、辺ごとに `ok` / `no_edge` / `no_callsite` / `callee_unresolved` を出す |
| `instrument.dart` | 方法 B。cells の**コピー**の、全部の関数の先頭に `__rv.rvEnter()` を差す(構文だけ。型は見ない) |
| `vs_miss.py` | `@miss` の行ごとに、同じファイルに「グラフにない観測の辺」があるかを数える。`--others` で、`@miss` のないファイルの辺も出す |

記録の側は `bench/zoo-dart/exec/zoo_mark/lib/zoo_mark.dart`(`ZOO_STACKS=1` のときだけ stack を持つ。ふだんの出力は変わらない)と、`exec/test/zoo_test.dart`(`ZOO_STACKS_OUT` に書く)にある。

## 手順(リポジトリの根から。`$T` は作業用のディレクトリ)

```sh
# 0. 準備: rivus を作り、extractor と cells の pub get
go build -o $T/rivus ./cmd/rivus
(cd extractor/dart && dart pub get) && (cd bench/zoo-dart/cells && dart pub get)

# 1. グラフ(元の cells から)
(cd bench/zoo-dart/cells && RIVUS_DART_EXTRACTOR="dart run $PWD/../../../extractor/dart/bin/rivus_dart.dart" \
  $T/rivus index -C . -graph $T/zoo-dart.db ./...)

# 2. 方法 A: 今ある harness で、sink を呼んだ所の stack を取る
(cd bench/zoo-dart/exec && dart pub get && ZOO_STACKS=1 ZOO_STACKS_OUT=$T/stacks.json dart test test/zoo_test.dart)
python3 bench/zoo-dart/observed/compare.py $T/stacks.json $T/zoo-dart.db --json $T/zoo-A.json
python3 bench/zoo-dart/observed/vs_miss.py $T/zoo-A.json bench/zoo-dart/cells/lib package:zoo_cells/ --others

# 3. 方法 B: cells と exec をコピーして、コピーの関数の先頭に印を差し、全部の呼び出しの stack を取る
mkdir $T/zoo2 && cp -R bench/zoo-dart/cells bench/zoo-dart/exec $T/zoo2/
dart --packages=extractor/dart/.dart_tool/package_config.json bench/zoo-dart/observed/instrument.dart $T/zoo2/cells/lib
(cd $T/zoo2/exec && dart pub get && ZOO_STACKS=1 ZOO_STACKS_OUT=$T/stacks-enter.json dart test test/zoo_test.dart)
python3 bench/zoo-dart/observed/compare.py $T/stacks-enter.json $T/zoo-dart.db --line-only --json $T/zoo-B.json
python3 bench/zoo-dart/observed/vs_miss.py $T/zoo-B.json bench/zoo-dart/cells/lib package:zoo_cells/ --others
```

方法 B は差した分だけ同じ行の桁がずれるので、`--line-only`(同じ行の呼び出し点のどれかが、その呼び先への辺を持てば `ok`)で比べる。グラフは、差す前の元の cells から作る(行番号が同じ)。
