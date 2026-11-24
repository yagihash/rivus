# ADR-044: 組み込みモデルの範囲 — 言語、標準ライブラリ、めちゃくちゃ有名なサードパーティ

- 状態: 一部覆った(→ADR-063)
- 関連: README, GCP-MODELS
- 状態の詳細: 063。追記で gin を外した; 覆した先・注意: 063。本文内矛盾あり
- 要旨: 有名サードパーティも組み込みに

- **決定（上の「組み込みは言語と標準ライブラリまで」を、こう改める）:** 全言語（Go、Dart、Rust）で、**めちゃくちゃ有名なサードパーティのライブラリ**も、組み込みでカバーする。範囲は、徐々に広げる。基準を厳密に決めるのは、いまは難しいので、下の目安で、1 つずつ、足すかどうかを決める。プロジェクト固有のもの（自前のラッパー、小さなフレームワーク）は、これまでどおり、ユーザーのルール
- **目安（暫定）:** (1) その言語で、広く使われている（公開されている利用数、ダウンロード数、いいね数で、上位であること）、(2) セキュリティに関わる面（リクエストの入り口、DB、HTTP クライアント、テンプレート、コマンド、ファイル）を持つ、(3) モデルが小さく、実在のプロジェクトか、テストで確かめられる、(4) ルールセットは、ライブラリごとに 1 ファイル（`rivus/<lang>/<name>@N`）で、外しやすくする
- **現状:** Go は gin（`examples/rules` から組み込みに戻した。ベンチは、組み込みのままで、同じ採点）。macaron は、Gogs のように、1 つのプロジェクトで使われる範囲なので、例のまま。Dart は `dart-packages.yaml`、Rust は `rust-web.yaml`（reqwest、sqlx、actix-web、axum のシンクのみ）が、すでにある。前に決めると言っていた、Dart と Rust のサードパーティのモデルを外すかどうかは、**外さない**ことで決まった
- **候補（順不同、あとで測りながら足す）:** Go: echo、chi、gorm、sqlx、pgx、go-redis。Rust: actix-web と axum の、リクエストの入り口、serde。Dart: shelf、dio、http
- **注意:** 組み込みのモデルが増えるほど、誤検出と取りこぼしの責任が、rivus にある。足すときは、ベンチか、テストで、効果を示す。モデルは、`-rules` と同じ書式で、ユーザーが上書きできる
- **追記（ADR-044、gin は外した）:** gin を組み込みに戻したが、「めちゃくちゃ有名」かどうかを調べると、Go の Web フレームワークでは最大（JetBrains 2025 の調査で 48%）だが、半数に満たず、`net/http` のように、ほぼ全員が使うものではない。しかも、入れた理由は、ベンチの採点に必要だったという都合が大きく、基準としては弱いので、`examples/rules` に戻した。**Go の組み込みは、いまは標準ライブラリだけ**。Go に足すなら、「分野ごとの第 1 位」のような、はっきりした基準を決めてから。Dart と Rust の既存のモデルは、そのまま
- **追記（Dart と Rust も、依存を読む）:** Go の `-deps` と同じ考えで、Dart と Rust の extractor も、依存の関数を IR に出し、`dep` のマークを付けるようにした。コア（`analyze -ir`）は、`PruneDeps` で、プロジェクトから届くものだけを残す。
  - **Dart:** 読んだライブラリが import している `package:` のライブラリ（SDK は除く）を、再帰的に解決して変換する。`--no-deps` で無効。依存のテキスト（スニペットなど）は出さない
  - **Rust:** `RUSTC_WORKSPACE_WRAPPER` を `RUSTC_WRAPPER` に変えて、すべてのクレートに extractor を通す。ワークスペースのメンバーかどうかは、cargo が付ける `CARGO_PRIMARY_PACKAGE` で見分ける。proc-macro は読まない。`--no-deps` で、メンバーだけ
  - **確かめたこと:** Rust のサンプルに、ワークスペースの外の依存クレートを足した。依存を通る流れ（`wrap`）は追われ、中で値を捨てる関数（`constant`）の後ろは、`--no-deps` では誤検出になるが、読むと正しく消える。呼ばれない関数（`never_called`）は IR から落ちる。Dart は、サンプルの findings が同じ 28 件のまま
  - **注意:** Dart で不確実性の合計が増える（shelf 単体で 154 → 382）のは、読むコードが増えて、依存の中の `dynamic_dispatch` が数えられるため（`unmodeled_library_call` は 36 → 14 に減る）。Go と違い、合計は良い指標ではない。Rust の `dyn Trait` の候補は、そのクレートの実装だけ（依存の実装は入らない）。実在の大きなプロジェクトでの測定は、まだ
- **追記（オプション名）:** Go の `-deps`（既定オン）を、`-no-deps`（無効にする）に改めた。3 言語（Go の `-no-deps`、Dart と Rust extractor の `--no-deps`）で、読むのが既定で、外すオプションの名前が揃った
- **追記（`rivus` コマンドから全言語を呼ぶ）:** `rivus analyze`（`query`、`index`、`resolve` も）が、ディレクトリの言語を、マーカーのファイルで自動で判別する（`go.mod`/`go.work`、`pubspec.yaml`、`Cargo.toml`）。`-lang auto|go,dart,rust` で指定できる。Go はこのプロセスの中で読み、Dart と Rust は extractor（`rivus-dart`、`rivus-rust`）をサブプロセスで呼んで、IR のストリームを読む。**複数の言語があれば、IR を 1 つにつなげて、1 回で解析する**（関数の名前は言語ごとに重ならないので、IR のレベルでつなぐだけでよい。結果のレポートを混ぜる必要がない）。混ぜるときは、Go のテキストもストリームの形（`goir.AddTexts`）にする
  - extractor の場所: `-dart-extractor`/`-rust-extractor`、なければ `RIVUS_DART_EXTRACTOR`/`RIVUS_RUST_EXTRACTOR`、なければ PATH の `rivus-dart`/`rivus-rust`。Dart は `pubspec.yaml` に `executables` を足したので、`dart pub global activate --source path extractor/dart` で `rivus-dart` が入る
  - **自動で見つけた言語の extractor がなければ、警告を出して飛ばす**（Go のプロジェクトに、道具の `pubspec.yaml` があるだけ、という場合）。`-lang` で明示した言語は、エラーにする。飛ばすのは「extractor が見つからない」ときだけで、動かして失敗したもの（異常終了、壊れた IR）はエラーにする。飛ばしたときは stderr だけでなく `load_errors` にも言語名と理由を残す（ADR-007。見ていない言語を、正常な結果として出さない）
  - `-no-deps` と `-tests` は、extractor にも渡す
