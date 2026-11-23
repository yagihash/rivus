# ADR-068: インターフェース越しの呼び出しで、候補の実装に sink のルールがあれば、その候補の sink として扱う(ADR-045 の延長)

- 状態: 有効
- 関連: MODELING §4
- 状態の詳細: 045 の延長

**背景:** `pgx` や `database/sql` を使うコードは、プロジェクトが定義したインターフェース越しに呼ぶ形が非常に多い(sqlc の `DBTX`、自前の `Querier`)。呼び出し点のシンボルは `(myapp.dbtx).QueryContext` で、sink のルールのある `(*database/sql.DB).QueryContext` ではない。サイトの列挙は `rules.Sinks[呼び出し点のシンボル]` だけを引いていたので、sink にならず、`dynamic_dispatch` と `unmodeled_library_call` の不確実性の注記だけが出た(finding なし)。関数値の呼び出し(`CalleeValue`)は ADR-045 で、候補ごとに sink の site を作っていた。

**決めたこと:**
1. **`internal/sites/enumerate.go`:** `CalleeDynamic` の呼び出しで、呼び出しグラフ(Go は CHA / VTA。Dart と Rust は extractor が持つ実装)の候補ごとに、ルールを引く(候補のシンボルは、IR に関数があればそのシンボル、なければ ID)。sink のルールがある候補ごとに 1 つ site を作る(`Site.Callee` を候補の ID に。ID は候補ごとに別)。レシーバは常にある(`sinksAt` の `recv` と `when_arg_type` は、そのまま通す)。インターフェースのメソッド自身のシンボルは、これまでどおりの 1 つの site で、候補のシンボルが同じなら作らない(重複を避ける)
2. **`internal/taint/calls.go`:** 候補の site は、候補のシンボルの文脈で `checkSink` を通す(レシーバと引数のオフセットは動的な呼び出しのまま)。finding の sink の段に、`dynamic_dispatch`(全候補つき)を付ける。複数の候補の 1 つとして届く、という不確実性は、候補が 1 つのときも出る(インターフェース越しである以上、実装が増えれば変わる)。既存の `dynamic_dispatch` の記録も、そのまま残る
3. **候補が分からない(0 個)とき:** sink は適用しない。全候補が分からないので、どの実装に該当するか言えない。これまでどおり `dynamic_dispatch`(実装が見つからない)と `unmodeled_library_call` の不確実性の注記が出る。候補の数に上限はない(CHA / VTA がそのまま。`max_implementations` のような切り捨ては、今のコードにない)

**限界:**
- ソースの `CallSources` は、候補には適用しない(sink だけ。source は結果の汚染の出所なので、候補を決めてから別に決める)
- 呼び出し元が渡した型で候補を絞る `dispatch` のガード(ADR-031)は、候補の sink の site には付かない(過大側。見逃しは増えない)
- 候補ごとの sink の finding は、候補の 1 つが一致すれば出る。実際には別の実装が呼ばれる経路でも出る(不確実性で示す)

**検証:** `testdata/go/cases/ownif`(自前のインターフェース越しの `(*sql.DB).QueryContext`、候補が 2 つ(標準ライブラリと自前の別の実装)で不確実性が付く、sink の引数でない値、sink でない実装だけ)。zoo の `dyn/sink-behind-own-interface` の `vuln2`(`@miss` を外した)。Dart と Rust のセル(自前のインターフェースの実装の本体に sink がある形)と、デコイは、変わらない。

`compare.sh`(軽い層 + Gitea v1.26.4): 削除 0。追加は、zoo の `vuln2` の 1 件だけ(意図した変化)。gogs 2 つ、gin-vue-admin、go-test-bench、gophish、casdoor、dart-pub、parseable、Gitea は、追加 0(`unmodeled_library_call` が Gitea で 1 件増えただけ)。実際のコードでは、候補の実装に sink のルールがあるインターフェース越しの呼び出しが、これらのコードにはなかった、ということ。


