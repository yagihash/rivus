# 設計: クロージャのキャプチャの値を、作った場所で適用する(C-7)

> **状態: 一部実装**(確認: 2026-11-22、9c26a39)
> - 実装済み: Go と Dart(ADR-059)。XTALK の節(ADR-079)。
> - 未実装: Rust の環境 `alloc` への適用(`captured` は `make_closure` だけ)。
> - 注意: 本文中の行番号(`valuecalls.go:319`、`build.go:362`、`summary.go:283` など)はずれている。「決めてほしいこと」のうち、回答済みのものが 2 か所残っている。

設計時の状態の記述(今の状態は上の枠が正本): **承認済み。最初のバージョン(Go と Dart)は実装した(ADR-059)**。Rust は次の段階(下の「Rust の節」に仕組みを書いた。実装していない)。以下の本文は、設計のときの記述のまま(「今の状態」は、実装前の main のもの)。実装で設計から変わったところは、末尾の「実装で分かったこと」に書いた。ここに書いた「今の状態」は、2026-11-21 の main(HEAD 8e8b866)のコードを読んで、curated のフィクスチャを流して確かめたもの。実装して確かめた数字ではないものは、「見込み」と書く。

## 問題

キャプチャした値が、**ヘルパーが呼ぶクロージャの中で**使われると、流れが完全に何も報告せずに失われる(Go でも Dart でも)。

```go
func run(f func()) { f() }
func main() {
	a := os.Getenv("A")                     // source
	run(func() { exec.Command(a).Run() })   // 見つからない。不確実性も 0
}
```

同じファイルの、クロージャを引数で受ける形(`runArg(a, func(s){…})`)、その場で呼ぶ形、`go func(){…}()`、`defer func(){…}()` は見つかる。キャプチャした値を、**別の関数が呼ぶ**クロージャの中で使う形だけが落ちる。Gitea のサービスはほぼ全部この形(`db.WithTx(ctx, func(ctx) error { …捕獲した値… })`)。Dart の `Button(onPressed: () => sink(t))`、`Timer.run`、`run(() => sink(a))`(CAP 12 件、DART-15 / DART-16)も同じ。

### なぜ落ちるか(コードの上で)

1. 束縛(クロージャがキャプチャした値 = `make_closure` の引数)を呼び出しに付けるのは、**クロージャを作った関数 `home` の中の呼び出し**だけ。`internal/valuecalls/valuecalls.go:319` `funcsOf` の `if f == home && len(in.Args) > 0 && cur.bind == nil`。ヘルパー `run` の中の `f()` は、`home` が `run` で、クロージャを作った `main` ではないので、束縛が付かない(候補にはなる)。
2. `internal/sg/build.go:362` `resolveValueCalls`: `c.Bind != nil` のときだけ `CandidateBindings` を作る。
3. `internal/taint/summary.go:283-286` `applySummary`: 束縛が無いと、クロージャの FreeVars を `KUndef`(何も運ばない値)で**何も報告せずに**埋める。不確実性を出さない。

つまり、クロージャの要約(「FreeVar → sink」「FreeVar への書き込み」)を、キャプチャした値で具体化できるのは、**作った場所だけ**なのに、作った場所では、呼ぶところが見えないので、要約を使わない。

## 規則(案)

**`make_closure`(Go と Dart)の命令で、クロージャの要約を、キャプチャの値(`make_closure` の引数)で具体化する。クロージャの宣言した引数(Params)は「不明」(汚染なし、`KUndef`)にする。** これで、クロージャの中の「キャプチャした値 → sink」と「キャプチャした変数への書き込み」が、**どこへ渡したかに関係なく**、作った関数に届く。

### (1) 適用する場所と、適用するもの

場所: taint の `transfer`(`internal/taint/transfer.go` の op の switch、`case ir.OpCall:` の隣に `case ir.OpMakeClosure:`)。`Args` が空でない `make_closure` だけ(空のものは `KFunc` の値で、キャプチャが無い。`sg/build.go:193`)。

適用するもの(`applySummary` の出力の種類のうち):

| 出力 | 適用する | 行き先 |
| --- | --- | --- |
| sink への到達(`sum.Sinks`) | **適用する** | キャプチャした値の汚染が sink に届けば finding。キャプチャした値が、作った関数のパラメータなら、記号的な到達(`noteSinkReach`)になり、呼び出し元に伝わる(ヘルパーを返すクロージャ `mkRun(s)` が効く) |
| キャプチャした変数への書き込み(`OutParam`、`argv[o.Index]` = キャプチャした値の指す先) | **適用する** | キャプチャした変数のセル(`out = a`)。`Go` の `var out string` と、Dart / Rust の変数の書き込み |
| グローバルへの書き込み(`OutGlobal`) | **適用する** | グローバルのセル |
| throw(`OutThrow`) | **適用しない。ただし何も報告せずに落とすことはしない** | throw は、呼ばれたところから抜ける。`make_closure` の場所に流すと、作った関数が throw することになり、過大。`try { run(() => throw x) } catch` は、見逃す。クロージャの要約に、**キャプチャの値に依る throw の出力があるとき**は、(4)の不確実性を出す条件に含める(「適用しなかった」ことが残る。detail に、throw は適用していないと書く) |
| 戻り値(`OutResult`) | **適用しない。ただし何も報告せずに落とすことはしない**(throw と同じ) | クロージャの結果は、呼ぶところで受け取るもの。`make_closure` の場所に、呼ぶところは無い(行き先が無い)。`cc.roots = nil`。**キャプチャの値に依る `OutResult` があるときは、(4)の不確実性を出す条件に含め、detail に「結果は適用していない」と書く**。この型が、C-7 のあとも落ちる: `produce(func() string { return a })`(ヘルパーの `f()` は束縛なしで一致し、キャプチャの値は `KUndef`)、Gitea の `db.WithTx2(ctx, func(ctx) (T, error) { return load(捕獲した値) })`(34 か所)、Dart の `items.map((x) => x + a)`、`Future.then((_) => a)`、MISSES の RET-CLOSURE(`holder-a/b`、`curry`、`getter-fn`)。次の段階(末尾)で適用する |

実装は、合成した `callCtx`(`in` は `make_closure` の命令、`recv=nil`、`args` は Params と同じ数の `KUndef`、`roots=nil`)と `applySummary(cc, closureFn, mc.Args, notes, nil)`。`applySummary` の `argv = cc.argv() + bindings`(Params の後ろに FreeVars)は、そのまま使える。**`summary.go` の中は変えなくてよい**(`len(bindings) == 0` のときだけ不足を `KUndef` で埋める分岐は、束縛を渡すので通らない。Params を呼び出し側で埋めるので、不一致のエラーにならない)。

### (2) 適用しないもの

* **戻り値**: クロージャの結果は、呼ぶところで受け取るもの。`make_closure` の場所に、呼ぶところは無い(行き先が無い)。`cc.roots = nil` にして、`OutResult` の `o.Index < len(cc.roots)` を偽にする。
* **引数に依る流れ**: クロージャの引数(Params)から sink への流れ(`runArg(a, func(s){ sink(s) })`)は、呼ぶ側の値(ヘルパーが渡す)に依る。`KUndef` なので何も運ばない。これは、今のとおり、呼ぶ側(ヘルパー内の `f(x)` の候補)で見つかる。**C-7 は、キャプチャした値だけに依る流れを足す。引数に依る流れは足さない**(これは案 2 = ADR-047 の作業)。
* キャプチャした値と引数の**両方に依る**流れ(`sink(x + a)`): 要素は 1 起点ずつなので、`a` の分は C-7 で、`x` の分は呼ぶ側で、それぞれ見つかる。

### (3) 飛ばす条件と、二重になったとき

**飛ばす条件: クロージャの値の使われ方が、作った関数の中での直接の呼び出し(その場で呼ぶ、`go`、`defer`)だけのとき。** それ以外の使い方(引数に渡す、代入、フィールドやグローバルに入れる、return、φ)が **1 つでも**あれば、適用する。作った関数の SSA の使用箇所(`make_closure` の結果の `Value` の使われ方)だけで決まり、**ほかの解析の結果(`valuecalls` の候補、上限 8、解決のされ方)に依らない**。

「候補として解決済み」は、飛ばす理由にならない。今の見逃し(CAP)は、まさに、**候補としては解決済みで、キャプチャの値が付いていない**形(`run(func() { sink(a) })`: `run` の中の `f()` の候補に、このクロージャは入っている)。候補の有無で決めると、候補の上限や解決のされ方で、適用したり適用しなかったりする。
前のバージョンの `sg.Instr.Seen`(「束縛つきで呼ばれた」マーク)は、**取り下げ**。使用箇所で決める。

