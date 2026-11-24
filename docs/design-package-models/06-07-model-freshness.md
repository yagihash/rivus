### 6.7 モデルの鮮度: 確認したバージョン、定期的な照合、利用者の実行のときのずれ(設計案)

問題: 外部のライブラリが更新されると、モデルが古くなる。「本体を読まない」宣言を付けたパッケージは、古いモデルが、**サイレントな見逃し**になる(新しい API はモデルになく `unmodeled_library_call` になるが、変わった動作や、増えたコールバックは、uncertainty なしに落ちる)。

**今ある仕組みで検知できる範囲(事実)**:
- 検知できる: 組み込みの行が、zoo の呼び出しに一致すること(`TestBuiltinRowsAreHitByTheZoo`、`cmd/rivus/dead_rows_test.go`。行のシンボルが、zoo の IR に出ること)。ただし、zoo は、実物でなく骨組みを使うセルがある(`bench/zoo-dart/cells/packages/http` は、「本物のパッケージはない。`Client` API の形」と自分で書いている。Go の `bench/zoo-go/dep` も自前のコード)。**骨組みは、実物の変更を検知できない**(骨組みに行が一致することを見ているだけ)。
- Renovate(`renovate.json`): `bench/**` は `ignorePaths` で、**zoo の依存は更新の対象外**。更新の対象は、rivus 自身の依存(`go.mod`、`extractor/*` の Cargo と pub、GitHub Actions)で、モデルのパッケージの更新を知らせる仕組みではない。
- 検知できない: 新しい API、動作の変更、利用者のプロジェクトのバージョンとのずれ。

#### 1. モデルに、確認したバージョンを書く(最初のモデルの前に、形式を決める)

**書式(推奨)**: モデルのファイルの YAML のトップレベルに、`verified_versions` を置く(`internal/model/model.go` の `RuleSet` のフィールド。`ruleset:` と同じ階層)。**宣言(パッケージ単位の「本体を読まない」。`body_replaced_packages` が実装中)とは別の場所**に置き、宣言は、その同じパッケージが `verified_versions` にあることを検証で要求する。

```yaml
ruleset: rivus/dart/http@1
verified_versions:
  - package: http            # Go: モジュールのパス / Dart: pub のパッケージ名 / Rust: クレート名
    versions: ">=1.2.0 <2.0.0"   # 確認した範囲(Go と Rust は semver の範囲。Dart は pub のバージョンの範囲)
    checked_with: 1.2.2      # 実際に確認したバージョン(1 つ)
    checked: 2026-11-23      # 確認した日
```

宣言と同じ場所に置く案との比較:

| | 別の場所(推奨) | 宣言と同じ場所 |
| --- | --- | --- |
| 宣言のないモデル(手で書いた行だけのファイル、`-no-deps` でも使うモデル)にも書ける | はい(利用者のずれの uncertainty(3)に要る) | いいえ(宣言のあるパッケージだけ) |
| 宣言との整合 | 検証で結ぶ(宣言したパッケージは、`verified_versions` に必須) | 1 つの項目なので、ずれない |
| 生成物の先頭の「入力のバージョン」 | 同じ情報(生成器が書く `input:` から `verified_versions` の `checked_with` を作れる) | 同じ |
| 1 つのファイルに複数のパッケージ(`dart-gcp.yaml`) | 項目を並べる | 宣言と同じ数 |

**検証**(`Rules.Validate` と、`internal/model/builtin` のテスト):
1. 書式: 未知のキーは失敗、必須の 3 つ(`package`、`versions`、`checked`)、`checked` は日付(未来でない)、`checked_with` は `versions` の範囲に入る。
2. バージョンの形: Go はモジュールの semver(`v1.2.3`。`+incompatible` と、疑似バージョンの扱いを決める)、Dart は pub のバージョンの範囲(`^1.2.0` を含む)、Rust は Cargo の semver の範囲。言語は、ファイルの行のシンボルの形(Go の `pkg.Func`、Dart の `package:...#`、Rust の `crate::`)で決める(1 つのファイルは 1 言語)。
3. 存在しないパッケージ: オフラインでは、`package` を持つ行のシンボルが、少なくとも 1 つあること(行のないパッケージを書いていない)。オンラインでは(生成器、定期の照合。ネットワークが要る)、レジストリ(proxy.golang.org、pub.dev、crates.io)にそのパッケージと `checked_with` のバージョンがあること。
4. 第三者のパッケージの行を持つ組み込みのファイルは、`verified_versions` が必須(標準ライブラリは不要)。入れる順: 新しいファイルから必須にし、既存のファイルは、順に足す(足すまでは警告)。

