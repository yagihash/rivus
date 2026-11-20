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
| throw(`OutThrow`) | **当てない(最初の版)** | throw は、呼ばれたところから抜ける。`make_closure` の場所に流すと、作った関数が throw することになり、過大。`try { run(() => throw x) } catch` は、まだ見逃す。決める点(下の「決めてほしいこと」) |
| 戻り値(`OutResult`) | 当てない | 下の(2) |

実装は、合成した `callCtx`(`in` は `make_closure` の命令、`recv=nil`、`args` は Params と同じ数の `KUndef`、`roots=nil`)と `applySummary(cc, closureFn, mc.Args, notes, nil)`。`applySummary` の `argv = cc.argv() + bindings`(Params の後ろに FreeVars)は、そのまま使える。**`summary.go` の中は変えなくてよい**(`len(bindings) == 0` のときだけ不足を `KUndef` で埋める分岐は、束縛を渡すので通らない。Params を呼び出し側で埋めるので、不一致のエラーにならない)。

### (2) 当てないもの

* **戻り値**: クロージャの結果は、呼ぶところで受け取るもの。`make_closure` の場所に、呼ぶところは無い(行き先が無い)。`cc.roots = nil` にして、`OutResult` の `o.Index < len(cc.roots)` を偽にする。
* **引数に依る流れ**: クロージャの引数(Params)から sink への流れ(`runArg(a, func(s){ sink(s) })`)は、呼ぶ側の値(ヘルパーが渡す)に依る。`KUndef` なので何も運ばない。これは、今のとおり、呼ぶ側(ヘルパー内の `f(x)` の候補)で見つかる。**C-7 は、捕獲した値だけに依る流れを足す。引数に依る流れは足さない**(これは案 2 = ADR-047 の仕事)。
* 捕獲した値と引数の**両方に依る**流れ(`sink(x + a)`): 要素は 1 起点ずつなので、`a` の分は C-7 で、`x` の分は呼ぶ側で、それぞれ見つかる。

### (3) 呼ぶところが見えているクロージャと、二重になったとき

「見えている」= 作った関数 `home` の中で、**束縛つきで呼ばれる**: その場で呼ぶ(`go func(){}()`、`defer`、`func(){}()`、`f := func(){…}; f()`: `Instr.Bindings`、`sg/build.go:84-90`)、または、`home` の中の関数値の呼び出しの候補として解決済み(`CandidateBindings`、`resolveValueCalls`)。

**案: 見えているクロージャは、`make_closure` で当てない**(最初の版)。理由: 今の finding の経路・数が、**1 件も変わらない**ことを検証の基準にできる(増えるのは、今まで見えなかったクロージャの分だけ)。見えているクロージャは、今でも見つかっている。

* 実装: `sg` の `resolveValueCalls` と、静的な束縛の設定(`build.go:84-90`)で、その `make_closure` の命令に、印(`Instr.Seen bool`、`sg` のフィールドを 1 つ足す)を付ける。taint は、印のある命令を飛ばす。
* 取りこぼし: 「見えている呼び出しが 1 つあり、同じクロージャを別の関数にも渡す」(`f := func(){…}; f(); other(f)`)は、`other` の中の呼び出しが見えない。印があるので、当てない(今と同じ、見逃す)。まれ。印を「脱出しない」(使われ方が、呼び出しのみ)に絞れば当てるが、最初の版は取らない。

**二重になる場合**(全部のクロージャに当てる案、または、上の取りこぼしを直す版): finding は `(origin, sink)` で 1 つにまとまる(`state.recordFinding` の `findingKey{origin, sink}`、`engine.collect` の `merged[k]`)。経路は、**ステップ数の少ないほう**が代表になる(`rf.steps.len() < cur.steps.len()` が `<` なので、同じ長さなら、最初に記録したほうが残る)。`make_closure` の経路は、ヘルパーを通らないので、たいてい短く、**こちらが代表になる**。代表が変わると、今ある finding の経路が変わる(finding の有無は変わらない)。`mergeParamSources` は、source が param の finding だけの、別の併合(関数をまたぐ source の同一視)で、`make_closure` の経路には関係しない。**最初の版で、見えているクロージャを飛ばす理由が、これ**。

### (4) 痕跡(不確実性)

呼ぶところが見えないクロージャに、捕獲の値を当てたことを、不確実性で残す。

