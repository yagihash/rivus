# 設計: クロージャの捕獲の値を、作った場所で当てる(C-7)

状態: **下書き。ユーザーの承認待ち**(承認されたら、DECISIONS.md に ADR(番号は次の空き)としてまとめて、実装に入る)。コアはまだ変えていない。ここに書いた「今の状態」は、2026-11-21 の main(HEAD 52e2ba8)のコードを読んで、curated のフィクスチャを流して確かめたもの。実装して確かめた数字ではないものは、「見込み」と書く。

## 問題

捕獲した値が、**ヘルパーが呼ぶクロージャの中で**使われると、流れが完全に黙って落ちる(Go でも Dart でも)。

```go
func run(f func()) { f() }
func main() {
	a := os.Getenv("A")                     // source
	run(func() { exec.Command(a).Run() })   // 見つからない。不確実性も 0
}
```

同じファイルの、クロージャを引数で受ける形(`runArg(a, func(s){…})`)、その場で呼ぶ形、`go func(){…}()`、`defer func(){…}()` は見つかる。捕獲した値を、**別の関数が呼ぶ**クロージャの中で使う形だけが落ちる。Gitea のサービスはほぼ全部この形(`db.WithTx(ctx, func(ctx) error { …捕獲した値… })`)。Dart の `Button(onPressed: () => sink(t))`、`Timer.run`、`run(() => sink(a))`(CAP 12 件、DART-15 / DART-16)も同じ。

### なぜ落ちるか(コードの上で)

1. 束縛(クロージャが捕獲した値 = `make_closure` の引数)を呼び出しに付けるのは、**クロージャを作った関数 `home` の中の呼び出し**だけ。`internal/valuecalls/valuecalls.go:319` `funcsOf` の `if f == home && len(in.Args) > 0 && cur.bind == nil`。ヘルパー `run` の中の `f()` は、`home` が `run` で、クロージャを作った `main` ではないので、束縛が付かない(候補にはなる)。
2. `internal/sg/build.go:362` `resolveValueCalls`: `c.Bind != nil` のときだけ `CandidateBindings` を作る。
3. `internal/taint/summary.go:283-286` `applySummary`: 束縛が無いと、クロージャの FreeVars を `KUndef`(何も運ばない値)で**黙って**埋める。不確実性を出さない。

つまり、クロージャの要約(「FreeVar → sink」「FreeVar への書き込み」)を、捕獲した値で具体化できるのは、**作った場所だけ**なのに、作った場所では、呼ぶところが見えないので、要約を使わない。

## 規則(案)

**`make_closure`(Go と Dart)の命令で、クロージャの要約を、捕獲の値(`make_closure` の引数)で具体化する。クロージャの宣言した引数(Params)は「不明」(汚染なし、`KUndef`)にする。** これで、クロージャの中の「捕獲した値 → sink」と「捕獲した変数への書き込み」が、**どこへ渡したかに関係なく**、作った関数に届く。

### (1) 当てる場所と、当てるもの

場所: taint の `transfer`(`internal/taint/transfer.go` の op の switch、`case ir.OpCall:` の隣に `case ir.OpMakeClosure:`)。`Args` が空でない `make_closure` だけ(空のものは `KFunc` の値で、捕獲が無い。`sg/build.go:193`)。

当てるもの(`applySummary` の出力の種類のうち):

| 出力 | 当てる | 行き先 |
| --- | --- | --- |
| sink への到達(`sum.Sinks`) | **当てる** | 捕獲した値の汚染が sink に届けば finding。捕獲した値が、作った関数のパラメータなら、記号的な到達(`noteSinkReach`)になり、呼び元に伝わる(ヘルパーを返すクロージャ `mkRun(s)` が効く) |
| 捕獲した変数への書き込み(`OutParam`、`argv[o.Index]` = 捕獲した値の指す先) | **当てる** | 捕獲した変数のセル(`out = a`)。`Go` の `var out string` と、Dart / Rust の変数の書き込み |
| グローバルへの書き込み(`OutGlobal`) | **当てる** | グローバルのセル |
| throw(`OutThrow`) | **当てない。ただし黙って落とさない** | throw は、呼ばれたところから抜ける。`make_closure` の場所に流すと、作った関数が throw することになり、過大。`try { run(() => throw x) } catch` は、見逃す。クロージャの要約に、**捕獲の値に依る throw の出力があるとき**は、(4)の不確実性を出す条件に含める(「当てなかった」ことが残る。detail に、throw は当てていないと書く) |
| 戻り値(`OutResult`) | **当てない。ただし黙って落とさない**(throw と同じ) | クロージャの結果は、呼ぶところで受け取るもの。`make_closure` の場所に、呼ぶところは無い(行き先が無い)。`cc.roots = nil`。**捕獲の値に依る `OutResult` があるときは、(4)の不確実性を出す条件に含め、detail に「結果は当てていない」と書く**。この型が、C-7 のあとも落ちる: `produce(func() string { return a })`(ヘルパーの `f()` は束縛なしで当たり、捕獲の値は `KUndef`)、Gitea の `db.WithTx2(ctx, func(ctx) (T, error) { return load(捕獲した値) })`(34 か所)、Dart の `items.map((x) => x + a)`、`Future.then((_) => a)`、MISSES の RET-CLOSURE(`holder-a/b`、`curry`、`getter-fn`)。次の段(末尾)で当てる |