実装は、段階 0 と 2 の PR に続ける(この文書は、形式だけ決める)。

#### 2. 定期的な照合(案。実装は生成器のあと)

最新のバージョン(または、`versions` の範囲の外の新しいバージョン)を取得し、型からモデルを生成し直して、今のモデルと比べる。差分の種類: **増えた API、消えた API、シグネチャが変わった API**。差分テストも実行し直す。差があれば、更新の PR を立てる(題の形: `feat(model): <言語> <パッケージ> のモデルを更新する(<旧バージョン> → <新バージョン>)`。手順書 7 節)。**クラウドの定期実行に載せる想定**(1 回の実行 = 1 パッケージ = 1 つの PR。`verified_versions` の `checked` が古い順に取る)。
- **骨組みのセルも対象にする**: zoo の骨組み(`bench/zoo-dart/cells/packages/http` など)のシグネチャを、実物の公開 API(新しいバージョン)と比べる(生成器の出力と、骨組みの公開宣言の比較。骨組みにあって実物にない API、実物にあって骨組みにない API)。これが、zoo が実物の変更を検知できない穴を塞ぐ。
- 取得したコードは実行しない(Rust は隔離が入るまで対象外。6.5)。

#### 3. 利用者の実行のときのずれ(案)

利用者のプロジェクトが使っているバージョンが、モデルの確認した範囲(`versions`)の外のとき、出力に uncertainty を出す。**既定は、uncertainty を出してモデルを使う**(止めない)。uncertainty の場所: (a) `summary` に、範囲の外のパッケージの一覧(パッケージ、使っているバージョン、確認した範囲)、(b) そのパッケージのモデルが関わった finding の uncertainty(`model_version_outside_verified`。種類は新設)。**フラグで、「範囲の外のパッケージは、宣言を無効にして本体を読む」を選べる**(宣言なしのパッケージは、モデルが使われたまま uncertainty だけ)。

**extractor がバージョンを出せるか(事実)**:

| 言語 | 今、IR にバージョンを出しているか | バージョンの取り方 |
| --- | --- | --- |
| Go | 出していない(IR のヘッダは `schema_version` だけ。`internal/ir/stream.go`、`ir.go`) | `go/packages` は `NeedModule` を既に使っていて(`internal/frontend/gossa/load.go` 164 行)、`pkg.Module.Version` が取れる(`collectInputs` が `pkg.Module` を既に読んでいる)。`go.mod` / `go.sum` のバージョンと同じ。取れる |
| Dart | 出していない | `package_config.json` の `rootUri` はバージョンを直接持たない。取り方は 2 つ: (a) `pubspec.lock` の `version:`(核の側でも、extractor でも読める。hosted のパッケージ。path / git の依存はバージョンがない)、(b) pub のキャッシュのパス `hosted/pub.dev/<名前>-<バージョン>`(extractor の `_packageDir` が、そのディレクトリを持っている。ただし、git の依存は `<名前>-<ハッシュ>`)。(a) を推奨 |
| Rust | 出していない | `Cargo.lock`(`version`)。またはドライバの中で、cargo が `rustc` の呼び出しに付ける環境変数 `CARGO_PKG_NAME` / `CARGO_PKG_VERSION`(cargo の仕様から。この環境では未確認)。`extractor/rust/src/main.rs` は、`cargo metadata --no-deps` を既に使っている |

**最小の拡張案**: コア側で、利用者の対象の `go.mod` / `go.sum`(Go は `go/packages` の `Module.Version`)、`pubspec.lock`、`Cargo.lock` を読む(**IR は変えない**)。`-ir` のファイルだけを渡された場合は、バージョンが分からないので、uncertainty は出さない(「バージョンは不明」と `summary` に 1 行)。次の段階で、IR のヘッダに `packages: [{name, version}]` を足す(extractor が出す)と、`-ir` でも uncertainty が出せる。index の鍵に、バージョンの一覧を入れる(バージョンが変わったら、別の index)。

#### 4. 生成器と差分テストの「対象外として明示する範囲」

別の実装の報告(rivus の側では未確認)から: **ジェネリクスの関数は、差分テストのコードを機械的に作れず、未検証で残った。コールバックを取る関数は、手で書く行になった。** どちらも、生成器と差分テストの、対象外として明示する範囲に入れる(手順書: 「手で書く」「未検証」と、PR の本文に書く)。Dart のジェネリクスのメソッド(`Dio.get<T>` など)も、同じく未検証で残る(型の引数に具体的な型を入れてプローブを作る案が成り立つかは、[課題 #157](../issues/157-dart-generic-method-model-diff.md))。**生成したモデルと生成器の一式は、取り込まない**(rivus の側で、型の情報から自分で作る)。