* 実装: taint の `captured` が、`make_closure` の結果の使用箇所を見る。`in.Dsts[0]` の `Value` から、それを引数(`Args`)に持つ命令を、コピーと φ を通ってたどり、すべてが「呼び出しの callee の位置(`Callee.Kind == CalleeValue` で、関数値が `Args[0]`、または、`Static` / `Bindings` が自分)」か、`go` / `defer` のフラグつきの呼び出しなら、飛ばす。1 つでも、引数の位置・`store`・`field_store`・`global_store`・return・φ(その先が上の条件を満たさない)・**別のクロージャの `make_closure` の引数の位置(このクロージャを、内のクロージャがキャプチャする。内のクロージャが呼んでも、ここからは見えない)**なら、適用する。使用箇所の表を `sg` の構築時に作る(`Value` ごとの使用の一覧が既にあるなら、それを使う。無ければ、`sg` に作る。**コアの変更**)。
* 検証の基準: **今の finding の経路が 1 件も変わらない**。その場で呼ぶだけのクロージャは、適用しないので、経路が変わらない。引数で値を渡す形(`runArg(a, func(s){…})`)は、キャプチャに依る効果が無ければ、適用しても何も足さない。**変わりうるのは、「キャプチャもあり、候補としても束縛つきで解決されている」形**(`home` の中の関数値の呼び出しの候補になっていて、かつ、別の使い方もある)。その件数を、前後の差分で数える。
* `cap-seen`(その場で呼び、かつ、渡す): 渡す使い方があるので、適用する。finding は `(origin, sink)` で 1 件にまとまる(`recordFinding` の `findingKey`、`engine.collect` の `merged[k]`)。代表の経路は、ステップ数の少ないほう(`<` なので、同じ長さなら先に記録したほう)。`make_closure` の経路は、ヘルパーを通らないので、たいてい短く、代表になる(経路が変わりうるのは、この形)。`mergeParamSources` は、source が param の finding だけの別の併合で、関係しない。

### (4) 不確実性の注記

呼ぶところが見えないクロージャに、キャプチャの値を適用したことを、不確実性で残す。

* kind: **新しい kind `assumed_call`**(`attributable_to: mixed`)。`unresolved_callee` は使わない。理由: `unresolved_callee` の意味は「呼び出し先が分からない」で、ここで分からないのは、**呼ぶ側**(このクロージャを、誰が、いつ、どの引数で呼ぶか)。`unresolved_callee` は「候補を増やせば直る」、こちらは「メモリをたどる、モデルを足す、で直る」で、修正方法が違う。同じ kind にすると、kind ごとの集計と `attributable_to` の指標が濁る。LLM の判定にも、「この finding は、クロージャが呼ばれると仮定した結果」を、finding ごとに見分けさせたい。名前は `assumed_call`(「呼ばれると仮定した」。代案: `closure_call_assumed`。短いほうにした)。
* **閉じた集合に 1 つ足す**: kind の一覧は閉じた集合なので、**ADR-008 の追記が必要**(`result.go` の定数、`attributable()`、出力のスキーマ、`docs` の kind の表、`cases_test.go` の kind ごとの期待)。
* 文面(`detail`): `the closure is made here and called where it cannot be seen (it is given to another function, or stored): what it captured is applied at the place it was made, and it is taken to be called`。日本語の意味: このクロージャが呼ばれるところは見えていない。作った場所で、キャプチャの値を適用した(呼ばれるものとして扱っている)。throw を適用しなかったときは、`; what it throws is not applied` を足す。
* `symbol` は、クロージャの関数 ID。`position` は `make_closure` の位置(`graph` の `refs` の `closure` と同じ)。`func` は作った関数。
* **出す条件**: クロージャの要約が、キャプチャした値に依る効果(FreeVar 由来の sink への到達、書き込み、**throw の出力**、**戻り値(`OutResult`)の出力**)を**持つ**ときだけ。適用しなかったもの(throw、結果)は、detail に `; what it throws is not applied` / `; what it returns is not applied` と書く。毎回出すと、Gitea で数千件になる(キャプチャのあるクロージャ 1,496)。`sum.Sinks` と `sum.Outputs` の要素が FreeVar の `ParamElem` を含むか、で決まる(`applySummary` の前に調べる)。
* finding の経路: `applySummary` の `notes` 引数に渡す。ステップ(`call.Notes`)と `w.setNotes` に載り、finding の `uncertainties` に出る。重複は、`engine.collect` が `kind|symbol|func|pos` で 1 つにする。
* **`confidence`**: この規則で見つかった finding の `confidence` が、**`no_uncertainty_on_path` にならない**こと(経路に `assumed_call` があるので)。**今の main では、規則が無いので、確かめられない**。期待として、実装後に、`cap-run-a`(go-curated)の finding の `confidence` と `uncertainties` に `assumed_call` が出ること、を受け入れの条件にする(`bench_curated_test.go` に、`@assumed` のような注釈で確かめるか、`cmd/rivus` のテストで、`cap-run-a` を `analyze` して JSON を見る。実装のときに決める)。

### (5) 失うもの・増えるもの

* **増える**: 見えないクロージャの中の、キャプチャした値 → sink の流れ。**1 度も呼ばれないクロージャの中の流れも、finding になる**(過検出)。呼ぶ順序も見ない(クロージャが、キャプチャした変数を書く前に呼ばれる、など)。`go-curated/api/capture.go` の `capNever` が、その形(今は検出されない。`@sink(none)` が、実装後に落ちる。受け入れの基準は、**`capNever` を `@fp` にするか、規則の限界として受け入れるか**を、ユーザーが決める)。
* **失うもの**: 無い見込み(適用するものは足すだけで、`summary.go` の既存の動作は変えない)。ただし、経路が変わる finding がある(上の(3)、見えているクロージャを飛ばせば無い)。
* **増える量の見積り**(Gitea、構文の上の近似、下の「測定」): キャプチャのあるクロージャ 1,496 のうち、その場で呼ばれない(引数、代入、return、リテラル)ものが 1,334(89%)。そのうち、`.Group`(ルーターの登録)413、`.WithTx` 189 + `.WithTx2` 34。ルーターの `Group(path, func(){ m.Get(...) })` の中身は、呼ばれるところが見えている(ライブラリが即座に呼ぶ)ので、過検出にならないが、`WithTx` 以外にも多い。
* 計算量: キャプチャのあるクロージャ 1 つにつき、`applySummary` 1 回(`callSig` で、変化の無い繰り返しは飛ばす)。Gitea で約 1,300 回。要約が大きいクロージャで、時間が増える可能性は、実装して測る(精度を優先する方針: 結果が正しいことが先)。

### (6) 入れ子のクロージャと、ループの中のクロージャ

* **入れ子**(外のクロージャのキャプチャを、内のクロージャがキャプチャする): 内の `make_closure` は、外のクロージャの本体の中にあり、その引数は、外のクロージャの FreeVar。外のクロージャの要約を作るとき、内の `make_closure` を適用すると、FreeVar に依る記号的な sink 到達(`noteSinkReach`)になる。外の `make_closure`(作った関数 `home`)で、それが具体化される。**要約の合成で、段数に依らず届く**。`capNested`(go-curated)がそのフィクスチャ。外のクロージャが見えている(その場で呼ぶ)場合に、内のクロージャが見えていないときは、内を適用する(マークは、命令ごと)。
* 届く順序(追記): 内側のクロージャの効果は、**外側のクロージャの要約に、外側のキャプチャの言葉(外側の FreeVar)で入る**。作った関数が外側を適用する、の順で届く(内側は、外側の本体の中の `make_closure` として、外側の要約を作るときに一致する)。内側の使われ方が「直接の呼び出しだけ」なら、飛ばす(外側の本体の中で、その場で呼ばれる)。`cap-nested` で確かめる(今は `@miss(silent)`)。
* **ループの中のクロージャ**: `make_closure` の命令は 1 つ。解析は流れに依らない(セルの和)ので、ループの何周目かを区別しない。キャプチャした変数がループ変数(Go 1.22 以降は周ごと)でも、セルは 1 つで、届く。`capLoop` がそのフィクスチャ。ループで変数を書き換える形(`x += c.Query(..)`)は、セルの和で届く(過大側)。

### (7) Rust と Dart

**Dart**: `make_closure` にキャプチャの値が付く(確かめた: `run_a` の IR は `make_closure [1] package:curated/pass_closure.dart#run_a$1`、クロージャ側は `param` の命令の `index` が宣言した引数の数以降なら FreeVar、Go と同じ規則。`sg/build.go:180-190`)。**そのまま同じ実装で足りる**(`Button(onPressed: …)`、`Timer.run`、`scheduleMicrotask`、`Future.delayed` は、全部、クロージャの作られる場所が同じ形)。フィクスチャ: `dart-curated/lib/callbacks.dart`(7 件が今 `@miss`)。

**Rust**: キャプチャのあるクロージャは **`make_closure` ではない**。確かめた(`rivus-rust` の出力の IR、`write_a`): 環境の構造体を、`alloc`(class `func`、type `{closure@src/capture.rs:9:9: 9:11}`)で作り、キャプチャした値を `field_store`(field `"0"`, `"1"`…。可変のキャプチャは、変数へのポインタ)で入れる。クロージャの関数は、**パラメータが 1 つ(その環境、`recv: true`)**で、`field_load` で取り出す。呼び出しは、`Fn::call` の dynamic 呼び出し(候補は、同じ引数の型を持つ全クロージャ = XTALK)。