- **追記（Go の標準ライブラリのモデルを足す、2 回目。ZITADEL、Gogs、gin-vue-admin の、モデルのない標準の呼び出しを数えて）:** `go-stdlib-more.yaml`（約 280 のシンボル）を足した。頻度の高い順に、`fmt.Println`、`strconv`、`context`、`log/slog`、`database/sql`、`os`、`net`、`regexp`、`text/template`、`crypto/*`、`errors.As` など。流れのあるものは `flows`、ないものは `pure`、`net/http.ServeFile` は path_traversal のシンク。結果（findings と採点は同じ）: gin-vue-admin の不確実性 550 → 263、go-test-bench 274 → 228、Gogs の `unmodeled_library_call` 310 → 172。3 つのプロジェクトで、モデルのない標準の呼び出しは、残り約 120 件（約 110 種、ほとんど 1 件ずつ）
  - **判断:** `database/sql` の読み取り（`Row.Scan`、`Rows.Scan`）は `pure`（DB を通る 2 次の流れは追わない）。`(*sync.Once).Do` と `Cleanup` は、引数の関数を呼ぶので、`pure` にすると関数の中の流れを捨てるおそれがあり、モデルを付けなかった。`template.Parse` はテンプレートの文字列の流れだけを足し、シンクにはしていない
  - **ZITADEL の試行（このまま ./... で）:** 生成コード（gRPC の protobuf）が repo になく、`missing_source` が 8,668 件で、findings は 0 件。12 パスで収束せず（gRPC の要約が振動）、ピーク 8 GB、約 100 秒。ベンチには使えない。収束しない件は、別に調べる
- **追記（ループの中の `append` でタイントが消えていた、Gitea の CVE の見逃しから見つけた）:** Gitea v1.26.4 の GHSA-4p89（リリースの編集で、フォームのキーを `append` で集めて、ストレージの削除に渡す）を見逃した。小さな例に削ると、**ループの中で、前のラウンドの `append` の結果に `append` するとき**（`for { out = append(out, s) }`）に、タイントが消える。原因: `prepareLocs`（各値が指す場所を、不動点まで求める）の途中で、`locs()` が結果をキャッシュしていた。ループの φ が、ループの末尾の値（まだ計算していない）を読むと、「場所なし」をキャッシュして、そのまま残った。**直した:** 不動点に着くまでキャッシュしない。そのままだと、ループで伸びる経路（ロードのロードの…）が収束しなくなるので、場所の経路を K で切る
  - 回帰のテストを `heap` に足した（`AppendInLoop`、`AppendInRangeLoop`、`ExpAppendLoopSinkInside` など）。採点は変わらない（go-test-bench 18/18、gin-vue-admin 2/2、Gogs 4/4）。findings は、gin-vue-admin 27 → 29、Gogs 76 → 78（ループの中の `append` の先の流れが、見つかるようになった）
  - Go のコードでは、ループで slice に集めるのは、ごく普通の書き方なので、**これは大きな見逃しだった**。実在のプロジェクトで、CVE を引いて、初めて見つかった
- **追記（Go の候補の精度: エントリーポイントからの到達可能性を試した）:** `main` と各パッケージの `init` を起点に、RTA で届く関数を求め、その関数だけで VTA を回すと、デッドコードで interface に変換された型が、候補に入らなくなる。Gogs（`main` あり、28,402 関数が届く）で試した結果、**候補の総数 1,064 → 1,023（−4%）、findings は同じ 78 件、採点も同じ**。VTA がすでに型の流れで絞っているので、到達可能性で減るのは、ほんの少し。組み込まなかった（実験のコードは戻した）。候補の精度の限界は、**文脈を区別しないこと**と、**本当に多態であること**（`SubInitializer` の 11 候補は、実際に 11 の実装がある）にあって、デッドコードではない
- **追記（到達可能性を、findings の事実として付ける）:** テストでしか使われないコード（モック、補助関数）は、`-tests` なしでも、非テストのファイルにあれば、読み込まれて、findings に出る（Gitea の `tests/test_utils.go` の SQL インジェクション 12 件など）。到達可能性で、候補を絞る効果は小さい（上の実験: −4%）けど、**findings の経路が、プログラムの起点から届くかどうか**は、読む人にとって価値のある事実。
  - IR の関数に `entry`（`main`、パッケージの `init`）を足した。Go は `main.main` と `init`、Dart は `main`、Rust は `entry_fn`。コアは、起点から、呼び出し、候補、関数の値をたどって届く関数を求める（`sg.Program.Reachable`）。起点がなければ（ライブラリ）、読んだコードの exported な関数が起点
  - finding に `reachability` を足した: `from_entry`（経路のすべての関数が届く）、`not_from_entry`（届かない関数が経路にある）。起点が分からないときは付けない。**消さずに、事実として付ける**（リフレクションで登録されるハンドラーを、見逃さないため）
  - Gogs（78 件）と gin-vue-admin（29 件）は、すべて `from_entry`
  - Gitea v1.26.4 の結果（修正後）: CVE 3 件すべてを検出（3/3、GHSA-4p89 を含む）。ただし 955 秒、9.6 GB、`depth_limit` 1,499 件、タイムアウトで収束せず。**性能が次の課題**
