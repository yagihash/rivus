# #103 Dart: package_config.json がない・古い対象で、何も報告せずに finding が 0 件になる

- 状態: **closed**
- 種類: サイレントな見逃し(直した)
- 言語: Dart
- ROADMAP の節: 1
- 守るもの: `cmd/rivus/dart_packageconfig_test.go`(`TestDartPackageWithoutPackageConfigIsSaid`)
- 題: Dart: package_config.json がない

## 今の状態(2026-11-22 の見直し)

直した: 46e28c0(`package_config.json` がない、または古いパッケージを、読めなかったものとして報告する。`dart pub get` を実行する、と。rivus は実行しない)。

## 経緯

2026-11-22 に、直してから記録した(「直ってから記録する」)。再現の詳細は書かない。