* **できるか**: 同じことは、**別の段階**で、できる。環境の `alloc` の type(`{closure@file:l:c}`)と、クロージャの関数の `params[0].type` が一致するので、alloc → クロージャの関数が引ける(extractor は変えなくてよい見込み。型の文字列が、ファイルと位置を含むので、一意)。`applySummary(cc, closureFn, nil, notes, nil)` を、`argv = [環境の値]`(束縛ではなく、Params の 1 つ目)で適用する。ただし、環境の `field_store` が `alloc` の後に来る(流れに依らない解析なので、順序は、問題にならないはずだが、**確かめていない**)。
* **効く範囲**: 読む側(`run(|| sink(&a))`)は、`Fn::call` の候補で、**今でも見つかる**(XTALK の過検出つき。`run-a/b` は `@fp`)。C-7 は、これを直さない(XTALK は別の原因。案 2 の作業)。`rust-curated/src/callbacks.rs` の 3 件(`returned`、`nested_cap`、`param_clean`)と `never_called` の検出されない側は、**今、見つかっている**(`@miss` が無い)。書く側(`write-a/b`、`push-list` など 7 件、ENV-PTR)は、環境のフィールドのポインタが、指す先と別名にならない(`locs.go:215`、`transfer.go:148`)ことが原因なので、**alloc で適用しても直らない**。ENV-PTR の修正が先。
* 結論(**条件つき**): 前のバージョンは「読む側は、今でも見つかる」を理由に、Rust を最初のバージョンの対象外にしたが、これは**呼ぶ側が見える形**(`Fn::call` の候補になる)でしか確かめていなかった。**渡すだけの形**で確かめた(`rust-curated/src/callbacks.rs`、今の main): `std::thread::spawn(move || sink(&t))`(`@miss(unmodeled_library_call)`)、キャプチャのあるクロージャを `Box<dyn Fn(i16, u128)>` にして構造体のフィールドに入れ、別の関数が呼ぶ(`@miss(silent)`)、モデルのない標準ライブラリの関数に渡す(`Vec::retain`、`@miss(silent)`)の **3 件とも見つからない**。つまり、Rust にも、サイレントな見逃しがある。**Rust の環境の `alloc` で適用する段階(上の「できるか」)は、次の段階に入れる**(最初のバージョンは Go と Dart。ENV-PTR の修正は、書く側 7 件で、別)。最初のバージョンを Go と Dart にするのは、実装の大きさ(extractor の type で引く、環境の `field_store` の順序の確認)を分けるためで、「Rust は見つかるから不要」ではない。`thread::spawn` と `retain` のクロージャは、他の呼び出しの候補にもなり、他のソースが誤って届く(`@noisy` 8 と 3)。

### (8) 案 2(ADR-047 の下書き: scratchpad/scc/adr-draft.md)との関係

どちらも、「**作った関数での束縛**」(`make_closure` の引数)を使う。案 2 の「呼び出し元 K が、自分で渡したクロージャ(`ExactFuncs`)の要約で、ヘルパーの記号的な出力を具体化する」には、クロージャの要約に**束縛も**必要(下書きに書いていない。C-7 と同じ情報)。そこで:

* `sg.ExactFuncs` を、ID の集合だけでなく、**`make_closure` の命令(束縛)も返す**ようにする(`ExactClosures`)。C-7 は、これを使わない(`make_closure` の命令を直接見る)が、案 2 が使うので、同じ補助関数にしてよい。
* **分担**: C-7 = **キャプチャの値だけに依る流れ**(作った場所で適用する)。案 2 = **引数に依る流れ**(ヘルパーが渡す値。呼ぶ側のコンテキストで具体化)。**両方に依る流れ**(`fireAndForget(t, (x) { out = x; })` の `out` に、`t` が届く: キャプチャした `out` への書き込みの値が、ヘルパーが渡す引数 `x`)は、案 2 が入るまで**見つからない**(`write-param`(Dart、Rust)で記録している)。クロージャの要約の要素を、FreeVar 由来か Params 由来かで分ければ、二重にならない: 案 2 の K での具体化は、Params 由来の要素だけを適用し、FreeVar 由来は C-7 に任せる。案 2 が先に入った場合も、C-7 の「使われ方」の条件は変わらない(K の束縛は、C-7 の条件に影響しない)。
* 順序: **C-7 は案 2 に依らない**。案 2 は SCC の構造(要約のデータ型、`applySummary` の展開)を変える大きな作業。C-7 は、小さく、独立に入れられる。案 2 を待たない。
* **再解析の辺**: 作った関数(`home`)は、クロージャの要約に依存する。クロージャがどの呼び出しの候補でもないとき、今は、その依存の辺が無い(`callers` は、呼び出しの候補から作る)。**辺を足す**(クロージャ → `home`: クロージャの要約が変わったら、`home` を再解析する)。クロージャが、作った関数を(直接でも間接でも)呼ぶ形(再帰: クロージャの中から `home` を呼ぶ、入れ子のトランザクション)では、**この辺で SCC ができる。それは正しい**(要約が互いに依存している)。案 2 の SCC の判定(ヘルパー → クロージャの辺を外す)とは別の辺で、C-7 は `order` / `levelsOf` を、この辺の分だけ変える。`engine.go` の依存の作り方を、実装のときに読む。

### (9) 実装の場所の推定

| 変えるところ | 内容 |
| --- | --- |
| `internal/sg`(使用箇所) | `make_closure` の結果の `Value` の**使われ方の一覧**(callee の位置か、`go` / `defer` の呼び出しだけか、それ以外か)を作る(`Seen` のマークは取り下げ。`sg` に使用の一覧が無ければ作る = **コアの変更**)。候補、`valuecalls` の結果には依らない |
| `internal/sg/build.go:84-90`(静的な束縛)、`:362` `resolveValueCalls` | **変えない**(前のバージョンは、ここでマークを付ける案だった) |
| `internal/taint/transfer.go:236` の switch | `case ir.OpMakeClosure:` を足し、`s.captured(in)` を呼ぶ |
| `internal/taint/calls.go`(新しい関数 `captured`) | `in.Args` が空、使われ方が「直接の呼び出しだけ」、クロージャの `sg.Func` が無い(`ByID`)、要約に FreeVar 由来が無い、のどれかなら何もしない。それ以外は、`callCtx{in: in, args: Params 数の KUndef, roots: nil, sym: ""}` で `applySummary(cc, fn, in.Args, notes, nil)`。`notes` は、(4)の不確実性 |
| `internal/taint/engine.go`(依存) | 要約の依存の辺(クロージャ → home)を足す(上の(8)の再解析の辺) |
| `internal/taint/summary.go:283-286` | **不確実性を出す(任意ではない)**。束縛が無い非 home の呼び出しで、FreeVars を `KUndef` で埋めるときに、不確実性を出す。C-7 のあとも、Rust(最初のバージョンの対象外)と、戻り値(上の(1))で、埋める場面が残る。何も報告せずに埋める場所を残さない。文面は `assumed_call` ではなく、**「このクロージャがキャプチャした値は、ここでは分からない」**(DART-15 の不確実性の注記の修正)。**C-2 の項目 2(別のエージェント a6ede9c0341ce3c28 が実装中)と同じ**なので、重複させない(C-2 が入ってから、C-7 は、その注記の上に載る) |
| `internal/valuecalls/valuecalls.go:319` | **変えない**。`ExactClosures`(案 2)は、ここではなく `sg.ExactFuncs`(`sg/build.go:393`)の拡張 |
| `internal/graphdb/write.go:261` | **変えない**(`refs` の `closure` は、構造の層。taint の変更と無関係) |
| `docs/DECISIONS.md` | ADR(次の空き番号)。ADR-045 の限界(「パラメータ由来の候補は、呼び出し側ごと」の下の、束縛が home でだけ付く点)に、「キャプチャした値は C-7 が適用する」と追記 |

`rivus resolve` の「見つからない理由」(source / sink の関数にある不確実性): C-7 の不確実性は、作った関数(`home` = source の関数のことが多い)に付くので、**流れが見つかっていれば**、`resolve` に出る必要は無い(見つかる)。見つからないとき(キャプチャした値が、sink に届かないとき)は、効果を持つクロージャが無いので、出さない。

### (10) 検証の計画

0. **最初に**: 収束する 4 つ(gogs、gin-vue-admin、ech0、go-test-bench)を、実行できる状態にする。C-1 では、scratchpad の複製に go.mod が無く、実行できなかった。`bench/README.md` の取得の手順で、複製を作り直し、**C-7 の前**の finding を基準として保存する。
1. **網羅の不変条件**(`internal/taint` の既存のもの): finding の増加が、`make_closure` の経路の finding だけ。使われ方で飛ばすバージョンは、**既存の finding が 1 件も消えず、経路も変わらない**ことを、全ベンチで確かめる。変わりうる形(「キャプチャもあり、候補としても束縛つきで解決されている」)は、差分で数える。
2. **ベンチの差分**: 収束する 4 つ(gogs、gin-vue-admin、ech0、go-test-bench)、`bench/go-curated`、`bench/go-synth`(S、M)、`testdata/go/cases`、Dart(`dart-curated`、`dart-pub`、`dart-corpus` 系)、Rust(変更なしのはず。finding が 1 件も変わらないことを確かめる)。**前後で findings を比べ、増えたもの全部を 1 件ずつ分類**(正しい流れ / 呼ばれないクロージャの過検出 / 順序の過検出)。Gitea は、C-1(ADR-048)のあとは、収束する見込み(競合なしの再実行を待つ)。収束するなら、**Gitea 全体で前後を比べる**(`./modules/...` と `services/` の一部は、その予備。`db.WithTx` を含む)。収束しなければ、予備で比べる。分類できなければ push しない。
3. **curated の `@miss` / `@fp` の増減**(下の表。**実装して確かめたものではない**):