- **追記（DB を source にする、3 つのやり方。DB の中の流れは追わずに、出口側を見る）:** 保存された値が、あとで HTML やパスに使われる流れ（2 次の流れ）を、「DB から読んだ値は source」として見る。ルール側に、3 つを足した。
  1. **`out`（引数を埋める source）:** `kind: call_result` の source に `out: [0]` と書くと、その呼び出しが、ポインタ経由で、引数の指す先にデータを書き込むものとして扱う（`rows.Scan(&x)`、`db.Find(&users)`）。`...any` の可変長引数（`Scan(&a, &b)`）は、渡したスライスの要素のポインタをたどる（メモリはエイリアスしないので、ポインタを書き込んだ場所で探す。`variadicElems`）。結果は、エラーや件数なので、`out` があるときは、結果には付けない
  2. **型のワイルドカード（`field: pkg.Type.*`）:** その型の、どのフィールドを読んでも source。ORM のモデルの型を、ユーザーが 1 回書けば済む
  3. **`boundary: stored_data`（propagator）:** 呼び出しが、DB のようなストアとの境界であることを示す。呼び出しの場所に、`stored_data_boundary`（`attributable_to: target`）を、不確実性として記録する。組み込みでは、`database/sql` の `Row.Scan` と `Rows.Scan` に付けた。`pure` と同時に使う
  - 組み込みの `database/sql` は、**既定では source にしない**（誤検出を減らす方針は変えない。DB 経由の長い経路が、Gogs で偽の findings を出していた）。使うなら、`examples/rules/go-database-sql.yaml` を、`-rules` で足す。`stored` というラベルで出るので、シンクの `accepts` で、`user_input` と区別できる
  - テスト: `testdata/go/cases/dbsrc`（文字列、`any` 経由の構造体、スライス、`sql.Rows.Scan` の可変長、型のワイルドカード、境界の記録）
- **追記（Gitea の性能調査、Go 1.27）:** Gitea v1.26.4（約 35 万行）の解析は 900〜980 秒（ロード 10〜30 秒、IR 20〜30 秒、グラフ 7〜10 秒、**taint が約 15 分**）、ピーク 9〜10 GB、`depth_limit` 約 1,500 件。測った結果:
  - **GC の設定（`GOGC` 25〜200、`GOMEMLIMIT`、Green Tea GC を外す）では、速くならない**。1 秒あたりの再解析数（`reanalyses`）は、GOGC=25: 465、50: 511、100: 529 で、ほぼ同じ。Green Tea を外すと、`heapBitsSmallForAddr`（小さなオブジェクトの走査）が 65% になり、むしろ重い。`GOGC=50` の既定は、そのままにした
  - CPU プロファイルでは、約半分が GC の走査（ヒープ 3.4 GB の小さなポインタ付きオブジェクト）、`sys` 時間も `user` の 2 倍ほど出る（原因は、まだ分からない。ページフォルトか、GC のスピン）
  - **割り当てのプロファイル（100 秒で約 110 GB）:** 最大は、`why`（経路の 1 ホップ。約 170 バイト）の割り当て 21%。要素がすでにセルにあっても、先に `why` を作っていた。**作る前に、「すでに普通の理由で入っている」かを調べて、飛ばすようにした**（`plainExists`）。再解析数 472 → 511（+8%）。次に、`collectUncached`（14%）、IR 変換の `emit`（11%、ロード時）
  - サンプリング（`sample`）では、`Elem`（`Label`、`Origin`、`Path`、`Mods` の文字列 4 つ）をキーにしたマップの**ハッシュと比較**（`mapaccess2`、`MemHash`、`strhash`）が、`copyLoc` の中で大きい。**次の一手は、`Elem` を整数の ID に直す（文字列を 1 回だけ内部化する）こと**。ハッシュが 1 回の整数になり、`why` の割り当ても減る。見込みは 2 倍前後だが、エンジンの中心の型の変更なので、設計を示してから進める
- **追記（並列化: 呼び出しグラフの段ごと、大きな SCC は中も並列。結果は決定的）:** 解析（`Engine.Run`）は、1 コアでしか動いていなかった（SCC を順に処理するループ）。Gitea では、呼び出しグラフが 92 段あり、一番広い段に 16,742 個の SCC がある一方、**最大の SCC が 907 関数**（325、395 関数のものも）で、これらを直列に反復する段階に、44 秒、18 秒、15 秒がかかっていた。
  - **段（`levelsOf`）:** SCC は、呼ぶ SCC の 1 つ上の段に置く。同じ段の SCC は、互いに依存しないので、`-workers`（既定は CPU の数）で、並列に解析する。共有状態（要約、`dirty`、`spent`、`degraded`、`states`、`notConverged`）は `Engine.mu` で守る。**グローバル変数への書き込みは、段の間は保留して、段の終わりにまとめて反映する**（集合の和なので、順序に依らない）
  - **大きな SCC（8 関数以上）:** 反復のたびに、全メンバーを、前の反復の要約を見て、並列に計算（`compute`）してから、SCC の順に反映（`commit`）する（Jacobi 型）。Gauss-Seidel より反復は増えうるが、メンバーは並列に動く
  - **決定性:** ワーカー数 1、4、12 で、finding の ID、不確実性、パス数が完全に同じ（Gogs で確認）。`go test -race` も通る（`gossa.Program` のソースのキャッシュ、`sg.Program.Global` の遅延作成に、ロックを足した）。採点は同じ（Gogs 4/4）
  - **速さ:** Gogs の taint は 5.3 → 3.6 秒。Gitea の 90 秒の窓で、再解析数 510/s → 745/s。**最初のパスが、窓の中で終わるようになった**（前は 1 パスの途中）。全体の時間は、実行して確かめる
  - `Stats` に `levels`、`largest_scc`、`widest_level`、`dep_spent_ms`、`own_spent_ms` を足した
