## 7a. 段階 4(extractor に宣言を渡す)の状態と測定(ADR-105。2026-11-23)

**実装済み: Go、Dart、Rust**(Rust は 2026-11-23。下の表と、「Rust の測定」)。`-read-declared-bodies` のときは、extractor に宣言を渡さない(本体を読む)。`-drop-after-read`(開発用)は、宣言のあるパッケージを extractor に読ませてから、IR のフィルタ(段階 2)で落とす(比較用)。

| 言語 | 規模 | 内容 |
| --- | --- | --- |
| Go | `gossa/load.go` 約 40 行、`cmd/rivus/common.go` 約 25 行 | `packages.Load` のあと、`go/ssa` の前に、宣言のある依存(標準ライブラリとプロジェクトは除く)の `Syntax` と `TypesInfo` を捨てる(`Options.ReplacedPackage`)。型ごとに作られるラッパーと `init` など、本体のある関数が残るので、`ReplacedFunc`(`taint.UnitOf` が宣言のある単位)で IR に出さない |
| Dart | `extract.dart` 約 85 行、フラグと `body_not_read` | `--body-replaced package:<名前>`。そのパッケージのファイルは、`getResolvedUnit` も、下ろしもしない。**クラスの階層、動的呼び出しの候補、本体のある演算子、コンストラクタは、ライブラリの要素から作る**(`_Index.library`)。ファイルを解決しないと、これらがなくなり、呼び出しの注記(`unmodeled_library_call` と動的呼び出しの候補なし)が、フィルタの IR と変わった(最初の実装で、`dart-gcp` の 注記 196 → 105)。要素から分からないこと(引数のないコンストラクタの本体と初期化子に仕事があるか)は、ないものとして扱う |
| Rust | `extractor/rust/src/main.rs` 約 40 行、`cmd/rivus/lang.go` 約 20 行、`internal/model/bodyreplaced.go` 約 15 行 | `--body-replaced <クレート名>`(繰り返し可。`rivus analyze` が宣言を渡す。`rivus debug extract -lang rust -body-replaced` でも渡せる)。ドライバは、依存のクレート(ワークスペースの構成員でないもの)で、名前が宣言にあるものは、`extract` の最初で、本体を 1 つも下ろさず、`body_not_read`(`unit` と、本体のある項目の数)だけを書く。**`cargo check` は、そのクレートも今までどおりコンパイルする**(rustc は、依存を省かない)。省けるのは、MIR を IR に下ろすことと、IR の書き出し、読み込み、解析。**宣言を渡す経路は環境変数 `RIVUS_BODY_REPLACED`**(cargo の実行を通してドライバに届く)。トレイト経由の呼び出しの候補と、他のクレートの値の `drop` は、`merge` が全クレートの impl に結びつける(`link_across_crates`)ので、**読まないクレートの関数の名前だけを `body_not_read` の `names` で `merge` に渡す**(`merge` が結びつけに使い、書き出す前に消す)。これを渡さないと、フィルタ(`-drop-after-read`)のときと、`drop` の呼び出しの注記が変わった(最初の実装で、`unmodeled_library_call` の注記が 1 つ少なかった)。同じクレートが 2 回ビルドされる(feature や対象が違う)ことがあるので、`body_not_read` は、`merge` が、クレートごとに 1 つにまとめる(関数の名前の数) |
| 共通 | IR に `body_not_read`(`unit`、`functions`)、`dropReplacedBodies` のあとの `PruneDeps` | 読まなかった関数の数を、`summary.body_replaced_packages[].functions_declared` に出す(`functions` は、読んで落とした数のままで、extractor が読まなかったパッケージには出ない)。落とした関数だけが届いていたもの(ジェネリクスのインスタンス、ラッパー、そこだけが import するパッケージ)を、フィルタの側でも落とす(これで、`stats.functions` が、extractor の段階と同じになる) |

**finding の同一性**(`-drop-after-read` との比較。`TestBodyReplacedInTheExtractorIsTheSameAsTheFilter`、`TestBodyReplacedGenericsInTheExtractor`、`TestBodyReplacedOfADartPackage`、`TestDartExtractorBodyReplaced`): 合成の Go(ジェネリクスと型のラッパーを含む)と Dart、公開のベンチ(下)で、**finding の ID の集合、`stats.functions`、`summary` の注記と数(`dispatch_candidates`、`program`、`top_symbols` など)が同じ。違うのは、`body_replaced_packages` の項目だけ**: フィルタは `functions`(読んで落とした関数の数)、extractor の段階は `functions_declared`(読まなかったパッケージが宣言している関数の数。`functions` は出さない)(数の意味を、項目の名前で分けた。Go: 宣言のある関数とメソッド。Dart: 最上位の関数、メソッド、コンストラクタ、アクセサ。到達の数ではない)。