| ベンチ | `@miss` の今 | C-7 で直る(見込み) | 備考 |
| --- | --- | --- | --- |
| go-curated | 10(うち 9 が `silent`) | 9: `cap-run-a/b`、`cap-write`、`cap-nested`、`cap-loop`、`cap-two-a`、`cap-returned`、`tx_capture`(`dynamic_dispatch`)、`tx_capture_plain` | 残る 1 は `options`(`RET-CLOSURE`: クロージャの**引数**への書き込み) |
| dart-curated | 41(うち 31 が `silent`) | `callbacks.dart` の 7 件 + `CAP` のうち、キャプチャした値だけで足りるもの(`run-a/b`、`write-a/b`、`add-list`、`nest-a/b`、`across-cap`、`builder`、`bound-prefix` など約 10)。`write-param`、`twice-args` は、引数に依る(案 2 の作業) | |
| rust-curated | 13(全部 `silent`) | 0 | (7)のとおり、対象外。`callbacks.rs` の 4 つは、今、見つかっている |

   * `@sink(none)` が増える側: `capNever`(go-curated)と、`never_called`(dart-curated)の `@sink(none)`、`rust-curated/callbacks.rs:never_called`(Rust は対象外なので変わらない)。**この 2 つが落ちる**。落ちたら、`@fp` に直すか(限界として記録)、規則を変えるか、決める。
   * `@miss(silent)` の割合: Go 9/10(今)→ 0/1、Dart 31/41 → 約 20/30(見込み。silent のうち CAP 系が消える)。
4. **go test ./...** と、`RIVUS_DART` / `RIVUS_RUST` つきの curated(Dart・Rust の extractor のテスト)を通す。
5. **不確実性の数**: 前後で、`assumed_call` が、効果を持つクロージャの数だけ増える。他の kind は変わらない。
6. **注釈に依らない不変条件**(共通の検査に入れる): 「経路か finding に `assumed_call` がある finding は、`confidence` が `no_uncertainty_on_path` でない」を、BENCH-2(`offsource_test.go` の共通の検査)と同じ仕組みで、**全部の finding** に効くようにする(3 つの curated、サンプル、dogfood)。`@assumed(tag)` の注釈(その finding に `assumed_call` が付くこと)は、これに加えるもので、不変条件は、注釈の無い finding にも効く。

## フィクスチャと、今の結果

作った(2026-11-21): go-curated `api/capture.go`(20 関数、`/captures` で登録)、dart-curated `lib/callbacks.dart`(13 関数)、rust-curated `src/callbacks.rs`(7 関数)。

### Go(今: 新しい `@miss` 11(silent)、見つかる 9、検出されない 6)

| タグ | 形 | 今の結果 |
| --- | --- | --- |
| `cap-run-a` / `cap-run-b` | 1 つのヘルパーを 2 つの呼び出し元が呼ぶ。それぞれのキャプチャした値が、もう一方のクロージャに混ざらない(`@nosink`) | `@miss(silent)` ×2。C-7 の後も、混ざらない(それぞれの `make_closure` で適用する) |
| `cap-const` | クロージャは定数を sink。キャプチャした値は別の sink | 見つかる(クロージャの中は `@sink(none)`) |
| `cap-param` | クロージャが、自分の引数(ヘルパーが渡す定数)を sink | `@sink(none)`。C-7 の後も検出されない(Params は `KUndef`) |
| `cap-arg` / `cap-inline` / `cap-go` / `cap-defer` | 引数で渡す、その場で呼ぶ、`go`、`defer` | 見つかる(対照) |
| `cap-write` | クロージャが、キャプチャした変数に書き込む | `@miss(silent)` |
| `cap-write-const` | クロージャが定数を書き込む | `@sink(none)` で検出されない |
| `cap-nested` | 内のクロージャが、外のクロージャのキャプチャをキャプチャ | `@miss(silent)` |
| `cap-loop` | ループの中でクロージャを作る | `@miss(silent)` |
| `cap-two-a` | 1 つの呼びで、2 つのクロージャ(1 つは定数) | `@miss(silent)`(定数の側は `@sink(none)`) |
| `cap-never` | 作って、呼ばない | クロージャの中は `@sink(none)` で検出されない。**C-7 の後は報告される**(過検出。限界) |
| `cap-returned` | 関数が返したクロージャ(キャプチャは、作った関数のパラメータ) | `@miss(silent)`。C-7 で直る(記号的な到達) |
| `cap-late` | 作ったあとで、キャプチャした変数が汚染される(`f := func(){ sink(x) }; x = source(); run(f)`。キャプチャは参照) | `@miss(silent)`。見つかるべき |
| `cap-handler` | クロージャのパラメータが source で、キャプチャした変数に書き、別のクロージャが sink(登録されるハンドラの形) | `@miss(silent)`。C-1(ADR-048、623b620)は main にマージされている。それでも見つからない: C-7 の後に、この書き込みが「関数がした書き込み」として残るか(C-1 と C-7 の両方の受け入れ)を、実装で確かめる |
| `cap-seen` | その場で呼び、かつ、渡す(`f(); run(f)`) | 見つかる(1 件)。C-7 の後も 1 件(`(origin, sink)` でまとまる)。使われ方の条件で、適用する側 |
| `cap-funccall` | キャプチャした値が関数で、クロージャがそれを、呼び出し元の値で呼ぶ(PARAM-CALL) | `@miss(silent)`。C-7 では**直らないはず**(キャプチャした関数 `g` の呼び出しは、候補が解決されない)。Dart は `relay-c-a/b`(既存) |
| `cap-result` | クロージャがキャプチャした値を**返し**、ヘルパーが返す(`produce(func() string { return a })`。`WithTx2` の型) | `@miss(silent)`。C-7 のあとは **`@miss(assumed_call)`**(結果は適用しない。不確実性の注記が付く)。次の段階で直る |
| `cap-sanitized` | クロージャが、キャプチャした値を sink の前にサニタイズする | `@sink(none)` で検出されない(C-7 の後も) |
| `tx_capture`、`tx_capture_plain`(既存、`gitea/routers/tx.go`) | `db.WithTx` | `@miss(dynamic_dispatch)`、`@miss(silent)` |

### Dart(今: 新しい `@miss` 11、見つかる 1、検出されない 2)

| タグ | 形 | 今の結果 |
| --- | --- | --- |
| `btn-a` / `btn-b` | `Button(onPressed: () => sink(t))`(2 つ。混ざらない) | `@miss(silent)` ×2 |
| `timer-run` / `periodic` / `timer-write` | `Timer.run`、`Timer.periodic`、書き込み | `@miss(unmodeled_library_call)`(`Timer.run` に、モデルが無い) |
| `microtask` / `delayed` | `scheduleMicrotask`、`Future.delayed` | `@miss(silent)` |
| `timer-param` | クロージャが、自分の引数を sink | `@sink(none)` で検出されない |
| `never` | 作って、呼ばない | `@sink(none)` で検出されない。C-7 の後は報告される |

| `late` | 作ったあとで、キャプチャした変数が汚染される | `@miss(silent)` |
| `seen` | その場で呼び、かつ、渡す | 見つかる(1 件) |
| `cap-result` / `cap-result-then` | `[1].map((e) => a).first`、`Future.then((_) => a)`(結果がキャプチャした値) | `@miss(silent)`。C-7 のあとは `@miss(assumed_call)`(次の段階で直る) |
| `cap-fn` | キャプチャした関数を、呼び出し元の値で呼ぶ(PARAM-CALL) | `@miss(silent)`。C-7 では直らないはず(`relay-c-a/b` が既存) |

(クロージャのパラメータが source の形と、サニタイズの形は、Dart の curated の規則に、`param_type` の source もサニタイザも無いので、Go だけ。)
`run-a/b`、`write-a/b`、`nest-a/b`、`add-list`、`write-param`(キャプチャと引数の両方に依る: 案 2 が入るまで見つからない)などは、`pass_closure.dart`、`capture.dart`、`nesting.dart` に既にある(重複させない)。

### Rust(今: 4 つは見つかる / 検出されない、渡すだけの 3 つは見つからない)

`returned`、`nested_cap`、`never_called`(検出されない側)、`param_clean`(検出されない側): **今、見つかっている**(`Fn::call` の候補、XTALK で全クロージャが候補)。**渡すだけの 3 つは、見つからない**: `spawned`(`std::thread::spawn`、`@miss(unmodeled_library_call)`、`@noisy(8)`)、`boxed`(`Box<dyn Fn>` をフィールドに入れ、別の関数が呼ぶ、`@miss(silent)`)、`retained`(`Vec::retain`、`@miss(silent)`、`@noisy(3)`)。`@fp` の数は変わらず、`@noisy` は 183 から 194 になった。書く側(`write-a/b` ほか)は、既存の `capture.rs`(7 件が `@miss(silent)`、ENV-PTR)。

## 測定(Go のキャプチャのあるクロージャの数)

`rivus index` の索引は、**作っていない**(スワップが 74%(4.5 GB / 6 GB)、別の解析が 5.3 GB で動いていて、50% を超えていた: 始めない)。`gr/gitea.db` は、`refs` の表が無い古いバージョン(schema 1)。gogs と gin-vue-admin の**ソースは、scratchpad/bench に無い**(truth の JSON だけ)。そこで、**構文の近似**(`go/parser` だけ、型もメモリも不要。scratchpad/c7/caps)で、Gitea と zitadel の**関数リテラル**を数えた。「キャプチャあり」= 関数の中の変数 / 引数 / 定数を、リテラルの外で宣言して使っているもの。

| 対象 | ファイル | 関数リテラル | キャプチャあり | 引数 | 代入 | return | リテラル | `go f(){}()` / `defer` / その場 |
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

advisor の意見を反映して決めた点: 飛ばす条件は**使われ方**(候補に依らない)、throw は適用しないが不確実性の注記に含める、注記は新しい kind `assumed_call`(ADR-008 の追記)、Rust は条件つき(渡すだけの形が見つからないことを確かめた: 次の段階に入れる)。残り:

