# ADR-106: source の新しい種類 `callback_param`(呼び出しに渡された関数の引数を source にする。課題 #149。2026-11-23)

- 状態: 有効
- 関連: DESIGN-callback-sources、課題 #149

**背景:** shelf_router の `router.get('/item/<id>', (Request r, String id) => ...)` の `id` は利用者の入力だが、型が `String` で `param_type` で指せず、`calls:` の行にも source を書く場所がない(サイレントな見逃し)。比較は [DESIGN-callback-sources.md](../DESIGN-callback-sources.md)(`calls:` の拡張、`callback_param`、型で決める案、入口の宣言、関数名の `param`)。

**決めたこと:**
1. `sources:` に `kind: callback_param`(`symbol`、`operand`、`from`、`to`、`labels`)を足す。「`symbol` の呼び出しの `operand` 番目のオペランドとして渡された関数の、`from` から `to` までの(宣言された)引数が source」。`operand` は必須、`from` の既定は 0、`to`(含む)の既定は最後。キーは、この kind だけが持つ(ほかの kind に書くとファイルごと拒否する)。
2. **`operand` の数え方は、同じファイルの `calls:` の `fn: argN` と同じ**: Go と Dart はレシーバを数えない。Rust(`self_first: true` の集合)は、`self` が `arg0`。1 つのファイルの中で数え方が 2 種類にならない。言語ごとの表は MODELING.md 4d 節。
3. **実装は、model と sites だけ**(`internal/model/model.go`、`internal/sites/callback.go`)。呼び出しのオペランドから、コピーと変換をたどって `make_closure` を引き、その関数の引数に、`param` / `param_type` と同じサイト(`ByIRParam`)を付ける。taint、IR、extractor は変えない。`Rules.CallbackParams`(シンボル別。集合の `self_first` を持つ)、`sites.Result.UnresolvedCallbacks`。
4. **引けない形は、段階 1 では引かない**: 変数が 2 つの関数を持つ(φ)、フィールド、要素、呼び出しの結果、呼ぶ関数の引数(登録のヘルパー)。引けない呼び出しは、その呼び出しの位置に `unresolved_callee`(新しい種類は足さない)を付ける。**注記は、呼び出しを持つ関数にある**(ヘルパーが登録するときは、ヘルパー)ので、source の関数と sink の関数の不確実性を数える `@miss` の分類(zoo)では `silent` になる。zoo のセル `F/dart/callback-param-helper` がそれを確認している。
5. **宣言がないとき(`callback_param` の行がないとき)は、結果も出力も index も今までと変わらない**(`CallbackParams` が空なら、sites は何もしない)。古い rivus は、新しい kind とキーをファイルごと拒否する(`KnownFields(true)`)。
6. **組み込みのモデルは変えない**(`dart-*.yaml` に shelf_router の行は入れない。モデルを書く作業は別)。仕組みの確認は、zoo のテスト用のルール(`bench/zoo-dart/cells/rules.yaml`)と、合成の小さい骨組み(`packages/zoo_router`)のセル 3 つ(関数リテラル、名前のある関数と変数、ヘルパー)で行う。Go は `internal/sites/callback_test.go`、Rust は書式の検査(`self_first` の集合で、`operand` が `arg0` から数える)だけ(隔離なしでは extractor を動かせない)。
7. `models coverage -yaml` の見出しに、行の例をコメントで足す。`bench/model-diff` は propagator の行だけを比べるので、変更なし。

**しないこと(この ADR では)**: dart_frog(関数名の `param`)、型で決める案(`param_after_type`)、入口の宣言(`entries:`)、名前つき引数(#145)、登録のヘルパーを通る関数の追跡。