**測定**(公開のベンチ。`dart-gcp` は 6 パッケージ、`dart-grpc` は `package:grpc`、Go は AWS SDK v2 `service/s3` の合成で `aws-sdk-go-v2/...` と `smithy-go/...`。`-body-replaced` で宣言。1 つずつ実行、1 回 540 秒で打ち切り。`uptime` の load average(1 分)は、Dart が 2.7〜3.8、Go の `なし` が 3.0、段階 2 が 11.7、段階 4 が 16.4(他の作業と同時なので、Go の段階 2 と 4 の差は、負荷が高かった側が不利)。単位: 秒、GB):

| 対象 | 実行 | 読み込み(extractor + IR) | 解析 | wall | 最大 RSS | 関数 | finding |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `dart-gcp` | 宣言なし | 14.6 | 0.33 | 15.4 | 1.59 | 4,593 | 19 |
| 同 | 段階 2(フィルタ) | 14.3 | 0.18 | 15.0 | 1.63 | 2,766 | 同じ 19 |
| 同 | **段階 4** | **10.7** | 0.15 | **10.9** | **1.23** | 2,766 | 同じ 19 |
| `dart-grpc` | 宣言なし | 9.5 | 0.11 | 9.6 | 1.15 | 2,409 | 3 |
| 同 | 段階 2 | 10.4 | 0.05 | 10.4 | 1.13 | 1,143 | 同じ 3 |
| 同 | **段階 4** | 10.1 | 0.04 | 10.1 | 1.12 | 1,143 | 同じ 3 |
| Go の S3 合成 | 宣言なし | 1.9 | 打ち切り(543) | 547 | 12.99 | 6,427 | 1 |
| 同 | 段階 2 | 2.0 | 0.00 | 2.5 | 1.25 | 5 | 同じ 1 |
| 同 | **段階 4** | **0.76** | 0.00 | **0.78** | **0.57** | 5 | 同じ 1 |

- **Dart の効果は、設計の試作(`dart-gcp` 50.6 → 22〜25 秒)より小さい**: 段階 4 で 14.3 → 10.7 秒(-25%)、RSS -25%。`dart-grpc` は、ほぼ変わらない(`grpc` が小さく、ほかのパッケージの要素の構築が支配的)。試作との差の大きな部分は、(i) 試作は `visit` が宣言のあるパッケージの `import` をたどらなかった(推移的な依存を読まない。下の PruneDeps の項)、(ii) 測定時の負荷(試作の 50.6 秒は load average が 12〜15 のとき。この測定の「宣言なし」は 14.6 秒)。ライブラリの要素モデルを作る(パース、`import` の解決)時間は、省けない。
- **Go の効果は、読み込みの 3 秒がほぼ半分以下**(段階 2 → 4 で 2.0 → 0.76 秒、RSS 1.25 → 0.57 GB。段階 2 の側の負荷が高かったので、差は大きめに出ている)。型検査(`NeedDeps`)は残り、SSA の本体と IR の下ろしが省かれる。
- **PruneDeps(到達性で、読まないパッケージだけが使う依存を落とす)は、Dart の extractor には入れなかった。** `visit` が、宣言のあるパッケージの `import` をたどらない版(`export` はたどる)を試した: finding の ID の集合は同じ(`dart-gcp` 19、`dart-grpc` 3)だが、`stats.functions` が変わる(2,766 → 2,671、1,143 → 1,034)。注記(`dispatch_candidates` の候補なし 9 → 15 など)も変わる。プロジェクトのコードが、宣言のあるパッケージのクラスを通して、そのクラスが継承する別パッケージのメソッドを呼ぶと、その本体を読まなくなるため(本体を読んだ実行と、結果が変わりうる)。時間の減少も、測定の誤差の中だった(12.0 秒、9.1 秒)。同一の約束を守るために入れず、判断を相談役に渡す。ただし、**落とした後の `PruneDeps` は入れた**(フィルタの側。読み込みは変わらない。落とした関数だけが届いていたものを落とすだけで、プロジェクトが届くものは落ちない。Go と Dart の `stats.functions` が、段階 2 と 4 で同じになるために要る)。

**Rust のクレート名の綴り**(ADR-105 の続き。2026-11-23): Cargo.lock とモデルの `package` は `actix-web`、rustc と IR の単位は `actix_web`(`-` と `_` は、Cargo では同じクレートの名前)。**どちらの綴りで宣言しても、同じクレートに当たる**: `Declaration.Covers` と `Rules.ReplacedBody` は、区切り(`/`、`.`、`:`)のない名前(Rust のクレートと、Go の標準ライブラリ。後者には `-` も `_` もない)に限って、`-` と `_` を同じとして比べる(`model.SameCrate`、ADR の検証済みバージョンの照合と同じ)。Go の import path(`example.com/a-b` と `example.com/a_b` は別のパッケージ)と Dart の `package:` は、今までどおり文字どおりに比べる。Rust の単位は、`UnitOf` が `::` の前の最初の区切りまでを返すので(`actix_web::web::Query::get` は `actix_web`)、`::` で区切られた単位の扱いは、変更が要らなかった。`/...`(配下)は、Rust には意味がないので、extractor には、`/...` を外して渡す。