1. **規則の採用**: 「見えないクロージャは、作った場所で適用する(呼ばれるものとして扱う)」。1 度も呼ばれないクロージャが過検出になる(`cap-never`、`never_called`)ことを、限界として受け入れるか。受け入れるなら、その 2 つを `@fp` にする。
2. **kind の名前**: `assumed_call`(案)でよいか。
3. **`confidence`** の受け入れの確認方法(`bench_curated_test.go` の注釈か、`cmd/rivus` のテストか)。
4. **Rust の次の段階**: 環境の `alloc` で適用する(渡すだけの 3 件が受け入れ)を、Go / Dart の直後に入れるか。ENV-PTR の修正(書く側 7 件)は別。
5. **測定の続き**: `rivus index` で `refs` を数える(スワップに余裕ができてから、1 度に 1 つ)。gogs と gin-vue-admin は、ソースを scratchpad/bench に置く必要がある(今は無い)。
6. 案 2(ADR-047)との順序: C-7 を先(小さく、独立)。案 2 は、`ExactFuncs` が束縛も返すように、C-7 のあとで足す。

## 次の段階(C-7 の最初のバージョンのあと): キャプチャの値に依る結果を、クロージャの値に持たせる

最初のバージョンは、結果(`OutResult`)を適用しない(不確実性の注記は出す)。次の段階: **`make_closure` の場所で、クロージャの要約の `OutResult`(キャプチャの値で具体化したもの)を、クロージャの値の、決まったパス(たとえば `/()`: 「呼ぶと、これが返る」)のセルに置く。関数値の呼び出しは、呼ぶ値の `/()` を、結果に流す。** 値(クロージャ)は、引数、フィールド、return を、ふつうの汚染と同じに運ばれるので、**ヘルパー越し、フィールド越し、返されたクロージャの呼び出しに、仕組みを足さずに届く**。案 2 を待たずに、RET-CLOSURE(`holder-a/b`、`curry`、`getter-fn`)と `WithTx2`、`map` / `then` の型が直る見込み。クロージャの値ごとに別のセルなので、交差は起きない。

* `/()` のパスの規則: 呼び出し側(`call` の関数値の呼び出し)が、候補が決まらなくても、呼ぶ値のセルを読む。関数値のセルに汚染が付くのは、`make_closure` だけ。
* **ユーザーの判断が必要**(新しいパスの規則を、汚染のモデルに足す)ので、**C-7 の最初のバージョンが入ってから、`cap-result` ほかの `@miss(assumed_call)` の数字と一緒に上げる**。

## Rust の dynamic な呼び出し(`Fn*::call`)の候補を、レシーバの出所で絞る(XTALK、案 A。実装した: ADR-079、下の「実装」)

### 問題と、この文書に入れる理由

Rust の `Fn::call` の候補は、「引数の型が合う全クロージャ」(抽出器の `closure_candidates`)で、呼び出し元ごとに絞られない(`bench/MISSES.md` の `XTALK`。`internal/valuecalls/valuecalls.go` は `CalleeValue` だけを解く)。parseable では、ある 1 か所の `api_fn(ingestor)` の候補 15 個のうち、実際に届くのは 8 個(その関数の 8 呼び出し元が渡すクロージャ)だけで、159 件のうち 35 件の経路がこの 1 か所を通り、本物は 3 組だった(sink 7 個 × source 5 個。sink 4 個は全組が過検出、3 個は 1 組ずつ本物)。

**この文書(C-7 の Rust の節)に入れる理由:** 環境の `alloc`(型 `{closure@file:l:c: l:c}`)を「このクロージャの関数の値」と認める規則が、C-7 の Rust 分(`alloc` → クロージャの関数を引いて、作った場所で適用する)と、この絞り込みで**同じでなければならない**。別の規則を 2 つ作らない。規則は 1 つ: **`sg.Program.EnvClosures`(ADR-058)**。型の位置(`file:line:col`)から、そこにある 1 つだけの関数を引く。1 つに決まらない(マクロが同じ位置に複数書く)ときは引かない。**引けない `alloc` は「出所不明」であり、絞り込みを止める**(下の条件 1)。C-7 は `EnvClosures` で「作った場所で適用する」、案 A は `EnvClosures` で「レシーバの出所を数える」。

### 規則(案)

`Fn::call` / `FnMut::call_mut` / `FnOnce::call_once` の dynamic 呼び出しで、レシーバ `r` の**出所の集合 S(r)**を求める。出所は、次のどれか:
* `alloc` の型が `{closure@…}` で、`EnvClosures` が引ける → その関数
* `make_closure`(キャプチャなしの関数アイテム、関数ポインタを含む)→ その関数(`Value.FuncValue`、ADR-058)

**S(r) が「もれなく」求まったときだけ**、候補を `S(r) ∩ 今の候補` に絞る。それ以外は、**今の全候補のまま**(今の挙動。精度は下がらない)。絞り込みは、見逃しを作る方向にしか誤れないので、「もれなく」の判定を厳しくする。

#### 条件 1: もれなく数え上げられたときだけ絞る

出所を求める各段階と、「もれなく」と言える根拠:

| 段階 | もれなくの根拠 | たどれないとき(= 全候補に戻す) |
|---|---|---|
| レシーバが `alloc` / `make_closure` の結果そのもの | その命令が定義(SSA の定義は 1 つ) | `EnvClosures` が引けない型、`unsupported` の命令 |
| `copy`(`view` でない)/ 関数型どうしの `convert` | 引数は 1 つで、値は変わらない(`FuncValue` と同じ範囲) | `view` の copy、型を変える `convert` |
| `clone`(`<F as Clone>::clone`、`Fn` のクロージャは `Clone`) | 結果は引数と同じクロージャの複製。**`Clone::clone` の呼び出し先が、クロージャの型の組み込みの clone**(本体なし)のときだけ。ユーザーの `impl Clone` は、出所不明 | 型がクロージャでない、ユーザーの `Clone` |
| φ(分岐の合流) | 全ての入り口を数える(1 つでも不明なら不明) | 入り口の 1 つでも不明 |
| 環境のフィールドへの `field_store` → `field_load`(クロージャを別のクロージャや async ブロックの環境に入れて、中で呼ぶ形。`for_each_live_node` の `api_fn.clone()` → `async move { api_fn(ingestor) }`) | 環境の `alloc` に対する `field_store` を**全部**数える(その `alloc` の値が、他の関数に渡って書かれうるなら不明)。同じフィールド名への store の出所の和集合。**環境が関数の外に出ない(引数、戻り値、グローバル、フィールドに入れて返す、が無い)ときだけ** | 環境が外に出る、`&mut` 経由の書き込み(ENV-PTR、後述) |
| パラメータ → 呼び出し元 | 条件 2 | 条件 2 |

**戻り値(`let f = make();` の `make` の結果)、グローバル / `static`、トレイトオブジェクト(`Box<dyn Fn>` をフィールドに入れて別の関数が呼ぶ)、未知の関数(本体なし、モデルなし)の結果、`Vec` や `HashMap` の要素**は、たどらない → 出所不明 → 全候補。ここでたどれるように見えるものを、足さない(足すなら、別の ADR で、フィクスチャといっしょに)。

#### 条件 2: パラメータ由来のとき、呼び出し元を全部知っていること

レシーバが、ヘルパーの**パラメータ**(`for_each_live_node(…, api_fn: F)`)から来るときは、S(r) = 全ての呼び出し元が渡す実引数の出所の和集合。それが「もれなく」であるのは、**呼び出し元の数え上げが閉じている**ときだけ。次のどれかなら、**閉じていない → 全候補**:
1. ヘルパーが、**関数ポインタ / 関数値として使われている**(`make_closure` の引数や `convert` に、その関数が出る = `refs` に `arg` / `other` の参照がある)。
2. ヘルパーが、**トレイトのメソッドの実装 / トレイトの既定メソッド**である(`dyn` / ジェネリックで呼ばれ、呼び出し元が名前に出ない)。
3. ヘルパーが、**外から呼べる**(下の「公開」)。
4. 呼び出し元のどれかで、実引数の出所が条件 1 で不明。
5. **`valuecalls` の上限**(CORE-D: 候補が `most = 8` を**超える**、呼び出し元をたどる深さ `maxDepth = 3`)に達した。**上限は、全候補に戻す理由であって、足りない分を捨てる理由ではない**(上限を超えたら、絞り込みを止める。`for_each_live_node` は呼び出し元がちょうど 8 で、上限の縁にある。1 つ増えたら絞り込みが止まる。この縁は、設計の測定の項目に入れる)。

**公開(条件 2 の 3)の扱い(決まった: 閉じた世界。ユーザーには「新しい前提を 1 つ入れた」と報告する):** その関数は、`parseable` の `pub mod` の連なりの先の `pub async fn` で、外のクレートから呼べる。名前で直接呼ぶ 8 か所(関数ポインタとして渡す所も、ほかのファイルから呼ぶ所も無い)は、すべてクレートの中だ。`pub` を文字どおりに「開いている」とすると、parseable の 35 件は直らない。そこで、**起点のあるプログラムでは、読んだコードを閉じた世界とみなす**。

**根拠(言い方):** 今の「全候補」は、**読んだコードの中の、型が合う関数値の集合**だ。読んでいないクレートの呼び出し元が渡すクロージャは、今も候補に入っていない。読んだコードの呼び出し元を**もれなく**数えて絞っても、**「読んだコードの中で起きる流れ」は 1 つも失わない**(別の呼び出し元の流れは、その呼び出し元の自分の呼び出しで、自分のクロージャを通って見つかる)。失うとすれば、**読んでいない呼び出し元が、読んだコードの中の関数値を渡してくる**場合(外のクレートが `for_each_live_node(parseable::some_pub_fn)` と書く)だけで、**起点のあるプログラムには、その外の呼び出し元がいない**(外のクレートが、このバイナリを呼ぶことは無い)。これは `Reachable` と同じ前提(「読んだコードの外から、起点を経ずに呼ばれるものは無い」)で、`not_from_entry` と `graph path --from-entry` が、すでに置いている。

