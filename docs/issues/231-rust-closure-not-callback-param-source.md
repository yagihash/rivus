# #231 Rust のクロージャが `callback_param` の source にならない

- 状態: **open**
- 優先度: 中
- 種類: サイレントな見逃し(IR の形と、サイトの列挙の食い違い)
- 言語: Rust
- ROADMAP の節: -
- 守るもの: -(zoo のセルは、直す PR が足す)
- 進める完了条件: -

## 何が起きるか(事実)

`callback_param` のルール(`bench/zoo-rust/rules.yaml` に足した試作のルール: symbol `zoo_dep::Router::get`、`operand: 2`(0 は receiver)、`from: 1`)がある呼び出しに、クロージャを渡すと、クロージャの引数が source にならない。名前のある `fn` を渡すと、source になる。

- `extractor/rust/src/lower.rs`(921 行付近)は、何も捕捉しないクロージャを、環境の型 `{closure@file:line:col: ..}` の `alloc` にする。関数を値として使う `FnDef` の定数は `make_closure` にする。
- `internal/sites/callback.go` の `functionOf` は `OpMakeClosure` だけを関数と見る。`OpAlloc` は「not known」で打ち切る。
- そのため、名前のある `fn` は引ける。クロージャは引けない。クロージャを変数に入れて渡す形も同じ。

試作のセル 3 つ(Go の `F/dart/callback-param-{closure,helper,named}` と同じ形。ルーターは `zoo-dep` に置いた)の結果(ルールを足したあと):

| セル | vuln | decoy | fixed |
| --- | --- | --- | --- |
| named(名前のある `fn`、クロージャを持つ変数) | 検出される | 検出されない(クロージャの変数のため) | 0 件 |
| closure(登録の場所に書いたクロージャ) | 検出されない | 検出されない | 0 件 |
| helper(`fn mount(r, h) { r.get(.., h) }` に、クロージャを渡す) | 検出されない | 検出されない | 0 件 |

ルールが無いときは、6 件とも検出されない(名前のある `fn` の vuln を含む)。

## 影響

Rust のルーターにクロージャを渡す形(actix-web、axum などの登録の書き方で一般的)は、`callback_param` のルールを書いても、source にならず、検出が失われる。uncertainty(`unresolved_callee`、detail は `callback_param_unresolved:` で始まる)は、登録の呼び出しのある関数に付く(試作では `zoo::reg`。`summary.callback_param_unresolved` にも出る)。source の関数(クロージャ)にも sink の関数にも付かないので、source と sink の側から見ると、サイレントな見逃し。

## 修正の案(案であって、試していない)

- `ir.ClosureIndex.Lookup`(`internal/ir/env.go`)は、環境の型から、そのクロージャの関数を引く既存の規則(グラフと `valuecalls` が使っている)。`functionOf` の `OpAlloc` の枝で、環境の型(`ir.IsEnvType`)のとき、これで引く。
- 索引は `callerWalk` が、最初のクロージャに会ったときに作る(プログラム全体の関数を 1 回読む)。`internal/sites/enumerate.go` の `givenSinks` の呼び出しは、変えない。
- 規模の見積もりは約 12 行。
- helper は、既存の `callerWalk` が `functionOf` を呼ぶので、同じ修正で動くはず。**未確認**。
- 環境の型が同じ場所に 2 つ以上の関数を持つ場合(マクロが 1 か所に複数のクロージャを書く)は、`Lookup` が `nil` を返すので、今と同じく引けない。その形にも、今と同じ uncertainty(呼び出しのある関数)が付く。