- **追記（Rust extractor が parseable で動く）:** fork の確認で、parseable（actix-web、約 6.8 万行）の抽出が、`Instance::resolve` の panic（const generics の呼び出し）で失敗した。**panic を `catch_unwind` で握りつぶすと、rustc のクエリが「実行中」のまま残り、最後に別の assertion で落ちる**ので、**panic を起こす呼び出しを避ける**ことにした: 総称引数に、値でない const（const パラメータ、束縛された const）が入る呼び出しは、`resolve` を呼ばず、「解決できない呼び出し」にする。レシーバのない、解決できない呼び出し（`T::default()`）は、`dynamic`（レシーバが必要）ではなく、外部の `static` にする。
  - 結果: `--no-deps` で、抽出 138 秒、IR 141 MB（8,557 関数）、解析 4 秒、findings 24 件（path_traversal 19、ssrf 4、open_redirect 1）。脆弱な関数（`alerts::target`、`filters`、`oidc`、`redirect_to_client`、`list_condition_expr`）は、IR に出ている
  - **まだ分かっていないこと:** 既知の 3 つの流れ（アラートの SSRF、Filters のパス操作、`redirect_to_client` の open redirect）が、見つかっているかは、これから調べる。findings の `reachability` は、すべて `not_from_entry`（ハンドラーが、フレームワークの登録の呼び出しを通ってしか、`main` から届かない。依存を読まない設定では、届く経路が切れる）
- **追記（Rust のベンチ: parseable、3 件中 2 件を検出）:** parseable（actix-web、約 6.8 万行）の、修正前のコミット `5c20b8e4` で、3 つの助言のシンクを正解にした（`bench/parseable/`）。結果は、**TP 2 / FN 1**（open redirect `oidc.rs:443` と、パス操作 `localfs.rs:383` を検出、アラート対象の SSRF `target.rs:494` を見逃し）。精度は測っていない（ほかの 91 件は読んでいない）。設定は `--no-deps`（依存を読んでも、findings は同じだった）、`rules.yaml`（actix の入力型を source、`insert_header`、`reqwest::Client::post`、`tokio::fs` をシンク）。
  - **見つかった原因と直したこと（findings が 24 → 91 件）:** (1) `async fn` の本体は、actix の executor が poll して初めて動くので、IR に「呼び出し」が現れず、ハンドラーの本体に、引数のタイントが届かなかった。**コルーチンを作る場所で、本体を、`Pin<&mut コルーチン>` を渡して呼んだものとして扱う**ことにした（executor が何をしても、本体は渡された引数で動くので、過大近似として健全）。`&mut コルーチン` は、コルーチンの入った**セル**でなければならない（値そのままでは、本体が `pointer` を読んだときに、中身が見えない）。(2) Rust の `param` に位置がなく（`Pos::none()`）、param の source が、finding の位置を持てなかった。関数の位置にした。(3) 総称引数に値でない const がある呼び出しは、`resolve` を呼ばない（上の記録）。レシーバのない、解決できない呼び出しは、外部の `static`
  - **まだ見逃している SSRF:** `Target` は、`web::Json` で受けたあと、グローバルの `TARGETS`（`RwLock` の中のマップ）に保存され、アラートの実行時に、別の関数が読んで、`reqwest::Client::post(self.endpoint)` に渡す。保存と読み出しの間（グローバル、`tokio::sync::RwLock`、`HashMap`）で切れていると見ている。DB と同じ「2 次の流れ」の形で、`-no-deps` では、`tokio` のロックなどが、モデルのない外部の呼び出しになっている
  - findings はすべて `not_from_entry`（ハンドラーは、フレームワークの登録の呼び出しを通ってしか、`main` から届かない）。事実としては正しいが、Rust では、判断の材料にならない
- **追記（parseable の精度、独立レビュアーが 25 か所のシンクを読んだ）:** 5 件が脆弱性（公開された報告のあるもの 3 件: OIDC の open redirect、Filters のパス操作の 2 か所。残り 2 件は、公開されていないため、場所と内容を、この文書に書かない）、**19 件が `partial`**（入力は本当に届くが、ストリームの存在確認、名前の検証、認証ミドルウェアによるヘッダーの上書きで止まる）、1 件が誤り。「入力が届く」は 25 件中 24 件、「悪用できる」は 25 件中 5 件。
  - **前の「3 件中 2 件を検出」は、過大評価だった:** Filters のパス操作は、シンクの場所（`localfs.rs:380/383`）には届いているが、**本当の流れ**（`filters::post` の body の `Filter.stream_name`）は、示されていない（別のハンドラー `filters::update` の `web::Path`、`alerts::modify_alert` の `web::Json` を通る流れが、同じ場所に届いている）。場所の一致だけを見る採点では、この区別ができない。正解ファイルに注釈を付けた。今は、TP 2 / FN 1（アラート対象の SSRF が見逃し。公開されていない件は、正解ファイルに入れない）
  - **精度を下げているパターン:** (1) `HttpRequest` 全体が source なので、`x-p-tenant` ヘッダーが、多くの経路の出発点になる（認証が、ほとんどの操作で上書きする）、(2) 存在確認や名前の検証のゲートを、ツールが見ない、(3) 構造体全体が汚染される（本当の source が body のフィールドなのに、別のフィールドが source として示される）、(4) 共通のストレージ操作は、最短経路が安全な呼び出し元を通り、本当に脆弱な呼び出し元が、報告に出ない
