# ADR-062: ビルドが本番から除くものは、既定で読まない(3 言語で揃える)。読まなかった数は出力に出す

- 状態: 一部覆った(→ADR-125)
- 状態の詳細: 「足すだけで `schema_version` は変えない」は、ADR-125 が覆した。ほかは有効
- 関連: README
- 要旨: 読まなかった数を `tests_skipped` に出す

**背景:** Go(`_test.go`)と Rust(`#[cfg(test)]`、`tests/`、`examples/`、`benches/`。ADR-042 の追記)は、既定でテストを読まなかった。Dart は、`test/` の全ファイルを読み、テストの `main` を入口に数えていた(`rivus analyze` の dart-pub で、IR に `test/` の関数が入り、`unmodeled_library_call` の上位が `test_descriptor`、`test_core`、`matcher` になっていた)。テストのコードは、本番に出荷されないので、finding の数と入口の数を変え、解析の時間を使う。

**決めたこと:**
1. **原則: ビルドが本番から除くものは、既定で読まない。** Go は `-tests` で、Dart は `--include-tests`(`rivus analyze -tests` が渡す)で、Rust は `--include-tests` で、読む。
2. **名前やパスだけでは外さない(対象が偽れるので)。** ビルドの仕組みが決めているものを使う: Go は `packages.Load` の `Tests`、Rust は cargo の target の種類(`cargo check` の既定)。Dart は、pub のパッケージの配置(`pubspec.yaml` があるディレクトリの直下の `test/`、`integration_test/`、`benchmark/`、`tool/`、`example/`)で、`lib/src/test/` や `lib/foo_test.dart` は外さない(ライブラリのコード)。**外したディレクトリのファイルを、本番のコードが import していれば、依存として読む**(流れを失わない)。
3. **Dart の dev の依存:** 依存のライブラリは、**読んだファイル(本番のコード)から import をたどって届くものだけ**を読む(もともとそうなっている)ので、`test/` を読まなければ、dev の依存と、その推移は、本番のコードが import していない限り、自動的に外れる。`pubspec.lock` の `transitive` / `direct dev` のマークでは分けない(本番も引く推移的な依存は、残る)。`pubspec.yaml` の `dev_dependencies`(`dependencies` にない名前)は、**数と説明**にだけ使う。**本番のコードが dev の依存を import しているとき**は、import の事実を優先して読み、uncertainty を 1 件出す。
4. **Rust の build-dependencies:** build script は、extractor が読まない(`is_build_script`)ので、build 時にだけ実行される crate は、本番の関数にならない。変更なし。
5. **読まなかった数を出す(必ず報告する):** IR に `tests_skipped` のレコード(足すだけ。`schema_version` は変えない)、出力の `stats.tests_skipped`(`files`、`targets`、`dev_dependencies`、`dev_dependency_names`、`notes`)、stderr の 1 行(「N test files, M dependencies declared for development were not read (--tests reads them: ...)」)。3 言語で同じ形。Go は、読み込んだパッケージのディレクトリの `_test.go` の数、Rust は除いた target(test、example、bench)の数と、`[dev-dependencies]` にだけある crate の数、Dart はファイル数と、読まなかった dev の依存の数。`-tests` のときは出ない。

**限界:** Dart の `example/` にある本物のアプリは既定で読まれない(`-C example` で、ルートにする)。Go のテスト専用の依存(`testify` など)は、`Tests` を外すと読み込まれないが、数には出ない(`dev_dependencies` は 0)。

**確かめたこと(Dart は、実際に既定でテストを読んでいた):** dart-pub(`--no-deps`)の IR で、関数 4847 のうち **2607 が `test/` のもの**、entry 579 のうち **428 がテストの `main`**。`--no-deps` なので、dev の依存の本体は、もともと入っていない(`unmodeled_library_call` の上位が `test_descriptor` などだったのは、`test/` のファイルが呼んでいたため)。

**検証(`bench/compare.sh 2b2dbe7 4fa0ce1`、dart-pub、dart-curated):**

| | 読む(`--include-tests`、前と同じ) | 既定(読まない) |
| --- | --- | --- |
| dart-pub 関数(`functions`) | 4847 | 2196 |
| entry | 579 | 146 |
| findings(すべて `from_entry`) | 19 | 17 |
| `reanalyses` | 6379 | 2971 |
| 解析の時間(`duration_ms`)/ ピーク RSS | 2789 ms / 510 MB | 2160 ms / 231 MB |
| 不確実性(`unmodeled_library_call`、`dynamic_dispatch`) | 18121(9020、8380) | 4919(909、3596) |
| `tests_skipped` | なし | `files: 392` |
| dart-curated | 111 findings、同じ | 同じ(`test/` なし) |

dart-pub で消えた 2 件は、**どちらも source と sink が `test/` のファイル**(`test/get/git/check_out_test.dart:153`→`:156` の `File.new`、`test/hosted/will_normalize_hosted_url_test.dart` の `main$1$4$1`)。本番のファイルの finding は消えていない。ラベル付きの 9 件は 9/9 のまま。テストは、関数の 54%、解析の時間の約 22%、メモリの約 55% を占めていた。

