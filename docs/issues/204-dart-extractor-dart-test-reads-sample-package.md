# #204 道具・ベンチ: extractor/dart で `dart test` を実行すると、`test/sample/test/sample_test.dart` の読み込みで失敗する

- 状態: **open**
- 種類: 道具・ベンチ
- 言語: Dart
- ROADMAP の節: 0
- 守るもの: -
- 題: `dart test`(extractor/dart)が、別のパッケージ(`test/sample`)のテストを拾って、`package:sample` を解決できず失敗する

## 原因(1 行)

`extractor/dart` で `dart test` を実行すると、`test/` の下にある別のパッケージ(`test/sample` は自分の `pubspec.yaml` を持つ。rivus が読む対象の見本)の `test/sample/test/sample_test.dart` まで拾われ、extractor のパッケージの設定では `package:sample/lib.dart` を解決できず、読み込みで失敗する(main でも同じ。この見本のテストは、実行するためでなく、`--include-tests` の読み方を確かめるためにある)。

## 直し方の候補(直すのは今ではない)

`extractor/dart/dart_test.yaml` に、`test/sample`、`test/monorepo`、`test/versions` を除くか、実行するテストのパスを決める。CI は `dart analyze lib bin` と Go のテストだけで、extractor の `dart test` を実行していない。