実装は、合成した `callCtx`(`in` は `make_closure` の命令、`recv=nil`、`args` は Params と同じ数の `KUndef`、`roots=nil`)と `applySummary(cc, closureFn, mc.Args, notes, nil)`。`applySummary` の `argv = cc.argv() + bindings`(Params の後ろに FreeVars)は、そのまま使える。**`summary.go` の中は変えなくてよい**(`len(bindings) == 0` のときだけ不足を `KUndef` で埋める分岐は、束縛を渡すので通らない。Params を呼び出し側で埋めるので、不一致のエラーにならない)。

### (2) 当てないもの

* **戻り値**: クロージャの結果は、呼ぶところで受け取るもの。`make_closure` の場所に、呼ぶところは無い(行き先が無い)。`cc.roots = nil` にして、`OutResult` の `o.Index < len(cc.roots)` を偽にする。
* **引数に依る流れ**: クロージャの引数(Params)から sink への流れ(`runArg(a, func(s){ sink(s) })`)は、呼ぶ側の値(ヘルパーが渡す)に依る。`KUndef` なので何も運ばない。これは、今のとおり、呼ぶ側(ヘルパー内の `f(x)` の候補)で見つかる。**C-7 は、捕獲した値だけに依る流れを足す。引数に依る流れは足さない**(これは案 2 = ADR-047 の仕事)。
* 捕獲した値と引数の**両方に依る**流れ(`sink(x + a)`): 要素は 1 起点ずつなので、`a` の分は C-7 で、`x` の分は呼ぶ側で、それぞれ見つかる。

### (3) 飛ばす条件と、二重になったとき

**飛ばす条件: クロージャの値の使われ方が、作った関数の中での直接の呼び出し(その場で呼ぶ、`go`、`defer`)だけのとき。** それ以外の使い方(引数に渡す、代入、フィールドやグローバルに入れる、return、φ)が **1 つでも**あれば、当てる。作った関数の SSA の使用箇所(`make_closure` の結果の `Value` の使われ方)だけで決まり、**ほかの解析の結果(`valuecalls` の候補、上限 8、解決のされ方)に依らない**。

「候補として解決済み」は、飛ばす理由にならない。今の見逃し(CAP)は、まさに、**候補としては解決済みで、捕獲の値が付いていない**形(`run(func() { sink(a) })`: `run` の中の `f()` の候補に、このクロージャは入っている)。候補の有無で決めると、候補の上限や解決のされ方で、当てたり当てなかったりする。
前の版の `sg.Instr.Seen`(「束縛つきで呼ばれた」印)は、**取り下げ**。使用箇所で決める。

* 実装: taint の `captured` が、`make_closure` の結果の使用箇所を見る。`in.Dsts[0]` の `Value` から、それを引数(`Args`)に持つ命令を、コピーと φ を通って辿り、すべてが「呼び出しの callee の位置(`Callee.Kind == CalleeValue` で、関数値が `Args[0]`、または、`Static` / `Bindings` が自分)」か、`go` / `defer` のフラグつきの呼び出しなら、飛ばす。1 つでも、引数の位置・`store`・`field_store`・`global_store`・return・φ(その先が上の条件を満たさない)・**別のクロージャの `make_closure` の引数の位置(このクロージャを、内のクロージャが捕獲する。内のクロージャが呼んでも、ここからは見えない)**なら、当てる。使用箇所の表を `sg` の構築時に作る(`Value` ごとの使用の一覧が既にあるなら、それを使う。無ければ、`sg` に作る。**コアの変更**)。
* 検証の基準: **今の finding の経路が 1 件も変わらない**。その場で呼ぶだけのクロージャは、当てないので、経路が変わらない。引数で値を渡す形(`runArg(a, func(s){…})`)は、捕獲に依る効果が無ければ、当てても何も足さない。**変わりうるのは、「捕獲もあり、候補としても束縛つきで解決されている」形**(`home` の中の関数値の呼び出しの候補になっていて、かつ、別の使い方もある)。その件数を、前後の差分で数える。
* `cap-seen`(その場で呼び、かつ、渡す): 渡す使い方があるので、当てる。finding は `(origin, sink)` で 1 件にまとまる(`recordFinding` の `findingKey`、`engine.collect` の `merged[k]`)。代表の経路は、ステップ数の少ないほう(`<` なので、同じ長さなら先に記録したほう)。`make_closure` の経路は、ヘルパーを通らないので、たいてい短く、代表になる(経路が変わりうるのは、この形)。`mergeParamSources` は、source が param の finding だけの別の併合で、関係しない。

### (4) 痕跡(不確実性)

呼ぶところが見えないクロージャに、捕獲の値を当てたことを、不確実性で残す。