- **追記（インターフェース呼び出しの候補を、呼び出し元が渡した型で絞る）:** Issue「インターフェースの呼び出しが、そのパスで作られない実装まで、シンクとして報告する」を受けて。原因は、候補が文脈を区別しないこと（VTA は、`get(b Backend)` に、すべての呼び出し元の型を合併する。要約も、呼び出し元を区別しない）。
  - **事実として付けた（案 1）:** finding に `dispatch_widened`（経路の上で、複数の実装のどれにも届くとして扱った呼び出しの数）と `dispatch_candidates`（その候補数の最大）。判断はせず、順位付けの材料にする
  - **絞る（案 4）:** レシーバが、**その関数のパラメータ**である、インターフェースのメソッド呼び出しで、候補が 2 つ以上あるとき、各候補の要約から出る流れに、`Guard` の一種（`disp`: レシーバの型と、すべての候補のレシーバの型）を付ける。呼び出し元は、引数が**そこで `make_interface` で作られた**値（動的な型が、オペランドの型にちょうど決まる）なら、その型が、**ほかの候補のレシーバの型に一致する**場合にだけ、この候補を通る流れを除去する（`feasible`）。
    - 安全側: 型が分からない（別の関数の戻り値、フィールドから読んだ値）、どの候補にも一致しない（埋め込みで昇格したメソッドなど）、レシーバの型が分からない候補がある、のときは、絞らない。つまり、見逃しは増えない（落とすのは、型から、その候補が呼ばれないと言えるものだけ）
    - `dispatch` ガードは、コードの条件ではないので、`conditional_nodes` には出さない
    - 値の型の方が、ポインタの型より `sameBase`（`*` を除いて比べる）で、値メソッドを、ポインタ経由で呼ぶ場合も、一致として扱う
  - **限界:** 絞れるのは、引数が呼び出しのすぐ前で作られているときだけ。パラメータを、さらに別の関数に渡して呼ぶと（`get2(b)` → `get(b)`）、内側のガードは、外側の呼び出し元では評価されない（絞らない）。構造体のフィールドに入れた値も、絞らない。これは、点の解析（points-to）で解く問題
  - 再現: `testdata/go/cases/dispatch`（絞れる例が出ない、別の型は出る、型が分からないと出る、埋め込みは出る）。gin-vue-admin で 1 件が消えた（`c.JSON` が `render.JSON` を渡しているのに、`IndentedJSON.Render` 経由で届いていた経路）。go-test-bench、Gogs は同じ
- **追記（gRPC サーバのカバレッジ。生成コードは、解析の前に済ませておく）:** gRPC のハンドラーへの入力は、生成コード（protoc-gen-go-grpc、protoc_plugin、tonic-build）と、サービスの登録（`any` と構造体のフィールドを通る）を経由する。呼び出しグラフは、そこを繋げない（unary の `dec(in)` は関数値の呼び出しで、候補がない）。そのため、**ハンドラーは、生成コードの規約から見つけて、入口として宣言する**。
  - **前提（決定）:** 生成コードは、**解析を実行する前に、生成済みであること**。コードの生成は、rivus の役割ではなく、利用者（CI）の手順にする。生成コードがないと、ハンドラーは見つからない（findings は出ない。出ないことを、見逃しと区別するため、将来、生成コードがありそうなのに見つからないときは、警告を出す）
  - **ライブラリのモデル（組み込み）:** Go `go-grpc.yaml`（`metadata.FromIncomingContext`、`ValueFromIncomingContext`、`ServerStream.RecvMsg` の out、`grpc.NewClient`/`Dial`/`DialContext` の SSRF）、Rust `rust-grpc.yaml`（tonic の `Request`、`Streaming`、メタデータの経路、`tokio::fs` のシンク）、Dart `dart-grpc.yaml`（`ServiceCall.clientMetadata`、`ClientChannel` の SSRF）。有名な第三者ライブラリを、組み込みにしてよい方針（ADR-044）のとおり
  - **unary と streaming のハンドラー（Go）:** ルールに `marker:` を足した（`kind: param`、`match: implementations`、シンボルなし）。そのメソッド名（glob）を持つインターフェース（`mustEmbedUnimplemented*`）の、ほかのメソッドを、実装に展開して、**メッセージ型（構造体へのポインタ）の引数だけ**を source にする。`context.Context` とストリームは、インターフェースなので外れる。マーカーのないインターフェースは、サービスではない。`require_unimplemented_servers=false` で生成したものは、マーカーがないので、プロジェクトのルールに書く。検索は、ソースから読んだパッケージだけ（生成コードがプロジェクトにあれば動く）
  - **Rust、Dart:** メッセージ型が生成型（Dart）、または `tonic::Request`（Rust）。Rust は `tonic::Request` などの `param_type` で、ハンドラーの引数が source になる（型の ID は、再エクスポートの `tonic::Request`）。Dart は、生成されたリクエストの型を、プロジェクトのルール（`kind: param_type`）に書く。マーカー方式は、まだ Go だけ（Dart の `ServiceBase` のサブクラス、Rust のトレイト実装は、別のリゾルバーが必要）
  - **直した不具合:** Dart の extractor が、単項と二項の `-`（`fixnum` の `Int32`）に同じ ID を付けて、`function ... is defined twice` で依存込みの抽出が失敗していた（protobuf を使う Dart プロジェクト全部が対象）。単項は `unary-` にした
  - テスト: `testdata/go/cases/grpcsrv`（メタデータ、`RecvMsg`、接続先）、`grpcunary`（生成コードの規約からのハンドラー）。テスト用に、同じ import パスのスタブ（`testdata/go/stubs/grpc`）を使う。**テストでは、ハンドラーの型が、インターフェースに変換されていないと、go/ssa がメソッドを作らない**（実際のプロジェクトは、`Register…Server(s, &impl{})` で変換される）

