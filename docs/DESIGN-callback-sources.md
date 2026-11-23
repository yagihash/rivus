# 設計: 引数として渡された関数の引数を source にする(callback params)

> **状態: 段階 1(b)は実装済み(ADR-106)。以下は実装前の案**(確認: 2026-11-23、137af2f)。コードは変えていない。ADR はまだ書かない(推奨が決まってから)。課題は [#149](issues/149-callback-params-source.md)。
> 利用者向けの書式の正本は [MODELING.md](MODELING.md)。この文書は、書式を足す前の比較と推奨。

## 1. 問題

shelf_router では、経路で取った文字列が、ハンドラの `Request` のあとの `String` の引数に入る。

```dart
router.get('/item/<id>', (Request request, String id) async {
  sinkCmd(id);   // 見逃される(サイレント)
  ...
});
```

`Request` は `param_type` の source(`dart-packages.yaml`)なので、`request.params['id']` で読む形は見つかる。`id` を直接受ける形は、型が `String` で、`param_type` では指せない。`calls:` の行も、呼び返す関数の引数に source を足せない(`with:` は `recv` と `argN` を渡すだけ)。これは shelf_router のいちばん普通の使い方で、モデルを足しても検出されない行が増える(`named:` のときと同じ形の不足)。

## 2. 今のエンジンの事実

| 項目 | 事実(読んだ場所) |
| --- | --- |
| source の種類 | `call_result`(呼び出しの結果。`out:` で引数への書き込み)、`param_type`(型が一致する引数すべて)、`field`、`param`(`symbol` の関数の `params` / `recv`)。`param` は `marker`(インタフェースのメソッド名)と `extends` + `with_param_type`(サブクラスのメソッドで、その型の引数を取るもの。引数のうち、その型でないものが入力)で、複数の関数をまとめて宣言できる(`internal/model/model.go` の `Source`、`Expand` / `ExpandSubtypes`) |
| source の置き場所 | source は「サイト」で、IR の関数の引数 1 つに付く(`internal/sites/enumerate.go`: `ParamFuncs[fn.Symbol]` と `ParamTypes`。`sites.ByIRParam`)。taint は、その関数の解析の始めに、引数のシードとして実際の要素を入れる(`internal/taint/transfer.go` の `seedParam`)。**呼び出し側の情報は要らない**。クロージャも、IR では関数なので、引数に付けば効く(`Request` が今それで効いている) |
| `param_type` の一致 | 型の文字列の一致(エイリアスは `Canon` でそろえる)。`String` や `int` のような一般の型は、すべての関数の引数に当たるので、指せない。Rust の `axum::extract::Path` は、ジェネリクスの `<T>` を除いた名前で書かれている(`rust-axum.yaml`)。型ベースの Rust の source は、すでに書ける |
| `calls:` | 「オペランド `fn` を、`with` の値で呼ぶ。結果は `ret`」。呼び出し点で、呼ばれる関数(関数、または、その場で書いたクロージャ 1〜4 個)の要約を、引数を `with` に束縛して適用する。`with` に書けるのは `recv`、`argN`、`none` だけ。**source を書く場所がない**。`calls:` が使われるのは、呼び出し点で行を適用するとき。shelf_router の `Router.get` は、ハンドラを List に入れるだけで、呼ぶのは `call` の中(`Function.apply`)なので、登録の呼び出しに `calls:` を書くと、「登録で呼ばれる」という誤った意味になる |
| 行のない呼び出し | `assumed_call`(`internal/taint/bodyreplaced.go`、`calls.go` の `captured`): 本体を読まないパッケージに渡された関数値は、他のオペランドを引数に、呼ばれたものとして扱う。これは `ret` を結果に流す推測で、引数に source を足すことはない |
| 動的な呼び出し | `Function.apply(handler, [req, ...])` は、リストの中身を引数に分けて呼ぶ形で、エンジンは、この対応(リストの 2 番目が 2 番目の引数)を追わない。呼び出し先が分からない値の呼び出しは `unresolved_callee` で、その先は見ない(`-resolve-func-values` が候補を補うのは、自分のコードで作った関数だけ。既定は off)。**この案は、その呼び出しを追わなくてよい**: 引数が source のとき、その値はクロージャの中で始まる |
| 入口 | `Func.Entry` は `main` と `init` だけ。入口の宣言は `kind: param`(ARCHITECTURE.md 6 節)。`entrypoints:` の節はない。`-entry` は、この文書の範囲の外 |
| モデルの読み込み | YAML は `KnownFields(true)`。知らないキーのあるファイルは、古い rivus が読み込みで拒否する |
| 関連の課題 | #145(`flows` と `sources` に名前つき引数を指す書式がない) |

## 3. 案

### (a) `calls:` の拡張

```yaml
- symbol: package:shelf_router/src/router.dart#Router.get
  calls:
    - fn: arg1
      params: 2
      with: [none, {param: 1, source: user_input}]
```

呼び出し点で、呼ばれる関数の要約を、`source` つきの引数で適用する。

- 層と規模: model 20 行(`WithList` の項目に `source`、検証)、taint の適用(`applyCallRow` で、引数の束縛にシードの要素を入れる)150 行前後、サイト(source の ID がないと finding の起点を作れないので、呼び出し点に合成のサイトを作る)80 行前後。IR と extractor は不要。
- 問題: (1) 登録の呼び出しが「呼ぶ」ことになるので、ハンドラの中の finding は、登録の呼び出しの文脈で適用された要約からしか出ない。クロージャ自身の解析(今 `Request` が効く経路)には source がない。(2) `calls:` の `mode` の意味(`call` / `go` / `defer`)と合わない: 登録は呼ばない。(3) 登録が別の関数(ヘルパー)の中にあるとき、そのヘルパーの要約を通る。

### (b) `sources` の新しい種類 `callback_param`

```yaml
sources:
  - kind: callback_param
    symbol: package:shelf_router/src/router.dart#Router.get   # この関数に渡された関数の
    operand: 1                  # 関数値のオペランド(レシーバを数えない。ほかのモデルの argN と同じ)
    from: 1                     # 1 番目から後ろの引数(0 始まり。省くと 0)
    # to: 2                     # (省略可)ここまで
    labels: [user_input]
```

呼び出しのオペランドの関数(IR の `make_closure` と関数の参照)を引いて、その関数の引数にサイトを付ける。あとは `param` / `param_type` と同じ道(`ByIRParam` と `seedParam`)。

- 層と規模:
  - model の読み込み: `Source` に `Operand`、`From`、`To`(`Symbol` は使う)。検証(`operand` と `from` が負でない、`symbol` が必須)。`Rules` に `CallbackParams map[string][]Source`。約 50 行。
  - sites: 今の呼び出しの走査(`OpCall`)の前に、プロジェクトの関数の全呼び出しを見て、`CallbackParams[callee]` に当たる呼び出しの、`operand` の局所変数の定義(`make_closure` か関数の参照。単純なコピーを数ステップたどる)を引いて、関数 ID → ソースの表を作る。その表で、関数ごとのサイトを足す。約 80 行。
  - taint: 変更なし。
  - 引けなかったとき(変数経由で複数の定義がある、引数のリストを通る、など)の注記: 約 30 行。注記の種類は、既存の `unresolved_callee` に詳細を付ける(新しい種類は足さない)。
  - extractor、IR: 変更なし(`make_closure` は IR にある。関数の参照が Dart と Rust でどの形かは、試作で確認する)。
  - 合計: 約 160 行 + テスト(zoo のセルと、`internal/sites` の単体テスト)。
- 利点: クロージャ自身の解析で source が効く(`Request` と同じ経路)。登録を呼び出しと見なさない。`calls:` と別の行なので、`calls:` の検証と衝突しない。

### (c) 型で決める

(c1) 既存の `param_type`: 型が一意のとき。Rust の `Path<T>`、`Query<T>`、`Json<T>`、`Form<T>`(axum はすでにある。actix-web の `web::Path` などは行を足すだけで、エンジンは変えない)。

(c2) 新しい `param_after_type: {type: T}`(「型 T の引数を持つ関数の、その後ろの引数」。`extends` を省いた `with_param_type`): Dart の `(Request r, String id)` と `onRequest(RequestContext c, String id)` と serverpod に当たる。型 `T` を持つ関数なら、どの関数でも当たる。

- 規模: model 30 行(`Source` の項目と検証)、sites 30 行(`ParamTypes` のループの隣)。約 60 行。
- 問題: `(Request r, String msg)` の形のヘルパー関数すべてで `msg` が source になる。ヘルパーに定数を渡す呼び出しも、過検出になる。型の条件だけで、「ハンドラ」と「ヘルパー」を区別できない。**精度よりコストを優先しない**ので、単独では推奨しない(`function 名の glob` と組み合わせるなら良い。(e) の dart_frog)。

### (d) 入口のルートの宣言(`entries` の拡張)

`entries:` の節を作り、`route: {symbol: ..., handler_operand: 1, params: ...}` のように、入口とその引数を宣言する。今の `kind: param` が入口の宣言を兼ねているので、新しい節を作る理由は、`Func.Entry` と到達性(`from_entry`)を変えたいときに限る。(b) は、`reachability` を変えない(ハンドラは `from_framework_callback` の扱いになりうる。ADR-073。実装のとき確認)。節を足すと、モデルの書式の大きな拡張になり、規模は 300 行以上(model、到達性、グラフの入口、ドキュメント)。ほかの案で足りるので、やらない。

### (e) 他

- (e1) **関数名の glob で決める `kind: param`**(ファイルのルート用): `kind: param` に `name: onRequest` と `with_param_type: package:dart_frog/...#RequestContext` を足し、`extends` なしで、「その名前の最上位の関数で、その型の引数を持つもの。ほかの引数が入力」を宣言する。`ExpandSubtypes` と同じ形(`Expansion` の結果をつける)で、規模は約 80 行(model 25、resolver 25(`cmd/rivus/common.go` の `subtypeResolver` の隣)、テスト)。dart_frog は、登録の呼び出しがないので、(b) では書けない。
- (e2) **`calls:` の `with` に、型つきの束縛**: (a) の変形で、規模も問題も同じ。
- (e3) **フレームワークの全体を、規則ファイルでなくプラグイン(コード)で**: 決定論の書式ではなくなる。やらない。

## 4. 比較(どこまで書けるか)

「○」は、その案で、検出の見逃しなしに書ける。「△」は、条件か過検出つき。「×」は、書けない。

| 対象 | (a) `calls:` の拡張 | (b) `callback_param` | (c1) `param_type` | (c2) `param_after_type` | (d) `entries` | (e1) 関数名の `param` |
| --- | --- | --- | --- | --- | --- | --- |
| shelf_router `router.get('/x/<id>', (Request r, String id) => ...)` | △(登録の呼び出しの文脈だけ。クロージャ自身の解析に source がない) | ○(クロージャを直接渡す形。名前つきの関数を渡す形も、引けるなら ○) | ×(`String`) | △(`(Request, String)` のヘルパーすべてが過検出) | ○(重い) | ×(名前がない) |
| gin `c.Param("id")` / `c.Query` | 不要 | 不要 | 不要 | 不要 | 不要 | 不要 |
| actix-web / axum `async fn h(Path(id): Path<String>, Query(q): Query<Q>)` | × | ×(登録 `route("/x", get(h))` の形が、ルーターごとに違い、型で決まるほうが単純) | ○(行を足すだけ。ジェネリクスと、タプル構造体の分解(`Path(id)`)の取り出しは、zoo のセルで確認) | ○ | ○ | × |
| dart_frog `Response onRequest(RequestContext c, String id)` | ×(登録がない) | ×(登録がない) | ×(`String`) | ○(`RequestContext` のあとの引数) | ○ | ○(`onRequest` に限れば、過検出も少ない) |
| serverpod `Future<String> hello(Session s, String name)` | × | ×(登録がない) | × | ○ | ○ | ○(既存の `extends: Endpoint` + `with_param_type: Session` で、すでに書ける。`kind: param` の `extends` の形) |

gin について。ハンドラは `func(*gin.Context)` で、入力はレシーバのメソッドの結果(`c.Param`、`c.Query`、`c.GetHeader`、`c.ShouldBind(&x)` は `out:`)。`call_result` の行(`go-net-http.yaml` と同じ書式)で足りて、ハンドラの登録を知る必要がない。この課題の対象ではない(モデルの行を足せばよい)。

## 5. 限界

- **関数値が変数を通る形**: `final h = (Request r, String id) => ...; router.get('/x/<id>', h);`。単純なコピーは引ける。条件分岐で違う関数が入る(複数の定義)、フィールドやリストに入れてから渡す、別の関数の引数として受けて渡す(ヘルパーが登録をまとめる)形は、(b) の最初のバージョンでは引けない。引けない呼び出しには、`unresolved_callee` の注記を付ける(サイレントにはしない)。登録のヘルパーの中の引数として渡す形は、ヘルパーの引数への `callback_param` 注記で、利用者に見える。
- **型が分からない動的な引数**: ハンドラの引数が `dynamic` や型なしでも、(b) は、位置で決めるので、効く。(c) は、型で決めるので、効かない。
- **ジェネリクス**: (c1) の `Path<T>` は、型の名前で一致するので、ジェネリクスの引数を除いた名前で書ける(`axum` の行が今そう)。ジェネリックな関数のハンドラ(型パラメータが `T` の引数)は、(b) では位置で効くが、型の条件のある (c) では効かない。
- **`Function.apply` で、動的に引数を渡す形**: エンジンは、この対応を追わない(2 節)。(b) と (c) は、それを追わずに、クロージャの引数をシードにするので、影響を受けない。ただし、(b) は、ハンドラの中の値が、登録した側の関数に戻る流れ(ハンドラの戻り値)を、追わない。これは今の `Request` と同じ。
- **登録ごとの意味の違い**: 引数が、すべて利用者の入力とは限らない(ミドルウェアの `Handler` は `Request` だけ。`Function.apply` で、取った名前が残りの引数)。`from:` と `to:` で範囲を書く。名前つき引数(`{required String id}`)は、位置の指定が合わない(#145 と同じ問題。最初のバージョンは位置の引数だけ)。

## 6. 推奨

**(b) `callback_param`** を段階で入れる。

1. **段階 1(最小)**: (b)。shelf_router の `Router.get`、`post` など(`handler` は引数 1)が書ける。zoo のセル `F/dart/shelf-router-extra-param` の `@miss(silent)` が外れる。gin は行を足すだけ(`call_result`)。規模は約 160 行 + テスト(2 節の表と、3 節 (b))。
2. **段階 2**: (e1)(関数名の `kind: param`)で dart_frog。serverpod は、既存の `extends` で、行を足すだけ。規模は約 80 行。
3. **段階 3**: Rust の型ベース。axum は既存。actix-web の `web::Path` / `Query` / `Json` の行を足す。タプル構造体の分解のパターンの引数が IR でどうなるか、zoo のセルを先に書いて確認する。エンジンの変更は、見つかった場合だけ。

(c2) は単独では入れない(過検出)。(a) と (d) は入れない(3 節)。

### 互換性

- 新しい `kind: callback_param` と、新しいキー `operand` / `from` / `to` は、古い rivus が(`KnownFields(true)` で)ファイルごと拒否する。同じバージョンの組み込みのモデルなら問題ない。プロジェクトの規則ファイルで新しい書式を使うと、古い rivus は、エラーで止まる(サイレントに行を捨てない)。`verified_versions`(DESIGN-package-models.md 6.7)と同じく、これは仕様でよい。モデルのファイルに、必要な rivus のバージョンを書くフィールドはまだない。
- `ruleset:` のバージョン(`rivus/dart/shelf_router@1`)は、`@2` に上げない(行を足すだけ)。

### `models coverage` / `models doctor` / 差分テスト

- `models coverage`: 関数の型のパラメータを持つ関数(コールバックを取る)は、今 `calls:` の候補として出る。`callback_param` の候補(「ハンドラを登録する関数」)は、同じ列を使い、行のテンプレートに `callback_param` の例をコメントで足す。判定は人が決める(ハンドラの引数が入力かは、型から分からない)。約 30 行。
- `models doctor`: この時点のコードに `models doctor` はない(`cmd/rivus/commands.go` の `models` は `coverage` ほか)。できたら、`operand` が関数型のパラメータを指すか、`from` が、その関数型の引数の数以下かを検査する。
- 差分テスト(`bench/model-diff`。`origin/feat/model-diff` にある。main にはない): コールバックを取る関数は「未検証」。`callback_param` も、本体との照合で決まる行ではない(source は、本体に書かれていない、フレームワークの約束)ので、`unverified: true` のままにして、**zoo の実際の走り**で確認する(セルは、実物の shelf_router を走らせ、`typed()` の値が引数に届くことを見る。`F/dart/shelf-router-extra-param`)。

### zoo

- `F/dart/shelf-router-extra-param` の `vuln` と `decoy` の `@miss(silent)` と `@ref(model-shelf_router-extra-path-params)` は、実装後に外れる(`@source` と `@sink` だけになる)。見逃しが減る方向で、ゲートは緩めない(`Zoo-Loosened` は不要)。`docs/issues/model-shelf_router-extra-path-params.md` と #149 は、このときに閉じられる。
- セルを足す: (1) 名前つきの関数を渡す、(2) 変数を通して渡す(限界。注記つきの見逃し)、(3) ハンドラが `Request` だけのとき(`from` の範囲の外。source が増えない)、(4) 登録のヘルパーを通す(限界)。

## 7. 迷ったこと・判断が要る点

- (b) の `operand` の数え方(レシーバを数えない)は、`calls:` の `fn: argN` に合わせた。Rust は、`calls:` で `self` を `arg0` にするので、Rust の行では `fn:` と同じ約束(レシーバを含めて数える)にするか、相談役の判断が要る。
- 注記の種類を新しく作らず `unresolved_callee` に詳細を付ける案は、`graph path` の `exhaustive` を false にする(安全側)。変更が小さいので、そう書いた。
- `from` を 0 始まりの引数の番号とした。`Request` のような「入口の型の引数」を除く目的なので、`from: 1` が普通になる。名前つき引数は #145 に任せた。
- dart_frog の段階 2 は、「ファイルのルート」の規約(`routes/` の下)の検査を入れない。関数名と型だけで決める。過検出が気になるなら、ファイルのパスの条件(`path_glob`)を足す。
