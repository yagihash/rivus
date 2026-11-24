# 見逃し(`@miss`)と過検出(`@fp`、`@noisy`)の、原因の型での分類

> uncertainty(2026-11-22): この文書は、見出し直後に書いた HEAD ad34d5b(2026-11-21)の時点の分類で、本文はその後に書き換えていない(あとの修正は追記)。
> 見出しの件数(Dart 27、Rust 12、Go 1 など)は、`bench/README.md` の現在の数(「The share of silent misses」の表)と違う。
> 現在の数は、`go test ./cmd/rivus -run Curated -v` の出力(`@miss` の注釈をテストが両側から確かめる)が正で、この文書の数ではない。

2026-11-21 の作業ツリー(HEAD ad34d5b)で、`dart-curated` / `rust-curated` / `go-curated` の全部の `@miss` と `@fp` と `@noisy` を、実際に流して確かめた。**エンジン、extractor は変えていない。** 原因の推定のうち、実験(下の「確認方法」)で確かめたものは、表の備考に書いた。確かめられていないものは「未特定」と書く。

## 確認方法

* 流れの有無と不確実性: `rivus analyze -ir <extractor の出力> -rules rules.yaml`(Go は `./...`)の `findings` と `uncertainties`。**uncertainty**は、`rivus resolve` が「見つからない理由」として出すのと同じ定義(source か sink の関数にある不確実性)で調べ、source / sink の行の近くのものも見た。ヘルパー側(経路の途中)にある不確実性は「ヘルパー側」と別に書く。
* **`@miss` の注釈(BENCH-1、2026-11-21)**: `@miss(silent)` か `@miss(<kind>)`(複数なら `,` 区切り)。ソースの注釈に書いてあり、`cmd/rivus/bench_curated_test.go` が両側から確かめる。**uncertainty**は、source の行と sink の行にあるサイト(`rivus analyze -sites`)の**関数**にある不確実性の kind(`rivus resolve` が `absent_now` に出すのと同じ定義)。次は数えない: 関数を持たない(実行全体の)もの(Rust の `depth_limit`/`path_limit` など)、`numeric_stop`(数は注入を運ばないので、見逃しの説明にならない)、**候補がちょうど 1 つの `dynamic_dispatch`**(メソッド呼び出しのたびに出て、コールバックが失われたことを示さない。以前の「弱い uncertainty」。advisor の方針で `silent` に数える)。`@miss(silent)` は、uncertainty が 1 つも無いこと、`@miss(<kind>)` は、その kind が(source / sink の関数のどちらかに)出ていることを確かめる。uncertainty が付くと(uncertainty を足す修正、DART-15 や CORE-D の効果)、`@miss(silent)` の行が「改善」として失敗する。uncertainty が消えると、`@miss(<kind>)` の行が落ちる。ヘルパー側(経路の途中)の不確実性は数えない。
  * 限界: uncertainty は、落ちた呼び出しを指すとは限らない。`dyn-arg` と `dyn-body`(DART-12)の `unmodeled_library_call` は、本体の無い既定のコンストラクタ(`Loud()`、`Maker()`)に付いたもので、落ちたメソッド呼び出しではない(それでも kind が出ているので、`@miss(unmodeled_library_call)`)。DART-12 が直って、`dynamic` の呼び出しに uncertainty が付いても、この 2 つの注釈は変わらない。
  * 注釈の書き換え(今回): 前の分類との差は、`takes-a` / `takes-b`(uncertainty は、source / sink の関数ではなく、間のクロージャの中の `g(a)` にあった: `silent`)、`dyn-arg` / `dyn-body`(上の限界)、`across-hub` ほかの「弱い」6 件と Rust の `across-each`(`silent`)。MISSES.md の前のバージョンが間違っていたものは、他に無かった。
* IR: `rivus debug extract -text` は Go 専用なので、`ir.Read` / `Func.Format` で Dart と Rust の IR を読んだ(extractor の出力 `*.jsonl`)。**どの `@miss` でも、IR に流れは載っている**(Dart のクロージャのキャプチャは `make_closure` の束縛、Rust は環境の構造体のフィールド)。extractor の側で落ちているものは、無い。
* 実験: bench のコピー(scratchpad)に、小さいケースを足して、原因を 1 つずつ切り分けた(呼び出し元の数、同じ関数の中で呼ぶか、ジェネリックかなど)。実験の結果は備考に「実験」と書いた。

## 原因の型

| 記号 | 型 | 落ちる場所(エンジンのコード) |
| --- | --- | --- |
| `CAP` | キャプチャした値が、作った関数の外から呼ぶと落ちる | internal/valuecalls/valuecalls.go:319 funcsOf(束縛は、クロージャを作った関数 `home` でだけ付く)→ internal/sg/build.go:362 resolveValueCalls(`c.Bind != nil` のときだけ CandidateBindings)→ internal/taint/summary.go:283-286 applySummary(束縛が無いと FreeVars を KUndef で埋める。不確実性なし) |
| `OVERFLOW` | ヘルパーに渡すクロージャが 9 種類以上で、呼び出しが未解決 | internal/valuecalls/valuecalls.go:124(`len(found) > most`、most=8)→ internal/taint/calls.go callBody の `len(fns)==0` 分岐(`s.unmodeled`: 引数を結果へ流すだけ。コールバックの中の sink・throw・書き込みは届かない) |
| `PARAM-CALL` | クロージャの関数引数 / キャプチャした関数の呼び出しが未解決 | internal/valuecalls/valuecalls.go:142 indexCallers(値の呼び出しで呼ばれる関数を callers に入れない。`g` の呼び出し元が見つからない)と classify の refParam(宣言した引数だけ。キャプチャした関数 `f` は refOther) |
| `RET-CLOSURE` | 関数が返したクロージャの呼び出し | Dart: internal/valuecalls/valuecalls.go classify(呼び出しの結果は refOther: 候補なし → unresolved_callee)。Go: VTA の候補は付くが束縛が無い(internal/taint/calls.go callees: `Bindings` は静的なクロージャだけ) |
| `STORE` | コールバックを List / フィールドに持ち、あとで呼ぶ | internal/valuecalls/valuecalls.go(冒頭のコメント: メモリをたどらない。`iter.next` / `field_load` は refOther)。呼び出しは、ヘルパー側の unresolved_callee / unmodeled_library_call |
| `CB-LIST` | 標準ライブラリのコールバックの決め打ちのリストにない | extractor/dart/lib/exprs.dart:_callbackMode(Future.then と、Iterable / Stream の数個だけ)と internal/model/builtin/dart-async.yaml / dart-core.yaml(モデルは args→結果、recv→結果だけ。コールバックの戻り値を結果へ繋がない) |
| `AWAIT-ERR` | await した Future.error が、catch に値を渡さない | internal/model/builtin/dart-async.yaml の Future.error(args→結果)と extractor の `[await]` 呼び出し(失敗した Future の await を throw にしない)。細部は未特定 |
| `ENV-PTR`(**直した、ADR-072**) | ポインタをフィールド(クロージャの環境)に入れて、そこから書くと別名にならない | (直す前)internal/taint/locs.go computeLocs(OpFieldLoad: `locs(base)+field+"*"` を指す)と internal/taint/transfer.go(OpFieldStore はポインタの値を保存するだけで、指す先と同一視しない)。関数内の `*e.o = x`(Env{o:&mut out})でも再現。(今)internal/taint/ptrcells.go: セルに入れたポインタの指す先を覚え(`ptrAlias`)、`field_load` の結果と、呼び出し先が書いたもの(`applySummary` の `OutParam`)を、指す先にも適用する。extractor は、環境のフィールドをクロージャの名前で呼ぶ(`<クロージャ>.0`)ので、どのクロージャか分からない `Fn::call` の候補が、別のクロージャの環境のセルを混ぜない |
| `GEN-DYN`(**直した、ADR-081**: モデルの位置が、trait 越しの呼び出しでもレシーバ = arg0。zoo の `dyn/generics-ptr` が見つかる。rust-curated の `across-each` は別の原因が残る) | ジェネリックな T に対する trait 呼び出し(dynamic)で、モデルの `from: args` が receiver を含まない | internal/taint/calls.go selectVals(`SelArg All` は cc.args)+ internal/sg/sg.go:139 CallArgs(dynamic は Args[1:])+ internal/model/builtin/rust-std.yaml(`Iterator::next` / `IntoIterator::into_iter` は from: args)。`for i in items`(items: Vec<T>)で流れが消える |
| `XTALK`(**絞った、ADR-079**。レシーバの出所を、もれなくたどれたときだけ絞る。rust-curated の `@fp` 18 → 7、`@noisy` 152 → 84 流れ。残りは、たどれない側: フィールド、`Box<dyn Fn>`、関数の結果、中継) | Fn*::call(dynamic)の候補が「型の合う全クロージャ」で、呼び出し元ごとに絞らない(ADR-072 のあと: 候補の書き込みが、別のクロージャの環境のセルを通って、別のクロージャの sink に届く過検出は消えた。zoo-rust の `@noisy` と、それに伴う `@fp` が減った。候補の集合そのものは絞っていない) | internal/valuecalls/valuecalls.go:50(CalleeValue だけを解決。Rust は dynamic)と internal/taint/calls.go:355-380 dispatchGuards / makeDispatchGuards(候補が method でない(`!fn.Recv`)と、ガードなし) |
| `MODEL-OVER` | モデルが、コールバックの戻り値を見ずに recv を結果へ流す | internal/model/builtin/dart-async.yaml Future.then / dart-core.yaml Iterable.map / rust-std.yaml Iterator::map・Option::and_then(recv/args→結果) |
| `KEYED-MAP` | マップのキーを区別しない(Set/Get のキーが違っても同じセル) | internal/taint/transfer.go OpIndexStore / OpIndexLoad(キー付きは、値を `[]` に持つ) |
| `ALLOWLIST` | 分岐条件(switch の許可リスト)を、汚染を除去するものと見ない | internal/taint/guards.go(条件は記録するだけで、評価しない: Guard の説明) |

