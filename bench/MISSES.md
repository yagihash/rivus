# 見逃し(`@miss`)と過検出(`@fp`、`@noisy`)の、原因の型での分類

2026-11-21 の作業ツリー(HEAD 3424363)で、`dart-curated` / `rust-curated` / `go-curated` の全部の `@miss` と `@fp` と `@noisy` を、実際に流して確かめた。**エンジン、extractor は変えていない。** 原因の見立てのうち、実験(下の「確かめ方」)で確かめたものは、表の備考に書いた。確かめられていないものは「未特定」と書く。

## 確かめ方

* 流れの有無と不確実性: `rivus analyze -ir <extractor の出力> -rules rules.yaml`(Go は `./...`)の `findings` と `uncertainties`。**痕跡**は、`rivus resolve` が「見つからない理由」として出すのと同じ定義(source か sink の関数にある不確実性)で調べ、source / sink の行の近くのものも見た。ヘルパー側(経路の途中)にある不確実性は「ヘルパー側」と別に書く。
* **黙った**: source か sink の関数、または行に、その見逃しに関わる不確実性が 1 つも出ていないもの。「弱い」は、候補が 1 つの `dynamic_dispatch`(メソッド呼び出しのたびに出る)だけで、コールバックが落ちたことを言っていないもの。「痕跡」は、落ちた呼び出しそのものを指す不確実性(`unresolved_callee`、モデルが無い、など)。Rust は、実行全体の `depth_limit`(`path_limit`、「流れは増えるが減らない」と書いてある)が常に出るが、見逃しの説明にならないので数えない。
* IR: `rivus extract -text` は Go 専用なので、`ir.Read` / `Func.Format` で Dart と Rust の IR を読んだ(extractor の出力 `*.jsonl`)。**どの `@miss` でも、IR に流れは載っている**(Dart のクロージャの捕獲は `make_closure` の束縛、Rust は環境の構造体のフィールド)。extractor の側で落ちているものは、無い。
* 実験: bench のコピー(scratchpad)に、小さいケースを足して、原因を 1 つずつ切り分けた(呼び元の数、同じ関数の中で呼ぶか、ジェネリックかなど)。実験の結果は備考に「実験」と書いた。

## 原因の型

