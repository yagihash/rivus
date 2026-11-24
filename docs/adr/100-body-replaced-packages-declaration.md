# ADR-100: モデルのファイルに、パッケージ単位で「本体を読まない」宣言を書ける(解析の側。ADR-092、098 の続き。2026-11-23)

- 状態: 有効
- 関連: MODELING 4a、ARCHITECTURE 5.5、AGENT-GUIDE、DESIGN-package-models
- 状態の詳細: ADR-092、098 の続き。`body_replaced_packages`、`-body-replaced`、`-read-declared-bodies`。extractor への受け渡し、組み込みへの宣言は未
- 要旨: `summary.body_replaced_packages`、呼び返しの `assumed_call`

**背景:** ADR-092 と 098 は、モデルが依存の本体の代わりをしてよい条件を決めたが、パッケージ単位で本体を読まない方法がなかった(全部読むか、`-no-deps` で全部読まないか。モデルがあっても、本体は IR に入り、解析される)。設計は `docs/DESIGN-package-models.md`。この ADR は、設計の段階 2(解析の側だけ)の決定。extractor に宣言を渡す段階(読み込みの時間を減らす)は、まだしない。

**決めたこと:**
1. **書式**: モデルのファイル(組み込みにも利用者のルールにも)のトップレベルに `body_replaced_packages: [{package, reason}]`。`reason` は必須(何を確認したか)。`package` は `taint.UnitOf` の単位(Go: import パス、Dart: `package:<名前>`、Rust: crate 名)。**書いたパッケージだけに効く**(暗黙の接頭辞一致はしない)。配下もまとめて指すときは `パス/...`(Go の書き方)。検証: 空、空白、`...` が末尾以外、`file:`(プロジェクト自身)、重複、`reason` なし。キー名は、設計の案の `unread_packages` ではなく `body_replaced_packages` にした。`summary.unread`(ビルドできなかったパッケージ)と意味が衝突するため。
2. **フラグ**: `-body-replaced <パッケージ>`(繰り返し可。一時的に試す用。理由は「コマンドラインで指定。確認はしていない」)と、宣言を無効にして本体を読む `-read-declared-bodies`(照合用)。両方、普段使うフラグ。
3. **落とす場所**: `analyze` が IR を読んだあと、`sites.Enumerate` と `sg.BuildConsuming` の前に、宣言のあるパッケージの**依存の関数**(`Func.Dep`)を IR から落とす(`cmd/rivus/bodyreplaced.go`)。プロジェクトの関数は落とさない(プロジェクトのルールファイルが、プロジェクト自身のコードを読まない宣言をできないようにする)。落とした関数は、`-no-deps` のときと同じ、本体のない関数になる。`dependency_sites.dependency_functions` は、読んだ関数の数なので、落とした関数は数えない。Go の標準ライブラリは、もともと読まない(宣言の対象外)。`-no-deps` のときは、依存の関数がないので、宣言は何もしない。
4. **呼び出し**: 行があればモデル、なければ `unmodeled_library_call`(`attributable_to` は今までどおり rivus。uncertainty の文に、パッケージの本体を読んでいないことを書く)。
5. **呼び返し**(利用者のコードへの呼び返しが落ちる形は、最もサイレントになりやすい): 宣言のあるパッケージの関数に、モデルの `calls:` の行がないとき、(a) 渡された関数の値(`func` の値、クロージャ)は、呼ばれるものとして扱う。ほかのオペランドを渡して呼び、結果を呼び出しの結果にする(今ある `calls:` の行と同じ仕組みで、合成の行を作る。`assumed_call` の uncertainty)。呼ぶ関数が分からないときは、「入るものが出る」流れ(`DerivedFlows`)。(b) プロジェクトの型を、インターフェースにして(Go の `make_interface`)渡したときは、その型のメソッド(8 個まで)を、同じく呼ばれるものとして扱う(`assumed_call`)。8 個を超えるときは、`unresolved_callee` の uncertainty だけ。モデルに `calls:` の行があるときは、モデルを信じ、何も足さない。**限界**: インターフェースの値が、呼び出しの所以外で作られたとき(フィールド、引数)と、Dart と Rust のインターフェースの値(Go の `make_interface` に当たる形を見ていない)は、見ない。宣言の前の確認(MODELING)で、`calls:` の行を書く。
6. **出力**: `summary.body_replaced_packages`(パッケージ、`declared_in`、`reason`、`functions`。ソート済み。落とした関数がないものは出さない)、`summary.body_replaced_ignored`(`-read-declared-bodies` が無視した宣言の数)。宣言がないときは、どちらも出ない。
7. **index の鍵**: `-body-replaced` の一覧(ソート、重複なし)と `-read-declared-bodies` が、鍵に入る(宣言がないときの鍵は、今までと同じ)。ルールファイルの宣言は、ファイルの内容で、組み込みの宣言は、組み込みのハッシュで鍵に入る。記録されたフラグは、問い合わせで引き継ぐ(`-tests` と同じ)。
8. **決定論**: 宣言の一覧は、パッケージと宣言の場所でソートする。

**測定**(公開の対象。rivus の側で測った。`-body-replaced` で宣言。extractor は全部読むので、読み込みの時間は変わらない。`uptime` の load average は 6〜22 で、他の作業と同時):

| 対象 | 宣言 | 解析(`duration_ms`) | 全体の wall | 最大 RSS | 関数 | finding の ID |
| --- | --- | --- | --- | --- | --- | --- |
| Go の AWS SDK v2 `service/s3` の合成 | なし | 打ち切り(`timed_out`。`-timeout 540s`) | 555 秒 | 10.8 GB | 6,442 | 同じ 1 件(収束しない) |
| 同 | `service/s3/...` | 12.3 秒 | 14.5 秒 | 1.69 GB | 3,423 | 同じ 1 件 |
| 同 | `service/s3/...`、`smithy-go/...` | 0.19 秒 | 2.5 秒 | 1.33 GB | 2,633 | 同じ 1 件 |
| `bench/dart-grpc` | なし | 0.14 秒 | 11.5 秒 | 1.09 GB | 2,409 | 3 件 |
| 同 | `package:grpc` | 0.10 秒 | 11.5 秒 | 1.07 GB | 2,150 | 同じ 3 件 |
| `bench/dart-gcp` | なし | 0.40 秒 | 16.7 秒 | 1.52 GB | 4,593 | 19 件 |
| 同 | `package:googleapis` | 0.35 秒 | 17.3 秒 | 1.53 GB | 3,488 | 同じ 19 件 |
| 同 | `googleapis`、`googleapis_auth`、`gcloud`、`google_cloud_storage`、`google_cloud_protobuf`、`google_cloud_rpc` | 0.21 秒 | 20.2 秒 | 1.56 GB | 2,994 | 同じ 19 件 |

Go では、解析の時間とメモリが、宣言で大きく減る(最大 RSS は、extractor の読み込みが支配するので、設計の試作の 0.67 GB には届かない。試作は、型検査の結果も捨てていた。それは、extractor に渡す段階の仕事)。Dart では、extractor が時間とメモリの大半なので、解析側だけの効果は小さい(段階 4 で効く)。

**しないこと(この ADR では)**: 組み込みモデルへの宣言(`dart-http`、Google Cloud、Go の AWS SDK)。宣言を付けてよい条件(設計 2 節 5)は、ADR-092 / 098 のまま。extractor への宣言の受け渡し。読まない依存だけが使う依存の、到達性による自動の除外(結果が変わりうるので、extractor の段階と合わせて決める)。