* kind: **新しい kind `assumed_call`**(`attributable_to: mixed`)。`unresolved_callee` は使わない。理由: `unresolved_callee` の意味は「呼び先が分からない」で、ここで分からないのは、**呼ぶ側**(このクロージャを、誰が、いつ、どの引数で呼ぶか)。`unresolved_callee` は「候補を増やせば直る」、こちらは「メモリを辿る、モデルを足す、で直る」で、直し方が違う。同じ kind にすると、kind ごとの集計と `attributable_to` の指標が濁る。LLM の判定にも、「この finding は、クロージャが呼ばれると仮定した結果」を、finding ごとに見分けさせたい。名前は `assumed_call`(「呼ばれると仮定した」。代案: `closure_call_assumed`。短いほうにした)。
* **閉じた集合に 1 つ足す**: kind の一覧は閉じた集合なので、**ADR-008 の追記が要る**(`result.go` の定数、`attributable()`、出力のスキーマ、`docs` の kind の表、`cases_test.go` の kind ごとの期待)。
* 文面(`detail`): `the closure is made here and called where it cannot be seen (it is given to another function, or stored): what it captured is applied at the place it was made, and it is taken to be called`。日本語の意味: このクロージャが呼ばれるところは見えていない。作った場所で、捕獲の値を当てた(呼ばれるものとして扱っている)。throw を当てなかったときは、`; what it throws is not applied` を足す。
* `symbol` は、クロージャの関数 ID。`position` は `make_closure` の位置(`graph` の `refs` の `closure` と同じ)。`func` は作った関数。
* **出す条件**: クロージャの要約が、捕獲した値に依る効果(FreeVar 由来の sink への到達、書き込み、**throw の出力**、**戻り値(`OutResult`)の出力**)を**持つ**ときだけ。当てなかったもの(throw、結果)は、detail に `; what it throws is not applied` / `; what it returns is not applied` と書く。毎回出すと、Gitea で数千件になる(捕獲のあるクロージャ 1,496)。`sum.Sinks` と `sum.Outputs` の要素が FreeVar の `ParamElem` を含むか、で決まる(`applySummary` の前に調べる)。
* finding の経路: `applySummary` の `notes` 引数に渡す。ステップ(`call.Notes`)と `w.setNotes` に載り、finding の `uncertainties` に出る。重複は、`engine.collect` が `kind|symbol|func|pos` で 1 つにする。
* **`confidence`**: この規則で見つかった finding の `confidence` が、**`no_uncertainty_on_path` にならない**こと(経路に `assumed_call` があるので)。**今の main では、規則が無いので、確かめられない**。期待として、実装後に、`cap-run-a`(go-curated)の finding の `confidence` と `uncertainties` に `assumed_call` が出ること、を受け入れの条件にする(`bench_curated_test.go` に、`@assumed` のような注釈で確かめるか、`cmd/rivus` のテストで、`cap-run-a` を `analyze` して JSON を見る。実装のときに決める)。

### (5) 失うもの・増えるもの

* **増える**: 見えないクロージャの中の、捕獲した値 → sink の流れ。**1 度も呼ばれないクロージャの中の流れも、finding になる**(過検出)。呼ぶ順序も見ない(クロージャが、捕獲した変数を書く前に呼ばれる、など)。`go-curated/api/capture.go` の `capNever` が、その形(今は静か。`@sink(none)` が、実装後に落ちる。受け入れの基準は、**`capNever` を `@fp` にするか、規則の限界として受け入れるか**を、ユーザーが決める)。
* **失うもの**: 無い見込み(当てるものは足すだけで、`summary.go` の既存の動作は変えない)。ただし、経路が変わる finding がある(上の(3)、見えているクロージャを飛ばせば無い)。
* **増える量の見積り**(Gitea、構文の上の近似、下の「測定」): 捕獲のあるクロージャ 1,496 のうち、その場で呼ばれない(引数、代入、return、リテラル)ものが 1,334(89%)。そのうち、`.Group`(ルーターの登録)413、`.WithTx` 189 + `.WithTx2` 34。ルーターの `Group(path, func(){ m.Get(...) })` の中身は、呼ばれるところが見えている(ライブラリが即座に呼ぶ)ので、過検出にならないが、`WithTx` 以外にも多い。
* 計算量: 捕獲のあるクロージャ 1 つにつき、`applySummary` 1 回(`callSig` で、変化の無い繰り返しは飛ばす)。Gitea で約 1,300 回。要約が大きいクロージャで、時間が増える可能性は、実装して測る(精度を優先する方針: 結果が正しいことが先)。

### (6) 入れ子のクロージャと、ループの中のクロージャ

* **入れ子**(外のクロージャの捕獲を、内のクロージャが捕獲する): 内の `make_closure` は、外のクロージャの本体の中にあり、その引数は、外のクロージャの FreeVar。外のクロージャの要約を作るとき、内の `make_closure` を当てると、FreeVar に依る記号的な sink 到達(`noteSinkReach`)になる。外の `make_closure`(作った関数 `home`)で、それが具体化される。**要約の合成で、段数に依らず届く**。`capNested`(go-curated)がそのフィクスチャ。外のクロージャが見えている(その場で呼ぶ)場合に、内のクロージャが見えていないときは、内を当てる(印は、命令ごと)。
* 届く順序(追記): 内側のクロージャの効果は、**外側のクロージャの要約に、外側の捕獲の言葉(外側の FreeVar)で入る**。作った関数が外側を当てる、の順で届く(内側は、外側の本体の中の `make_closure` として、外側の要約を作るときに当たる)。内側の使われ方が「直接の呼び出しだけ」なら、飛ばす(外側の本体の中で、その場で呼ばれる)。`cap-nested` で確かめる(今は `@miss(silent)`)。
* **ループの中のクロージャ**: `make_closure` の命令は 1 つ。解析は流れに依らない(セルの和)ので、ループの何周目かを区別しない。捕獲した変数がループ変数(Go 1.22 以降は周ごと)でも、セルは 1 つで、届く。`capLoop` がそのフィクスチャ。ループで変数を書き換える形(`x += c.Query(..)`)は、セルの和で届く(過大側)。