* kind: **`unresolved_callee`**(既存。`attributable_to` は `mixed`: 呼ぶところが、読んだコードの外(ライブラリ)にあるのか、見えていないだけなのか、分からない)。新しい kind は足さない(`result.go` の kind の一覧、出力のスキーマ、ドキュメント、`cases_test.go` の kind ごとの期待を変えなくてよい)。
* 文面(`detail`): `the closure is made here and called where it cannot be seen (it is given to another function, or stored): what it captured is applied at the place it was made, and it is taken to be called`
  日本語の意味: このクロージャが呼ばれるところは見えていない。作った場所で、捕獲の値を当てた(呼ばれるものとして扱っている)。
* `symbol` は、クロージャの関数 ID。`position` は `make_closure` の位置(`graph` の `refs` の `closure` と同じ位置)。`func` は作った関数。
* **出す条件**: クロージャの要約が、捕獲した値に依る効果(FreeVar 由来の sink への到達、書き込み)を**持つ**ときだけ。捕獲があるだけで毎回出すと、Gitea で数千件の不確実性になる(下の測定: 捕獲のあるクロージャ 1,496)。効果を持つかは、`sum.Sinks` と `sum.Outputs` の要素が FreeVar の `ParamElem` を含むか、で決まる(`applySummary` の前に調べる)。
* finding の経路: `applySummary` の `notes` 引数に渡す。経路のステップ(`call.Notes`)と `w.setNotes` に載る。finding の `uncertainties` に出る。
* 重複: `engine.collect` が `kind|symbol|func|pos` で同じものを 1 つにする(`seenU`)。

痕跡が付くので、`@miss(silent)` が、フィクスチャの上で、「改善」として落ちる(流れが見つかるので `@miss` ごと消える)。

### (5) 失うもの・増えるもの

* **増える**: 見えないクロージャの中の、捕獲した値 → sink の流れ。**1 度も呼ばれないクロージャの中の流れも、finding になる**(過検出)。呼ぶ順序も見ない(クロージャが、捕獲した変数を書く前に呼ばれる、など)。`go-curated/api/capture.go` の `capNever` が、その形(今は静か。`@sink(none)` が、実装後に落ちる。受け入れの基準は、**`capNever` を `@fp` にするか、規則の限界として受け入れるか**を、ユーザーが決める)。
* **失うもの**: 無い見込み(当てるものは足すだけで、`summary.go` の既存の動作は変えない)。ただし、経路が変わる finding がある(上の(3)、見えているクロージャを飛ばせば無い)。
* **増える量の見積り**(Gitea、構文の上の近似、下の「測定」): 捕獲のあるクロージャ 1,496 のうち、その場で呼ばれない(引数、代入、return、リテラル)ものが 1,334(89%)。そのうち、`.Group`(ルーターの登録)413、`.WithTx` 189 + `.WithTx2` 34。ルーターの `Group(path, func(){ m.Get(...) })` の中身は、呼ばれるところが見えている(ライブラリが即座に呼ぶ)ので、過検出にならないが、`WithTx` 以外にも多い。
* 計算量: 捕獲のあるクロージャ 1 つにつき、`applySummary` 1 回(`callSig` で、変化の無い繰り返しは飛ばす)。Gitea で約 1,300 回。要約が大きいクロージャで、時間が増える可能性は、実装して測る(精度を優先する方針: 結果が正しいことが先)。

### (6) 入れ子のクロージャと、ループの中のクロージャ

* **入れ子**(外のクロージャの捕獲を、内のクロージャが捕獲する): 内の `make_closure` は、外のクロージャの本体の中にあり、その引数は、外のクロージャの FreeVar。外のクロージャの要約を作るとき、内の `make_closure` を当てると、FreeVar に依る記号的な sink 到達(`noteSinkReach`)になる。外の `make_closure`(作った関数 `home`)で、それが具体化される。**要約の合成で、段数に依らず届く**。`capNested`(go-curated)がそのフィクスチャ。外のクロージャが見えている(その場で呼ぶ)場合に、内のクロージャが見えていないときは、内を当てる(印は、命令ごと)。
* **ループの中のクロージャ**: `make_closure` の命令は 1 つ。解析は流れに依らない(セルの和)ので、ループの何周目かを区別しない。捕獲した変数がループ変数(Go 1.22 以降は周ごと)でも、セルは 1 つで、届く。`capLoop` がそのフィクスチャ。ループで変数を書き換える形(`x += c.Query(..)`)は、セルの和で届く(過大側)。

### (7) Rust と Dart

**Dart**: `make_closure` に捕獲の値が付く(確かめた: `run_a` の IR は `make_closure [1] package:curated/pass_closure.dart#run_a$1`、クロージャ側は `param` の命令の `index` が宣言した引数の数以降なら FreeVar、Go と同じ規則。`sg/build.go:180-190`)。**そのまま同じ実装で足りる**(`Button(onPressed: …)`、`Timer.run`、`scheduleMicrotask`、`Future.delayed` は、全部、クロージャの作られる場所が同じ形)。フィクスチャ: `dart-curated/lib/callbacks.dart`(7 件が今 `@miss`)。

