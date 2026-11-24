### 6.10 確認していないバージョンへの適用の uncertainty(`summary.unverified_versions`。Go、Dart、Rust。6.7 の 3 の実装)

解析した対象が使っている module のバージョンが、`verified_versions` の範囲の外のとき、**モデルの行は当てたまま**、`summary.unverified_versions` に 1 件ずつ出す(`[{package, used, verified: [範囲...], source: "go.mod"}]`。`package` は、使っている module の path(`/v*` の記録のときは、対象が持つ major の path)。`body_replaced_packages` と同じく、package でソートされ、決定論)。finding が 0 件でも出る。

- **出す条件**: (1) `verified_versions` の記録があり、その `package`(`/v*` を含む)が、対象が使っている module の path に当たる。(2) 使っているバージョン(完全なバージョン。`go list` の `Module.Version`)が、その module に当たるどの記録の `versions` にも入らない。記録が複数あれば、どれかに入れば出さない。
- **記録のない module には出さない**(組み込みのファイルの大半は、記録がない。全部の解析に uncertainty が付くとうるさく、「確認していない」と言うにも、何を基準にするかがない)。記録を持つモデルが当たっていないとき(対象がその module を使っていない)は、出力は今までと同じ。記録を足すのは、手順書の 2 節の 3 ステップ(新しいファイルは必須)で進む。
- **バージョンが分からない module は出さない**: ディレクトリへの `replace`、別の module への `replace`(`go.mod` のバージョンは、実際に読むコードのバージョンではない)、主 module。バージョンの接尾辞(`-pre`、`+incompatible`)は読まない(疑似バージョンは、前のバージョンの番号とみなされる)。
- **実装**: `gossa.Program.ModuleVersions`(読み込みのとき、`packages.Package.Module` から)、`model.Rules.UnverifiedVersions`、`report.Summary.UnverifiedVersions`。`-ir` で渡された IR と、`-index` の IR からの解析は、モジュール情報がないので出ない。
**Dart(`pubspec.lock`)**: Dart の extractor は、IR にバージョンを出さないので、コアが、対象のディレクトリ(`-C`)の `pubspec.lock` を読む(IR は変えない。6.7 の 3 の最小の拡張案)。Dart の extractor が動いた解析だけが読む(`-ir` と `-index` からの解析は、ディレクトリを読まないので出ない)。`source` は `pubspec.lock`。
- 読むのは `packages:` のうち `source: hosted` の `version`。`path` と `git` の依存は、比べるバージョンがないので出さない。`package` は、pub のパッケージ名そのまま(`dio`、`shelf_router`)。
- `pubspec.lock` がない(`dart pub get` をしていない)ときは、uncertainty も、エラーもない。読めない(YAML が壊れている)ときは、標準エラーに 1 行出して、uncertainty は出さない。
- **未対応**: 複数のパッケージの monorepo や pub のワークスペース(`pubspec.lock` が、`-C` のディレクトリでなく、親のディレクトリにある)。`-C` のディレクトリに `pubspec.lock` がなければ出さない(Dart の extractor も、1 つのパッケージを `-C` に指す前提。解析の警告にもそう書いてある)。
- プレリリース(`6.0.0-beta.1`)は、Go と同じく、接尾辞を読まずに数字だけで比べる(`6.0.0` として、`<6.0.0` の範囲の外)。
- **記録は、言語ごとに照合する**。記録は、どの言語のファイルにあるかを持つ(`model.RuleSetLanguage`。組み込みのファイル名 `dart-*.yaml`、`rust-*.yaml`、`go-*.yaml`、またはルールセット名 `rivus/<言語>/...`)。Go の `go.mod` は Go の記録と、`pubspec.lock` は Dart の記録と、`Cargo.lock` は Rust の記録とだけ照合する。名前が同じで言語が違う記録(Dart の `http` と Rust の `http`)は、別々に扱う。言語が分からないファイル(プロジェクトの `.rivus/rules` や `-rules` で渡したファイル。名前に言語を示すものがない)の記録は、今までどおり、どの言語の依存にも当たる。`rivus models versions` の JSON は変えない。

**Rust(`Cargo.lock`)**: Rust の extractor が動いた解析だけが、`-C` の `Cargo.lock` を読む(IR は変えない。extractor も変えない)。`source` は `Cargo.lock`。
- 読むのは `[[package]]` のうち、`source` が registry(`registry+`、`sparse+`)のもの。path の crate(`source` なし。対象自身を含む)と git の crate は、比べるバージョンがないので出さない。`package` は、lock の綴り(`actix-web`)で出す。照合は `-` と `_` を同じとみなす(`model.SameCrate`。モデルが `google_cloud_storage` と書いても、lock の `google-cloud-storage` に当たる)。
- 1 つの crate が lock に複数のバージョンで入ることがある(`[[package]]` が 2 つ)。全部を見て、**どれかが範囲の外なら** 1 件にする(crate ごとに 1 件)。`used` は、lock の順で最初の外のバージョン、`all_used` が全部(順は lock のまま)。
- `Cargo.lock` がなければ、uncertainty もエラーもない(`--locked` なしで取得した対象)。ワークスペースの子の crate を `-C` に指したとき、`Cargo.lock` がワークスペースの根にあれば読まない(出ない)。未対応。
- プレリリース(`1.0.0-rc.1`)は、Go、Dart と同じ扱い(接尾辞を読まない)。
- 複数のバージョンを持つ lock(Cargo.lock で起きる)のために、`model.UnverifiedVersion` に `all_used` を足した。バージョンが 1 つのときは出ない(Go の JSON は変わらない)。