### 追記: 言語の文法の取りこぼしを埋めた（Dart と Rust）

- Dart extractor: extension のメンバーは、レシーバーを最初の引数に取る関数にした（今まではレシーバーが渡らず、`this` が nil で、extension を通る流れが全部切れていた）。`Ext(x).m()`（extension override）、コンストラクタの tear-off（`C.new`）、型引数付きの関数参照、dot shorthand（`.red`、`.make(x)`、`.new(x)`）、型リテラル、コレクションの `for`（古典的な形、識別子、パターン）を落とさず lower する。
- Rust extractor: `ptr::copy_nonoverlapping` の MIR 文（最適化したビルドで出る）を、`load` と `store` にする。インラインアセンブリは、入力を引数、出力を結果にした `std::arch::asm` の呼び出しにする（今までは、ブロックを `unreachable` で終えて、それ以降のコードを落としていた）。モデルは `rust-std.yaml` に足した。
- どれも、取りこぼしていたものを拾うだけの変更で、`unsupported` を出していた箇所は 0 になった。

### 追記: 生成コードが見つからないときの警告（gRPC、Go）

- `marker:` のルールに `marker_call:`（サービスを動かすときに呼ぶ関数。go-grpc は `google.golang.org/grpc.NewServer`）を足した。その関数をプログラム自身のコードが呼んでいて、マーカーを持つインターフェースが 1 つも見つからなければ、`missing_source`（`attributable_to: target`）の不確実性 `generated_code_missing` を出す。クライアントだけのプログラムは、`NewServer` を呼ばないので、警告しない。
- Rust（tonic）は、ハンドラーが `tonic::Request` を引数に取るので、`param_type` ですでに全部見つかる。Dart は、リクエストが生成クラスなので、型の継承関係が IR にないと自動では見つけられない。IR に型の階層を足すのは契約の変更なので、まだやっていない（プロジェクトのルールで `param_type` を書く）。

### 追記: IR に型の階層（`type` レコード）を足した、`extends:` のルール（Dart の gRPC ハンドラー）

- **決定:** IR のストリームに、`{"kind":"type","type":{"id":...,"supers":[...]}}` のレコードを足した。クラスが、直接に、継承・実装・mixin している型の ID を持つ。足すだけの変更で、`schema_version` は変えない（古いストリームは、階層なしとして読める。このレコードを知らない古い読み取り側は、未知のレコードとして拒む。extractor と本体は同じバージョンで配る前提）。
- **ルール:** `kind: param` に、`extends: <型 ID>` と `with_param_type: <型 ID>` を足した。「`extends` のサブタイプ（推移的）の、解析対象のコードのメソッドのうち、`with_param_type` の型の引数を取るもの」が入口になり、その型の引数とレシーバを除いた引数が入力になる。`exclude_methods`（glob）で、生成コードのラッパーを外す。`marker_call` は、Go の `marker:` と同じ使い方で、その関数を呼んでいるのに 1 つも見つからなければ、`generated_code_missing` を出す。
- **Dart の gRPC:** `Service` のサブクラスで `ServiceCall` を取るメソッドが、ハンドラーになる（`dart-grpc.yaml`）。リクエストの型を、プロジェクトのルールに 1 つずつ書く必要はなくなった。生成コード（`.pb.dart`、`.pbgrpc.dart`）は、解析の前に済んでいる前提（コミット済み、または `protoc` の実行後）。
- **しなかったこと:** Rust の tonic は、ハンドラーが `tonic::Request` を取るので、`param_type` で足りる。Go は、`gossa` が型の情報を持っているので、`marker:` のまま。

### 追記: Dart のベンチマーク（dart-lang/pub）

- `bench/dart-pub`: pub の `extractTarGz` の zip slip（GHSA-q739-79rh-vmvp / CVE-2026-27704）。修正の 1 つ前のコミット（`5e24336e`）で、正解にした 9 サイトを全部見つけた（recall 1.00）。ほかに 8 サイトを報告するが、正解には入れていない（同じ流れが届くヘルパー 5、Windows のシンボリックリンク作成 1、プロジェクトのテスト 2）。修正のコミットでも同じ結果になる（`p.normalize` と `p.isWithin` はサニタイザーとして知られていない）ので、精度ではなく、大きな実コードでの recall の動作確認である。pub のエコシステムには、このほかに、taint の流れとして測れる勧告がなかった（調査: GitHub の勧告 14 件）。
- 実コードで、Dart extractor の不具合を 4 つ見つけて直した: 入れ子のコレクション要素、null-aware 要素（`?x`）、同名の名前付きコンストラクタとメソッド（またはゲッター）の ID の衝突、名前のない extension の ID の衝突。

### 追記: Rust の static（グローバル）と、別名を返すモデル（**提案。未決定**）