**閉じた世界とみなす条件(3 つ、全部):**
1. **起点がある**(`Reachable` の `starts`: `main` があり、それは変数の初期化ではない)**かつ、外から呼ばれるマークを持つものが無い。** 次があれば、**開いたまま**(全候補に戻す): `#[no_mangle]`、`extern "C"` の関数、`#[export_name]`、proc macro の crate、`cdylib` / `staticlib` / `dylib` の crate-type。**ライブラリ(起点なし)も、開いたまま**(exported が起点になる = 呼び出し元が外にいる)。マークは、抽出器が IR に出す(今は `exported` と `entry` だけ。`#[no_mangle]` / `export_name` / `extern` のマークと、crate-type を、`ir.Func` / `ir.Program` に足す。**IR の追加**。足されるまで、案 A は、マークの有無が分からないので、閉じた世界にしない = 全候補)。
2. **「読んだコードの呼び出し元をもれなく」の範囲を、広くとる:** (i) **読んだ依存のクレート**(`--no-deps` でない実行)の呼び出し元と、(ii) **テスト(`--include-tests`)の中**の呼び出し元を、数える(数えない = 見逃す、ではなく、数えて、渡されたクロージャも出所に入れる)。(iii) **関数アイテムとして値で使われた所**(`let f = for_each_live_node;`、`.map(for_each_live_node)`、`convert` を経た関数ポインタ)が 1 つでもあれば、**呼び出し元が閉じていない → 全候補**(`Reachable` と graph の `refs` の数え方に揃える: `refs` の `arg` / `other` の参照、`Value.FuncValue`)。
3. **前提の名前を、出力に残す:** `narrowed_from` に加えて、絞った不確実性に **`basis: closed_world_entry`** のフィールドを付け、1 文で言えるようにする: 「起点のあるプログラムとして読んだので、外からの呼び出しは数えていない」(`reachability_note` と同じ作り。全言語で同じ文字列。絞ったすべての呼び出しに付く。`basis` が無いのは、パラメータに由来しない絞り込み = レシーバが、その関数の中で作られているとき)。

**Dart の同じ規則(dart-pub の dynamic、レシーバが `make_closure`、257 件):** 同じ段(条件 1 の表。Dart は `make_closure` がキャプチャの値を持つので `EnvClosures` は不要)を使う。呼び出し元が閉じている条件は、**ライブラリ非公開**(名前が `_` で始まる。`library` の外から見えない)かつ、**tear-off されていない**(関数値として使われない = `refs` に参照がない)かつ、**メソッドなら、オーバーライドされない**(サブクラスの呼び出しが、名前に出ない)こと。公開の関数は、Rust の公開と同じ閉じた世界(上の 3 条件。Dart の「外から呼ばれるマーク」は、`@pragma("vm:entry-point")`、FFI の export とコールバック、ライブラリでの起点なし)に従う。Dart への適用は、Rust の実装と測定が済んでから。

#### 条件 3: 絞ったことを出力に出す

その dynamic な段の不確実性(`kind: dynamic_dispatch`)に、**絞る前の候補数と、絞った根拠**を残す。既存の `Uncertainty` は `Candidates`(絞ったあとの候補)、`Detail` を持つ。足すもの(出力の追加だけ。スキーマバージョンは上げない。ADR-052 / ADR-054 と同じ扱い):
* `Uncertainty.narrowed_from`(整数): 絞る前の候補数。絞っていない呼び出しには付けない。
* `Detail` に: 「候補を 15 → 8 に絞った(根拠: レシーバの出所。パラメータ `api_fn` の呼び出し元 8 か所が渡すクロージャ)」。
* finding の `dispatch_candidates`(段の最大の候補数)は、絞ったあとの数。summary の `dispatch_candidates`(区間の件数)も、絞ったあとで数える。絞った件数は、summary の新しいフィールド `dispatch_narrowed`(件数、絞る前の合計と絞ったあとの合計)に出す。
* 絞らなかった理由(もれなくなかった / 条件 2 の何番か)は、**不確実性にしない**(今と変わらない状態だから)。数だけ、`stats` に出す(`dispatch_not_narrowed: {open_callers, unknown_origin, limit}`)。上限(条件 2 の 5)に一致したものは、`stats` の数が、見えるようにする。

#### 条件 4: 検証

* **rust-curated:** `@source` / `@sink` の注釈を、**1 つも落とさない**(`compare.sh` の削除が 0、`@miss` が増えない)。`pass_closure.rs` の `@noisy`(`arg-a/b` の 40 など、16 か所・183 件)と、XTALK の `@fp`(12 件)が減る。減った分は、`README.md` と `MISSES.md` の数字と注釈を更新する(減るのは良い方向だが、増えるものが無いことを、1 件ずつ確かめる)。
* **parseable:** `compare.sh` で finding の削除が 0。`truth-5c20b8e4.json` に、本物の 3 組(公開の ID のない、第三者のコードの組。場所は書かない)を、**source との組で**足す(ラベルを、sink の位置だけでなく、source の位置も持つ形にして、組が残ることを見る)。過検出の側は、`@nosink` 相当(truth の `negative` に、sink 4 個の全組と、本物の sink への別の source)として足し、**絞ると 35 → 4 件前後になる**ことを、期待値として書く(残る 1 件は、下の ENV-PTR の例)。
* **「絞ると見逃す」形のフィクスチャ 3 つ**(`rust-curated` に足す。**3 つとも、絞らず、全候補のまま出ること**を、`@sink` の組が全部残ることで見る):
  1. **出所が 2 つあり、片方がたどれない:** `fn run(f: F) { f() }` を、`run(|x| sink_a(x))`(`EnvClosures` で引ける)と、`run(make_unknown())`(戻り値。たどれない)の 2 か所から呼ぶ。S は一部しか求まらない → 全候補。`make_unknown()` が返すクロージャの sink が、見つかり続けること。
  2. **`pub` なヘルパー(条件 2 の 3):** ライブラリのクレート(起点がない)で、`pub fn run_pub(f: F) { f() }` を、クレート内の 1 か所が呼ぶ。外の呼び出し元が渡すクロージャがあるかもしれない → 全候補(案(b)でも、起点がないので全候補)。同じ形で、起点のある `bin + lib`(parseable の形)のものは、案(b)では絞る、案(a)では絞らない、として、決めた方の期待値を書く。
  3. **クロージャをフィールドに入れてから呼ぶ:** `struct H { f: Box<dyn Fn(String)> }`、`H { f: Box::new(move |x| sink(x)) }` を作り、別の関数が `(h.f)(v)` と呼ぶ(`callbacks.rs` の `boxed` と同じ形)。出所は、フィールド越し(トレイトオブジェクト)→ たどれない → 全候補。`boxed` の `@sink` が、見つかり続けること。
* **絞る前の数と、絞った数を、実際に見る:** `for_each_live_node` が 15 → 8 になること(`narrowed_from: 15`、`candidates` の数が 8)。呼び出し元がちょうど 8(上限の縁)のとき、絞り込みが働くこと。9 個目の呼び出し元を足したフィクスチャで、絞り込みが止まり、全候補のまま出ること(条件 2 の 5)。
* **Go と Dart の finding が変わらない:** 同じ規則を Dart に適用するまでは、`compare.sh` で、go-curated、dart-curated、gogs、dart-pub の finding と `reachability` の変化が 0。

### 実装(ADR-079): 設計との違いと、測ったこと

`internal/valuecalls/narrow.go`(`Resolve` の最後。呼び出し元の索引は、絞る前の候補で作り、判断をすべて決めてから適用する)。出所の規則は、設計の条件 1 のとおり(`alloc` の環境 = `ir.ClosureIndex`(`sg.linkEnvClosures` と同じ 1 つの規則。`sg/env.go` をこれに寄せた)、`make_closure`、`copy`、`convert`、φ、`clone`、環境のフィールドの `field_store`(フィールドの名前がクロージャの名前を持つので、どの関数の store も数える。アドレスを取って書く使い方があれば、たどらない)、`&f` のセル(`store` / `load` / `copy` / `Fn::call` のレシーバだけで、ほかに渡らない)、パラメータ → 呼び出し元)。**たどれないものは全部、全候補に戻す**(関数の結果、フィールド、グローバル、`Box<dyn Fn>` を作る呼び出し、未知の命令)。

