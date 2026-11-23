# 確認していないバージョンへの適用の注記(`summary.unverified_versions`)が、Dart と Rust に出ない

- 状態: **closed**(Dart と Rust を直した。`pubspec.lock` と `Cargo.lock` を、コア側で読む。守るもの: `cmd/rivus/lockversions_test.go`)
- 種類: 機能(モデルの鮮度。設計 6.10)
- 言語: Dart、Rust
- ROADMAP の節: -
- 守るもの: `cmd/rivus/lockversions_test.go`(`TestUnverifiedVersionsDartLock`、`TestUnverifiedVersionsCargoLock`)
- 題: 確認していないバージョンへの適用の注記(`summary.unverified_versions`)が、Dart と Rust に出ない(直した: `pubspec.lock`、`Cargo.lock` から取る)

優先度: 低め(Go は出る。Dart と Rust のモデルの `verified_versions` は、Go より少ない)。

## 何が見逃されるか(または、何が誤って報告されるか)

Go の解析は、対象が使っている module のバージョンが `verified_versions` の範囲の外のとき、`summary.unverified_versions` に出す(設計 6.10)。Dart と Rust は、同じことをしない。範囲の外のバージョンの `package`(Dart)や crate(Rust)に、モデルの行が、何も報告せずに当たる。

## 原因

Go は `go list` の `Module.Version` からバージョンが分かる。Dart と Rust の extractor は、IR にバージョンを出さない(設計 6.7 の 3 の表)。

## 直す方向の候補

- Dart: 対象の `pubspec.lock` の `version:`(hosted の依存。path と git の依存はバージョンがないので出さない)を、コア側で読む。
- Rust: `Cargo.lock` の `version`(registry の依存)を、コア側で読む。
- どちらも、IR は変えない(設計 6.7 の 3 の「最小の拡張案」)。`-ir` だけを渡されたときは、バージョンが分からないので出さない。`Rules.UnverifiedVersions` はそのまま使える(module のバージョンの表を渡す)。

## 再現するセル

なし(Go の合成のテスト `cmd/rivus/unverified_test.go` が、Go の形)。Dart と Rust は、`pubspec.lock` と `Cargo.lock` を持つ小さな合成のプロジェクトで確認する。
