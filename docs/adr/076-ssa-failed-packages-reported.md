# ADR-076: 型エラーで SSA が作られなかったパッケージを、出力で示す(読めなかったパッケージの数と名前、落ちた関数の数)

- 状態: 有効
- 関連: README
- 要旨: `summary.unread`

**背景:** 型エラーが 1 行あるパッケージ(`a`)に依存するパッケージ(`b`)は、go/packages が `IllTyped` とマークを付け、go/ssa は SSA を作らない(`a` 自身も)。`packages` には載ったままで、`b` の関数は解析されず、そのことは出力のどこにもなかった(`a` の型エラーだけが `missing_source` で出る)。全部の関数が失われても、終了コードは 0、stderr は無言。結果が「コードの解析」に見えて、実際は「作れた分の解析」だった。

**決めたこと(最初のバージョンは、uncertainty だけ。finding と終了コードは変えない):**
1. Go の読み込み(`internal/frontend/gossa/unread.go`)で、読んだ(`packages` に載せた)のに SSA が作られなかったパッケージごとに、`load_errors` に 1 件を足す: `unread: true`、`unread_functions`(構文に本体のある関数とメソッドの数)、文面は「not analysed: 原因; its N function(s) are not in the result」。原因は、自分に型エラーがあればそのエラー、なければ import をたどって見つけた、エラーのあるパッケージ(「it depends on X, which does not type-check: …」)。`load_errors` は今までどおり `missing_source` の不確実性になるので、パッケージごとに `uncertainties` にも出る。
2. 出力の `summary.unread`(1 つもなければフィールドなし): `packages`(読めなかったパッケージの数)、`functions`(それによって、解析から落ちた関数の数)、`top`(関数の多い順に上位 10 の、パッケージ・関数の数・理由)。
3. stderr に 1 行(`rivus: N package(s) were read and not built …, so their M function(s) are not analysed: a (n), b (m), …; see summary.unread`)。
4. `packages` は変えない(読んだものを載せる、という今の意味のまま)。終了コードは 0 のまま。フラグ `-fail-on-incomplete` は別の判断(未実装)。

**確かめたこと:** `testdata/go/cases/illtyped`(独立したモジュール: 型エラーのあるパッケージが、ほかのケースを全部読むテストを壊すため。`broken` に型エラー、`user` がそれを import、`top` が `user` を import、`fine` は無関係): `summary.unread` が 3 パッケージ・5 関数(`broken` 2、`user` 2、`top` 1)、`user` と `top` の理由が `broken` を指定する、`fine` の finding は 1 件のまま、`packages` は 5 つ(`lib` を含む)のまま(`TestUnreadPackagesAreTold`、`TestUnreadIsAbsentWhenEverythingBuilt`)。

**限界:** 構文エラーで本体が読めなかった関数は数えない(読めた分だけ)。ジェネリクスの具体化や、無名関数は、数えない(宣言の数)。Rust と Dart は、このバージョンの対象外(抽出器の `load_errors` は、それぞれの言語のもの)。