- **直したこと（extractor）:** Rust の `&STATIC`（static への参照の定数）は、今まで、ただの定数（`const`、nil）として出ていた。それを `addr_of`（`global` に static の名前）にした。どの関数も、同じ static を、同じ 1 つの変数として見る。parseable の finding は、80 件のまま変わらなかった。
- **まだ届かない流れ:** parseable の `src/alerts/target.rs:494`（アラートの宛先の SSRF。リクエストで作った対象が、グローバルの `TARGETS`（`LazyLock<…RwLock<HashMap>>`）に保存され、あとで別の場所が読む）。再現は、`Mutex<String>` を持つ `static` に、一方の関数が `*S.m.lock().unwrap() = source()` と書き、他方が `S.m.lock().unwrap().clone()` を sink に渡す形で、同じ形で切れる。
- **原因:** モデルの `flows: [{from: args, to: result}]` は、「引数の中身の taint を、結果にコピーする」意味で、`Deref::deref`、`Mutex::lock`、`RwLock::write`、`Result::unwrap`、`DerefMut::deref_mut` のように「同じ場所への参照を返す」関数の別名関係を持てない。だから、ガードを通した書き込み（`*guard = x`、`guard.push(x)`）が、元のグローバルの場所に届かない。
- **案:** フローの属性に `alias: true`（結果は、引数が指す場所を指す）を足して、`locs(結果)` を `locs(引数)` と同じにする。Rust の標準ライブラリのモデルのうち、参照を返すものにだけ付ける。ほかのモデルと既存の結果には影響しない（付けたものだけ）。コスト: エンジンの別名の扱いに触れるので、`collect` のキャッシュ（root ごとのバージョン）との整合を確かめる必要がある。
- **要判断:** エンジンの設計に触れるので、まだ入れていない。`std::sync::RwLock::read/write` のモデルが、標準のものにないことも見つかった（tokio のものだけある）。

### 追記: モデルの `alias: true` を入れた（上の提案を実装）

- **決定:** フロー `{from: args, to: result, alias: true}` は、「結果は、その引数と同じ場所を指す」を表す。`locs(結果)` に、引数の `locs` を足す（結果自身の root も残す）。引数が、ローカルのセルのアドレス（ガードを変数に入れて `&mut guard` で渡す形）なら、そのセルに `store` された値の `locs` も足す。taint のコピーは、`alias` が無いフローと同じに行う。
- **付けたもの:** Rust の std の `Deref::deref`、`DerefMut::deref_mut`、`Mutex::lock`、`RwLock::read/write` と `try_*`、`Result::unwrap/expect`、`RefCell::borrow/borrow_mut`、tokio の `Mutex::lock`、`RwLock::read/write`。
- **もう 1 つ直した:** `sg.Program.Global`（パッケージ変数のアドレスの値）に、クラス `pointer` を付けた。付いていなかったので、`p := &G` の `copy` が、ポインタのコピーとして扱われず、グローバルの場所がたどれなかった。
- **確認:** Mutex / RwLock<HashMap> / RwLock<Vec> を持つ `static` に、一方の関数が書き、他方が読む 3 つの流れが通る（`extractor/rust/test/sample/src/global_flows.rs`）。go-test-bench（37 件）、gogs（78 件）、gin-vue-admin（28 件）、Gitea（246 件）、parseable（80 件）は、変更の前後で、finding が完全に同一。
- **検証:** 関数には、結果が同じ場所を指す引数が 1 つしかない。1 つのモデルに alias のフローが 2 つ、1 つのルールセットの中に、同じシンボルで alias を持つモデルが 2 つあると、パースで弾く。複数のルールセットにまたがる重複（組み込みとプロジェクトのルール）は、合成後の `Rules.Validate()`（`AliasArgs()` のエラー）が弾く。読み込み側（`cmd/rivus` とテスト用の `testutil.Rules`）が、これを呼ぶ。弾かないと、後のものが、何も報告せずに前のものに取って代わる。
- **まだ届かない:** parseable の `src/alerts/target.rs:494`。tokio の `RwLock` と `.await`（async の状態機械）、`entry().or_default().insert()`、`Option::ok_or`、`call` の動的ディスパッチと、鎖がまだ長い。

### 追記: `unsupported` 命令の記録（CORE-1）と、例外のデータフロー（CORE-2）

- **CORE-1:** コアが `ir.OpUnsupported` を無視していた。`unsupported_construct`（`attributable_to: rivus`）の不確実性を、その場所に出すようにした。`missing_source`（`target`）にしなかったのは、対象のコードが足りないのではなく、rivus が読めなかったものだから。結果のスキーマのバージョンは、kind の追加なので上げていない（読み取り側は、知らない kind を、そのまま持てばよい）。ARCHITECTURE 2.3 の「`unsupported` は理由の文字列を持ち、記録の起点になる」を、実装が満たした。
- **CORE-2:** `throw` の値が、`catch` のハンドラーに届かなかった（`transfer()` に `OpThrow` と `OpExceptionValue` の case がなく、サマリにも「投げる」出力がなかった）。関数ごとに、投げた値を 1 つの場所（結果の番号 -2）に集めて、`exception_value` がそこから読む。サマリに出力の種類 `throw` を足して、呼び出し元は、呼び出し先が投げた値を、自分の同じ場所に受ける。どの `throw` を、どのハンドラーが受けるかは、区別しない（本当より多く、少なくはならない）。投げる値は、interface に入れたときと同じく、オブジェクトなら `/*` の下に置く。

### 追記: 2026-11-19 のレビューへの対応（Dart、Rust）

レビュー（`dc82535` 時点の静的読解）の指摘のうち、フィクスチャを先に足して失敗することを確かめてから直したもの。どれも、フィクスチャは、修正前に失敗した。

