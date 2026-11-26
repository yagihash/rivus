# #233 Rust: マクロが同じ位置にクロージャを複数展開すると、環境の型から関数が引けない

- 状態: **closed**(`vuln2` は、環境の `alloc` に関数の ID を出す PR で直った。2026-11-26。`Lookup` が 0 個と 2 個以上を区別して数える安全網は、別の PR)
- 種類: 見逃し(`callback_param_unresolved`)
- 言語: Rust
- ROADMAP の節: -
- 守るもの: zoo `F/rust/macro-closure-twice`(vuln2)
- 進める完了条件: -

## 何が起きるか(事実)

Rust の extractor は、値を捕捉する(または何も捕捉しない)クロージャを、環境の型 `{closure@file:line:col: ..}` の `alloc` として出す。コアは、この型の文字列を位置で引いて、そのクロージャの関数を知る(`internal/ir/env.go` の `ClosureIndex.Lookup`。使う場所: `internal/sg/env.go`、`internal/sites/callback.go` の `functionOf`、`internal/valuecalls/narrow.go`)。

`macro_rules!` が、同じ位置に、クロージャを 2 つ以上展開すると、環境の型が同じになる(位置は、マクロの定義の中の位置)。`Lookup` は、その位置の関数が 1 つでないとき nil を返すので、関数が引けない。

zoo のセル `F/rust/macro-closure-twice` の extractor の出力で確かめた事実:

- マクロの定義の中のクロージャ(24 行 14 列)の環境の型は、どの展開でも同じ位置の `{closure@cells/src/rust/macro_closure_twice.rs:..: ..}` になる。関数は `vuln::{closure#0}`、`vuln::{closure#1}` のように別々にある。
- `callback_param` のルール(`zoo_dep::Router::get`)に、マクロが 2 回展開したクロージャを渡すと(`vuln2`)、クロージャの引数が source にならず、見逃す。同じマクロを 1 回だけ呼ぶと(2 つ目の呼び出しを外して確かめた)、見つかる。
- 見逃しの原因は、同じ位置の関数が 2 つあって `Lookup` が nil を返すこと。`functionOf` は、渡された関数が「not known」として `callback_param_unresolved` を残す(`internal/sites` の単体テストで確かめた)。zoo の `@miss` には、そのとき `unmodeled_library_call` と付いていた。
- 値を捕捉するクロージャをヘルパーに渡して呼ぶ形(`vuln`)は、今のところ見つかる(動的ディスパッチの候補から、呼び出しごとの束縛で引ける)。偽の流れも出ない。ただし、ヘルパーのクロージャの引数の数が、zoo のほかのクロージャと同じだと、動的ディスパッチの候補にほかのセルのクロージャが入り、ほかのセルの source が流れ込む(同じ引数の数のクロージャが、ほかに無いことに頼って、セルは書いてある)。

## 直した内容

環境の `alloc` に、そのクロージャの関数の ID を `Instr.Func` で出す(extractor: 何も捕捉しないクロージャの定数の経路と、`Aggregate` の経路。本体が読まれているクロージャだけ)。コアは `ir.ClosureIndex.FuncOf` を 1 つの入口にして、ID があって、その関数がプログラムにあればそれ、なければ従来どおり型から `Lookup` する(`sg` の `linkEnvClosures`、`sites` の `functionOf`、`valuecalls` の `narrow`)。IR の版は `rivus.ir/1` のまま(形は変わらない)。zoo の `vuln2` の `@miss` を外した。