### (7) Rust と Dart

**Dart**: `make_closure` に捕獲の値が付く(確かめた: `run_a` の IR は `make_closure [1] package:curated/pass_closure.dart#run_a$1`、クロージャ側は `param` の命令の `index` が宣言した引数の数以降なら FreeVar、Go と同じ規則。`sg/build.go:180-190`)。**そのまま同じ実装で足りる**(`Button(onPressed: …)`、`Timer.run`、`scheduleMicrotask`、`Future.delayed` は、全部、クロージャの作られる場所が同じ形)。フィクスチャ: `dart-curated/lib/callbacks.dart`(7 件が今 `@miss`)。

**Rust**: 捕獲のあるクロージャは **`make_closure` ではない**。確かめた(`rivus-rust` の出力の IR、`write_a`): 環境の構造体を、`alloc`(class `func`、type `{closure@src/capture.rs:9:9: 9:11}`)で作り、捕獲した値を `field_store`(field `"0"`, `"1"`…。可変の捕獲は、変数へのポインタ)で入れる。クロージャの関数は、**パラメータが 1 つ(その環境、`recv: true`)**で、`field_load` で取り出す。呼び出しは、`Fn::call` の dynamic 呼び出し(候補は、同じ引数の型を持つ全クロージャ = XTALK)。

* **できるか**: 同じことは、**別の段**で、できる。環境の `alloc` の type(`{closure@file:l:c}`)と、クロージャの関数の `params[0].type` が一致するので、alloc → クロージャの関数が引ける(extractor は変えなくてよい見込み。型の文字列が、ファイルと位置を含むので、一意)。`applySummary(cc, closureFn, nil, notes, nil)` を、`argv = [環境の値]`(束縛ではなく、Params の 1 つ目)で当てる。ただし、環境の `field_store` が `alloc` の後に来る(流れに依らない解析なので、順序は、問題にならないはずだが、**確かめていない**)。
* **効く範囲**: 読む側(`run(|| sink(&a))`)は、`Fn::call` の候補で、**今でも見つかる**(XTALK の過検出つき。`run-a/b` は `@fp`)。C-7 は、これを直さない(XTALK は別の原因。案 2 の仕事)。`rust-curated/src/callbacks.rs` の 3 件(`returned`、`nested_cap`、`param_clean`)と `never_called` の静かな側は、**今、見つかっている**(`@miss` が無い)。書く側(`write-a/b`、`push-list` など 7 件、ENV-PTR)は、環境のフィールドのポインタが、指す先と別名にならない(`locs.go:215`、`transfer.go:148`)ことが原因なので、**alloc で当てても直らない**。ENV-PTR の修正が先。
* 結論(**条件つき**): 前の版は「読む側は、今でも見つかる」を理由に、Rust を最初の版の対象外にしたが、これは**呼ぶ側が見える形**(`Fn::call` の候補になる)でしか確かめていなかった。**渡すだけの形**で確かめた(`rust-curated/src/callbacks.rs`、今の main): `std::thread::spawn(move || sink(&t))`(`@miss(unmodeled_library_call)`)、捕獲のあるクロージャを `Box<dyn Fn(i16, u128)>` にして構造体のフィールドに入れ、別の関数が呼ぶ(`@miss(silent)`)、モデルのない標準ライブラリの関数に渡す(`Vec::retain`、`@miss(silent)`)の **3 件とも見つからない**。つまり、Rust にも、黙った見逃しがある。**Rust の環境の `alloc` で当てる段(上の「できるか」)は、次の段に入れる**(最初の版は Go と Dart。ENV-PTR の修正は、書く側 7 件で、別)。最初の版を Go と Dart にするのは、実装の大きさ(extractor の type で引く、環境の `field_store` の順序の確認)を分けるためで、「Rust は見つかるから要らない」ではない。`thread::spawn` と `retain` のクロージャは、他の呼び出しの候補にもなり、他のソースが誤って届く(`@noisy` 8 と 3)。

### (8) 案 2(ADR-047 の下書き: scratchpad/scc/adr-draft.md)との関係

どちらも、「**作った関数での束縛**」(`make_closure` の引数)を使う。案 2 の「呼び元 K が、自分で渡したクロージャ(`ExactFuncs`)の要約で、ヘルパーの記号的な出力を具体化する」には、クロージャの要約に**束縛も**必要(下書きに書いていない。C-7 と同じ情報)。そこで:

