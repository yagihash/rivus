# ADR-071: モデルの `calls:`(ライブラリの関数が、渡された関数を呼ぶ)。渡された関数が分かれば、その要約を呼び出しの位置で適用する(関数値の設計の段階 2)

- 状態: 有効
- 関連: MODELING §3
- 状態の詳細: 080 が `method:` を追加
- 要旨: 段階 2

**背景:** `Iterable.map(f)`、`Option::map(f)`、`Once.Do(f)`、`strings.Map(f, s)` などは、引数の関数を呼ぶ。今のモデル(`flows`)は「引数が結果に入る」としか表せず、コールバックの結果(呼び出し元がキャプチャした値と、ライブラリが渡す値で作られる)と、中の sink への流れは、作った場所で適用する仕組み(ADR-059、`assumed_call`)の、結果を適用しない近似に頼っていた。設計は `docs/DESIGN-callouts.md` の 3.4 と段階 2。

**決めたこと:**
1. YAML に `calls:`(`fn`、`with`、`ret`、`throw`、`writes`、`mode`)。位置の名前は `flows` と同じ(`recv`、`argN`、`resultN`)で、値は全体(射影なし)。`feed` は書式に入れていない(`fold` はまだモデルにしない)。書式は `docs/MODELING.md` の 3。
2. **`ret` を持つ行は、`flows` の代わりに導いた流れを持つ**: `with` の各値から `ret` へ素通し。使うたびに `assumed_call`。`ret` があり、`with` がなく、`flows` もない行は、読み込みでエラー。書いた `flows` は、導いたものに足す。
3. エンジン(`internal/taint/callrows.go`): モデルのある呼び出しで、`fn` の値が `sg.ExactClosures` で 1〜4 個に決まり、本体があれば、各々の要約を `applySummary` で適用する(パラメータ = `with`、キャプチャした値 = `make_closure` の引数。Rust のクロージャは、環境の alloc を第 1 引数に)。結果・throw・書き込みは `ret` / `throw` / `writes` の行き先へ。決まらないときは、導いた流れ + `assumed_call`。導く流れもないときは `unresolved_callee`(渡された関数は追っていない)を 1 つ。呼び出しは、渡された関数の要約に依るので、呼び出しグラフのエッジ(`callbackFuncs`)と、再実行の判定(`callBindings`)に入れた。
4. **二重に数えない**: 渡されたクロージャの使い方が、すべて、具体化される行の `fn`(または、その場で呼ぶ)なら、作った場所で適用しない(`coveredByModel`)。残りの使い方があれば、今のとおり作った場所でも一致する。
5. 最初のモデル(標準ライブラリ): Go `Once.Do`、`sort.Slice` / `SliceStable` / `Search`、`slices.SortFunc` / `SortStableFunc` / `IndexFunc` / `ContainsFunc`、`strings.Map` / `IndexFunc` / `TrimFunc` / `FieldsFunc`。Dart `Iterable`(`map`、`where`、`firstWhere`、`lastWhere`、`any`、`every`、`forEach`)、`List.map` / `forEach`、`Set.forEach`、`Map.forEach`、`Future.then` / `catchError` / `whenComplete`、`Stream.map` / `where` / `listen` / `forEach`。Rust `Once::call_once`、`Option` / `Result` の `map`、`and_then`、`map_err`、`ok_or_else`、`unwrap_or_else`、`filter`、`Iterator` の `map`、`filter`、`filter_map`、`flat_map`、`for_each`、`any`、`all`、`find`、`position`、`take_while`、`thread::spawn`(`mode: go`)。Rust は、抽出器が、すでに一部の API でクロージャの呼び出しを作っている(名前による表)ので、その行には、元の `flows` を残した(**削除が起きない側**)。
6. **実行の検査**(`TestZooCallbacksGo` / `Dart` / `Rust`。`bench/zoo-{go,dart,rust}/callbacks`): 目印を記録するクロージャを渡して、実際に呼び、`with`(渡された値)と `ret`(戻り値が結果に出たか)を、行と照合する。行が呼ばれていない、行と食い違う、は、モデルの誤り(直す。`@miss` にしない)。照合そのものが落とせることは `TestCallbackCheckFindsAWrongRow`。Go 12 行、Dart 18 行、Rust 21 行は、全部、一致した。`HttpRequest.listen` などは、サーバが必要なので、`calls` を書いていない。

**確かめたこと(`compare.sh b820590 d277d63`、軽い層):** 実コード(gogs 2 つ、gin-vue-admin、go-test-bench、gophish、casdoor、dart-pub、parseable)は、finding の追加 0・削除 0。`assumed_call` の数(不確実性): gogs-9a9388ac 157 → 156、gogs-48946299 158 → 157、dart-pub 192 → 140、parseable 0 → 28(Rust のクロージャが引数の関数で、導いた流れが使われた。経路が変わった finding が 16)。`unmodeled_library_call` は gogs 2 つで 300 → 291、304 → 295。curated: dart-curated は追加 4(`cap_result`、`cap_result_then`、`catch_error_value`、`map_entries`。どれも、実行で本物と分かっている流れの `@miss` が外れた)と、削除 1(`map_const_result` の `sink(out.first)`: `items.map((s) => 'fixed')` の結果に source は入らない。`@fp` だったもの。同じ source は `sink(items)` で見つかるまま)。rust-curated は追加 1(`spawned`)。zoo: `clos/to-lib` は Dart の 3 つ(`vuln2`、`vuln3`、`decoy` の流れ)と Rust の `vuln` が見つかる側に。Rust の `clos/capture-write-by-ref/vuln5`(`call_once` が呼ぶ)も出た。Go の `vuln2` は `sync.OnceValue`(結果が関数値)で、段階 4 の作業。デコイの誤報・修正済みのケースの誤報は、3 言語とも増えていない。

**限界:** 射影がない(`recv` は要素ごとでなく全体)。`fold` の accumulator(`feed`)、結果が関数値の API、フィールドに入れた関数を呼ぶものは、まだ。デファクトのライブラリ(`dio`、`tokio` など)は、このあと。Rust は、渡された関数が `&closure` や、他の関数の引数のとき、決まらず、導いた流れ(`assumed_call`)に戻る。