**設計に足したこと(3 つ):**
1. **`Func.private`(IR の追加)**: ソースに `pub` が無い関数(`fn name` で始まる)。呼び出し元は、その crate の中だけ。**起点の無いライブラリでも、private なヘルパーは絞る**(zoo は `lib` で、ヘルパーは private)。`pub` なヘルパーは、起点がある(閉じた世界)ときだけ絞る。マクロが書く関数や、ソースを読めないものは、private としない(見えた分だけ)。
2. **`Func.external_entry` と、`caller_marks` の記録(IR の追加)**: `#[no_mangle]`、`#[export_name]`、`extern "C"`(シンボルの名前が、rustc の名前でないもの、または Rust でない ABI)、`cdylib` / `staticlib` / `dylib` の crate の全関数。**1 つでもあれば、閉じた世界にしない**。`caller_marks` は、「この抽出器は、上の 2 つのマークを付ける」という記録(古い抽出器の IR は、マークが無いのではなく、知らないので、閉じた世界にしない)。複数の言語を 1 つのプログラムに合わせたときも、閉じた世界にしない。
3. **ガード(ADR-045)**: 絞った呼び出しで、候補が 1 つのパラメータだけを通ってきたとき(`Callee.CandParam`)、`valueGuards` と同じガード「パラメータ p に渡された関数がこの候補」を付ける。呼び出し元が自分で作ったクロージャを渡していれば、ほかの候補を通る流れを除去できる。**ガードは、レシーバがパラメータでなくセルの中(`f(s)` の `impl Fn`: Fn::call のレシーバは `&f` のセル)でも付く**(今までは、`dispatchGuards` が、レシーバがパラメータそのものでないと付けなかった)。`ExactFuncs` は、環境のあるクロージャを「分からない」とするので、ガードの判定は、`ExactClosures` の ID で比べる。絞り込みで候補が減らなくても(全部が呼び出し元から届く)、ガードは付く(`two_callers` のデコイ)。

**測ったこと(rust-curated の既存の部分、zoo、本体を変えないフィクスチャ):**

| | 前 | 後 |
| --- | --- | --- |
| rust-curated `@fp` | 18 | 7(`fnself.rs` の 6 件: ADR-047 の受け入れが達した。`pass_closure.rs` の `arg-a/b`、`sinkwith-a/b`、`structs.rs` の `emit-two`(2 つ目)) |
| rust-curated `@noisy` | 152 流れ(16 か所) | 84 流れ(3 か所: `fire_and_forget` の 2 つと `shared::each`。残りは、`Runner::go`、`call_dyn`、`Vec<Box<dyn Fn>>` など、たどれない側のヘルパーから来る) |
| zoo(Rust)の `@fp` / デコイの誤検出 | 21 / 9 of 60 | 19 / 8 of 60(`clos/two-callers` の 1 件。`clos/relay` は残る: ガードが中継を通らない) |
| 本物の流れ(`@source` と `@sink` の組) | rust-curated 142、zoo 112 | 142、112(1 つも落ちない) |

rust-curated は、`src/main.rs`(`fn main() {}`)を足して、起点のあるプログラム(`bin + lib`。parseable の形)にした。起点の無い側(`bench/rust-curated-lib`)と、外から呼べる側(`bench/rust-curated-ffi`: `main` と `#[no_mangle]`)は、別の小さな crate で、`pub` なヘルパーは絞らず(`@fp` が残る)、private なヘルパーは絞る(検出されない)ことを、`TestCuratedRustOpen` が固定する。「絞ると見逃す」形(`src/narrow.rs`): (1) 出所が 2 つで、片方がたどれない(キャプチャのあるクロージャを返す関数)、(2) クロージャをフィールドに入れて呼ぶ、(3) 中継、(4) 呼び出し元が 8 つ(上限の縁: 候補 10 → 8 に絞る)と 9 つ(`most` を超える: 絞らない)。3 つとも、`@sink` が全部見つかる。

**分かったこと:** キャプチャの無いクロージャは、関数の結果でも、型から作る値(`alloc` の環境)になるので、その値は、型で決まるクロージャそのもの(出所が分かる)。キャプチャのあるクロージャを返す関数の結果は、呼び出しの結果なのでたどらない。`&|..|` で `&dyn Fn` にしたキャプチャの無いクロージャは `const nil` になる(`pass_closure.rs` の `dyn_ref_a`)ので、たどれず、全候補のまま。

**今回の実装で入れなかったこと:** (1) 衝突した関数の名前(抽出器の `merge` が 1 つに潰す)に呼び出し元が入っているかの確認(parseable で衝突は `tracing` / `clap` / `serde` の生成物の名前だけだった)。名前を全部 `load_error` に出す変更は、別に。(2) 呼び出し元のガードが中継(`wrapper(g)` → `run(g)`)を通ること。(3) `Box<dyn Fn>` を作る呼び出しの先(`Box::new(closure)`)をたどること。(4) `async` の本体(ADR-085 でたどれるようにした。呼び出し元ごとのガードは入れていない: docs/DESIGN-async-dispatch.md)と、クロージャのクロージャの環境(たどれない → 全候補)。(5) Dart への適用(下)。

**Dart(ADR-075 の限界 3: レシーバの静的な型の全実装が候補)は、同じ根か:** 同じ「根」ではないが、同じ「型」の問題。Rust の `Fn::call` は、候補が「型の合う全クロージャ」で、**レシーバの値の出所**で絞る。Dart の dynamic な呼び出し(`StringSink` の自前の実装 4 つ)は、候補が「レシーバの静的な型の全実装」で、**レシーバのクラス**(`make_interface` 相当のオブジェクトを作った所)で絞れる。どちらも、「レシーバに流れ込む値の出所を、もれなくたどれたときだけ、候補を絞る」(条件 1。呼び出し元のパラメータは、条件 2)で、`valuecalls` の `classify` / 呼び出し元の索引は、言語に依らない。違いは、出所の種類(Dart は、クラスのインスタンスを作る `alloc` / コンストラクタの呼び出し。Rust は、クロージャの環境)と、「閉じた世界」のマーク(Dart は、`_` で始まる名前、`@pragma("vm:entry-point")`、tear-off)。そのため、**この文書の条件 1〜3 と `closed_world_entry` を、そのまま Dart に使える**。実装は、別の機会(Rust の測定が済んだので、次)。Go は、すでに VTA の呼び出しグラフで絞られている。

### ENV-PTR との関係(環境のフィールドを分ける話): 例として、parseable の 1 件

残る 1 件(parseable の、クロージャが 2 つの値をキャプチャする形。場所は書かない)は、**候補の絞り込みでは直らない**。クロージャは、片方を URL に、もう片方を本文にだけ使う 2 つの値をキャプチャする。しかし、環境の `alloc` を **1 つの値**として扱うので、`dates` の汚染が、`stream_name` と同じ環境から取り出される値(`url` の元)に混ざる。これは、環境のフィールドごとに(`field_store` の `"0"`、`"1"`)別の場所として扱えば、直る。**ENV-PTR(環境のフィールドの、別名と、指す先。`locs.go:215`、`transfer.go:148`)と同じ根:** 環境のフィールドが、構造体のフィールドと同じ精度で扱われていない。issue #82 に、この 1 件を例として足す(書く側 7 件の `@miss(silent)` と、読む側の混ざり 1 件が、同じ原因)。ENV-PTR を直すと、この 1 件も消える見込み(ADR-072 で、環境のフィールドは、クロージャの名前と番号で分けた。parseable での結果は ADR-072 に書く)。**案 A の効果の見込み(35 → 4)と、ENV-PTR の効果(4 → 3)は、別の変更として測る。**

### 呼び出し元の数え上げの前提: 抽出器が、同じ名前の別の関数を 1 つに潰す(確かめた)

Rust の抽出器(`extractor/rust/src/main.rs` の `merge`)は、クレートごとの出力を 1 つにまとめるとき、**同じ id の関数が 2 つあり、内容が違うと、片方を落とす**(行の小さい方を残し、`load_error` に「two crates have a function of this name and they differ」と出る。同じパッケージの `lib.rs` と `main.rs` は、既定で crate 名が同じ)。呼び出し元の数え上げ(条件 2)が、**落ちた方に呼び出し元がいると、狂う**。

**parseable で確かめた(IR の `load_error`、抽出器の `take(20)` を外して全部出した):** 860 件の衝突。名前は、`__CALLSITE::META`(`tracing` のマクロの静的変数)、`DEFAULT_VALUE`(`clap`)、`serde` の derive の `__FieldVisitor` / `__Visitor` の関数、の 3 種類**だけ**(`for_each_live_node`、その 8 呼び出し元、`cluster` / `catalog` の関数は、1 つも入っていない)。**呼び出し元 8 か所の数え方は、狂っていない。**
ただし、(1) `load_error` が名前を出すのは**最初の 20 件だけ**(残りは件数のみ)なので、読む人は、自分の関数が衝突に入っているかを、知れない、(2) `lib` と `bin` の両方が同じモジュールを `mod` で持つクレートでは、本物の関数が失われる。

**設計に入れる:** 条件 2 に、次を足す。**衝突した名前の集合に、ヘルパーか、その呼び出し元のどれかが入っているときは、絞らない**(全候補)。そのために、**`load_error` の名前を、全部出す**(20 件の上限を外す。件数が多いなら、別の `kind: duplicate_functions` の 1 レコードに、名前の配列を持つ)。実装の前に、この上限を外す変更を、先に入れる(出力の追加)。

### CORE-M: `valuecalls` が、一部の呼び出し元しか追えないとき、見つかった分を全部として扱う(案 A の前に塞ぐ)

**抜け:** `internal/valuecalls/valuecalls.go` の `funcsOf` は、値の出所が `refOther`(フィールドの読み出し、戻り値、グローバル、未対応の命令。`classify` の既定)に該当すると、**何も報告せずに戻る**。見つかった関数(`found`)が 1 つ以上あれば、それが全部として `ValueCands` になる(`found` が 0 のときだけ `unresolved_callee` / `ValueLimit` が出る)。`r.cut` も、`found` が 0 のときにしか見ない。パラメータの呼び出し元をたどるときの、引数の数が足りない(`k >= len(cin.Args)`)と、`form` が作れない呼び出し元も、何も報告せずに `continue` する。**つまり、案 A の条件 1(全部たどれたときだけ絞る)が、今の `valuecalls` で、すでに破れている。**