* `sg.ExactFuncs` を、ID の集合だけでなく、**`make_closure` の命令(束縛)も返す**ようにする(`ExactClosures`)。C-7 は、これを使わない(`make_closure` の命令を直接見る)が、案 2 が使うので、同じ補助関数にしてよい。
* **分担**: C-7 = **捕獲の値だけに依る流れ**(作った場所で当てる)。案 2 = **引数に依る流れ**(ヘルパーが渡す値。呼ぶ側のコンテキストで具体化)。**両方に依る流れ**(`fireAndForget(t, (x) { out = x; })` の `out` に、`t` が届く: 捕獲した `out` への書き込みの値が、ヘルパーが渡す引数 `x`)は、案 2 が入るまで**見つからない**(`write-param`(Dart、Rust)で記録している)。クロージャの要約の要素を、FreeVar 由来か Params 由来かで分ければ、二重にならない: 案 2 の K での具体化は、Params 由来の要素だけを当て、FreeVar 由来は C-7 に任せる。案 2 が先に入った場合も、C-7 の「使われ方」の条件は変わらない(K の束縛は、C-7 の条件に影響しない)。
* 順序: **C-7 は案 2 に依らない**。案 2 は SCC の構造(要約のデータ型、`applySummary` の展開)を変える大きな作業。C-7 は、小さく、独立に入れられる。案 2 を待たない。
* **再解析の辺**: 作った関数(`home`)は、クロージャの要約に依存する。クロージャがどの呼び出しの候補でもないとき、今は、その依存の辺が無い(`callers` は、呼び出しの候補から作る)。**辺を足す**(クロージャ → `home`: クロージャの要約が変わったら、`home` を再解析する)。クロージャが、作った関数を(直接でも間接でも)呼ぶ形(再帰: クロージャの中から `home` を呼ぶ、入れ子のトランザクション)では、**この辺で SCC ができる。それは正しい**(要約が互いに依存している)。案 2 の SCC の判定(ヘルパー → クロージャの辺を外す)とは別の辺で、C-7 は `order` / `levelsOf` を、この辺の分だけ変える。`engine.go` の依存の作り方を、実装のときに読む。

### (9) 実装の場所の見立て

| 変えるところ | 内容 |
| --- | --- |
| `internal/sg`(使用箇所) | `make_closure` の結果の `Value` の**使われ方の一覧**(callee の位置か、`go` / `defer` の呼び出しだけか、それ以外か)を作る(`Seen` の印は取り下げ。`sg` に使用の一覧が無ければ作る = **コアの変更**)。候補、`valuecalls` の結果には依らない |
| `internal/sg/build.go:84-90`(静的な束縛)、`:362` `resolveValueCalls` | **変えない**(前の版は、ここで印を付ける案だった) |
| `internal/taint/transfer.go:236` の switch | `case ir.OpMakeClosure:` を足し、`s.captured(in)` を呼ぶ |
| `internal/taint/calls.go`(新しい関数 `captured`) | `in.Args` が空、使われ方が「直接の呼び出しだけ」、クロージャの `sg.Func` が無い(`ByID`)、要約に FreeVar 由来が無い、のどれかなら何もしない。それ以外は、`callCtx{in: in, args: Params 数の KUndef, roots: nil, sym: ""}` で `applySummary(cc, fn, in.Args, notes, nil)`。`notes` は、(4)の不確実性 |
| `internal/taint/engine.go`(依存) | 要約の依存の辺(クロージャ → home)を足す(上の(8)の再解析の辺) |
| `internal/taint/summary.go:283-286` | **不確実性を出す(任意ではない)**。束縛が無い非 home の呼び出しで、FreeVars を `KUndef` で埋めるときに、不確実性を出す。C-7 のあとも、Rust(最初の版の対象外)と、戻り値(上の(1))で、埋める場面が残る。黙って埋める場所を残さない。文面は `assumed_call` ではなく、**「このクロージャが捕獲した値は、ここでは分からない」**(DART-15 の痕跡の修正)。**C-2 の項目 2(別のエージェント a6ede9c0341ce3c28 が実装中)と同じ**なので、重複させない(C-2 が入ってから、C-7 は、その痕跡の上に載る) |
| `internal/valuecalls/valuecalls.go:319` | **変えない**。`ExactClosures`(案 2)は、ここではなく `sg.ExactFuncs`(`sg/build.go:393`)の拡張 |
| `internal/graphdb/write.go:261` | **変えない**(`refs` の `closure` は、構造の層。taint の変更と無関係) |
| `docs/DECISIONS.md` | ADR(次の空き番号)。ADR-045 の限界(「パラメータ由来の候補は、呼び出し側ごと」の下の、束縛が home でだけ付く点)に、「捕獲した値は C-7 が当てる」と追記 |

`rivus resolve` の「見つからない理由」(source / sink の関数にある不確実性): C-7 の不確実性は、作った関数(`home` = source の関数のことが多い)に付くので、**流れが見つかっていれば**、`resolve` に出る必要は無い(見つかる)。見つからないとき(捕獲した値が、sink に届かないとき)は、効果を持つクロージャが無いので、出さない。

### (10) 検証の計画