**Rust**: 捕獲のあるクロージャは **`make_closure` ではない**。確かめた(`rivus-rust` の出力の IR、`write_a`): 環境の構造体を、`alloc`(class `func`、type `{closure@src/capture.rs:9:9: 9:11}`)で作り、捕獲した値を `field_store`(field `"0"`, `"1"`…。可変の捕獲は、変数へのポインタ)で入れる。クロージャの関数は、**パラメータが 1 つ(その環境、`recv: true`)**で、`field_load` で取り出す。呼び出しは、`Fn::call` の dynamic 呼び出し(候補は、同じ引数の型を持つ全クロージャ = XTALK)。

* **できるか**: 同じことは、**別の段**で、できる。環境の `alloc` の type(`{closure@file:l:c}`)と、クロージャの関数の `params[0].type` が一致するので、alloc → クロージャの関数が引ける(extractor は変えなくてよい見込み。型の文字列が、ファイルと位置を含むので、一意)。`applySummary(cc, closureFn, nil, notes, nil)` を、`argv = [環境の値]`(束縛ではなく、Params の 1 つ目)で当てる。ただし、環境の `field_store` が `alloc` の後に来る(流れに依らない解析なので、順序は、問題にならないはずだが、**確かめていない**)。
* **効く範囲**: 読む側(`run(|| sink(&a))`)は、`Fn::call` の候補で、**今でも見つかる**(XTALK の過検出つき。`run-a/b` は `@fp`)。C-7 は、これを直さない(XTALK は別の原因。案 2 の仕事)。`rust-curated/src/callbacks.rs` の 3 件(`returned`、`nested_cap`、`param_clean`)と `never_called` の静かな側は、**今、見つかっている**(`@miss` が無い)。書く側(`write-a/b`、`push-list` など 7 件、ENV-PTR)は、環境のフィールドのポインタが、指す先と別名にならない(`locs.go:215`、`transfer.go:148`)ことが原因なので、**alloc で当てても直らない**。ENV-PTR の修正が先。
* 結論: **Rust は、C-7 の対象にしない**(最初の版は Go と Dart だけ)。Rust の別の段は、(a) ENV-PTR の修正(書く側 7 件)、(b) 環境の alloc で当てる(読む側の XTALK に依らない流れ、`thread::spawn` のような、呼ぶところが見えないもの)。(b)は、効果の測定が先(Rust の測定は別のエージェントが進行中)。

### (8) 案 2(ADR-047 の下書き: scratchpad/scc/adr-draft.md)との関係

どちらも、「**作った関数での束縛**」(`make_closure` の引数)を使う。案 2 の「呼び元 K が、自分で渡したクロージャ(`ExactFuncs`)の要約で、ヘルパーの記号的な出力を具体化する」には、クロージャの要約に**束縛も**必要(下書きに書いていない。C-7 と同じ情報)。そこで:

* `sg.ExactFuncs` を、ID の集合だけでなく、**`make_closure` の命令(束縛)も返す**ようにする(`ExactClosures`)。C-7 は、これを使わない(`make_closure` の命令を直接見る)が、案 2 が使うので、同じ補助関数にしてよい。
* **分担**: C-7 = 捕獲した値だけに依る流れ(作った場所で当てる)。案 2 = 引数に依る流れ(ヘルパーが渡す値。呼ぶ側のコンテキストで具体化)。クロージャの要約の要素を、FreeVar 由来か Params 由来かで分ければ、**二重にならない**: 案 2 の K での具体化は、Params 由来の要素だけを当て、FreeVar 由来は C-7 に任せる。案 2 が先に入った場合は、K で束縛つきで当てるので、`make_closure` の「見えている」印が、K の束縛でも付く(印が付くクロージャは、C-7 が飛ばす = 二重にならない)。
* 順序: **C-7 は案 2 に依らない**。案 2 は SCC の構造(要約のデータ型、`applySummary` の展開)を変える大きな作業。C-7 は、小さく、独立に入れられる。案 2 を待たない。
* 案 2 は SCC の判定から、ヘルパー → クロージャの辺を外す。C-7 は、辺に触れない(クロージャの要約を使うだけで、`order` / `levelsOf` / `callers` は変えない)が、**`make_closure` の命令が、クロージャの要約に依る**ので、「クロージャの要約が変わったら、作った関数を dirty にする」辺が、**1 本増える**(クロージャ → home)。home が、クロージャを(直接でも間接でも)呼ぶ関数に呼ばれる場合、循環ができる可能性がある。実装時に、`engine` の依存(`callers` の作り方)を読んで、`make_closure` → クロージャの要約を、`callers` に足す。