**再現(確かめた):**
* **Dart: 再現した。** `apply((s){}, 'x')`(たどれる)と `apply(h.cb, source())`(フィールドの読み出し = `refOther`)の 2 呼び出し元があるヘルパー `apply(f, s) { f(s); }`: 候補は前者 1 つだけになり、**finding も不確実性も出ない**(`h.cb` の中の `sink(s)` を見逃す)。`apply2((s){}, 'x')` と `apply2(pick(), source())`(戻り値)も同じ。**対照**: たどれない呼び出し元だけの `apply3(pick(), source())` は、`unresolved_callee` が 1 件出る。つまり、**全部がたどれないときは報告し、一部がたどれないときは報告しない**。再現の入力: `rev/taintB/d1/lib/v.dart`(コーディネーターのスクラッチパッド)、出力 0 件、`summary.program` の不確実性は `apply3` の 1 件のみ。
* **Go: 再現しなかった。** 同じ形(`apply(func(string){}, "x")` と `apply(h.cb, source())`、`apply2(mk(), source())`)で、finding は 2 件とも見つかる。Go の抽出器が、VTA の呼び出しグラフの候補を `Callee.Candidates` に入れるので、`valuecalls` は `funcsOf` まで来ない(「The extractor's call graph says」の枝)。**ただし**、Go でも候補が付かない呼び出し(抽出器の呼び出しグラフが解かないもの)では、同じ抜けに当たりうる(未確認。Go のフィクスチャを、`valuecalls` の単体テストで固定する)。
* **Rust: 再現が不確か。** `fn apply(f: fn(&str), s: &str) { f(s) }` を、`apply(noop, "x")` と `apply(h.cb, &source())`(`h.cb` は構造体のフィールドの関数ポインタ)で呼ぶと、finding は 0 件、不確実性も 0 件(`a` の呼び出しを外して、`b` だけにしても 0 件・0 件)。**部分的にたどれたときの、サイレントな見逃し、とは断定できない**(たどれない呼び出し元だけのときも、不確実性が出ていない。別の原因がある可能性がある。Rust の `f(s)`(関数ポインタ)の呼び出しが `valuecalls` を通るかを、先に確かめる)。Rust の `Fn::call`(dynamic)は、そもそも `valuecalls` を通らない(`CalleeValue` だけを解く)。

**修正方法(案):**
1. `funcsOf` が、**「不完全」のマーク**を返す: `refOther` に該当した、呼び出し元の引数が足りない、`form` が作れない、`maxDepth` で切った(`r.cut`)、のどれかで、`incomplete = true`。`found` があっても、`incomplete` なら、
   * **不確実性を出す**(`unresolved_callee`。detail に「一部の呼び出し元の値は、たどれなかった。たどれた分は n 個」)。`ValueLimit = "partial"`、`ValueFound = n` を残す(`candidate_limit` と同じ作り)。
   * **見つかった分は、候補に残す**(finding を減らさない。見つかった分で今見つかっている流れを、消さない)。ただし、**「全部」ではないマーク**(`ValueCand` を持つ呼び出しの `Incomplete`)を付け、案 A の絞り込みは、このマークのある呼び出しを使わない。**見つかった分だけを「全部」として扱うのが抜け**なので、これで、(i) 見逃しをサイレントにしない(不確実性が付く)、(ii) 今ある finding は消さない、(iii) 案 A の絞り込みが、見つかった分だけに絞るのを止める、の 3 つを満たす。
2. **候補を空にして、`unresolved_callee` だけにする案は採らない**: 今、「見つかった分」で見つけている finding が消える。本物でも、全部がたどれた上での絞り込みではないが、finding の削除は、検証(`compare.sh`)で止める対象になる。第 1 段階は、finding を減らさず、不確実性を足す。マークのある呼び出しの候補を、型の合う全関数(Rust の `Fn::call` の候補、Dart の全 `make_closure`)に広げるのは、別の判断(精度を下げずに見逃しを減らす方向だが、XTALK を増やす)。
3. 案 A の条件 1・2 は、この `incomplete` を使う: **`incomplete` のある呼び出しは、絞らない**。CORE-D の上限(`most`、`maxDepth`)も、同じマークで、全候補に戻す。

**順序:** CORE-J、CORE-K、C-7 のあと、**CORE-M → 案 A**(案 A の絞り込みは、`incomplete` のマークが無いと、条件 1 を守れない)。**CORE-M の検証:** Dart の再現の入力を、`dart-curated` に足す(`@miss(unresolved_callee)` ではなく、`@sink` が見つかり続け、不確実性が付く)。`dart-pub`、go-curated、dart-curated、rust-curated の finding の追加・削除が 0、不確実性は増える(増えた分が、`incomplete` の呼び出し)。

### 決めてほしいこと(実装に入る前に。実装は CORE-J / CORE-K / C-7 のあと、CORE-M → 案 A)

1. **(決まった: 案(b)。条件つき。上の「閉じた世界」)**
2. `valuecalls` の上限(`most = 8`、`maxDepth = 3`)を、この絞り込みで上げるか(上げないなら、`for_each_live_node` は、上限の縁。1 つ増えたら絞り込みが止まる)。CORE-D の判断と同じ。
3. 出力の追加(`narrowed_from`、`dispatch_narrowed`、`dispatch_not_narrowed`): 追加だけで、スキーマバージョンを上げないでよいか。
4. Dart への適用の時期(Rust の測定のあと)。

## Rust の節(次の段階: 実装していない)

最初のバージョンは Go と Dart(`make_closure`)。Rust のキャプチャのあるクロージャは `make_closure` ではなく、**環境の構造体の `alloc`** で作られる。同じ規則(使われ方が、ホーム関数内の直接の呼び出しだけなら何もしない。それ以外は、作った場所で、キャプチャした値を、クロージャの要約に適用する)を、この `alloc` に載せる。仕組みの推定(main の `sg.Program.EnvClosures`、ADR-058 の上で):

* **形**: `alloc`(`InstrInfo.Env` が型 `{closure@file:l:c: l:c}`)で環境を作り、キャプチャした値を `field_store`(field `"0"`, `"1"` …。可変のキャプチャは、変数へのポインタ)で入れる。クロージャの関数は、パラメータが 1 つ(その環境、`recv: true`)で、`field_load` で取り出す。`sg.Program.EnvClosures[alloc の命令]` が、そのクロージャの関数(型の位置が、関数の位置と、ちょうど 1 つ一致するとき)を引く。
* **適用する**: `captured` の相当を、`EnvClosures` にある `alloc` に足す。`applySummary(cc, closureFn, nil, notes, nil)`、`argv = [環境の値]`(束縛ではなく、Params の 1 つ目。`make_closure` と違い、キャプチャした値は環境のセルの中にあって、クロージャの関数は `field_load` で読む)。`Params` は環境 1 つだけなので、呼び出しの引数(宣言した引数)は要約の中で、環境と別のパラメータとして出る(その分は `KUndef`)。環境の `field_store` が `alloc` の後に来るが、解析は流れに依らない(セルの和)ので、順序は問題にならない見込み(**確かめていない**)。
* **使われ方(飛ばす条件)**: `alloc` の結果(環境の値)の使われ方を、同じ表(`sg` の使用箇所)で見る。呼び出し(`Fn::call` の dynamic 呼び出し)の位置、または、`field_store` / `field_load` だけなら、ホーム関数内の直接の呼び出し。`field_store`(環境への書き込み)は、環境を作る側の使い方なので、**数えない**(`make_closure` には無い、Rust 固有の例外)。環境の値が、引数、`store`、`return`、別の `alloc` の `field_store` の値に入るなら、適用する。
* **不確実性の注記**: 同じ `assumed_call`(`attributable_to: mixed`)。
* **効く範囲(`rust-curated/src/callbacks.rs`)**: 渡すだけの形の 3 件(`spawned` = `std::thread::spawn`、`boxed` = `Box<dyn Fn>` をフィールドに入れて別の関数が呼ぶ、`retained` = `Vec::retain`)。読む側(`run(|| sink(&a))`)は `Fn::call` の候補で今も見つかる(XTALK の過検出つき。`run-a/b` は `@fp`)。書く側(`write-a/b`、`push-list` など 7 件、ENV-PTR)は、環境のフィールドのポインタが、指す先と別名にならない(`locs.go:215`、`transfer.go:148`)ことが原因で、**`alloc` で適用しても直らない**。ENV-PTR の修正が先(ADR-072 で直した。書く側 7 件のうち、`write-a`、`write-b`、`write-param`、`push-list`、`late`、`for-each-cap` が見つかる。環境のフィールドは、クロージャの名前で呼ぶので、候補どうしが環境のセルを混ぜない)。
* **気をつけること**: 型 `{closure@file:l:c}` が、マクロで複数の場所に書かれるクロージャは、`EnvClosures` が引けない(ちょうど 1 つ一致のときだけ引く)。そのクロージャは、今と同じく、何も報告せずに失われる。`assumed_call` ではなく、引けなかったことの不確実性の注記が必要(次の段階で決める)。`thread::spawn` と `retain` のクロージャは、`Fn::call` の候補にもなり、他のソースが誤って届く(`@noisy` 8 と 3)ことは、この段で直らない(XTALK)。
* **受け入れ(次の段階)**: `callbacks.rs` の渡すだけの 3 件が見つかる。`cap-run` 相当の「その場で呼ぶ」クロージャで、finding が増えず、`assumed_call` も出ない。`parseable` で、追加の finding を分類する。