| 記号 | 型 | 落ちる場所(エンジンのコード) |
| --- | --- | --- |
| `CAP` | 捕獲した値が、作った関数の外から呼ぶと落ちる | internal/valuecalls/valuecalls.go:319 funcsOf(束縛は、クロージャを作った関数 `home` でだけ付く)→ internal/sg/build.go:362 resolveValueCalls(`c.Bind != nil` のときだけ CandidateBindings)→ internal/taint/summary.go:283-286 applySummary(束縛が無いと FreeVars を KUndef で埋める。不確実性なし) |
| `OVERFLOW` | ヘルパーに渡すクロージャが 9 種類以上で、呼び出しが未解決 | internal/valuecalls/valuecalls.go:124(`len(found) > most`、most=8)→ internal/taint/calls.go callBody の `len(fns)==0` 分岐(`s.unmodeled`: 引数を結果へ流すだけ。コールバックの中の sink・throw・書き込みは届かない) |
| `PARAM-CALL` | クロージャの関数引数 / 捕獲した関数の呼び出しが未解決 | internal/valuecalls/valuecalls.go:142 indexCallers(値の呼び出しで呼ばれる関数を callers に入れない。`g` の呼び元が見つからない)と classify の refParam(宣言した引数だけ。捕獲した関数 `f` は refOther) |
| `RET-CLOSURE` | 関数が返したクロージャの呼び出し | Dart: internal/valuecalls/valuecalls.go classify(呼び出しの結果は refOther: 候補なし → unresolved_callee)。Go: VTA の候補は付くが束縛が無い(internal/taint/calls.go callees: `Bindings` は静的なクロージャだけ) |
| `STORE` | コールバックを List / フィールドに持ち、あとで呼ぶ | internal/valuecalls/valuecalls.go(冒頭のコメント: メモリを辿らない。`iter.next` / `field_load` は refOther)。呼び出しは、ヘルパー側の unresolved_callee / unmodeled_library_call |
| `CB-LIST` | 標準ライブラリのコールバックの決め打ちのリストにない | extractor/dart/lib/exprs.dart:_callbackMode(Future.then と、Iterable / Stream の数個だけ)と internal/model/builtin/dart-async.yaml / dart-core.yaml(モデルは args→結果、recv→結果だけ。コールバックの戻り値を結果へ繋がない) |
| `AWAIT-ERR` | await した Future.error が、catch に値を渡さない | internal/model/builtin/dart-async.yaml の Future.error(args→結果)と extractor の `[await]` 呼び出し(失敗した Future の await を throw にしない)。細部は未特定 |
| `ENV-PTR` | ポインタをフィールド(クロージャの環境)に入れて、そこから書くと別名にならない | internal/taint/locs.go:215 computeLocs(OpFieldLoad: `locs(base)+field+"*"` を指す)と internal/taint/transfer.go:148(OpFieldStore はポインタの値を保存するだけで、指す先と同一視しない)。関数内の `*e.o = x`(Env{o:&mut out})でも再現 |
| `GEN-DYN` | ジェネリックな T に対する trait 呼び出し(dynamic)で、モデルの `from: args` が receiver を含まない | internal/taint/calls.go selectVals(`SelArg All` は cc.args)+ internal/sg/sg.go:139 CallArgs(dynamic は Args[1:])+ internal/model/builtin/rust-std.yaml(`Iterator::next` / `IntoIterator::into_iter` は from: args)。`for i in items`(items: Vec<T>)で流れが消える |
| `XTALK` | Fn*::call(dynamic)の候補が「型の合う全クロージャ」で、呼び元ごとに絞らない | internal/valuecalls/valuecalls.go:50(CalleeValue だけを解決。Rust は dynamic)と internal/taint/calls.go:355-380 dispatchGuards / makeDispatchGuards(候補が method でない(`!fn.Recv`)と、ガードなし) |
| `MODEL-OVER` | モデルが、コールバックの戻り値を見ずに recv を結果へ流す | internal/model/builtin/dart-async.yaml Future.then / dart-core.yaml Iterable.map / rust-std.yaml Iterator::map・Option::and_then(recv/args→結果) |
| `KEYED-MAP` | マップのキーを区別しない(Set/Get のキーが違っても同じセル) | internal/taint/transfer.go OpIndexStore / OpIndexLoad(キー付きは、値を `[]` に持つ) |
| `ALLOWLIST` | 分岐条件(switch の許可リスト)を、汚染を落とすものと見ない | internal/taint/guards.go(条件は記録するだけで、評価しない: Guard の説明) |

案 2 = ADR-047 の下書き(ヘルパーの要約に、コールバックの出力を「仮想のパラメータ」で持たせ、呼び元が、自分で渡したクロージャ(`ExactFuncs`)の要約で具体化する)。「直る(見込み)」は、下書きの記述から読んだ推測で、実装して確かめたものではない。直るには、`ExactFuncs` が、クロージャの束縛(`make_closure` の引数)も返す必要がある(下書きに書いていない)。

## 未解決のレビュー指摘(RUST-13、RUST-14、DART-12)と、そのフィクスチャ