### (9) 実装の場所の見立て

| 変えるところ | 内容 |
| --- | --- |
| `internal/sg/sg.go` の `Instr` | `Seen bool`(その `make_closure` が、`home` の中で束縛つきで呼ばれる)を足す |
| `internal/sg/build.go:84-90`(静的な束縛)、`:362` `resolveValueCalls` | 束縛を付けるとき、`make_closure` の命令(`mc`、`fn.instrAt(...)`)の `Seen` を true にする。`instrAt` は既にある |
| `internal/taint/transfer.go:236` の switch | `case ir.OpMakeClosure:` を足し、`s.captured(in)` を呼ぶ |
| `internal/taint/calls.go`(新しい関数 `captured`) | `in.Args` が空、`Seen`、クロージャの `sg.Func` が無い(`ByID`)、要約に FreeVar 由来が無い、のどれかなら何もしない。それ以外は、`callCtx{in: in, args: Params 数の KUndef, roots: nil, sym: ""}` で `applySummary(cc, fn, in.Args, notes, nil)`。`notes` は、(4)の不確実性 |
| `internal/taint/engine.go`(依存) | 要約の依存の辺(クロージャ → home)を足す(上の(8)の最後) |
| `internal/taint/summary.go:283-286` | **変えない**。(任意: 束縛が無い非 home の呼び出しで、FreeVars を `KUndef` で埋めるときに、不確実性を出す。C-7 が入れば、捕獲した値の分は届くので、残る欠落は、引数に依る分だけ) |
| `internal/valuecalls/valuecalls.go:319` | **変えない**。`ExactClosures`(案 2)は、ここではなく `sg.ExactFuncs`(`sg/build.go:393`)の拡張 |
| `internal/graphdb/write.go:261` | **変えない**(`refs` の `closure` は、構造の層。taint の変更と無関係) |
| `docs/DECISIONS.md` | ADR(次の空き番号)。ADR-045 の限界(「パラメータ由来の候補は、呼び出し側ごと」の下の、束縛が home でだけ付く点)に、「捕獲した値は C-7 が当てる」と追記 |

`rivus resolve` の「見つからない理由」(source / sink の関数にある不確実性): C-7 の不確実性は、作った関数(`home` = source の関数のことが多い)に付くので、**流れが見つかっていれば**、`resolve` に出る必要は無い(見つかる)。見つからないとき(捕獲した値が、sink に届かないとき)は、効果を持つクロージャが無いので、出さない。

### (10) 検証の計画

1. **網羅の不変条件**(`internal/taint` の既存のもの): finding の増加が、`make_closure` の経路の finding だけ。見えているクロージャを飛ばす版は、**既存の finding が 1 件も消えず、経路も変わらない**ことを、全ベンチで確かめる。
2. **ベンチの差分**: 収束する 4 つ(gogs、gin-vue-admin、ech0、go-test-bench)、`bench/go-curated`、`bench/go-synth`(S、M)、`testdata/go/cases`、Dart(`dart-curated`、`dart-pub`、`dart-corpus` 系)、Rust(変更なしのはず。finding が 1 件も変わらないことを確かめる)。**前後で findings を比べ、増えたもの全部を 1 件ずつ分類**(正しい流れ / 呼ばれないクロージャの過検出 / 順序の過検出)。Gitea は、今、収束しない(ADR-047 の下書きの測定)ので、`./modules/...` と、`services/` の一部で比べる(`db.WithTx` を含む)。分類できなければ push しない。
3. **curated の `@miss` / `@fp` の増減**(下の表。**実装して確かめたものではない**):

