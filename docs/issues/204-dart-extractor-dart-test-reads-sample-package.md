# #204 道具・ベンチ: extractor/dart で `dart test` を実行すると、`test/sample/test/sample_test.dart` の読み込みで失敗する

- 状態: **closed**
- 種類: 道具・ベンチ
- 言語: Dart
- ROADMAP の節: 0
- 守るもの: -
- 題: `dart test`(extractor/dart)が、別のパッケージ(`test/sample`)のテストを拾って、`package:sample` を解決できず失敗する

## 原因(1 行)

`extractor/dart` で `dart test` を実行すると、`test/` の下にある別のパッケージ(`test/sample` は自分の `pubspec.yaml` を持つ。rivus が読む対象の見本)の `test/sample/test/sample_test.dart` まで拾われ、extractor のパッケージの設定では `package:sample/lib.dart` を解決できず、読み込みで失敗する(main でも同じ。この見本のテストは、実行するためでなく、`--include-tests` の読み方を確かめるためにある)。

## 修正の中身

`extractor/dart/dart_test.yaml` の `paths` で、実行するテストを `test/smoke_test.dart` だけにした(`dart_test.yaml` には、パスを除く設定がなく、glob も使えない。一覧にするしかない)。extractor 自身のテストはそれまで 1 本もなかった(Go のテストが走らせていた)ので、`test/smoke_test.dart`(見本を読んで、JSON Lines が出ること)を足した。見本は動かさない(参照が `cmd/rivus`、CI、`scripts/dev-setup.sh`、ADR に多い)。

CI は `dart analyze lib bin` と Go のテストだけで、extractor の `dart test` を実行していない。回すかは、PR の本文で提案した。
