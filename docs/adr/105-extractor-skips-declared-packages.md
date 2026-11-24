# ADR-105: 宣言のあるパッケージの本体を、extractor が読まない(Go と Dart。ADR-100 の続き。2026-11-23)

- 状態: 有効
- 関連: ADR-100、DESIGN-package-models(2 節、7a 節)

**背景:** ADR-100 は、宣言のあるパッケージの関数を、IR を読んだあと、解析の前に落とした。extractor は全部読むので、読み込みの時間とメモリは減らなかった(Dart は、extractor が時間の 9 割)。設計は `docs/DESIGN-package-models.md` の 2 節と 7a 節。

**決めたこと:**
1. **宣言を extractor に渡す**(`-read-declared-bodies` のときは渡さない)。Go: `packages.Load` のあと、`go/ssa` の前に、宣言のある依存の `Syntax` と `TypesInfo` を捨てる(型検査は残る)。Dart: `--body-replaced package:<名前>`(繰り返し可)。そのパッケージのファイルは、解決も下ろしもしない。Rust は未実装(隔離と nightly がなく、測れない。CI で確認する)。
2. **結果を変えない。** finding の ID の集合、`stats.functions`、`summary` の uncertainty と数は、IR のフィルタ(`-drop-after-read`)と同じ。Dart は、ファイルを解決しないので、クラスの階層、動的呼び出しの候補、本体のある演算子、コンストラクタを、ライブラリの要素から作る(`_Index.library`)。これがないと、候補がなくなり、uncertainty が変わった。要素から分からないこと(引数のないコンストラクタの本体と初期化子に仕事があるか)は、ないものとして扱う。
3. **`summary.body_replaced_packages` の項目を分ける**(同じ名前で違う意味にしない): `functions`(#179 から。読んで落とした数)は、そのままの意味で、extractor が読まなかったパッケージには出さない(読んでいないので、落とした数はない)。extractor が読まなかったパッケージには、`functions_declared`(パッケージが宣言している数。IR の `body_not_read`。到達の数ではない)を出す。`-drop-after-read` と Rust は `functions`。
4. **落とした後に `PruneDeps`**: 落とした関数だけが届いていたもの(ジェネリクスのインスタンス、ラッパー、そこだけが import するパッケージ)を、フィルタの側でも落とす。段階 2 と 4 の `stats.functions` を同じにするため。プロジェクトが届くものは落ちない。 **事実: 宣言があるときの Go の `stats.functions` が、#179(ADR-100)のときと変わる**(宣言がないときは変わらない)。AWS SDK v2 の `service/s3` の合成で、`aws-sdk-go-v2/...` と `smithy-go/...` を宣言したとき、46 が 5 になる(宣言のあるパッケージの本体が作った、標準ライブラリのジェネリクスのインスタンスなど、41 関数が落ちる)。finding の ID は同じ。
5. **読まないパッケージだけが使う依存を、extractor が読まない版(Dart の `visit` が `import` をたどらない)は、入れなかった。** finding の ID は同じだが、`stats.functions` と uncertainty が変わり(宣言のあるパッケージのクラスが継承する、別のパッケージのメソッドの本体を読まなくなる)、時間の減少も誤差の中だった。結果を変えない約束のほうを選んだ。
6. 開発用のフラグ `-drop-after-read`(読んでから落とす、段階 2 の実行。比較用)。

**測定**(7a 節の表): `dart-gcp`(6 パッケージ)で、読み込み 14.3 → 10.7 秒、最大 RSS 1.63 → 1.23 GB。`dart-grpc` はほぼ変わらない。Go の S3 の合成で、読み込み 2.0 → 0.76 秒、RSS 1.25 → 0.57 GB。finding の ID は、いずれも同じ。

**しないこと:** Rust。到達性で読まない依存を落とすこと(5)。組み込みモデルへの宣言。

