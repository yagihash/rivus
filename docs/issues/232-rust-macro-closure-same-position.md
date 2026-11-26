# #232 Rust: マクロが同じ位置にクロージャを複数展開すると、環境の型から関数が引けない

- 状態: **open**
- 種類: 見逃し(trace あり。`unmodeled_library_call`)
- 言語: Rust
- ROADMAP の節: -
- 守るもの: zoo `F/rust/macro-closure-twice`(vuln2)
- 進める完了条件: -

## 何が起きるか(事実)

Rust の extractor は、値を捕捉する(または何も捕捉しない)クロージャを、環境の型 `{closure@file:line:col: ..}` の `alloc` として出す。コアは、この型の文字列を位置で引いて、そのクロージャの関数を知る(`internal/ir/env.go` の `ClosureIndex.Lookup`。使う場所: `internal/sg/env.go`、`internal/sites/callback.go` の `functionOf`、`internal/valuecalls/narrow.go`)。

`macro_rules!` が、同じ位置に、クロージャを 2 つ以上展開すると、環境の型が同じになる(位置は、マクロの定義の中の位置)。`Lookup` は、その位置の関数が 1 つでないとき nil を返すので、関数が引けない。

zoo のセル `F/rust/macro-closure-twice` の extractor の出力で確かめた事実:

- マクロの定義の中のクロージャ(18 行 14 列のあたり)の環境の型は、どの展開でも同じ位置の `{closure@cells/src/rust/macro_closure_twice.rs:..: ..}` になる。関数は `vuln::{closure#0}`、`vuln::{closure#1}` のように別々にある。
- `callback_param` のルール(`zoo_dep::Router::get`)に、マクロが 2 回展開したクロージャを渡すと(`vuln2`)、クロージャの引数が source にならず、見逃す。同じマクロを 1 回だけ呼ぶと(2 つ目の呼び出しを外して確かめた)、見つかる。
- 見逃しには trace が付く(`unmodeled_library_call`)。`callback_param_unresolved` ではない。
- 値を捕捉するクロージャをヘルパーに渡して呼ぶ形(`vuln`)は、今のところ見つかる(動的ディスパッチの候補から、呼び出しごとの束縛で引ける)。偽の流れも出ない。ただし、ヘルパーのクロージャの引数の数が、zoo のほかのクロージャと同じだと、動的ディスパッチの候補にほかのセルのクロージャが入り、ほかのセルの source が流れ込む(同じ引数の数のクロージャが、ほかに無いことに頼って、セルは書いてある)。

## 修正の方向(案であって、試していない)

環境の `alloc` に、関数の ID を直接出す(extractor の変更)。位置で引く規則に頼らなくなる。