- **Dart（`extractor/dart`）:** DART-1（catch の中の例外で finally が実行されない）、DART-2（factory コンストラクタ。リダイレクトの factory を含む）、DART-3（`super(...)`、`: this.named(...)`）、DART-4（`analysis_options.yaml` の `exclude` で外された生成コード）、DART-5（`dynamic` のメンバ）、DART-6（トップレベル変数と static フィールドの初期化子）、DART-7（`continue <caseLabel>`）、DART-8（パターン代入と `for-in` の識別子）、DART-9（`Future.then`、`Stream.listen`、`Iterable.forEach` などのコールバック）、DART-10（利用者定義の演算子と `[]`）。解決できない名前、飛び先が分からない `break` / `continue`、下げられない代入先は、何も報告せずに捨てることはせずに `unsupported` を出す。
- **Rust（`extractor/rust`）:** RUST-1（`Drop` の終端が `Drop::drop` を呼ぶ。直接の型のみで、フィールドの drop glue はたどらない）、RUST-2（クロージャ。ジェネリックな関数の中の `f(v)` は、引数の型が合うクロージャを候補にし、呼び出し元が渡した型で、dispatch ガードが絞る。クロージャの型が分かる std の高階関数（`Iterator::for_each`、`map` など、`Option::map` など）は、レシーバの中身でクロージャの本体を呼ぶ）、RUST-3（trait の候補を、trait の名前で一致させる。trait の既定のメソッドの本体も候補に入れる）。
- **コア:** CORE-1（`unsupported` 命令を不確実性にする。上の追記）、CORE-2（例外のデータフロー。上の追記）。モデルのある trait のメソッドに、プロジェクトの実装があるときは、モデルに加えて、実装の本体も読む（`Iterator::next` を実装する型を `dyn` で呼ぶ形が、モデルに隠れていた）。
- **精度の変化（parseable）:** クロージャを通る流れが見えるようになり、finding が 102 件から 136 件になった（正解にした 4 サイトのうち 3 は、前後で同じ）。追加された経路を読むと、クロージャの環境（キャプチャした値）が 1 つのオブジェクトとして扱われ、`for_each_live_node` を通って、ノード間通信の URL に届くものが多く、誤検出の可能性が高い。クロージャの環境のフィールドごとの区別（キャプチャした値を、フィールドとして追う）が、次の課題。
- **追加で直したもの:** CORE-3（代表の選び方を、最短から、「到達可能、不確実性が少ない、広げた呼び出しが少ない、短い」の順に変えた。`other_sources` の各項目に、経路の長さ、不確実性の数などを残す。上の「情報は落とさない」は、これで実態に合う）、CORE-4（切ったアクセスパスと、まとめたパラメータのセルを、`path_limit`（`depth_limit` の kind）として、関数の一覧つきで 1 件だけ記録する）、RUST-4（下げられない構文、終端、書き込みの projection で `unsupported` を出す。`ThreadLocalRef` は static と同じく `addr_of`）、RUST-5（`-Zmir-opt-level=1` を固定する）、RUST-6（`unwrap` のパニックをなくす）、RUST-7（union の書いたフィールド）、RUST-8（`extern` の関数の呼び出しに、IR の `ffi` を付けて、`ffi_boundary` にする）、RUST-9（const generic が開いたままの呼び出しは、RUST-3 の候補で解ける）、RUST-10（同じ名前の関数が複数のクレートにあるとき、内容で決定的に選んで、`load_error` に残す）、RUST-11（何も報告せずに捨てる配管を `load_error` にする）、DOC-1（README）。
- **レビューにない不具合を、直す途中で見つけて直したもの:** Rust の配列の添字（セルに値で持った配列は、1 段のデリファレンスのあとで添字を引く。ローカルの配列でも、要素の流れが全部失われていた）。Dart の `x++` と `--x` と `x += 1` の、ローカルでない変数への読み書き（読む要素も、書く要素も、演算の側が持っていた。書き込みは何も報告せずに捨てられていた）、依存パッケージの中のクロージャが `dep` のマークを持たなかったこと。
- **指摘が当たらなかったもの:** DART-10 の `bindPattern` に `default` がない件は、`DartPattern` が sealed で、`switch` が網羅されているので、コンパイラが保証している（足すと、到達不能の警告）。RUST-12（rustc の内部の名前への依存）は、ツールチェインを固定していて（`nightly-2026-11-15`）、async の流れをサンプルのテストが守っているので、コードは変えない。ツールチェインを上げるときは、Rust のテストを必ず実行すること。
- **CI:** Dart と Rust のサンプルの解析結果に、`unsupported` が 0 件であることの確認を、テストに入れた（`TestDartExtractor`、`TestRustExtractor`）。公開パッケージでは `bench/dart-corpus/run.sh`。CI への組み込みは、まだ。
- **まだ手を付けていないもの:** CORE-5（数値・真偽値で汚染を止めたことの記録。検討事項）、クロージャの環境のフィールドごとの区別（RUST-2 の精度）、parseable の SSRF（`src/alerts/target.rs:494`）。

### 追記: 数値・真偽値で追うのを止めたことの記録（CORE-5）

- **決定:** 数値型と bool 型の値には、既定で、taint を載せない（インジェクションの対象にならないため。ルールの sink が、サイズや件数のときは、`-numeric-taint`）。これは、意図して再現率を捨てている箇所なので、**実際の source のラベル**が、数値や bool になって止まった場所を、`numeric_stop`（`attributable_to: rivus`）として、source の種類（シンボルとラベル）ごとに 1 件、記録する（リクエスト型の引数は、受け取る関数ごとに別のサイトなので、サイトごとだと、同じことが関数の数だけ出る。Gogs で 271 件になった）。最初の場所、ラベル、サイトの数、起きた関数の一覧（10 件まで）を持つ。入力が数値に変えられる場所（`strconv.Atoi` のような `pure` のモデルの呼び出し、数値型の値への追加）が、これに該当する。
- **「どの場所でも出る記録」にしなかった理由:** 「数値型で止めた」とだけ出すと、ほぼ全部のプログラムで出て、レビューする人の役に立たない。止まったのが「実際の入力」のときだけにして、数を絞った。
- **限界（出力の説明にも書いた）:** 関数のサマリを通って止まる分（呼び出し先の中で、引数の記号的なデータが数値になる）は、記録されない。呼び出し先が、実際のデータを渡されたときにだけ見える。完全な形は、数値を追うバージョンの解析も実行して、差分の flow を別に出すこと（約 2 倍の時間）で、必要になったら足す。