**Rust の finding の同一性**(`TestBodyReplacedOfARustCrateIsTheSameAsTheFilter`): 宣言を extractor に渡した実行と、`-drop-after-read` の実行で、レポート全体(finding、不確かさ、`summary` の `body_replaced_packages` 以外、`stats.functions`)が同じ。固定の入力は、`extractor/rust/test/sample` と、その依存 `dep`(名前を `dep-crate` に変えたコピー。`-` を含む名前の経路を通すため)。綴りの違う 2 つの宣言(`dep-crate` と `dep_crate`)と、`-read-declared-bodies`(宣言なしの実行と同じ)も同じテストで確認する。**この確認は extractor が要る**: `RIVUS_RUST` がなければスキップし、`RIVUS_REQUIRE_RUST=1` で失敗する(CI の `rust` ジョブ。macOS では、サンドボックスの下で動き、他の OS では、リポジトリの中の入力なので、`-unsafe-host-exec` で動く)。**違うのは次の 2 つ**: (i) `body_replaced_packages` の項目(`functions` と `functions_declared`)。(ii) 読まなかったクレートの中の、「2 つのクレートに同じ名前の関数がある」の読み込みの注記(`load_errors` と `missing_source` の不確かさ)と、`fn_skipped`(`summary.functions_skipped`): 読まないクレートは、関数ごとの `catch_unwind` に届かないので、その数が減り、同じ名前の関数も、宣言のあるクレートの中の分だけ減る(下の測定で、5,097 → 4,998)。フィルタの実行は、読んで、落としているので、これらの注記が残る。finding と、`summary.program` などの数は、同じ。

**Rust の測定**(2026-11-23。`actix-web` 4 だけに依存する小さいプロジェクト(`/run?cmd=` の値を `Command` に渡す。リポジトリの外に作った。`cargo generate-lockfile` で 155 クレート)。macOS のサンドボックスの下の `rivus analyze -lang rust`。extractor はこの変更のもの。load average は 14〜20(他の作業と同時なので、時間は ±3 秒のぶれ)。宣言は、A: `actix-*` の 8 クレート(`-body-replaced`)。B: A と `tokio`、`tokio-util`、`h2`、`regex`、`regex-automata`、`regex-syntax`、`serde`、`serde_json`、`bytes`、`futures-util`、`mio` の計 19 クレート。単位: 秒、GB):

| 実行 | 読み込み(extractor + IR) | 解析 | 最大 RSS | IR の大きさ(`debug extract -all`) | IR の関数 | finding |
| --- | --- | --- | --- | --- | --- | --- |
| 宣言なし(3 回) | 22.8 / 23.8 / 25.7 | 0.003 以下 | 3.74 | 653 MB | 58,540 | 1 |
| A: フィルタ(段階 2) | 22.7 / 24.5 | 0.003 以下 | 3.74 | (同じ) | 58,540 | 同じ 1 |
| A: **段階 4** | **21.7 / 22.0** | 0.003 以下 | 3.74 | **596 MB(-9%)** | **53,657** | 同じ 1 |
| B: フィルタ | 21.6 / 23.4 / 27.5 | 0.003 以下 | 3.73 | (同じ) | 約 58,540 | 同じ 1 |
| B: **段階 4** | **18.3 / 21.8 / 23.0** | 0.003 以下 | 3.74 | **462 MB(-29%)** | **37,950** | 同じ 1 |

- **予想どおり、extractor の時間と最大メモリは減らない。** `cargo check` は、宣言のあるクレートも含めて、全部コンパイルする(最大 RSS 3.7 GB は、rustc が依存をコンパイルするときの値。宣言のあるなしで変わらない)。読み込みの時間の差は、ぶれの範囲(A では 1〜2 秒、B で 3〜5 秒短い側に出たが、負荷の違う実行の間の差と区別がつかない。両方の実行の組を、同じ負荷で繰り返していない)。**減るのは、IR の大きさ(宣言するクレートの数と大きさによる。A で 9%、B で 29%)と、IR の読み込みと、それ以降(解析の入力)**。この小さいプロジェクトは、依存の関数を数個しか呼ばないので(`stats.functions` は 11。到達性で落ちたあと)、解析の時間は 3 ミリ秒以下で、解析の側の効果は測れなかった(Go の S3 の合成のように、依存の関数が多く届くプロジェクトで出る)。
- **したがって、Rust の段階 4 は、時間とメモリを減らす手段ではなく、IR を小さくし(ひいては、解析に届く依存の本体を宣言のとおりにする)、宣言の意味を、extractor と解析で同じにするためのもの。** Rust の `cargo check` に宣言を渡して、コンパイルそのものを省く方法はない(依存のクレートは、型のために、コンパイルされる)。コスト(時間、メモリ)を理由に依存を切る(ADR-092)Rust の場合の効果は、ここでは確認できていない。
- 同じクレートが、2 回ビルドされる(例: `actix-router` で、関数の名前の数が 357 と 376。feature か対象が違う)ので、`functions_declared` は、クレートごとに、名前の重なりを除いた数(`actix_router` で 375)。到達の数でも、`functions`(読んで落とした数)と同じ数でもない。

