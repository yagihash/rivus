# 176: Dart: Flutter のメンバーを含む workspace で、`dart pub get` が通らない

- 状態: **open**
- 種類: 道具(重さ: 中)
- 言語: Dart
- ROADMAP の節: 3
- 守るもの: -

`workspace:` に Flutter のパッケージ(`sdk: flutter`)を含むリポジトリは、Dart SDK だけの `dart pub get` が通らない。測定(`bench/deps_bench.py`)では、コピーの側のルートの `pubspec.yaml` から、Flutter のメンバーを外して、通している(`dart_remove_flutter_workspace_members`)。利用者は同じことを手でしなければならず、案内がない。

直す方向: 案内(README)に、読みたいのがサーバ側のパッケージだけのときの手順を書く。または、rivus が `pub get` の失敗の理由(Flutter のメンバー)を、そう言って報告する。