0. **最初に**: 収束する 4 つ(gogs、gin-vue-admin、ech0、go-test-bench)を、走らせられる状態にする。C-1 では、scratchpad の複製に go.mod が無く、走らせられなかった。`bench/README.md` の取得の手順で、複製を作り直し、**C-7 の前**の finding を基準として保存する。
1. **網羅の不変条件**(`internal/taint` の既存のもの): finding の増加が、`make_closure` の経路の finding だけ。使われ方で飛ばす版は、**既存の finding が 1 件も消えず、経路も変わらない**ことを、全ベンチで確かめる。変わりうる形(「捕獲もあり、候補としても束縛つきで解決されている」)は、差分で数える。
2. **ベンチの差分**: 収束する 4 つ(gogs、gin-vue-admin、ech0、go-test-bench)、`bench/go-curated`、`bench/go-synth`(S、M)、`testdata/go/cases`、Dart(`dart-curated`、`dart-pub`、`dart-corpus` 系)、Rust(変更なしのはず。finding が 1 件も変わらないことを確かめる)。**前後で findings を比べ、増えたもの全部を 1 件ずつ分類**(正しい流れ / 呼ばれないクロージャの過検出 / 順序の過検出)。Gitea は、C-1(ADR-048)のあとは、収束する見込み(競合なしの再実行を待つ)。収束するなら、**Gitea 全体で前後を比べる**(`./modules/...` と `services/` の一部は、その予備。`db.WithTx` を含む)。収束しなければ、予備で比べる。分類できなければ push しない。
3. **curated の `@miss` / `@fp` の増減**(下の表。**実装して確かめたものではない**):

| ベンチ | `@miss` の今 | C-7 で直る(見込み) | 備考 |
| --- | --- | --- | --- |
| go-curated | 10(うち 9 が `silent`) | 9: `cap-run-a/b`、`cap-write`、`cap-nested`、`cap-loop`、`cap-two-a`、`cap-returned`、`tx_capture`(`dynamic_dispatch`)、`tx_capture_plain` | 残る 1 は `options`(`RET-CLOSURE`: クロージャの**引数**への書き込み) |
| dart-curated | 41(うち 31 が `silent`) | `callbacks.dart` の 7 件 + `CAP` のうち、捕獲した値だけで足りるもの(`run-a/b`、`write-a/b`、`add-list`、`nest-a/b`、`across-cap`、`builder`、`bound-prefix` など約 10)。`write-param`、`twice-args` は、引数に依る(案 2 の仕事) | |
| rust-curated | 13(全部 `silent`) | 0 | (7)のとおり、対象外。`callbacks.rs` の 4 つは、今、見つかっている |

   * `@sink(none)` が増える側: `capNever`(go-curated)と、`never_called`(dart-curated)の `@sink(none)`、`rust-curated/callbacks.rs:never_called`(Rust は対象外なので変わらない)。**この 2 つが落ちる**。落ちたら、`@fp` に直すか(限界として記録)、規則を変えるか、決める。
   * `@miss(silent)` の割合: Go 9/10(今)→ 0/1、Dart 31/41 → 約 20/30(見込み。silent のうち CAP 系が消える)。
4. **go test ./...** と、`RIVUS_DART` / `RIVUS_RUST` つきの curated(Dart・Rust の extractor のテスト)を通す。
5. **不確実性の数**: 前後で、`assumed_call` が、効果を持つクロージャの数だけ増える。他の kind は変わらない。
6. **注釈に依らない不変条件**(共通の検査に入れる): 「経路か finding に `assumed_call` がある finding は、`confidence` が `no_uncertainty_on_path` でない」を、BENCH-2(`offsource_test.go` の共通の検査)と同じ仕組みで、**全部の finding** に効くようにする(3 つの curated、サンプル、dogfood)。`@assumed(tag)` の注釈(その finding に `assumed_call` が付くこと)は、これに加えるもので、不変条件は、注釈の無い finding にも効く。

## フィクスチャと、今の結果

作った(2026-11-21): go-curated `api/capture.go`(20 関数、`/captures` で登録)、dart-curated `lib/callbacks.dart`(13 関数)、rust-curated `src/callbacks.rs`(7 関数)。

### Go(今: 新しい `@miss` 11(silent)、見つかる 9、静か 6)

