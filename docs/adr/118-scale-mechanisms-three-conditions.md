# ADR-118: 規模のために精度を落とす仕組みは、3 つの条件を満たすものだけにする

- 状態: 有効
- 関連: PRINCIPLES 2
- 要旨: 過大近似の向きだけ・順序とマシンに依らない・落とした所を出力が言う。既存の仕組みを 3 条件で分類した(まだ満たさないもの: 関数ごとの実時間、反復の上限、数値の停止、関数値の候補の上限、など)

**背景:** rivus は、大きなプロジェクトと依存を解析するために、精度を落とす仕組みをいくつか持つ(セルの widening、反復の上限、アクセスパスの深さ、関数ごとの実時間、候補の数の上限、など)。個々の仕組みは、ADR ごとに決めた。しかし、「どんな仕組みなら、規模のために精度を落としてよいか」の基準は、1 か所に無かった。この基準が無いと、新しい規模の対策(メモリの予算、SCC の粗い解き直し、依存の要約の再利用など)が、順序やマシンの負荷で結果が変わる形で入りうる。ADR-054 は、関数ごとの実時間で結果が負荷に依ることを、例外として明記している。

**決めたこと:**

1. 次の原則を置く(`docs/PRINCIPLES.md` 2 節に 1 段落)。

   **規模のために精度を落とす仕組みは、次の 3 つを満たすものだけ許す。**
   - (i) 精度を落とすのは、過大近似の向きだけ(流れを足す側。流れを消す側ではない)。
   - (ii) 落とすかどうかの決定が、処理の順序にも、計算機にも依らない(時間、負荷、メモリの空きで決めない)。入力だけで決まる量(要約の大きさなど)で決める。
   - (iii) 出力が、どこで落としたかを言う(`uncertainties`、`summary`、`stats` のどれかに、場所と理由が残る)。

2. この ADR では、**上限の値も、コードも変えない**。下の表は、今のコードを 3 つの条件で分類した記録。満たさない条件がある仕組みは、それぞれ別の ADR と PR で、満たす形に置き換える(置き換える前の今の動作は、ここに書いたとおり)。
3. 新しい仕組み(予算、粗い解き直し、要約の再利用など)を足すときは、その ADR に、3 つの条件を 1 行ずつ書く。

**表の読み方:** 「満たす」は、コードとコメントを読んで確認した。「満たさない」は、どの条件かを書く。「未確認」は、コードを読んだが、テストや測定では確認していない。報告の場所は、不確実性(`uncertainties`)の `kind` と `detail` の先頭の語、`stats`、`summary` のフィールド名。