別セッションのレビューの識別子(docs には無い)。2026-11-21、main(HEAD 1a429f0 の作業ツリー)で、原因のコードが残っていることと、落ちるフィクスチャを確かめた。**コアは変えていない。** 対応する issue は無い(#76〜#83 は、どれもこの 3 つの原因を挙げていない)。上の集計と表は、この節のフィクスチャを足す前の数のまま(足したあとの数は、下の表)。

| 指摘 | 原因のコード(今) | フィクスチャ | 今の結果 |
| --- | --- | --- | --- |
| RUST-13: `Fn` / `FnMut` を `&self` で呼ぶと、受け手がセルで、呼び元ごとに絞れない | internal/taint/calls.go:357 `dispatchGuards`(`recv.Kind != sg.KParam` で nil)。さらに makeDispatchGuards は、候補が method でない(`!fn.Recv`)と、ガードを付けない(クロージャは全部これ) | rust-curated/src/fnself.rs: `fn_ret_clean` / `fn_mut_ret_clean`(`@sink(none) @fp`)、`fn_sink_a/b`、`fn_mut_sink_a/b`(`@nosink` + `@fp`)。対照の `fn_ref_clean`(`&impl Fn`、受け手がパラメータそのもの) | 6 か所とも `@fp`(落ちる)。対照も落ちる: 受け手がセルであることだけが原因ではなく、クロージャの候補にガードが付かない(`!fn.Recv`)ことも原因。案 2 の受け入れは、この 7 か所の `@fp` が全部消えること |
| RUST-14: モデルを当てると、`read_impls` でない動的呼び出しの本体を読まない | internal/taint/calls.go:182-196(`modelled` のあと、`!rules.ReadsImpls(cc.sym)` で return)。`read_impls: true` は rust-std.yaml と rust-misc.yaml だけ | rust-curated/src/modelled.rs: `model_body`(source が実装の本体の中、`@miss`)、`model_drop`(実装が引数を捨てる、`@sink(none) @fp`)。rules.yaml に、プロジェクトの trait のメソッド(`Render::render`、`Mask::mask`)の propagator(recv→result)を足した | `@miss` 1、`@fp` 1(どちらも黙る)。モデルを外して流すと、`@miss` の流れは見つかり、`@fp` は静かになる(実験)。本体を読む修正(別件)が要る |
| DART-12: `dynamic` のメソッド呼び出しが、`dart:core#dynamic.invoke` の静的呼び出しになり、モデルで覆われて痕跡が残らない | extractor/dart/lib/exprs.dart:347(`_methodInvocation`)、internal/model/builtin/dart-core.yaml:8(`flows: [{from: args, to: result}]`) | dart-curated/lib/dynamic_calls.dart: `dynamic_arg`(メソッドの中の sink、`@miss`)、`dynamic_body`(メソッドの本体の source、`@miss`)、`dynamic_drop`(`@sink(none) @fp`) | `@miss` 2、`@fp` 1。モデルを外す対照は、していない |

足したあとの数: Dart `@miss` 34、`@fp` 5。Rust `@miss` 12、`@fp` 22、`@noisy` は 183 件のまま(新しいフィクスチャは、他と型が重ならない署名(`u8`、`u16`、`u32`、`u64`、`i8`)で、他のクロージャを候補にしない)。

既存の `@miss` / `@fp` のうち、この 3 つに当たるもの: **RUST-13 は、XTALK の `@fp`(12)と `@noisy`(16 か所)が同じ形**(ヘルパーが `f(..)` で呼ぶ `impl Fn`)。1 件ずつ、受け手がセルかは確かめていない。**RUST-14 と DART-12 に当たる既存のものは、無い**(`@miss` の原因の型のどれにも、モデルが本体を隠すものは無かった)。

案 2(ADR-047)で RUST-13 が直る見込み: 案 2 は、ヘルパーの要約にコールバックの出力を持たせ、呼び元が渡したクロージャで具体化するので、`dispatchGuards` を通らない。下書きの記述から読んだ推測で、実装して確かめたものではない。RUST-14 と DART-12 は、案 2 では直らない(別の修正)。

## 集計

### `@miss`

| 言語 | `@miss` | 黙った | 弱い痕跡 | 痕跡あり | 案 2 で直る(見込み) | 不明 | 直らない | 未特定 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Dart | 32 | 19 | 6 | 7 | 11 | 6 | 15 | 0 |
| Rust | 11 | 10 | 1 | 0 | 0 | 0 | 11 | 0 |
| Go | 1 | 1 | 0 | 0 | 0 | 0 | 1 | 0 |
| 計 | 44 | 30 | 7 | 7 | 11 | 6 | 27 | 0 |

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
| `ENV-PTR` | 0 | 7 | 0 | 7 |
| `GEN-DYN` | 0 | 1 | 0 | 1 |

### `@fp` と `@noisy`

| 言語 | 種類 | 件数 | 原因の型 | 案 2 で直る(見込み) |
| --- | --- | --- | --- | --- |
| Dart | `@fp` | 4 | OVERFLOW 2, MODEL-OVER 2 | 直る 2、直らない 2 |
| Rust | `@fp`(`@nosink` 8 + `@sink(none)` 6) | 14 | XTALK 12(うち `@nosink` 8)、MODEL-OVER 2 | 直る 9、不明 3、直らない 2 |
| Rust | `@noisy` | 183 件、16 か所 | XTALK(全部) | 直る 11 か所 156 件、不明 5 か所 27 件 |
| Go | `@fp` | 5 | KEYED-MAP 4、ALLOWLIST 1 | 直らない 5 |

Rust の `@fp` 14 件の内訳: XTALK 12 件(直る 9: arg-a/b、run-a/b、sinkwith-a/b の 6 件と ret-clean、produce、fn-drop。不明 3: relay-c-a/b と emit-two の 2 つ目)、MODEL-OVER 2 件(直らない)。
Rust の `@noisy` は README に「17 か所」とあるが、ソースの注釈は 16 か所で、合計 183 件になる(下の表)。

## 黙った見逃し(先頭にまとめる)

見逃していて、source / sink の関数に何の不確実性も出ていないもの。`rivus resolve` の `uncertainties` を見ても、手がかりが無い。

### Dart(19 件)

| タグ | source → sink | 原因の型 | ヘルパー側の痕跡 |
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

### Rust(10 件)

| タグ | source → sink | 原因の型 | ヘルパー側の痕跡 |
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

Rust の `across-each` は、`numeric_stop`(`vec!` のマクロの中。見逃しの説明にならない)が出ているので「弱い」にした。

### Go(1 件)

* `options`: api/more.go:32 → api/more.go:33(ヘルパー側: api/more.go:25 の `o(q)` に dynamic_dispatch)

## Dart

`@miss` 32。落ちる場所は、原因の型の表を見る。

| タグ | source → sink | 痕跡 | 型 | 案 2 | 備考 |
| --- | --- | --- | --- | --- | --- |
| `across-hub` | lib/across.dart:25 → 24 | 弱い: dynamic_dispatch(mixed、候補 1)が across.dart:24,25。ヘルパー shared.dart:15 に unresolved_callee | `STORE` | 直らない | Hub の `_subs`(List)に入れて emit で呼ぶ |
| `across-cap` | lib/across.dart:30 → 34 | 黙った | `CAP` | 直る | `each([1], (i){ out = t; })`。別パッケージの helper |
| `write-a` | lib/capture.dart:8 → 12 | 黙った | `CAP` | 直る | `run(() { out = t; })`、`out` はセル(make_closure の 2 つ目の束縛) |
| `write-b` | lib/capture.dart:17 → 21 | 黙った | `CAP` | 直る |  |
| `write-param` | lib/capture.dart:38 → 42 | 黙った | `CAP` | 直る | `fireAndForget(t, (x){ out = x; })`。引数 x ではなく、捕獲した out のセルが落ちる |
| `add-list` | lib/capture.dart:48 → 52 | 黙った | `CAP` | 直る | `run(() { items.add(t); })` |
| `relay-c-a` | lib/capture.dart:76 → 77 | 黙った(ヘルパー側: capture.dart:72 に unresolved_callee) | `PARAM-CALL` | 不明 | `relayCapture(f) => run(() => f())`: 内側のクロージャが捕獲した関数 `f()` を呼ぶ。draft 条件 3(中継)に当たる |
| `relay-c-b` | lib/capture.dart:81 → 82 | 黙った(同上) | `PARAM-CALL` | 不明 |  |
| `holder-a` | lib/capture.dart:102 → 104 | 痕跡: unresolved_callee(capture.dart:104、sink の行) | `RET-CLOSURE` | 直らない | `holder(a)` が `() => s` を返し、`get()` で呼ぶ |
| `holder-b` | lib/capture.dart:108 → 110 | 痕跡: unresolved_callee(capture.dart:110) | `RET-CLOSURE` | 直らない |  |
| `emit` | lib/classes.dart:19 → 18 | 弱い: dynamic_dispatch(mixed、候補 1)が classes.dart:18,19。ヘルパー classes.dart:11 に unresolved_callee | `STORE` | 直らない | Emitter の `_subs` に入れて emit で呼ぶ |
| `emit-two` | lib/classes.dart:28 → 26 | 弱い(同上) | `STORE` | 直らない |  |
| `lazy` | lib/classes.dart:40 → 42 | 弱い: dynamic_dispatch(classes.dart:42)。ヘルパー classes.dart:36 に unresolved_callee | `STORE` | 直らない | `Lazy(this._make)` がフィールドに持ち、getter で呼ぶ |
| `builder` | lib/classes.dart:61 → 63 | 弱い: dynamic_dispatch(mixed)が classes.dart:62,63 | `CAP` | 不明 | `Builder().with_((x) => x.add(a))`: 捕獲した `a`。`f(this)` で呼ぶ。add が dynamic(候補 1) |
| `getter-fn` | lib/classes.dart:83 → 87 | 痕跡: unresolved_callee と unmodeled_library_call(Source.new、本体なし)が classes.dart:87 | `RET-CLOSURE` | 直らない | getter が `() => source()` を返す。source はクロージャの中 |
| `through` | lib/errors.dart:53 → 57 | 黙った(ヘルパー側: helpers.dart:4 に unresolved_callee) | `OVERFLOW` | 直る | `transform(a, (x) => throw AppError(x))`。transform へ渡すクロージャが 9 種類以上(bench には多数)。4 種類なら見つかる(実験で確認) |
| `future-error` | lib/errors.dart:86 → 88 | 黙った | `AWAIT-ERR` | 直らない | `await Future<void>.error(AppError(source()))` を `on AppError catch (e)` で受ける |
| `on-error` | lib/errors.dart:94 → 95 | 黙った | `CB-LIST` | 直らない | `Future.error(a).then((v) => v, onError: (e) => sink(e))`: `then` の 2 つ目のコールバックを呼ばない |
| `catch-error` | lib/futures.dart:35 → 37 | 黙った | `CB-LIST` | 直らない | `Future.error(source()).catchError((e) { sink(e); })` |
| `when-complete` | lib/futures.dart:44 → 48 | 黙った | `CB-LIST` | 直らない | `whenComplete(() { out = t; })`: コールバックを呼ばず、捕獲もある(CAP も) |
| `completer` | lib/futures.dart:58 → 60 | 黙った | `CB-LIST` | 直らない | `Future<String>.sync(() => t)`: コールバックの戻り値が結果に繋がらない |
| `map-entries` | lib/iterables.dart:79 → 80 | 痕跡: dynamic_dispatch(rivus、「モデルがあれば解決する」)が iterables.dart:80 | `CB-LIST` | 直らない | `Map.forEach((k, v) => ...)`: 2 引数のコールバック。モデルも extractor の決め打ちも無い |
| `put-if-absent` | lib/iterables.dart:85 → 87 | 黙った | `CB-LIST` | 直らない | `m.putIfAbsent("k", () => t)`: モデルは recv→結果だけ |
| `nest-a` | lib/nesting.dart:8 → 12 | 黙った(ヘルパー側: helpers.dart:4) | `CAP` | 直る | `transform("const", (x) => transform(a, (y) => y))`: 外側のクロージャが `a` を捕獲 |
| `nest-b` | lib/nesting.dart:16 → 20 | 黙った(同上) | `CAP` | 直る |  |
| `twice-args` | lib/nesting.dart:43 → 47 | 黙った | `CAP` | 直る | `fireAndForget(a, (x) => out.add(x))`: 捕獲した `out` |
| `takes-a` | lib/nesting.dart:52 → 54 | 痕跡: unresolved_callee(nesting.dart:53、クロージャの中の `g(a)`) | `PARAM-CALL` | 不明 | `viaInner("const", (g) => g(a))`: クロージャが関数引数 `g` を呼ぶ(コールバックのコールバック) |
| `takes-b` | lib/nesting.dart:58 → 60 | 痕跡: unresolved_callee(nesting.dart:59) | `PARAM-CALL` | 不明 |  |
| `curry` | lib/nesting.dart:133 → 135 | 痕跡: unresolved_callee(nesting.dart:135) | `RET-CLOSURE` | 直らない | `add(a)("x")`: クロージャが返すクロージャ |
| `run-a` | lib/pass_closure.dart:49 → 50 | 黙った | `CAP` | 直る | `run(() => sink(a))`: 実験で、呼び元が 1 つでも落ちる |
| `run-b` | lib/pass_closure.dart:54 → 55 | 黙った | `CAP` | 直る |  |
| `bound-prefix` | lib/tearoff.dart:111 → 112 | 弱い: dynamic_dispatch(mixed)が tearoff.dart:112 | `CAP` | 不明 | `transform(s, stamp)`(メソッドの tear-off が `this` を捕獲)。transform の 9 種類超(OVERFLOW)も重なる |

### Dart の `@fp`(4)

| タグ | 場所(source → sink) | 型 | 案 2 | 備考 |
| --- | --- | --- | --- | --- |
| `ret-clean` | pass_closure.dart:21 → 23 | `OVERFLOW` | 直る(見込み) | `transform(c, (x) => "fixed")`: transform が未解決(9 種類超)で、引数→結果の保守側が働く。呼び元 1 つの実験では出ない |
| `tear-drop` | tearoff.dart:21 → 22 | `OVERFLOW` | 直る(見込み) | `transform(a, c.drop)`: 同上 |
| `then-const` | futures.dart:23 → 25 | `MODEL-OVER` | 直らない | `f.then((v) => "fixed")`: Future.then のモデルが recv→結果 |
| `map-const` | iterables.dart:58 → 60 | `MODEL-OVER` | 直らない | `items.map((s) => "fixed")`: Iterable.map のモデルが recv→結果 |

`@noisy` は Dart には無い(クロージャが、呼び元ごとに結び付く: `valueGuards`)。

## Rust

### Rust の `@miss`(11)

| タグ | source → sink | 痕跡 | 型 | 案 2 | 備考 |
| --- | --- | --- | --- | --- | --- |
| `across-each` | src/across.rs:18 → 19 | 弱い: numeric_stop(rivus、`vec!` のマクロ内 macros.rs:60 に記録。この flow と無関係に見える)。ヘルパー側 shared/src/lib.rs:8 に dynamic_dispatch(「実装が分からない」) | `GEN-DYN` | 直らない | `shared::each<T>(items: Vec<T>, f)`: `for i in items` が dynamic `into_iter` / `next`。同じ形を同じクレートに書いても、T がジェネリックなら再現(実験)。Fn 版は見つかる |
| `across-hub` | src/across.rs:25 → 24 | 黙った(ヘルパー側: shared/src/lib.rs:25 に unmodeled_library_call) | `STORE` | 直らない | `Vec<Box<dyn Fn>>` に入れて emit で呼ぶ |
| `across-cap` | src/across.rs:30 → 32 | 黙った | `ENV-PTR` | 直らない | `shared::each(vec![1], \|_i\| out = t.clone())` |
| `write-a` | src/capture.rs:8 → 12 | 黙った | `ENV-PTR` | 直らない | `run(\|\| { out = t; })`。呼び出しが同じ関数内(`let f = \|\| {out = t;}; f();`)でも落ちる(実験) |
| `write-b` | src/capture.rs:17 → 21 | 黙った | `ENV-PTR` | 直らない |  |
| `write-param` | src/capture.rs:38 → 42 | 黙った | `ENV-PTR` | 直らない | `fire_and_forget(t, \|x\| { out = x; })` |
| `push-list` | src/capture.rs:48 → 50 | 黙った | `ENV-PTR` | 直らない | `run(\|\| items.push(t))` |
| `late` | src/capture.rs:134 → 137 | 黙った | `ENV-PTR` | 直らない | `let mut set = \|x\| v = x; set(a);`(ヘルパーなし) |
| `for-each-cap` | src/iters.rs:26 → 28 | 黙った | `ENV-PTR` | 直らない | `items.into_iter().for_each(\|x\| last = x)` |
| `emit` | src/structs.rs:74 → 73 | 黙った(ヘルパー側: structs.rs:66 に unmodeled_library_call) | `STORE` | 直らない | `Vec<Box<dyn Fn>>` に入れて emit で呼ぶ |
| `emit-two` | src/structs.rs:83 → 81 | 黙った(同上) | `STORE` | 直らない |  |

Rust の `@miss` は、案 2 では 1 つも直らない(11 件とも、別の原因)。

### Rust の `@fp`(14)

| タグ | 場所 | 型 | 案 2 | 備考 |
| --- | --- | --- | --- | --- |
| `arg-a / arg-b` | pass_closure.rs:34→30, 29→35(@nosink) | `XTALK` | 直る(見込み) | `fire_and_forget(a, \|x\| sink(&x))` が、もう一方の呼び元の source に届く |
| `run-a / run-b` | pass_closure.rs:54→50, 49→55(@nosink) | `XTALK` | 直る(見込み) | `run(\|\| sink(&a))` |
| `sinkwith-a / sinkwith-b` | pass_closure.rs:130→126, 125→131(@nosink) | `XTALK` | 直る(見込み) |  |
| `relay-c-a / relay-c-b` | capture.rs:80→76, 75→81(@nosink) | `XTALK` | 不明 | 中継(`relay_capture(f) { run(\|\| f()) }`)は draft の条件 3 |
| `ret-clean` | pass_closure.rs:21 → 23 | `XTALK` | 直る(見込み) | `transform(c.clone(), \|_x\| String::from("fixed"))`: 他の呼び元の恒等クロージャの戻りが、この結果に混ざる(候補の全部の要約を当てるため) |
| `prod(produce)` | pass_closure.rs:42(42 件の source) | `XTALK` | 直る(見込み) | `produce(\|x\| sink(&x))` の x に、他の呼び元の source が届く |
| `fn-drop` | fnvalues.rs:20 → 21 | `XTALK` | 直る(見込み) | `transform(a.clone(), drop_it)`: 同上 |
| `emit-two(2 つ目)` | structs.rs:→82(5 件) | `XTALK` | 不明 | `e2.on(Box::new(\|s\| sink(s)))`: `Vec<Box<dyn Fn>>`(STORE)の呼び出しも、型で全部に当たる |
| `map-const` | iters.rs:50 → 52 | `MODEL-OVER` | 直らない | `iter().map(\|_s\| "fixed")`: Iterator::map のモデル(args→結果) |
| `opt-then-const` | iters.rs:69 → 71 | `MODEL-OVER` | 直らない | `and_then(\|_s\| Some("fixed"))`: Option::and_then のモデル |

### Rust の `@noisy`(183 件、16 か所)

どれも同じ型(XTALK)。ヘルパーの `f(s)` の候補が、型の合う全クロージャ(extractor の `candidates`)で、`valueGuards`(ADR-045)が、`CalleeValue` の呼び出しだけに働く(Rust のクロージャの呼び出しは `dynamic std::ops::Fn*::call*`)。そのため、あるクロージャの中の sink に、同じ型のクロージャを受け取る全部のヘルパーの、全部の呼び元の source が届く。

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

| タグ | source → sink | 痕跡 | 型 | 案 2 | 備考 |
| --- | --- | --- | --- | --- | --- |
| `options` | api/more.go:32 → 33 | 黙った(ヘルパー側: api/more.go:25 に dynamic_dispatch) | `RET-CLOSURE` | 直らない | `WithName(n)` が返すクロージャを、可変長の引数(スライス)で受けた `build` が `o(q)` で呼ぶ。実験: 返されたクロージャを同じ関数で呼んでも、ヘルパーに渡しても落ちる |

### Go の `@fp`(5)

| タグ | 場所 | 型 | 案 2 | 備考 |
| --- | --- | --- | --- | --- |
| ctxkey @nosink(mw_user) | flows.go:15 ← middleware.go:28 | `KEYED-MAP` | 直らない | `c.Set("user", ...)` と `c.GetString("next")` のキーが違うのに混ざる |
| mw_user @nosink(ctxkey) | flows.go:26 ← middleware.go:12 | `KEYED-MAP` | 直らない | 同上 |
| safe_next | flows.go:20 ← middleware.go:12, 28 | `KEYED-MAP` | 直らない | `GetString("safe_next")`(浄化した値のキー)に、別のキーの値が届く |
| cache Get("table") | misc.go:69 → 70 | `KEYED-MAP` | 直らない | `cache.Set("name", x)` のあと `cache.Get("table")` |
| switch allow-list | misc.go:75 → 78 | `ALLOWLIST` | 直らない | `switch col { case "name","email": ... }` の中の sink |

## 実験で確かめたこと(再現するコード)

* Dart: `run(() => sink(a))` と `run(() { out = t; })` は、呼び元が 1 つでも落ちる。同じクロージャを同じ関数の中で呼ぶ(`final f = () => sink(a); f();`)と見つかる。`fireAndForget(a, (x) => sink(x))`(捕獲なし)は見つかる(CAP)。
* Dart: `transform(a, (x) => throw AppError(x))` は、`transform` に渡すクロージャが 4 種類なら見つかり、11 種類なら落ちる。クロージャの中の `sink(x)` も、10 種類のときは 0 件、4 種類のときは 4 件(OVERFLOW)。`transform(c, (x) => "fixed")` の過検出も、11 種類のときだけ出る。
* Dart: `rc(f) { run(() => f()); }`、`viaInner(a, (g) => g("q"))` は落ち、`unresolved_callee` はヘルパーの中のクロージャに出る(PARAM-CALL)。
* Dart: `catchError`、`putIfAbsent`、`Map.forEach`、`whenComplete`、`await Future.error(...)` はすべて落ちる。同じ形の `forEach`、`fold` は見つかる(CB-LIST、AWAIT-ERR)。
* Rust: `*e.o = e.t`(`Env { o: &mut out, t }`、ヘルパーなし、関数 1 つ)で落ちる。`fn w(o: &mut String, t)` を呼ぶ形は見つかる(ENV-PTR)。
* Rust: `fn g<T>(items: Vec<T>) -> Vec<T> { for i in items { o.push(i) } }` は落ち、`Vec<String>` では見つかる。`Fn` の呼び出しでなく、`for` の `into_iter` / `next`(dynamic)で落ちる(GEN-DYN)。
* Go: 返したクロージャ(`With(src())` を変数に入れて `o(q)`)、クロージャをヘルパーに渡す形(`apply(func(q *Q){ q.name = n }, q)`)は落ちる。クロージャをその場で作って呼ぶ形は見つかる(RET-CLOSURE と CAP)。

## issue(黙った見逃し、実コードで起きそうなもの)

同じ原因は 1 つにまとめた(8 件)。

| issue | 型 | 内容 |
| --- | --- | --- |
| #76 | `CAP` | ヘルパーに渡したクロージャの捕獲値(読み書き)が落ちる(Dart、Go) |
| #77 | `OVERFLOW` | ヘルパーに渡すクロージャが 9 種類以上で、呼び出しが未解決(Dart) |
| #78 | `RET-CLOSURE` | 関数が返したクロージャの呼び出し(Dart、Go の functional options) |
| #79 | `STORE` | List / フィールドに持たれたコールバック(Dart、Rust) |
| #80 | `CB-LIST`、`AWAIT-ERR` | 標準ライブラリのコールバックの決め打ちのリスト(Dart) |
| #81 | `PARAM-CALL` | クロージャの関数引数 / 捕獲した関数の呼び出し(Dart) |
| #82 | `ENV-PTR` | `&mut` を持つフィールド(クロージャの環境)経由の書き込み(Rust) |
| #83 | `GEN-DYN` | ジェネリックな T の trait 呼び出しで、モデルが receiver を見ない(Rust)。`-rules` で `from: recv` を足すと直ることを確認した |

issue にしていない `@miss` / `@fp`: `MODEL-OVER`(過検出のみ)、`KEYED-MAP`、`ALLOWLIST`(Go の過検出。黙った見逃しではない)、`XTALK`(過検出)。

## 再現の手順

```sh
export RIVUS_DART="mise exec dart@3.13.6 -- dart" RIVUS_RUST=$PWD/extractor/rust/target/debug/rivus-rust
go test ./cmd/rivus -run Curated -v     # 注釈の検査(この表の元の数)
# 痕跡は、extractor の出力に対して:
(cd extractor/dart && dart run bin/rivus_dart.dart ../../bench/dart-curated > dart.jsonl)
(cd bench/dart-curated && rivus analyze -ir ../../extractor/dart/dart.jsonl -rules rules.yaml)   # findings と uncertainties
```
