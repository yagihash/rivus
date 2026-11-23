## 9. 再現

```sh
# Go の合成: ~/tmp/rivus-pkgmodels/s3demo (go mod tidy だけ。取得したコードは実行しない)
rivus analyze -v -pass-stats -pass-stats-top 0 -detail summary -timeout 540s
rivus analyze -no-deps -v ...
# Dart: bench/dart-gcp、bench/dart-grpc を、リポジトリの外にコピーして dart pub get --offline
RIVUS_DART=dart RIVUS_DART_EXTRACTOR="dart run <worktree>/extractor/dart/bin/rivus_dart.dart" \
  rivus analyze -lang dart -C <copy> -v -pass-stats -pass-stats-top 0 -detail summary -timeout 540s
```

試作(宣言を、環境変数で指定する、リポジトリの外のコピー)は、この文書の 3 節の表を作った。本実装は、7 節の段階 2 と 4。