案 2 = ADR-047 の下書き(ヘルパーの要約に、コールバックの出力を「仮想のパラメータ」で持たせ、呼び出し元が、自分で渡したクロージャ(`ExactFuncs`)の要約で具体化する)。「直る(見込み)」は、下書きの記述から読んだ推測で、実装して確かめたものではない。直るには、`ExactFuncs` が、クロージャの束縛(`make_closure` の引数)も返す必要がある(下書きに書いていない)。

## 未解決のレビュー指摘(RUST-13、RUST-14、DART-12)と、そのフィクスチャ

別セッションのレビューの識別子(docs には無い)。2026-11-21、main(HEAD 793a803 の作業ツリー)で、原因のコードが残っていることと、失敗するフィクスチャを確かめた。**コアは変えていない。** 対応する issue は無い(#76〜#83 は、どれもこの 3 つの原因を挙げていない)。上の集計と表は、この節のフィクスチャを足す前の数のまま(足したあとの数は、下の表)。

| 指摘 | 原因のコード(今) | フィクスチャ | 今の結果 |
| --- | --- | --- | --- |
| RUST-13: `Fn` / `FnMut` を `&self` で呼ぶと、レシーバがセルで、呼び出し元ごとに絞れない | internal/taint/calls.go:357 `dispatchGuards`(`recv.Kind != sg.KParam` で nil)。さらに makeDispatchGuards は、候補が method でない(`!fn.Recv`)と、ガードを付けない(クロージャは全部これ) | rust-curated/src/fnself.rs: `fn_ret_clean` / `fn_mut_ret_clean`(`@sink(none) @fp`)、`fn_sink_a/b`、`fn_mut_sink_a/b`(`@nosink` + `@fp`)。対照の `fn_ref_clean`(`&impl Fn`、レシーバがパラメータそのもの) | 6 か所とも `@fp`(落ちる)。対照も落ちる: レシーバがセルであることだけが原因ではなく、クロージャの候補にガードが付かない(`!fn.Recv`)ことも原因。案 2 の受け入れは、この 7 か所の `@fp` が全部消えること |
| RUST-14: モデルを適用すると、`read_impls` でない動的呼び出しの本体を読まない | internal/taint/calls.go:182-196(`modelled` のあと、`!rules.ReadsImpls(cc.sym)` で return)。`read_impls: true` は rust-std.yaml と rust-misc.yaml だけ | rust-curated/src/modelled.rs: `model_body`(source が実装の本体の中、`@miss`)、`model_drop`(実装が引数を捨てる、`@sink(none) @fp`)。rules.yaml に、プロジェクトの trait のメソッド(`Render::render`、`Mask::mask`)の propagator(recv→result)を足した | **ADR-075 で `@miss` は直った**(モデルを適用したあと、本体も読む)。`@fp`(`model_drop`)は残る: モデルは「レシーバが結果に入る」と宣言し続け、本体の和でも消えない |
| DART-12: `dynamic` のメソッド呼び出しが、`dart:core#dynamic.invoke` の静的呼び出しになり、モデルで覆われて uncertainty が残らない | extractor/dart/lib/exprs.dart:347(`_methodInvocation`)、internal/model/builtin/dart-core.yaml:8(`flows: [{from: args, to: result}]`) | dart-curated/lib/dynamic_calls.dart: `dynamic_arg`(メソッドの中の sink、`@miss`)、`dynamic_body`(メソッドの本体の source、`@miss`)、`dynamic_drop`(`@sink(none) @fp`) | `@miss` 2、`@fp` 1。モデルを外す対照は、していない |

足したあとの数: Dart `@miss` 34、`@fp` 5。Rust `@miss` 12、`@fp` 22、`@noisy` は 183 件のまま(新しいフィクスチャは、他と型が重ならない署名(`u8`、`u16`、`u32`、`u64`、`i8`)で、他のクロージャを候補にしない)。

既存の `@miss` / `@fp` のうち、この 3 つに該当するもの: **RUST-13 は、XTALK の `@fp`(12)と `@noisy`(16 か所)が同じ形**(ヘルパーが `f(..)` で呼ぶ `impl Fn`)。1 件ずつ、レシーバがセルかは確かめていない。**RUST-14 と DART-12 に該当する既存のものは、無い**(`@miss` の原因の型のどれにも、モデルが本体を隠すものは無かった)。

案 2(ADR-047)で RUST-13 が直る見込み: 案 2 は、ヘルパーの要約にコールバックの出力を持たせ、呼び出し元が渡したクロージャで具体化するので、`dispatchGuards` を通らない。下書きの記述から読んだ推測で、実装して確かめたものではない。RUST-14 と DART-12 は、案 2 では直らない(別の修正)。

## 集計

### `@miss`

| 言語 | `@miss` | `@miss(silent)` | `@miss(<kind>)` | 案 2 で直る(見込み) | 不明 | 直らない | 未特定 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Dart | 34 | 27 | 7 | 11 | 6 | 17 | 0 |
| Rust | 13 | 13 | 0 | 0 | 0 | 13 | 0 |
| Go | 3 | 2 | 1 | 0 | 0 | 3 | 0 |
| 計 | 50 | 42 | 8 | 11 | 6 | 33 | 0 |

サイレントな見逃しの割合(`@miss(silent)` / `@miss`): Dart 27/34 = 79%、Rust 13/13 = 100%、Go 2/3 = 67%、計 42/50 = 84%(定義と意味は README.md の「The share of silent misses」。テストのログに `silent misses: S / M (P%)` が出る)。

(Rust 13 と Go 3 は、下の「CVE の形のフィクスチャ」の節の 3 件を足した数。この節より上の表と文は、足す前の数のまま。)

**別名の限界(`alias-ptr`)を足したあと: Go 15(silent 14、uncertainty あり 1)、計 `@miss` 78、silent 66 / 78 = 85%。**(ADR-050 のあとの数 77 / 65 = 84% に、Go 1 を足した。上の C-7 の段落の数は、足す前のまま。)

**C-7 のフィクスチャを足したあと(`docs/DESIGN-capture.md`、今の数。テストが数える): `@miss` 75 = Dart 45(silent 35、uncertainty あり 10)、Rust 16(silent 15、uncertainty あり 1)、Go 14(silent 13、uncertainty あり 1)。silent 63 / 75 = 84%。** 足した分(Dart `callbacks.dart` 11、Go `api/capture.go` 11、Rust `callbacks.rs` 3)は、キャプチャした値が、ヘルパーが呼ぶクロージャの中で落ちる形(`CAP` ほか)で、**エンジンが悪化したのではなく、見えていなかった見逃しを記録した結果**。Rust の 3 件(`thread::spawn` = `unmodeled_library_call`、`Box<dyn Fn>` をフィールドに入れて別の関数が呼ぶ、`Vec::retain`)は、渡すだけの形で、今、**見つからない**(Rust もサイレントな見逃し。C-7 の最初のバージョンの対象外とした前提の、「読む側は今でも見つかる」は、呼ぶ側が見える形だけで成り立つ)。

`@miss(<kind>)` の 7 件: Dart の `holder-a`、`holder-b`、`curry`(`unresolved_callee`)、`getter-fn`(`unmodeled_library_call,unresolved_callee`)、`map-entries`(`dynamic_dispatch`、候補なし)、`dyn-arg`、`dyn-body`(`unmodeled_library_call`、上の限界)。残りの 40 件は、source と sink の関数に、uncertainty が無い(`silent`)。前のバージョンの「サイレント 30、弱い uncertainty 7、uncertainty あり 7」(44 件。Dart 32、Rust 11)に、RUST-13/14・DART-12 のフィクスチャの 3 件(`dyn-arg`、`dyn-body`、`model-body`)を足した 47 件を、定義に当てはめ直した: サイレント 30 + 弱い 7 + `takes-a/b` 2 + `model-body` 1 = 40(`silent`)、uncertainty あり 7 - 2 + `dyn-*` 2 = 7。

原因の型ごとの件数(`@miss`):

| 型 | Dart | Rust | Go | 計 |
| --- | --- | --- | --- | --- |
| `CAP` | 12 | 0 | 0 | 12 |
| `OVERFLOW` | 1 | 0 | 0 | 1 |
| `PARAM-CALL` | 4 | 0 | 0 | 4 |
| `RET-CLOSURE` | 4 | 0 | 1 | 5 |
| `STORE` | 4 | 3 | 0 | 7 |
| `CB-LIST` | 6 | 0 | 0 | 6 |
| `AWAIT-ERR` | 1 | 0 | 0 | 1 |
| `ENV-PTR`(ADR-072 で 6 件が見つかる。残りは `across-cap`) | 0 | 7 | 0 | 7 |
| `GEN-DYN` | 0 | 1 | 0 | 1 |

### `@fp` と `@noisy`

| 言語 | 種類 | 件数 | 原因の型 | 案 2 で直る(見込み) |
| --- | --- | --- | --- | --- |
| Dart | `@fp` | 4 | OVERFLOW 2, MODEL-OVER 2 | 直る 2、直らない 2 |
| Rust | `@fp`(`@nosink` 8 + `@sink(none)` 6) | 14 | XTALK 12(うち `@nosink` 8)、MODEL-OVER 2 | 直る 9、不明 3、直らない 2 |
| Rust | `@noisy` | 183 件、16 か所 | XTALK(全部) | 直る 11 か所 156 件、不明 5 か所 27 件 |
| Go | `@fp` | 5 | KEYED-MAP 4、ALLOWLIST 1 | 直らない 5 |

Rust の `@fp` 14 件の内訳: XTALK 12 件(直る 9: arg-a/b、run-a/b、sinkwith-a/b の 6 件と ret-clean、produce、fn-drop。不明 3: relay-c-a/b と emit-two の 2 つ目)、MODEL-OVER 2 件(直らない)。
**ADR-079 のあと(`valuecalls/narrow.go`)**: Rust の `@fp` 14 件のうち、XTALK の直る 9 件に該当する 5 件(arg-a/b、sinkwith-a/b、emit-two の 2 つ目)と、`fnself.rs` の 6 件が消えた(rust-curated の `@fp` は 18 から 7)。`@noisy` は 16 か所 152 流れから 3 か所 84 流れ。zoo(Rust)は、`clos/two-callers` のデコイが消えた(`clos/relay` は残る)。

Rust の `@noisy` は README に「17 か所」とあるが、ソースの注釈は 16 か所で、合計 183 件になる(下の表)。

## サイレントな見逃し(`@miss(silent)`、先頭にまとめる)

見逃していて、source / sink の関数に何の不確実性も出ていないもの(候補が 1 つの `dynamic_dispatch` だけのものを含む)。`rivus resolve` の `uncertainties` を見ても、手がかりが無い。

### Dart(27 件)

| タグ | source → sink | 原因の型 | ヘルパー側の uncertainty |
| --- | --- | --- | --- |
| `across-cap` | lib/across.dart:30 → lib/across.dart:34 | `CAP` | (なし) |
| `write-a` | lib/capture.dart:8 → lib/capture.dart:12 | `CAP` | (なし) |
| `write-b` | lib/capture.dart:17 → lib/capture.dart:21 | `CAP` | (なし) |
| `write-param` | lib/capture.dart:38 → lib/capture.dart:42 | `CAP` | (なし) |
| `add-list` | lib/capture.dart:48 → lib/capture.dart:52 | `CAP` | (なし) |
| `relay-c-a` | lib/capture.dart:76 → lib/capture.dart:77 | `PARAM-CALL` | ヘルパー側: capture.dart:72 に unresolved_callee |
| `relay-c-b` | lib/capture.dart:81 → lib/capture.dart:82 | `PARAM-CALL` | 同上 |
| `through` | lib/errors.dart:53 → lib/errors.dart:57 | `OVERFLOW` | ヘルパー側: helpers.dart:4 に unresolved_callee |
| `future-error` | lib/errors.dart:86 → lib/errors.dart:88 | `AWAIT-ERR` | (なし) |
| `on-error` | lib/errors.dart:94 → lib/errors.dart:95 | `CB-LIST` | (なし) |
| `catch-error` | lib/futures.dart:35 → lib/futures.dart:37 | `CB-LIST` | (なし) |
| `when-complete` | lib/futures.dart:44 → lib/futures.dart:48 | `CB-LIST` | (なし) |
| `completer` | lib/futures.dart:58 → lib/futures.dart:60 | `CB-LIST` | (なし) |
| `put-if-absent` | lib/iterables.dart:85 → lib/iterables.dart:87 | `CB-LIST` | (なし) |
| `nest-a` | lib/nesting.dart:8 → lib/nesting.dart:12 | `CAP` | ヘルパー側: helpers.dart:4 |
| `nest-b` | lib/nesting.dart:16 → lib/nesting.dart:20 | `CAP` | 同上 |
| `twice-args` | lib/nesting.dart:43 → lib/nesting.dart:47 | `CAP` | (なし) |
| `run-a` | lib/pass_closure.dart:49 → lib/pass_closure.dart:50 | `CAP` | (なし) |
| `run-b` | lib/pass_closure.dart:54 → lib/pass_closure.dart:55 | `CAP` | (なし) |
| `across-hub` | lib/across.dart:25 → lib/across.dart:24 | `STORE` | dynamic_dispatch(候補 1)だけ。ヘルパー側に unresolved_callee |
| `emit` | lib/classes.dart:19 → lib/classes.dart:18 | `STORE` | dynamic_dispatch(候補 1)だけ |
| `emit-two` | lib/classes.dart:28 → lib/classes.dart:26 | `STORE` | 同上 |
| `lazy` | lib/classes.dart:40 → lib/classes.dart:42 | `STORE` | 同上 |
| `builder` | lib/classes.dart:61 → lib/classes.dart:63 | `CAP` | 同上 |
| `bound-prefix` | lib/tearoff.dart:111 → lib/tearoff.dart:112 | `CAP` | 同上 |
| `takes-a` | lib/nesting.dart:52 → lib/nesting.dart:54 | `PARAM-CALL` | (なし)。unresolved_callee は、間のクロージャの中の nesting.dart:53 |
| `takes-b` | lib/nesting.dart:58 → lib/nesting.dart:60 | `PARAM-CALL` | 同上 |

### Rust(12 件)

| タグ | source → sink | 原因の型 | ヘルパー側の uncertainty |
| --- | --- | --- | --- |
| `across-hub` | src/across.rs:25 → src/across.rs:24 | `STORE` | ヘルパー側: shared/src/lib.rs:25 に unmodeled_library_call |
| `across-cap` | src/across.rs:30 → src/across.rs:32 | `ENV-PTR` | (なし) |
| `write-a` | src/capture.rs:8 → src/capture.rs:12 | `ENV-PTR` | (なし) |
| `write-b` | src/capture.rs:17 → src/capture.rs:21 | `ENV-PTR` | (なし) |
| `write-param` | src/capture.rs:38 → src/capture.rs:42 | `ENV-PTR` | (なし) |
| `push-list` | src/capture.rs:48 → src/capture.rs:50 | `ENV-PTR` | (なし) |
| `late` | src/capture.rs:134 → src/capture.rs:137 | `ENV-PTR` | (なし) |
| `for-each-cap` | src/iters.rs:26 → src/iters.rs:28 | `ENV-PTR` | (なし) |
| `emit` | src/structs.rs:74 → src/structs.rs:73 | `STORE` | ヘルパー側: structs.rs:66 に unmodeled_library_call |
| `emit-two` | src/structs.rs:83 → src/structs.rs:81 | `STORE` | 同上 |
| `across-each` | src/across.rs:18 → src/across.rs:19 | `GEN-DYN` | `numeric_stop`(`vec!` のマクロの中。見逃しの説明にならないので数えない)だけ。ヘルパー側に dynamic_dispatch |

### Go(1 件)

* `options`: api/more.go:32 → api/more.go:33(ヘルパー側: api/more.go:25 の `o(q)` に dynamic_dispatch)

## Dart

`@miss` 32。落ちる場所は、原因の型の表を見る。

| タグ | source → sink | uncertainty(注釈) | 型 | 案 2 | 備考 |
| --- | --- | --- | --- | --- | --- |
| `across-hub` | lib/across.dart:25 → 24 | `silent`(候補 1 の dynamic_dispatch だけ): dynamic_dispatch(mixed、候補 1)が across.dart:24,25。ヘルパー shared.dart:15 に unresolved_callee | `STORE` | 直らない | Hub の `_subs`(List)に入れて emit で呼ぶ |
| `across-cap` | lib/across.dart:30 → 34 | `silent` | `CAP` | 直る | `each([1], (i){ out = t; })`。別パッケージの helper |
| `write-a` | lib/capture.dart:8 → 12 | `silent` | `CAP` | 直る | `run(() { out = t; })`、`out` はセル(make_closure の 2 つ目の束縛) |
| `write-b` | lib/capture.dart:17 → 21 | `silent` | `CAP` | 直る |  |
| `write-param` | lib/capture.dart:38 → 42 | `silent` | `CAP` | 直る | `fireAndForget(t, (x){ out = x; })`。引数 x ではなく、キャプチャした out のセルが落ちる |
| `add-list` | lib/capture.dart:48 → 52 | `silent` | `CAP` | 直る | `run(() { items.add(t); })` |
| `relay-c-a` | lib/capture.dart:76 → 77 | `silent`(ヘルパー側: capture.dart:72 に unresolved_callee) | `PARAM-CALL` | 不明 | `relayCapture(f) => run(() => f())`: 内側のクロージャがキャプチャした関数 `f()` を呼ぶ。draft 条件 3(中継)に該当する |
| `relay-c-b` | lib/capture.dart:81 → 82 | `silent`(同上) | `PARAM-CALL` | 不明 |  |
| `holder-a` | lib/capture.dart:102 → 104 | `@miss(unresolved_callee)`: unresolved_callee(capture.dart:104、sink の行) | `RET-CLOSURE` | 直らない | `holder(a)` が `() => s` を返し、`get()` で呼ぶ |
| `holder-b` | lib/capture.dart:108 → 110 | `@miss(unresolved_callee)`: unresolved_callee(capture.dart:110) | `RET-CLOSURE` | 直らない |  |
| `emit` | lib/classes.dart:19 → 18 | `silent`(候補 1 の dynamic_dispatch だけ): dynamic_dispatch(mixed、候補 1)が classes.dart:18,19。ヘルパー classes.dart:11 に unresolved_callee | `STORE` | 直らない | Emitter の `_subs` に入れて emit で呼ぶ |
| `emit-two` | lib/classes.dart:28 → 26 | `silent`(同上) | `STORE` | 直らない |  |
| `lazy` | lib/classes.dart:40 → 42 | `silent`(候補 1 の dynamic_dispatch だけ): dynamic_dispatch(classes.dart:42)。ヘルパー classes.dart:36 に unresolved_callee | `STORE` | 直らない | `Lazy(this._make)` がフィールドに持ち、getter で呼ぶ |
| `builder` | lib/classes.dart:61 → 63 | `silent`(候補 1 の dynamic_dispatch だけ): dynamic_dispatch(mixed)が classes.dart:62,63 | `CAP` | 不明 | `Builder().with_((x) => x.add(a))`: キャプチャした `a`。`f(this)` で呼ぶ。add が dynamic(候補 1) |
| `getter-fn` | lib/classes.dart:83 → 87 | `@miss(unresolved_callee)`: unresolved_callee と unmodeled_library_call(Source.new、本体なし)が classes.dart:87 | `RET-CLOSURE` | 直らない | getter が `() => source()` を返す。source はクロージャの中 |
| `through` | lib/errors.dart:53 → 57 | `silent`(ヘルパー側: helpers.dart:4 に unresolved_callee) | `OVERFLOW` | 直る | `transform(a, (x) => throw AppError(x))`。transform へ渡すクロージャが 9 種類以上(bench には多数)。4 種類なら見つかる(実験で確認) |
| `future-error` | lib/errors.dart:86 → 88 | `silent` | `AWAIT-ERR` | 直らない | `await Future<void>.error(AppError(source()))` を `on AppError catch (e)` で受ける |
| `on-error` | lib/errors.dart:94 → 95 | `silent` | `CB-LIST` | 直らない | `Future.error(a).then((v) => v, onError: (e) => sink(e))`: `then` の 2 つ目のコールバックを呼ばない |
| `catch-error` | lib/futures.dart:35 → 37 | `silent` | `CB-LIST` | 直らない | `Future.error(source()).catchError((e) { sink(e); })` |
| `when-complete` | lib/futures.dart:44 → 48 | `silent` | `CB-LIST` | 直らない | `whenComplete(() { out = t; })`: コールバックを呼ばず、キャプチャもある(CAP も) |
| `completer` | lib/futures.dart:58 → 60 | `silent` | `CB-LIST` | 直らない | `Future<String>.sync(() => t)`: コールバックの戻り値が結果に繋がらない |
| `map-entries` | lib/iterables.dart:79 → 80 | `@miss(dynamic_dispatch)`: dynamic_dispatch(rivus、「モデルがあれば解決する」)が iterables.dart:80 | `CB-LIST` | 直らない | `Map.forEach((k, v) => ...)`: 2 引数のコールバック。モデルも extractor の決め打ちも無い |
| `put-if-absent` | lib/iterables.dart:85 → 87 | `silent` | `CB-LIST` | 直らない | `m.putIfAbsent("k", () => t)`: モデルは recv→結果だけ |
| `nest-a` | lib/nesting.dart:8 → 12 | `silent`(ヘルパー側: helpers.dart:4) | `CAP` | 直る | `transform("const", (x) => transform(a, (y) => y))`: 外側のクロージャが `a` をキャプチャ |
| `nest-b` | lib/nesting.dart:16 → 20 | `silent`(同上) | `CAP` | 直る |  |
| `twice-args` | lib/nesting.dart:43 → 47 | `silent` | `CAP` | 直る | `fireAndForget(a, (x) => out.add(x))`: キャプチャした `out` |
| `takes-a` | lib/nesting.dart:52 → 54 | `silent`(unresolved_callee は、間のクロージャの中の nesting.dart:53 で、source / sink の関数ではない) | `PARAM-CALL` | 不明 | `viaInner("const", (g) => g(a))`: クロージャが関数引数 `g` を呼ぶ(コールバックのコールバック) |
| `takes-b` | lib/nesting.dart:58 → 60 | `silent`(同上、nesting.dart:59) | `PARAM-CALL` | 不明 |  |
| `curry` | lib/nesting.dart:133 → 135 | `@miss(unresolved_callee)`: unresolved_callee(nesting.dart:135) | `RET-CLOSURE` | 直らない | `add(a)("x")`: クロージャが返すクロージャ |
| `run-a` | lib/pass_closure.dart:49 → 50 | `silent` | `CAP` | 直る | `run(() => sink(a))`: 実験で、呼び出し元が 1 つでも落ちる |
| `run-b` | lib/pass_closure.dart:54 → 55 | `silent` | `CAP` | 直る |  |
| `bound-prefix` | lib/tearoff.dart:111 → 112 | `silent`(候補 1 の dynamic_dispatch だけ): dynamic_dispatch(mixed)が tearoff.dart:112 | `CAP` | 不明 | `transform(s, stamp)`(メソッドの tear-off が `this` をキャプチャ)。transform の 9 種類超(OVERFLOW)も重なる |

### Dart の `@fp`(4)

| タグ | 場所(source → sink) | 型 | 案 2 | 備考 |
| --- | --- | --- | --- | --- |
| `ret-clean` | pass_closure.dart:21 → 23 | `OVERFLOW` | 直る(見込み) | `transform(c, (x) => "fixed")`: transform が未解決(9 種類超)で、引数→結果の保守側が働く。呼び出し元 1 つの実験では出ない |
| `tear-drop` | tearoff.dart:21 → 22 | `OVERFLOW` | 直る(見込み) | `transform(a, c.drop)`: 同上 |
| `then-const` | futures.dart:23 → 25 | `MODEL-OVER` | 直らない | `f.then((v) => "fixed")`: Future.then のモデルが recv→結果 |
| `map-const` | iterables.dart:58 → 60 | `MODEL-OVER` | 直らない | `items.map((s) => "fixed")`: Iterable.map のモデルが recv→結果 |

`@noisy` は Dart には無い(クロージャが、呼び出し元ごとに結び付く: `valueGuards`)。

## Rust

### Rust の `@miss`(11)

| タグ | source → sink | uncertainty(注釈) | 型 | 案 2 | 備考 |
| --- | --- | --- | --- | --- | --- |
| `across-each` | src/across.rs:18 → 19 | `silent`(numeric_stop は数えない): numeric_stop(rivus、`vec!` のマクロ内 macros.rs:60 に記録。この flow と無関係に見える)。ヘルパー側 shared/src/lib.rs:8 に dynamic_dispatch(「実装が分からない」) | `GEN-DYN` | 直らない | `shared::each<T>(items: Vec<T>, f)`: `for i in items` が dynamic `into_iter` / `next`。同じ形を同じクレートに書いても、T がジェネリックなら再現(実験)。Fn 版は見つかる |
| `across-hub` | src/across.rs:25 → 24 | `silent`(ヘルパー側: shared/src/lib.rs:25 に unmodeled_library_call) | `STORE` | 直らない | `Vec<Box<dyn Fn>>` に入れて emit で呼ぶ |
| `across-cap` | src/across.rs:30 → 32 | `silent` | `ENV-PTR`(ADR-072 のあとも落ちる。呼ぶ側 `shared::each` が `for i in items { f(i) }` で、`IntoIterator::into_iter` / `Iterator::next` の候補が 0。`GEN-DYN` と同じ根に見える。未確認) | 直らない | `shared::each(vec![1], \|_i\| out = t.clone())` |
| `write-a` | src/capture.rs:8 → 12 | ~~`silent`~~ → **見つかる(ADR-072)** | `ENV-PTR` | 直った | `run(\|\| { out = t; })`。呼び出しが同じ関数内(`let f = \|\| {out = t;}; f();`)でも落ちる(実験) |
| `write-b` | src/capture.rs:17 → 21 | ~~`silent`~~ → **見つかる(ADR-072)** | `ENV-PTR` | 直った |  |
| `write-param` | src/capture.rs:38 → 42 | ~~`silent`~~ → **見つかる(ADR-072)** | `ENV-PTR` | 直った | `fire_and_forget(t, \|x\| { out = x; })` |
| `push-list` | src/capture.rs:48 → 50 | ~~`silent`~~ → **見つかる(ADR-072)** | `ENV-PTR` | 直った | `run(\|\| items.push(t))` |
| `late` | src/capture.rs:134 → 137 | ~~`silent`~~ → **見つかる(ADR-072)** | `ENV-PTR` | 直った | `let mut set = \|x\| v = x; set(a);`(ヘルパーなし) |
| `for-each-cap` | src/iters.rs:26 → 28 | ~~`silent`~~ → **見つかる(ADR-072)** | `ENV-PTR` | 直った | `items.into_iter().for_each(\|x\| last = x)` |
| `emit` | src/structs.rs:74 → 73 | `silent`(ヘルパー側: structs.rs:66 に unmodeled_library_call) | `STORE` | 直らない | `Vec<Box<dyn Fn>>` に入れて emit で呼ぶ |
| `emit-two` | src/structs.rs:83 → 81 | `silent`(同上) | `STORE` | 直らない |  |

Rust の `@miss` は、案 2 では 1 つも直らない(11 件とも、別の原因)。

### Rust の `@fp`(14)

| タグ | 場所 | 型 | 案 2 | 備考 |
| --- | --- | --- | --- | --- |
| `arg-a / arg-b` | pass_closure.rs:34→30, 29→35(@nosink) | `XTALK` | 直る(見込み) | `fire_and_forget(a, \|x\| sink(&x))` が、もう一方の呼び出し元の source に届く |
| `run-a / run-b` | pass_closure.rs:54→50, 49→55(@nosink) | `XTALK` | 直る(見込み) | `run(\|\| sink(&a))` |
| `sinkwith-a / sinkwith-b` | pass_closure.rs:130→126, 125→131(@nosink) | `XTALK` | 直る(見込み) |  |
| `relay-c-a / relay-c-b` | capture.rs:80→76, 75→81(@nosink) | `XTALK` | 不明 | 中継(`relay_capture(f) { run(\|\| f()) }`)は draft の条件 3 |
| `ret-clean` | pass_closure.rs:21 → 23 | `XTALK` | 直る(見込み) | `transform(c.clone(), \|_x\| String::from("fixed"))`: 他の呼び出し元の恒等クロージャの戻りが、この結果に混ざる(候補の全部の要約を適用するため) |
| `prod(produce)` | pass_closure.rs:42(42 件の source) | `XTALK` | 直る(見込み) | `produce(\|x\| sink(&x))` の x に、他の呼び出し元の source が届く |
| `fn-drop` | fnvalues.rs:20 → 21 | `XTALK` | 直る(見込み) | `transform(a.clone(), drop_it)`: 同上 |
| `emit-two(2 つ目)` | structs.rs:→82(5 件) | `XTALK` | 不明 | `e2.on(Box::new(\|s\| sink(s)))`: `Vec<Box<dyn Fn>>`(STORE)の呼び出しも、型で全部に一致する |
| `map-const` | iters.rs:50 → 52 | `MODEL-OVER` | 直らない | `iter().map(\|_s\| "fixed")`: Iterator::map のモデル(args→結果) |
| `opt-then-const` | iters.rs:69 → 71 | `MODEL-OVER` | 直らない | `and_then(\|_s\| Some("fixed"))`: Option::and_then のモデル |

### Rust の `@noisy`(183 件、16 か所)

どれも同じ型(XTALK)。ヘルパーの `f(s)` の候補が、型の合う全クロージャ(extractor の `candidates`)で、`valueGuards`(ADR-045)が、`CalleeValue` の呼び出しだけに働く(Rust のクロージャの呼び出しは `dynamic std::ops::Fn*::call*`)。そのため、あるクロージャの中の sink に、同じ型のクロージャを受け取る全部のヘルパーの、全部の呼び出し元の source が届く。

| 場所 | 件数 | クロージャ | 案 2 |
| --- | --- | --- | --- |
| across.rs:19 | 42 | shared::each の closure | 直る(見込み) |
| across.rs:24 | 5 | Hub::on の closure(Vec<Box<dyn Fn>>) | 不明 |
| capture.rs:76 | 6 | relay_capture(a) | 不明 |
| capture.rs:81 | 6 | relay_capture(b) | 不明 |
| fnvalues.rs:57 | 3 | for_each(\|s\| sink(s))(std のコールバック) | 直る(見込み) |
| iters.rs:21 | 3 | for_each(\|s\| sink(s)) | 直る(見込み) |
| iters.rs:104 | 3 | any(\|s\| { sink(s); ... }) | 直る(見込み) |
| pass_closure.rs:30 | 40 | fire_and_forget(a) | 直る(見込み) |
| pass_closure.rs:35 | 40 | fire_and_forget(b) | 直る(見込み) |
| pass_closure.rs:50 | 6 | run(\|\| sink(&a)) | 直る(見込み) |
| pass_closure.rs:55 | 6 | run(\|\| sink(&b)) | 直る(見込み) |
| pass_closure.rs:61 | 7 | run(move \|\| sink(&a)) | 直る(見込み) |
| pass_closure.rs:126 | 3 | sink_with(a) | 直る(見込み) |
| pass_closure.rs:131 | 3 | sink_with(b) | 直る(見込み) |
| structs.rs:73 | 5 | Emitter::on(e) | 不明 |
| structs.rs:81 | 5 | Emitter::on(e1) | 不明 |

## Go

### Go の `@miss`(1)

| タグ | source → sink | uncertainty(注釈) | 型 | 案 2 | 備考 |
| --- | --- | --- | --- | --- | --- |
| `options` | api/more.go:32 → 33 | `silent`(ヘルパー側: api/more.go:25 に dynamic_dispatch) | `RET-CLOSURE` | 直らない | `WithName(n)` が返すクロージャを、可変長の引数(スライス)で受けた `build` が `o(q)` で呼ぶ。実験: 返されたクロージャを同じ関数で呼んでも、ヘルパーに渡しても落ちる |

### Go の `@miss`: 別名の限界(`alias-ptr`、2026-11-21)

| タグ | source → sink | uncertainty | 型 | 備考 |
| --- | --- | --- | --- | --- |
| `alias-ptr` | go-curated/api/alias.go:`store` → `use`(`testdata/go/cases/paramshare/ctxptr` が同じ形の param source 版。CORE-K(ADR-065)のあとは、こちらの `reader` に `unmodeled_library_call` の uncertainty が出る。go-curated の `alias.go` は silent のまま) | **`silent`**(source の関数 `store`・sink の関数 `use` に不確実性なし。`uncertainties` は `init` の `unmodeled_library_call` だけ) | `ALIAS`(完全なポインタ解析を作らない、という受け入れ済みの近似) | `h.S.Last = <source>` と書く関数と、同じ `state` を自分のポインタ `s` で受け取って `s.Last` を sink に渡す関数。呼び出し元が `use(h)` / `reader(c)`(同じ親を渡す)なら見つかる(`ctxstate`)。別のポインタ(`use(s)`)で渡すと見つからない。C-1(ADR-048)より前、ae5fdf6 でも見つからない(退行ではない) |

### Go の `@fp`(5)

| タグ | 場所 | 型 | 案 2 | 備考 |
| --- | --- | --- | --- | --- |
| ctxkey @nosink(mw_user) | flows.go:15 ← middleware.go:28 | `KEYED-MAP` | 直らない | `c.Set("user", ...)` と `c.GetString("next")` のキーが違うのに混ざる |
| mw_user @nosink(ctxkey) | flows.go:26 ← middleware.go:12 | `KEYED-MAP` | 直らない | 同上 |
| safe_next | flows.go:20 ← middleware.go:12, 28 | `KEYED-MAP` | 直らない | `GetString("safe_next")`(浄化した値のキー)に、別のキーの値が届く |
| cache Get("table") | misc.go:69 → 70 | `KEYED-MAP` | 直らない | `cache.Set("name", x)` のあと `cache.Get("table")` |
| switch allow-list | misc.go:75 → 78 | `ALLOWLIST` | 直らない | `switch col { case "name","email": ... }` の中の sink |

## 実験で確かめたこと(再現するコード)

* Dart: `run(() => sink(a))` と `run(() { out = t; })` は、呼び出し元が 1 つでも落ちる。同じクロージャを同じ関数の中で呼ぶ(`final f = () => sink(a); f();`)と見つかる。`fireAndForget(a, (x) => sink(x))`(キャプチャなし)は見つかる(CAP)。
* Dart: `transform(a, (x) => throw AppError(x))` は、`transform` に渡すクロージャが 4 種類なら見つかり、11 種類なら落ちる。クロージャの中の `sink(x)` も、10 種類のときは 0 件、4 種類のときは 4 件(OVERFLOW)。`transform(c, (x) => "fixed")` の過検出も、11 種類のときだけ出る。
* Dart: `rc(f) { run(() => f()); }`、`viaInner(a, (g) => g("q"))` は落ち、`unresolved_callee` はヘルパーの中のクロージャに出る(PARAM-CALL)。
* Dart: `catchError`、`putIfAbsent`、`Map.forEach`、`whenComplete`、`await Future.error(...)` はすべて落ちる。同じ形の `forEach`、`fold` は見つかる(CB-LIST、AWAIT-ERR)。
* Rust: `*e.o = e.t`(`Env { o: &mut out, t }`、ヘルパーなし、関数 1 つ)で落ちた。`fn w(o: &mut String, t)` を呼ぶ形は見つかる(ENV-PTR)。ADR-072 で、`rust-curated/src/capture.rs` の `struct_mut_field` / `struct_mut_field_helper` が見つかるようになった。
* Rust: `fn g<T>(items: Vec<T>) -> Vec<T> { for i in items { o.push(i) } }` は落ち、`Vec<String>` では見つかる。`Fn` の呼び出しでなく、`for` の `into_iter` / `next`(dynamic)で落ちる(GEN-DYN)。
* Go: 返したクロージャ(`With(src())` を変数に入れて `o(q)`)、クロージャをヘルパーに渡す形(`apply(func(q *Q){ q.name = n }, q)`)は落ちる。クロージャをその場で作って呼ぶ形は見つかる(RET-CLOSURE と CAP)。

## issue(サイレントな見逃し、実コードで起きそうなもの)

同じ原因は 1 つにまとめた(8 件)。

| issue | 型 | 内容 |
| --- | --- | --- |
| #76 | `CAP` | ヘルパーに渡したクロージャのキャプチャ値(読み書き)が落ちる(Dart、Go) |
| #77 | `OVERFLOW` | ヘルパーに渡すクロージャが 9 種類以上で、呼び出しが未解決(Dart) |
| #78 | `RET-CLOSURE` | 関数が返したクロージャの呼び出し(Dart、Go の functional options) |
| #79 | `STORE` | List / フィールドに持たれたコールバック(Dart、Rust) |
| #80 | `CB-LIST`、`AWAIT-ERR` | 標準ライブラリのコールバックの決め打ちのリスト(Dart) |
| #81 | `PARAM-CALL` | クロージャの関数引数 / キャプチャした関数の呼び出し(Dart) |
| #82 | `ENV-PTR` | `&mut` を持つフィールド(クロージャの環境)経由の書き込み(Rust) |
| #83 | `GEN-DYN` | ジェネリックな T の trait 呼び出しで、モデルが receiver を見ない(Rust)。`-rules` で `from: recv` を足すと直ることを確認した |

issue にしていない `@miss` / `@fp`: `MODEL-OVER`(過検出のみ)、`KEYED-MAP`、`ALLOWLIST`(Go の過検出。サイレントな見逃しではない)、`XTALK`(過検出)。

## 再現の手順

```sh
export RIVUS_DART="mise exec dart@3.13.6 -- dart" RIVUS_RUST=$PWD/extractor/rust/target/debug/rivus-rust
go test ./cmd/rivus -run Curated -v     # 注釈の検査(この表の元の数)
# 痕跡は、extractor の出力に対して:
(cd extractor/dart && dart run bin/rivus_dart.dart ../../bench/dart-curated > dart.jsonl)
(cd bench/dart-curated && rivus analyze -ir ../../extractor/dart/dart.jsonl -rules rules.yaml)   # findings と uncertainties
```

## BENCH-2: 経路に source の関数が出ない finding(`offSourcePath`)

finding の `source.position` の関数(`source.func`)が、最初の経路(`paths[0]`)のステップのどれにも出ない finding。出ないのは、source が、別の関数が書き戻したオブジェクトから届いたとき(CORE-A / CORE-B。`TestParamSourceSharedObject`)。検査は `cmd/rivus/offsource_test.go` の `offSourcePath` で、`TestDogfood`、`TestCurated{Go,Dart,Rust}`、`TestParamSourceSharedObject`、`TestGlobalWriterIsOnThePath` が使う。数は上限(今の値)で、上がると失敗する。下がっても失敗する(上限を下げる)。CORE-A(ADR-048)と CORE-B(ADR-049。グローバルのストアに書いた側のステップを持たせた)で、全部 0 になった。下の表は、2026-11-21(HEAD d85ee95 + CORE-A の前)の値で、記録として残す:

| 場所 | 数 | 型 |
| --- | --- | --- |
| `TestDogfood`(rivus 自身、88 件) | 77 | 1 つの source が書き戻したオブジェクトを、他の source(8 つのコマンド)が読む(CORE-A / CORE-B)。経路が、読む側の関数から始まる |
| `paramshare/global` | 1 | CORE-B: finding も偽(CORE-A)で、経路に `runB` が無く、`global_var` の不確実性も付かない |
| `paramshare/repro` | 1 | CORE-B: 本物の流れが 2 つあり、`runB` の経路は `runA` を通る |
| `paramshare/globalw` | 1 → 0 | CORE-B(直った): 経路は `R` の load から始まり、書き込み側の `W` は `global_var` の候補にしか出ない |
| `paramshare` の他(ptr、local、distinct、samelocal、str、ptype) | 0 | (ptr / local の finding は、CORE-A で偽だが、経路には source の関数がある) |
| go-curated / dart-curated / rust-curated | 0 / 0 / 0 | 違反は無い(`@noisy` の 183 件、`@fp` を含む) |

## CVE の形のフィクスチャ(T-5、2026-11-21)

`bench/*/truth*.json` の正解(Gitea v1.26.4 の CVE 3 件、gin-vue-admin、Gogs、dart-pub、parseable)の、source から sink までの流れの**構造**を、小さいフィクスチャにして、curated に足した(コードのコピーではない)。実際のコードは、そのコミット / タグの GitHub のソースを読んだ(scratchpad のコピーは、修正後だった)。重い解析(Gitea 等)の代わりに、**CI(`go test ./...`)で、退行に気づくため**。**コアは変えていない。**

| 正解 | フィクスチャ | 形 | 今の main(HEAD 6e6f17e) |
| --- | --- | --- | --- |
| Gitea GHSA-4p89 `release.go:338` | go-curated `gitea/routers/release.go` → `gitea/service/release.go:47`、`gitea/storage/storage.go:38` | ハンドラーが `ctx.Req.Form` を `range` し、キーを `strings.HasPrefix` で選んで、**ループの中の `append`**(`k[len(prefix):]`)で集める。サービスの `UpdateRelease`(6 引数)に渡し、`db.WithTx` のクロージャ(行の読み出しだけ)のあと、**ループで** `storage.Attachments.Delete(AttachmentRelativePath(uuid))`(インターフェース、実装 2 つ)。実物の v1.26.4 では、この `Delete` は `WithTx` のクロージャの**外**にある | 見つかる(`@source(release_del)`、sink 2 か所) |
| Gitea GHSA-x77v `oauth.go:311` | `gitea/routers/oauth.go` | `goth.User`(値の構造体)を引数に取る関数が、`gothUser.AvatarURL` を、別の関数に渡して、`http.NewRequestWithContext(ctx, GET, url, nil)` | 見つかる(`oauth_avatar`) |
| Gitea GHSA-8vmh `openid.go:26` | `gitea/routers/openid.go` → `gitea/openid/openid.go` → `ext/openidgo` | `setting.AppURL + ctx.Req.URL.String()[1:]` を、自前のラッパー(`Verify(fullURL)`)経由で、依存のライブラリの `Verify`(ルールが入口をシンクに指定) | 見つかる(`openid_verify`) |
| gin-vue-admin GHSA-gv3w | `gva/api/autocode.go` → `gva/service/autocode.go` | `c.Bind(&req)`(`out` の source)、`req.Snake = strings.ToLower(req.PlugName)`、構造体を値で渡し、`filepath.Join` + `fmt.Sprintf` で作ったパスを `os.MkdirAll` / `os.OpenFile` へ | 見つかる(`gva_plug`、sink 2 か所) |
| Gogs GHSA-qf5v(`PutContents`) | `gogs/routes/repo.go` → `gogs/database/repo.go` | `c.Param("*")` を、オプションの構造体に入れて、メソッド経由で `path.Join(localPath, ...)` → `os.MkdirAll` / `os.WriteFile` | 見つかる(`gogs_put`、sink 2 か所) |
| Gogs GHSA-mrph(フックの名前) | `gogs/routes/repo.go`(`SettingsGitHooksEdit`) | `c.Param(":name")` を、依存の `Repository.Hook("hooks", name)` に(呼び出しがシンク) | 見つかる(`gogs_hook`) |
| dart-pub GHSA-q739 | dart-curated `lib/pub_tar.dart` | `while (await reader.moveNext())` のループで `reader.current` の `entry.name`(getter が source)を、`joinAll([destination, ...entry.name.split('/')])` に入れ、`switch (entry.type)` の 4 つの枝で、`ensureDir`、`deleteIfLink`、`createFileFromStream`、`_chmod`、`createSymlink`(9 か所) | 9 か所とも見つかる(`pub_name`) |
| parseable `oidc.rs:443`(open redirect) | rust-curated `src/parseable.rs`(`logout` → `redirect_to_client`) | `web::Query<..>` の `redirect` を `ResponseBuilder::insert_header(("Location", url))` へ | 見つかる(`pb_redirect`) |
| parseable `localfs.rs:383` | 同(`post_filter`) | `web::Json<Filter>` の `stream_name` で作ったパスを、`Box<dyn ObjectStorage>` の `put_object`(実装 2 つ)の `fs::write` へ | 見つかる(`pb_store`) |
| parseable `target.rs:494`(SSRF) | 同(`post_target`、`fire_alert`) | `web::Form<Target>`(parseable では Json。別の source の行を持たせるため型を変えた)を、`OnceLock<RwLock<HashMap>>` に入れ、別の関数が読んで `client.post(&self.endpoint)` | **見つからない(`@miss(silent)`)**。ADR の記録(2/3 検出、これは見逃し)と同じ。uncertainty: source / sink の関数には無い。`targets()` に `unmodeled_library_call`(`OnceLock::new`、`get_or_init`)がある(source でも sink でもない関数) |
| parseable の、クライアントが宛先を選ぶ形の SSRF(未公表のため、件数と種類だけ) | rust-curated `src/parseable.rs` | 同ファイルの写しを参照 | 見つかる(`pb_node`) |

### 新しく足した見逃し(3 件)

Gitea の CVE の 3 件(上の表)は**見つかる**。足したときに、**Gitea のほぼ全部のサービスの書き方**の見逃しが見つかった(`db.WithTx(ctx, func(ctx) error { ... })`: 関数が、キャプチャした値を使うクロージャを、ヘルパーに渡して、ヘルパーが呼ぶ)。Go でも、Dart の `CAP` と同じ形が起きる:

| タグ | source → sink | uncertainty | 型 | 備考 |
| --- | --- | --- | --- | --- |
| `tx_capture` | gitea/routers/tx.go:17 → 20 | `@miss(dynamic_dispatch)`: sink の関数(`DeleteInTx$1`)に、`ObjectStorage.Delete` の dynamic_dispatch(候補 2)。`db.WithTx` の `f(ctx)` にも dynamic_dispatch(候補は、プログラムのクロージャ 3 つ: `DeleteInTx$1`、`RunInTx$1`、`UpdateRelease$1`) | `CAP`(Go 版) | `db.WithTx(ctx, func(c) error { return storage.Attachments.Delete(uuid) })`。`uuid` は、`ctx.Req.FormValue` から |
| `tx_capture_plain` | gitea/routers/tx.go:26 → 29 | `@miss(silent)`(`sys.Run` は、インターフェースの後ろではない) | `CAP`(Go 版) | 同じ形で、シンクが直接の関数。**サイレントな見逃し** |
| `pb_target` | src/parseable.rs:28 → 173 | `@miss(silent)` | 保存と読み出し(グローバルの `OnceLock<RwLock<HashMap>>`)。ADR の推定で、私は切り分けていない | 上の表 |

小さい例での確かめ(`mini`): `func C(s string) { withTx(func() error { sink(s); return nil }) }`(`withTx(f) { return f() }`)は、findings 0 件、不確実性 0 件。クロージャが**引数**を取る形(`withTx(func(a string) error { sink(a) }, s)`)と、クロージャを変数に入れて**その場で**呼ぶ形(`f := func() { sink(s) }; f()`)は、見つかる。

### 退行の二分探索の材料(ADR の 3/3)

* ADR の「3/3 検出」の追記は、`5754550`(2026-11-19、`feat: findingに到達可能性…、GiteaのCVEを3/3検出`)。「ループの中の `append`」の修正は、`288dc80`(2026-11-19)。
* **`5754550` と `288dc80` で、同じ小さい例(上の Gitea 3 件)を流した結果は、今の main と同じ**(3 件とも見つかる。`288dc80` の親では、`release` の流れだけが見つからない: `append` の修正のフィクスチャとして働く)。`tx_capture` / `tx_capture_plain` は、この 2 つのコミットでも見つからない(`mini` も同じ)ので、**これらは、2026-11-19 以降の退行ではない**(最初から、この形は追っていない)。
* つまり、保存した最近の実行(C-1 の off / on)の **1/3** は、小さい形の退行では説明できない。残る候補は、規模に依存するもの(`depth_limit` が約 1,500 件で、経路が切れる、`-func-timeout` で関数が止まる、収束しない、`-k` や実行の設定が違う)。ADR の 3/3 は、`depth_limit` 1,499 件、収束せずの結果だった。

## C-7 のあと(ADR-059): `CAP` の見逃しが直った

`make_closure` のキャプチャした値を、作った場所で、クロージャの要約に適用した(渡す、保存する、返すクロージャだけ。不確実性 `assumed_call`)。`@miss` の数(テストが数える、6c5c28c → 今): Go 15 → 3(silent 5 → 2、uncertainty あり 10 → 1)、Dart 47 → 30(silent 31 → 10、uncertainty あり 16 → 20)、Rust 16 のまま(対象外。次の段階は `docs/DESIGN-capture.md` の「Rust の節」)。外れた `@miss`: Go の `cap-run-a/b`、`cap-write`、`cap-nested`、`cap-loop`、`cap-two-a`、`cap-returned`、`cap-late`、`cap-handler`、`cap-funccall`(設計では直らない見込みだったが、直った)、`tx_capture`、`tx_capture_plain`。Dart の `btn-a/b`、`timer-run`、`periodic`、`timer-write`、`microtask`、`delayed`、`late`、`write-a/b`、`add-list`、`across-cap`、`run-a/b`、`relay-c-a/b`、`when-complete`。`assumed_call` の `@miss`(結果を適用しない): Go の `cap-result`、Dart の `cap-result`、`cap-result-then`、`cap-fn`、`nest-a/b` ほか(次の段階: 結果をクロージャの値に持たせる)。残る `CAP` の見逃しは、キャプチャと引数の両方に依る流れ(案 2)、`bound-prefix`(メソッドの tear-off、R-14)。

**限界(バックログ B)**: `expandParam` が、構造体全体の要素(`Param(i, "")`)を読むとき、サブセルの要素(フィールドの指す先の要素)を、根へ持ち上げる。構造体を丸ごとコピーして書き戻す形(`before := Picture; Picture = before`)で、あるフィールドの指す先の汚染が、全フィールドに広がる。C-7 は、これを gogs の `conf.SetMockPicture$1`(テストのヘルパー)で露出させ(+45 件)、案 D(同じグローバルの全体を読んだ値の書き戻しだけの書き込みは適用しない)で、その形だけを避けた。ふつうの呼び出し(`setG(before)`)には、同じ持ち上げが残る。優先度の根拠: gogs の基準の 77 件には、`conf.Picture` を通るものは 0 件(数えた)。