| タグ | 形 | 今の結果 |
| --- | --- | --- |
| `cap-run-a` / `cap-run-b` | 1 つのヘルパーを 2 つの呼び元が呼ぶ。それぞれの捕獲した値が、もう一方のクロージャに混ざらない(`@nosink`) | `@miss(silent)` ×2。C-7 の後も、混ざらない(それぞれの `make_closure` で当てる) |
| `cap-const` | クロージャは定数を sink。捕獲した値は別の sink | 見つかる(クロージャの中は `@sink(none)`) |
| `cap-param` | クロージャが、自分の引数(ヘルパーが渡す定数)を sink | `@sink(none)`。C-7 の後も静か(Params は `KUndef`) |
| `cap-arg` / `cap-inline` / `cap-go` / `cap-defer` | 引数で渡す、その場で呼ぶ、`go`、`defer` | 見つかる(対照) |
| `cap-write` | クロージャが、捕獲した変数に書き込む | `@miss(silent)` |
| `cap-write-const` | クロージャが定数を書き込む | `@sink(none)` で静か |
| `cap-nested` | 内のクロージャが、外のクロージャの捕獲を捕獲 | `@miss(silent)` |
| `cap-loop` | ループの中でクロージャを作る | `@miss(silent)` |
| `cap-two-a` | 1 つの呼びで、2 つのクロージャ(1 つは定数) | `@miss(silent)`(定数の側は `@sink(none)`) |
| `cap-never` | 作って、呼ばない | クロージャの中は `@sink(none)` で静か。**C-7 の後は報告される**(過検出。限界) |
| `cap-returned` | 関数が返したクロージャ(捕獲は、作った関数のパラメータ) | `@miss(silent)`。C-7 で直る(記号的な到達) |
| `cap-late` | 作ったあとで、捕獲した変数が汚染される(`f := func(){ sink(x) }; x = source(); run(f)`。捕獲は参照) | `@miss(silent)`。見つかるべき |
| `cap-handler` | クロージャのパラメータが source で、捕獲した変数に書き、別のクロージャが sink(登録されるハンドラの形) | `@miss(silent)`。C-1(ADR-048、a540f30)は main に入っている。それでも見つからない: C-7 の後に、この書き込みが「関数がした書き込み」として残るか(C-1 と C-7 の両方の受け入れ)を、実装で確かめる |
| `cap-seen` | その場で呼び、かつ、渡す(`f(); run(f)`) | 見つかる(1 件)。C-7 の後も 1 件(`(origin, sink)` でまとまる)。使われ方の条件で、当てる側 |
| `cap-funccall` | 捕獲した値が関数で、クロージャがそれを、呼び元の値で呼ぶ(PARAM-CALL) | `@miss(silent)`。C-7 では**直らないはず**(捕獲した関数 `g` の呼び出しは、候補が解決されない)。Dart は `relay-c-a/b`(既存) |
| `cap-result` | クロージャが捕獲した値を**返し**、ヘルパーが返す(`produce(func() string { return a })`。`WithTx2` の型) | `@miss(silent)`。C-7 のあとは **`@miss(assumed_call)`**(結果は当てない。痕跡が付く)。次の段で直る |
| `cap-sanitized` | クロージャが、捕獲した値を sink の前にサニタイズする | `@sink(none)` で静か(C-7 の後も) |
| `tx_capture`、`tx_capture_plain`(既存、`gitea/routers/tx.go`) | `db.WithTx` | `@miss(dynamic_dispatch)`、`@miss(silent)` |

### Dart(今: 新しい `@miss` 11、見つかる 1、静か 2)

| タグ | 形 | 今の結果 |
| --- | --- | --- |
| `btn-a` / `btn-b` | `Button(onPressed: () => sink(t))`(2 つ。混ざらない) | `@miss(silent)` ×2 |
| `timer-run` / `periodic` / `timer-write` | `Timer.run`、`Timer.periodic`、書き込み | `@miss(unmodeled_library_call)`(`Timer.run` に、モデルが無い) |
| `microtask` / `delayed` | `scheduleMicrotask`、`Future.delayed` | `@miss(silent)` |
| `timer-param` | クロージャが、自分の引数を sink | `@sink(none)` で静か |
| `never` | 作って、呼ばない | `@sink(none)` で静か。C-7 の後は報告される |

| `late` | 作ったあとで、捕獲した変数が汚染される | `@miss(silent)` |
| `seen` | その場で呼び、かつ、渡す | 見つかる(1 件) |
| `cap-result` / `cap-result-then` | `[1].map((e) => a).first`、`Future.then((_) => a)`(結果が捕獲した値) | `@miss(silent)`。C-7 のあとは `@miss(assumed_call)`(次の段で直る) |
| `cap-fn` | 捕獲した関数を、呼び元の値で呼ぶ(PARAM-CALL) | `@miss(silent)`。C-7 では直らないはず(`relay-c-a/b` が既存) |

(クロージャのパラメータが source の形と、サニタイズの形は、Dart の curated の規則に、`param_type` の source もサニタイザも無いので、Go だけ。)
`run-a/b`、`write-a/b`、`nest-a/b`、`add-list`、`write-param`(捕獲と引数の両方に依る: 案 2 が入るまで見つからない)などは、`pass_closure.dart`、`capture.dart`、`nesting.dart` に既にある(重複させない)。

### Rust(今: 4 つは見つかる / 静か、渡すだけの 3 つは見つからない)

`returned`、`nested_cap`、`never_called`(静かな側)、`param_clean`(静かな側): **今、見つかっている**(`Fn::call` の候補、XTALK で全クロージャが候補)。**渡すだけの 3 つは、見つからない**: `spawned`(`std::thread::spawn`、`@miss(unmodeled_library_call)`、`@noisy(8)`)、`boxed`(`Box<dyn Fn>` をフィールドに入れ、別の関数が呼ぶ、`@miss(silent)`)、`retained`(`Vec::retain`、`@miss(silent)`、`@noisy(3)`)。`@fp` の数は変わらず、`@noisy` は 183 から 194 になった。書く側(`write-a/b` ほか)は、既存の `capture.rs`(7 件が `@miss(silent)`、ENV-PTR)。

## 測定(Go の捕獲のあるクロージャの数)