| ベンチ | `@miss` の今 | C-7 で直る(見込み) | 備考 |
| --- | --- | --- | --- |
| go-curated | 10(うち 9 が `silent`) | 9: `cap-run-a/b`、`cap-write`、`cap-nested`、`cap-loop`、`cap-two-a`、`cap-returned`、`tx_capture`(`dynamic_dispatch`)、`tx_capture_plain` | 残る 1 は `options`(`RET-CLOSURE`: クロージャの**引数**への書き込み) |
| dart-curated | 41(うち 31 が `silent`) | `callbacks.dart` の 7 件 + `CAP` のうち、捕獲した値だけで足りるもの(`run-a/b`、`write-a/b`、`add-list`、`nest-a/b`、`across-cap`、`builder`、`bound-prefix` など約 10)。`write-param`、`twice-args` は、引数に依る(案 2 の仕事) | |
| rust-curated | 13(全部 `silent`) | 0 | (7)のとおり、対象外。`callbacks.rs` の 4 つは、今、見つかっている |

   * `@sink(none)` が増える側: `capNever`(go-curated)と、`never_called`(dart-curated)の `@sink(none)`、`rust-curated/callbacks.rs:never_called`(Rust は対象外なので変わらない)。**この 2 つが落ちる**。落ちたら、`@fp` に直すか(限界として記録)、規則を変えるか、決める。
   * `@miss(silent)` の割合: Go 9/10(今)→ 0/1、Dart 31/41 → 約 20/30(見込み。silent のうち CAP 系が消える)。
4. **go test ./...** と、`RIVUS_DART` / `RIVUS_RUST` つきの curated(Dart・Rust の extractor のテスト)を通す。
5. **不確実性の数**: 前後で、`unresolved_callee` が、効果を持つクロージャの数だけ増える。他の kind は変わらない。

## フィクスチャと、今の結果

作った(2026-11-21): go-curated `api/capture.go`(15 関数、`/captures` で登録)、dart-curated `lib/callbacks.dart`(10 関数)、rust-curated `src/callbacks.rs`(4 関数)。

### Go(今: 新しい `@miss` 7、見つかる 8、静か 5)

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
| `tx_capture`、`tx_capture_plain`(既存、`gitea/routers/tx.go`) | `db.WithTx` | `@miss(dynamic_dispatch)`、`@miss(silent)` |

### Dart(今: 新しい `@miss` 7、静か 2)

| タグ | 形 | 今の結果 |
| --- | --- | --- |
| `btn-a` / `btn-b` | `Button(onPressed: () => sink(t))`(2 つ。混ざらない) | `@miss(silent)` ×2 |
| `timer-run` / `periodic` / `timer-write` | `Timer.run`、`Timer.periodic`、書き込み | `@miss(unmodeled_library_call)`(`Timer.run` に、モデルが無い) |
| `microtask` / `delayed` | `scheduleMicrotask`、`Future.delayed` | `@miss(silent)` |
| `timer-param` | クロージャが、自分の引数を sink | `@sink(none)` で静か |
| `never` | 作って、呼ばない | `@sink(none)` で静か。C-7 の後は報告される |

`run-a/b`、`write-a/b`、`nest-a/b`、`add-list` などは、`pass_closure.dart`、`capture.dart`、`nesting.dart` に既にある(重複させない)。

### Rust(今: 全部、見つかる / 静か)

`returned`、`nested_cap`、`never_called`(静かな側)、`param_clean`(静かな側): **今、見つかっている**(`Fn::call` の候補、XTALK で全クロージャが候補)。`thread::spawn` のような、引数の無いクロージャは、他のファイルの全呼び出しの候補になってしまい、既存の `@noisy` の数が変わるので、入れていない。書く側(`write-a/b` ほか)は、既存の `capture.rs`(7 件が `@miss(silent)`、ENV-PTR)。

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

1. **規則の採用**: 「見えないクロージャは、作った場所で当てる(呼ばれるものとして扱う)」。1 度も呼ばれないクロージャが過検出になる(`capNever`、`never_called`)ことを、限界として受け入れるか。受け入れるなら、その 2 つを `@fp` にする(`@sink(none)` + `@fp`)。
2. **見えているクロージャを飛ばすか**(推奨: 飛ばす。今の finding の経路が 1 件も変わらないことを、基準にできる)。飛ばさない場合は、経路が変わる finding を、全部確かめる。
3. **throw**: 最初の版で当てない(推奨)か。Dart で `try { await run(() => throw …) } catch` の形が、見つかるようになるが、作った関数が throw するという過大な主張になる。
4. **痕跡の kind**: 既存の `unresolved_callee`(推奨。スキーマ変更なし)か、新しい kind(例: `closure_assumed_called`。`attributable_to` が `rivus` になる)か。
5. **Rust**: 最初の版は対象外(推奨)。ENV-PTR の修正と、環境の `alloc` で当てるのは、別の段。
6. **測定の続き**: `rivus index` で `refs` を数える(スワップに余裕ができてから、1 度に 1 つ)。gogs と gin-vue-admin は、ソースを scratchpad/bench に置く必要がある(今は無い)。
7. 案 2(ADR-047)との順序: C-7 を先(小さく、独立)。案 2 は、`ExactFuncs` が束縛も返すように、C-7 のあとで足す。