| 仕組み | 今の規則(コードの場所) | (i) 過大近似の向きだけ | (ii) 順序・マシンに依らない | (iii) 出力が言う | 足りない条件 |
|---|---|---|---|---|---|
| 関数ごとの実時間(`func_timeout`) | 1 関数の解析が `-func-timeout`(既定 30 秒)を超える、または繰り返しの解析の合計がその 4 倍を超えると止める(`engine.go` の `spent`、`degFuncTimeout`)。そこまでの要約で先へ進む | 満たさない: 止まった関数の要約は、流れを落とす(文面が「its summary may miss flows」) | 満たさない: 時計なので、負荷で止まる所が変わる(ADR-054 の例外) | 満たす: `depth_limit`、detail `func_timeout:`、`stats.func_timeouts`、`summary.load_dependent` | (i)、(ii)。入力の大きさで決める規則に置き換える候補 |
| 全体の実時間(`-timeout`、`run_timeout`) | 既定 0(上限なし)。指定したとき、時計で止めて、見つけた分を出す | 満たさない: まだ解析していない関数は、結果にない | 満たさない: 時計 | 満たす: `depth_limit`、`run_timeouts`、`load_dependent`、「the time limit was reached」 | (i)、(ii)。利用者が指定したときだけ働く(既定では働かない) |
| 作業量の予算(`budget`) | 1 関数が追加する要素の数が `Budget`(既定 400000)を超えると止める(`DefaultOptions`、`degBudget`) | 満たさない: 止まった関数は、流れを落とす | 満たす: 要素の数だけで決まる(ADR-054) | 満たす: `depth_limit`、detail `budget:`、`stats.budget_exhausted` | (i) |
| セルの widening(1 セル 96 要素) | 1 つのセルの要素が 96 を超えると、同じパラメータの部分を表す要素を、パラメータ全体を表す 1 つにする(`state.go` の `maxCellElems`、`targetOf`) | 満たす: 全体は部分を含む(コメント: 真実より多く、少なくはならない) | 未確認: 96 を超える時点は、要素が入る順に依る。最終の要素が順序に依らず同じかは、確認していない | 満たす(関数の単位): `depth_limit`、detail `path_limit:`(切った関数の一覧)。どのセルかは言わない | 未確認(ii)。(iii)はセルの単位では言わない |
| 出力の widening(1 出力 24 要素) | 1 つの出力が 24 要素を超えると、同じパラメータの要素を全体を表す 1 つにまとめる(`summary.go` の `maxOutElems`、`widen`) | 満たす: 同上(コメント、ADR-014、ADR-031) | 未確認: 要素の集合に対して決まる。集合が反復の途中で順序に依って変わりうることは、ADR-057 の往復で見えた | 満たさない: `widen` を呼ぶ所に、記録を残す呼び出しを見つけられなかった(セルの widening の記録は、`state` が `widened` を立てたときだけ) | (iii)。未確認(ii) |
| `absorbWhole` | `widen` の前に、全体を表す要素があるとき、同じガード・同じ操作の一部を表す要素を落とす(`summary.go`、ADR-057) | 満たす: 意味は変わらない(精度を落とさない) | 満たす: 要素の集合だけで決まる | 精度を落とさないので、記録は要らない | なし(精度を落とす仕組みではない) |
| 反復の上限(passes) | 全体の見直しを `MaxPasses`(既定 12、`-max-passes`)で、SCC の中を 8 回で切る(`engine.go` の `for pass`、`for iter := 0; iter < 8`)。関数の中の局所の解析が収束しないときも切る | 満たさない: 不動点の手前で切ると、要約が流れを欠く | 回数は入力だけで決まる。切ったときの結果が、SCC の順に依らないかは、未確認(大きい SCC は、`-workers` に依らないよう、各回を一斉に公開する: `engine.go` のコメント #96) | 満たす: `recursion_approximation`(「did not stabilise within the pass limit」、「did not reach a fixed point」)、`stats.converged`、`unstable`、`unstable_count`、`not_converged` | (i)。未確認(ii) |
| ガードの上限(`guard_limit`) | 1 つの流れが依存するガードが `maxGuards`(4)を超えると、ガードなしで残す(`guards.go`、`calls.go`、`summary.go`) | 満たす: 「どの条件でも起こりうる」になる。真実より多い | 未確認: ガードの数だけで決まる。順序は確認していない | 満たす: `depth_limit`、detail `guard_limit:`(関数ごと) | 未確認(ii) |
| 経路の 200 段の上限(`maxPathSteps`。出力の中の名前は `path_steps_omitted`、`paths[].omitted_steps`) | 経路が 200 段を超えると、先頭と末尾の 100 段ずつを見せ、途中を省く(`engine.go`、`state.go` の `maxPathSteps`)。省いた段の不確実性、モデル、到達性は、集計に入る | 満たす: 見せる段を減らすだけで、流れを消さない | 満たす: 段の数だけで決まる | 満たす: `depth_limit`、detail `path_steps_omitted:`、`paths[].omitted_steps` | なし。注意: コードの中で `path_limit` と呼ぶのは、この上限ではなく、次の行のアクセスパスの切断の detail の語 |
| アクセスパスの深さ(`-k`、既定 3。`depth_limit`) | アクセスパスを深さ `k` で切り、切った所より下は、まるごと汚染として扱う(`state.go` の `k()`、`targetOf`) | 満たす: 切った所より下は、足す側にだけ働く | 満たす: 深さだけで決まる | 満たす(関数の単位): `depth_limit`、detail `path_limit:`(切った関数の一覧)、`summary.access_path_depth` | なし。切った段ごとの記録は、次の行 |
| `access_path_truncated` | 経路の、切ったパスを通った段に付ける uncertainty(ADR-117。切ったセルに入れた段、切ったパスから読み出した段) | (uncertainty なので、落とす仕組みではない) | 満たす: 切った事実で決まる | 上の行の (iii) を、段の単位で満たす仕組み | なし |
| `pointer_alias` | ポインタをセルに持つ書き込みや読み出しを、ポインタが保存された場所へのエイリアスとして扱う(`ptrcells.go` の `ptrAlias`、`pointerAliasDetail`) | 満たす: 「流れは存在しないかもしれない」と uncertainty で示す側(足す側) | 未確認: 保存の集合から作る。順序は確認していない | 満たす: `pointer_alias`、経路の段ごと | 未確認(ii) |
| `assumed_call` | 作った所で見えない呼び出し(別の関数に渡す、保存する、返す)のクロージャを、作った所で呼んだものとして、捕捉したものを適用する(ADR-059、`callrows.go`、`bodyreplaced.go`) | 満たす: 呼ばれたものとして足す側 | 未確認: 呼び出しの形だけで決まる。順序は確認していない | 満たす: `assumed_call`(`attributable_to: mixed`) | 未確認(ii) |
| 関数値の候補の上限(32、ADR-050) | 呼び出しの候補が 32 を超えると、候補を 1 つも付けない(`ir.ValueEnumerateMost`、`internal/ir/ir.go`。数えて切るのは `internal/valuecalls/valuecalls.go`)。呼び出し元をたどる深さの上限(3)で切れて、候補が 1 つも見つからないときも、候補を付けない(`caller_depth_limit`)。1 つ以上見つかった途中で切れたときは、見つかった候補を残す(`partial`) | 満たさない: 候補なしの呼び出しは、コールバックの中の流れを落とす(#77。33 種類以上の形が見逃しのまま) | 満たす: 候補の数・深さだけで決まる | 一部: `depth_limit`、detail `candidate_limit:`(`internal/taint/summary.go`、`internal/taint/funccells.go`、`internal/taint/calls.go`)、`caller_depth_limit:`(文面は `internal/ir/ir.go` の `ValueLimitDetail`)。`candidate_limit_calls` という名前の stat は、コードに無い。3 段で切れて、途中で 1 個以上見つかったときは、`unresolved_callee`、detail `partial:`(理由つき)を付ける(`internal/taint/calls.go`、`ir.ValuePartial`)。渡した側の関数の uncertainty は、#77 に残っている | (i)、(iii)の一部 |
| 数値の停止(`numeric stop`、`-numeric-taint` で追う) | 数値と真偽値の型に入るとき、source のデータを追わない(`state.go` の `numericRoot`、`add`、`noteNumericStop`) | 満たさない: 数値の型を通る流れを、設計として消す(「注入を運べない」という前提) | 満たす: 型の分類だけで決まる | 一部: `numeric_stop`(source ごとに 1 件、最初の場所)。サマリを通って止まる分は記録しない(PRINCIPLES 2 節の「現状」)。関数ごとの報告は PR #312 | (i)、(iii)の一部 |
| `-no-deps` | 依存のソースを読まない。依存への呼び出しは、すべて推測になる(`rivus analyze -h`) | 未確認: モデルのない呼び出しの扱い(`unmodeled_library_call`)が足す側かは、ここでは確認していない | 満たす: フラグ。利用者が決める | 一部: 呼び出しごとに `unmodeled_library_call`。フラグ自体は、出力に記録されない(PRINCIPLES 6 節の「現状」) | 未確認(i)、(iii)の一部。既定では働かない(依存は既定で読む) |
| 依存の深さ(`-dep-k`) | 依存の関数だけ、アクセスパスの深さを `n` にする(`state.go` の `k()`)。ADR-118 を書いた時点の既定は 0(`-k` と同じ) | 満たす: 切った所より下は足す側(ADR-117) | 満たす: 深さだけで決まる | 満たす: `access_path_truncated`、`summary.access_path_depth`(`{project, dependency}`) | 既定は 2(ADR-119)。機能の一覧には入れない(調整の指定。`cmd/rivus/flagsides.go` の `flagInfo`) |
| `body_replaced_packages`(`-body-replaced`) | 宣言したパッケージの本体を読まず、モデルが代わりに言う(`bodyreplaced.go`) | 未確認: モデルの行が本体より少ないとき、流れが落ちる。モデルのない呼び出しは `unmodeled_library_call` | 満たす: 宣言で決まる | 満たす: `summary.body_replaced_packages`(理由つき)、`body_replaced_ignored` | 未確認(i) |

**表からの、満たさないものの一覧:**

- (ii) を満たさない: 関数ごとの実時間、全体の実時間(`-timeout` を指定したとき)。どちらも、規則を入力の大きさで決める形に置き換える候補(作業量の予算 `budget` は、すでにその形)。
- (i) を満たさない: 上の 2 つ、作業量の予算、反復の上限、関数値の候補の上限、数値の停止。このうち予算と反復の上限は、止める所を粗くする(流れを足す側で止める)ことで満たせるかを、別に検討する。
- (iii) を満たさない・一部: 出力の widening(24 要素)、関数値の候補の上限(3 段で切れた途中の候補)、数値の停止(サマリを通る分)、`-no-deps`(フラグ自体を出力に記録しない)。
- 未確認: セル・出力の widening、反復の上限、ガードの上限、`pointer_alias`、`assumed_call` の (ii)(順序を入れ替えても結果が同じことのテストが無い)。

**変わらないもの:** 上限の値、既定、finding の集合、出力のフィールド。

**帰結:** 規模の対策は、この 3 条件で評価できる。予算を単位にするときは、入力だけで決まる単位(要約の大きさなど)で書き、メモリの空きを見て動かない。表の「満たさない」は、置き換えの課題として、`docs/issues/` に 1 件ずつ起こす(この PR では起こさない)。

## 追記(2026-11-23): 出力の widening の、段の単位の uncertainty

出力の widening(24 要素)の (iii) を、段の単位でも満たした(課題 #186 の残り)。

- **付ける所**: 要約を適用する段で、畳んだ出力(`Output.Widened`)の、パラメータ全体になった要素(`Param` で、パスが空)から作る段。ステップの uncertainty として付く。種類は `depth_limit`、detail は `out_widened:` で始まる(`out_limit:` の記録と同じ種類に合わせた。ADR-117 の `access_path_truncated` のような、専用の種類は足さない)。経路の `steps[].notes` に出る。
- **要素の同一性、合成、署名には触れない**: `OutElem`、`Elem`、署名、`joinSummaries` は変えない。#332 で足した `Output.Widened` を読むだけ。畳んだ関数を呼ぶ関数の要約には、`Widened` を伝えない(ステップの uncertainty は `Steps` と一緒に呼び出し元へ届く)。
- **`confidence` は変わる**: uncertainty は段に付き、その段を通る finding の `uncertainties` にも入る(`access_path_truncated` と同じ扱い。違いは、`access_path_truncated` が 1 つの finding に 10 件までで打ち切るのに対し、こちらは位置ごとに 1 件で、打ち切らないこと)。このため、`no_uncertainty_on_path` だった finding が `requires_review` になることがある。段に uncertainty があるのに `no_uncertainty_on_path` のまま、という矛盾を避けるため。ADR-117 の失敗(ほぼ全部に付く)は、起こりにくい: memos の一時の測定で、174 件中 12 件(約 7%)。手元の対象(go-curated、rivus 自身、合成の `all`、`middleware -stages 18`、`widen`、`types`)では、finding の ID と `stats` は変わらず、`confidence` が変わるのは、畳んだ出力を通るだけの小さい例 1 件(他に不確実性がない finding)だった。
- **要約の大きさ**: 1 回の呼び出しで、uncertainty を 1 つだけ作り、その呼び出しの段が共有する。合成の `all` の、解析の終わりの生きているヒープと割り当ての数は、前後で 0.1% 以内(実行ごとのゆれの範囲)。
- **精度**: もとからパラメータ全体だった要素(畳んでいない)にも、畳んだ出力の中なら付く(過大に付く)。畳んだ要素だけに付けるには、`OutElem` にマークを持たせる必要がある(要素の同一性に近い所に触れるので、していない)。
- **順序に依らないこと(ii)**: 付く段と付かない段の集合が、宣言の順、ファイルの順、`-workers` に依らないことを、テスト(`TestWidenedOutputStepNoteOrder`)で確かめた。

## 追記(2026-11-24): 値を選ぶための測定の入口(作業量の出力)

時計を作業量の規則に置き換える前に、作業量の上限の値を選ぶ材料として、`stats` に関数ごとの `adds`(集合に入った要素の数。`Budget` と同じ数え方)の分布を出すようにした: `adds_total`、`adds_max_single_run`、`adds_max_function_total`、`adds_per_function`(関数ごとの合計を 1e3、1e4、1e5、1e6 で分けた 5 行。各行に関数の数と `adds` の合計)。関数名は出ない。数えるだけで、止める条件も、結果も、finding も変えない(`adds` はすでに数えていて、`commit` で関数ごとに足すだけ)。値は入力で決まり、`-workers` に依らない(時計で止まった関数だけは、止まるまでの値)。選ぶ対象と確かめる対象を分ける(`docs/PRINCIPLES.md`)ため、上限は選ぶ対象の分布から決め、別の対象で確かめる。