`rivus index` の索引は、**作っていない**(スワップが 74%(4.5 GB / 6 GB)、別の解析が 5.3 GB で動いていて、50% を超えていた: 始めない)。`gr/gitea.db` は、`refs` の表が無い古い版(schema 1)。gogs と gin-vue-admin の**ソースは、scratchpad/bench に無い**(truth の JSON だけ)。そこで、**構文の近似**(`go/parser` だけ、型もメモリも要らない。scratchpad/c7/caps)で、Gitea と zitadel の**関数リテラル**を数えた。「捕獲あり」= 関数の中の変数 / 引数 / 定数を、リテラルの外で宣言して使っているもの。

| 対象 | ファイル | 関数リテラル | 捕獲あり | 引数 | 代入 | return | リテラル | `go f(){}()` / `defer` / その場 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Gitea(`_test.go` を除く) | 2,113 | 2,107 | 1,496 | 1,055 | 158 | 97 | 24 | 51 / 95 / 16 |
| Gitea の `modules/` | 704 | 560 | 305 | 128 | 54 | 49 | 9 | 29 / 33 / 3 |
| zitadel | 2,294 | 2,745 | 2,383 | 669 | 55 | 901 | 71 | 28 / 654 / 3 |
| go-curated(参考) | 36 | 34 | 25 | 13 | 1 | 5 | 0 | 2 / 3 / 1 |

読み方:
* 「その場で呼ばれる」(`go` / `defer` / その場): Gitea で 162 / 1,496 = 11%。**残りの 89%(1,334)が、C-7 の対象になりうる**(その場では呼ばれない。引数・代入・return・リテラル)。上限: 代入のうち、`f := func(){}; f()` で、同じ関数で呼ばれるもの(見えている)は、この近似では分けられない。引数のうち、`.Group`(ルーターの登録、413)は、ヘルパー(ライブラリ)が即座に呼ぶもので、効果は、`WithTx`(189 + `WithTx2` 34 = 223)、`QueryAndProcessByChunk`(42)、`.Go`(19)、`.Iterate`(17)など。
* `WithTx` の 223 は、`tx_capture` の形そのもの(Gitea のサービス)。
* **`rivus index` の `refs` の `closure` を使った数(どの呼び出しの候補にもならないもの)は、未測定**(スワップに余裕ができたら: `rivus index -C <gitea>` で作り、`refs.kind='closure'` で、`edges` の `from_func = owner_func` かつ `to_func = target_func` が無いものを数える)。

## 決めてほしいこと(実装に入る前に)

advisor の意見を反映して決めた点: 飛ばす条件は**使われ方**(候補に依らない)、throw は当てないが痕跡に含める、痕跡は新しい kind `assumed_call`(ADR-008 の追記)、Rust は条件つき(渡すだけの形が見つからないことを確かめた: 次の段に入れる)。残り:

1. **規則の採用**: 「見えないクロージャは、作った場所で当てる(呼ばれるものとして扱う)」。1 度も呼ばれないクロージャが過検出になる(`cap-never`、`never_called`)ことを、限界として受け入れるか。受け入れるなら、その 2 つを `@fp` にする。
2. **kind の名前**: `assumed_call`(案)でよいか。
3. **`confidence`** の受け入れの確かめ方(`bench_curated_test.go` の注釈か、`cmd/rivus` のテストか)。
4. **Rust の次の段**: 環境の `alloc` で当てる(渡すだけの 3 件が受け入れ)を、Go / Dart の直後に入れるか。ENV-PTR の修正(書く側 7 件)は別。
5. **測定の続き**: `rivus index` で `refs` を数える(スワップに余裕ができてから、1 度に 1 つ)。gogs と gin-vue-admin は、ソースを scratchpad/bench に置く必要がある(今は無い)。
6. 案 2(ADR-047)との順序: C-7 を先(小さく、独立)。案 2 は、`ExactFuncs` が束縛も返すように、C-7 のあとで足す。

## 次の段(C-7 の最初の版のあと): 捕獲の値に依る結果を、クロージャの値に持たせる

最初の版は、結果(`OutResult`)を当てない(痕跡は出す)。次の段: **`make_closure` の場所で、クロージャの要約の `OutResult`(捕獲の値で具体化したもの)を、クロージャの値の、決まったパス(たとえば `/()`: 「呼ぶと、これが返る」)のセルに置く。関数値の呼び出しは、呼ぶ値の `/()` を、結果に流す。** 値(クロージャ)は、引数、フィールド、return を、ふつうの汚染と同じに運ばれるので、**ヘルパー越し、フィールド越し、返されたクロージャの呼び出しに、仕組みを足さずに届く**。案 2 を待たずに、RET-CLOSURE(`holder-a/b`、`curry`、`getter-fn`)と `WithTx2`、`map` / `then` の型が直る見込み。クロージャの値ごとに別のセルなので、交差は起きない。

* `/()` のパスの規則: 呼び出し側(`call` の関数値の呼び出し)が、候補が決まらなくても、呼ぶ値のセルを読む。関数値のセルに汚染が付くのは、`make_closure` だけ。
* **ユーザーの判断が要る**(新しいパスの規則を、汚染のモデルに足す)ので、**C-7 の最初の版が入ってから、`cap-result` ほかの `@miss(assumed_call)` の数字と一緒に上げる**。
