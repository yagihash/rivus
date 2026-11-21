# 設計: 構造体のフィールドを sink にする(引数や受け手の「どの部分」が危険か、を書けるようにする)

状態: **設計のみ(実装なし)**。決める人: アーキテクチャ助言者とユーザー。末尾の「決めること」に、まだ決まっていない点を書く。

## 1. 問題

sink の書式(`model.Sink`)は、**位置**(引数 n、受け手)までしか言えない。位置に渡された値の、どの部分が危険かは言えない。`checkSink` は、その位置の値の下にある汚れを、**構造を無視して全部**(`flatten(locs(v))`)見る。

Google Cloud のライブラリ(と、`googleapis` が生成する API 全般)は、**要求を構造体で組み立てて、1 回の呼び出しで渡す**。

| API | 危険なのは | 危険でない(同じ値の中) |
| --- | --- | --- |
| `spanner.Statement{SQL, Params}` を `txn.Query(ctx, stmt)` に渡す | `SQL` | `Params`(バインド変数。汚れていてよい) |
| `cloudtasks` の `CreateTaskRequest{Task{MessageType: &Task_HttpRequest{HttpRequest{Url, Headers, Body}}}}` を `CreateTask(ctx, req)` に渡す | `Url`(SSRF)、`Headers` | `Body` |
| `bigquery` の `Query{QueryConfig{Q, Parameters}}` を `q.Run(ctx)` / `q.Read(ctx)` で実行 | `Q` | `Parameters` |

今のモデルで `args: [1]` と書くと、`Params` や `Body` だけが汚れた呼び出しでも sink が鳴る(偽陽性。`bench/zoo-go/cells/cont/struct_whole_to_sink` が、構造体を丸ごと渡す形の今の振る舞い)。書かなければ(sink にしなければ)、見逃しになる。関数の引数の sink だけでは、書けない。

## 2. 案の比較

### 案 A: `field_store` に sink の site を置く(`kind: field` の sink)

ソースの `kind: field`(`pkg.Type.Field` の読み出しが source)と対になる、「`pkg.Type.Field` への書き込みが sink」。

- 要る物: 新しい種類の site(書き込みの命令に付ける。`enumerate.go` の `OpFieldStore` / `OpAddrOf`)、書き込む値が sink に届く新しい道(要約の `SinkReach` の、新しい出どころ)。
- **finding の場所が、代入の行になる**(危険な処理が起きる、送る・実行する呼び出しではない)。
- **組み立てただけで使われない構造体でも鳴る**(`stmt := spanner.Statement{SQL: x}` を作って捨てる)。
- **見逃す形**: 構造体の丸ごとのコピー(`*dst = *src`、値渡しのコピー)、ポインタ越し、**読んでいない関数が中で詰める形**(`spanner.NewStatement(sql)`、protobuf の `Unmarshal`、ビルダー)、埋め込み(`Base.Url` を `x.Url` と書く)。これらは、フィールドへの `field_store` が、読んだコードに現れない。**見逃しが増える方向**。
- 組み立てた場所と使う場所が、別の関数のとき、「どちらが sink か」が決まらない(組み立てる関数は、使われ方を知らない)。

### 案 B(主): 今ある呼び出しの sink に、「値の、どのフィールドか」を書けるようにする

`symbol: ...CreateTask, args: [1]` の、位置 1 の値の下の、`Task.MessageType.HttpRequest.Url` だけを見る。

- **finding の場所は、送る・実行する呼び出し**(今の sink と同じ)。
- 構造体がどう組み立てられ、どこを通って来たか(コピー、別の関数、フィールド経由、`flows` のあるモデル)は、**今ある状態(セル)と要約の仕組みが、そのまま運ぶ**。案 A の見逃しの形は、案 B では、構造体が呼び出しに届きさえすれば、今までの汚れの流れで解ける。
- 変えるのは、`checkSink` が見る範囲(`flatten(locs(v))` の全部 → 指定した道の下だけ。あとで述べる「道の上の汚れ」も含める)と、`model.Sink` の項目 1 つ。新しい種類の site も、`SinkReach` の新しい道もない(5 節で確かめた)。
- **今の偽陽性(構造体を丸ごと渡す形で、`Body` だけ汚れていても `URL` の sink が鳴る)が、指定のある sink では消える。これは finding が減る変更**。9 節の受け入れで、消えた finding が「危険でないフィールドだけが汚れていた」ものであることを示す。

### 比べると

| | 案 A | 案 B |
| --- | --- | --- |
| finding の場所 | 代入の行 | 呼び出し(今と同じ) |
| 使われない構造体 | 鳴る(過検出) | 鳴らない(呼び出しがない) |
| 丸ごとのコピー、ポインタ、別の関数で組み立てる | 見逃す | 今の状態が運ぶ |
| 読んでいない関数(`NewStatement`、`Unmarshal`)が詰める | 見逃す | モデルの `flows` が丸ごと運ぶ(道の上の汚れとして鳴る、不確実性つき。5 節の表) |
| 実装 | 新しい site、新しい `SinkReach` の道 | `Sink` の項目、`checkSink` の範囲 |
| 見逃し | 増える | 増えない(下のとおり、細かさがないときは鳴る) |
| 使う呼び出しが、読んだコードに現れない形 | **鳴る**(書く場所が見える) | 見えない |

最後の行だけが、案 A の利点で、10 節に、足す条件を書く。

## 3. 書式

位置と道の組(`parts`)を、**複数**書けるようにする。

```yaml
sinks:
  - symbol: (*cloud.google.com/go/spanner.ReadOnlyTransaction).Query
    category: sql_injection
    parts:
      - {arg: 1, field: SQL}

  - symbol: (*cloud.google.com/go/cloudtasks/apiv2.Client).CreateTask
    category: ssrf
    parts:
      - {arg: 1, field: Task.MessageType.HttpRequest.Url}
      - {arg: 1, field: Task.MessageType.HttpRequest.Headers}

  - symbol: (*cloud.google.com/go/bigquery.Query).Run
    category: sql_injection
    parts:
      - {recv: true, field: QueryConfig.Q}

  - symbol: (*cloud.google.com/go/spanner.ReadWriteTransaction).BatchUpdate
    category: sql_injection
    parts:
      - {arg: 1, field: "[].SQL"}       # []Statement の、各要素の SQL
```

- 糖衣(1 つだけのとき): `args: [1], field: Task.MessageType.HttpRequest.Url` / `recv: true, field: Path`。`parts` の 1 要素と同じ。位置が 2 つ以上あるのに `field` だけを書くのは、読み込みで拒否する。
- **`parts` を書かなければ、今までどおり全部**(既存のルールは変わらない)。`args` / `recv`(全部)と `parts` を同じ sink に書いたとき、**同じ位置は全部が勝つ**(`parts` は、その位置については何も制限しない)。`args` が空で `parts` があるときは、「全部の引数」とは読まず、`parts` の位置だけ(今の「空 = 全部の引数」は、`parts` のない sink だけ)。同じ symbol の複数の行は、今までどおり和。
- `field` は、**位置の値の中の、フィールド名の列**(`.` 区切り)。先頭は、位置の値の型の**フィールド**(型の名前ではない: `CreateTaskRequest` ではなく `Task` から)。名前は、**IR が言うフィールド名**そのまま(Go は Go のフィールド名。Dart は `url`、Rust は構造体のフィールド名か `0`)。要素の区切りは `[]`(スライス、配列、マップの値)。
- **型のパス(`pkg.Type.Field`)との関係**: `kind: field` の source は、`pkg.Type.Field` で、**アクセスする側の型**から決まる。`parts` の `field` は、位置の値から**下る**道で、型は書かない(型は、位置の値の静的な型で、`when_arg_type` で絞れる)。道の名前の誤りは、モデルの読み込みでは見つからない(モデルは型を知らない)ので、**モデルを書くときの規則に、「脆弱とおとりの 2 つの試験を付ける」を入れる**(`docs/MODELING.md` に節を足す)。補助として、`rivus` に、「この位置の値の下に、どの道のセルがあるか」を出す診断(5 節の測定にも使う)を足す。
- **道の突き当たりの正規化(`*` は透明)**: エンジンのアクセスパスには、`/*`(ポインタの指す先、**インターフェースに包んだ値の中身**)の段が入る(`make_interface` は、ポインタの指す先を、インターフェースの値の `/*` の下に置く)。モデルの書き手が、それを知らなくてよいように、**照合は、`*` の段を読み飛ばして**行う: セルの道から `*` を除いたものと、`field` を比べる。oneof(`Task.MessageType` はインターフェース、`Task_HttpRequest{HttpRequest: ...}` が中身)は、`MessageType` の次に、そのまま `HttpRequest`(ラッパーのフィールド名)と書く。読み飛ばしで広がるのは、`*` が実際に別の物を指す形だけで、見逃しにはならない。ADR-060(ポインタレシーバのメソッドで `/*` の下を読み書きする)が直した、「`/*` が 1 段ずれる」場所と同じ所を通るので、**ずれても拾える**ように、この読み飛ばしにした(`Param0` の展開は、今までの場所と `/*` を足した場所の和、と同じ考え)。
- **埋め込み**: Go の埋め込みは、IR でも道に出る(`Query` の `QueryConfig`)ので、`QueryConfig.Q` と書く(昇格したフィールドの、短い名前 `Q` は読まない。最初は、書いた道だけ)。

## 4. 意味: `checkSink` が見る範囲

位置の値の `locs(v)`(セルの根と道)に対して、`parts` の `field` ごとに、次の 2 種類の汚れを見る。どちらも `collect` が、すでに持っている(`collectUncached` は、「道の下」と、「道を包む、上のセルが丸ごと汚れている」の両方を返す)。

1. **下**: 道のセルとその下(`Url` のセル、`Headers` の要素)。**細かく分かっている**。普通の finding。
2. **上**: 道を包む、上のセル(`""` や `Task/MessageType/*`)に、丸ごとの汚れがある。**どのフィールドの汚れか、分からない**。鳴らす + 不確実性(次の節)。

指定した道と並ぶ、別のフィールドだけが汚れている(`Body` だけ)ときは、どちらにも当たらない: **鳴らない**。これが、偽陽性の消える所。

実装の注: 読み飛ばし(`*`)のために、`Loc{根, 道 + field}` を 1 つ作るのではなく、根のセルの道の一覧(`pathsOf`)から、「`*` を除いた道が、`field` の下(または、`field` の上)」のものを選ぶ。指定のある sink でだけ行うので、全体の費用は変わらない(測る: 9 節の段階 0)。

`literal_only`(L1。引数がすべて定数のとき)は、指定のある sink では、構造体のリテラルを定数とは見ない(`isConstantRef` が、構造体を作る命令を通さない)。つまり、「定数だけ」と言わない側に倒れる(過検出側で、見逃しではない)。

## 5. 道の細かさが残っていないとき(見逃しにしない)

**値が、指定した道まで細かく分かれていない**とき、フィールドを区別できない。区別できないので、**鳴らす + 不確実性**にする。最初から入れる。

| 形 | セルの状態 | 結果 |
| --- | --- | --- |
| モデルのない関数が返した構造体(`flows: [{from: arg0, to: result}]` で、結果が丸ごと汚れる。`spanner.NewStatement(sql)`、`json.Unmarshal(src, &req)`) | 根の `""` が汚れている(上) | 鳴る + `field_imprecision`(why: `whole_value`) |
| アクセスパスの切り詰め(k = 3)に当たった深い道 | `Task/MessageType/*` で切られ、`HttpRequest/Url` と `HttpRequest/Body` が 1 つのセルに混ざる(上) | 鳴る + `field_imprecision`(why: `path_limit`) |
| 要素(スライス、マップ)の中のフィールド | `[]` の要素は区別しない(`container_element_imprecision` の、今の扱い)。要素のセルが上 | 鳴る + `field_imprecision`(why: `element`)。今の `container_element_imprecision` と同じ道 |
| 細かく書き込まれた(`stmt.SQL = x`、リテラル `Statement{SQL: x}`) | `SQL` のセル(下) | 普通の finding(不確実性なし) |

**判定は 1 つ**: `collect` が返した `found` の `Cell.Path` が、**照合した道より短い(上)**なら、`field_imprecision`。同じか、長ければ(下)、なし。**この規則が、3 行を、同じ仕組みで言う**(丸ごと汚れた構造体も、k で切れた道も、要素も、「道を包む上のセルが汚れている」形)。

**不確実性の種類**: 今の `depth_limit`(`path_limit`)は、「関数の中で、道を切った」という、関数単位の記録(`engine.coarsened`)で、finding の経路には付かない。ここで言いたいのは、**その finding の sink で、「どのフィールドかは分からない」**ことなので、**sink の段(`sinkStep` の `Notes`。今、動的な呼び出しの `dynamic_dispatch` が付く所)に付ける新しい種類 `field_imprecision`** を足す(出力の追加だけ。`rivus.result/1` の版は上げない。ADR-034)。`detail` に、`why` と、指定した道、汚れていたセルの道を書く。代案: `path_limit` と `container_element_imprecision` を再利用する(新しい種類なし)。**推す: 新しい種類**(読む人が、「sink の指定した道の上が汚れていた」と、「関数の中で道を切った」を、見分けられる)。

**k = 3 の扱い**: 道が `Task` / `MessageType` / `*` / `HttpRequest` / `Url` と、**段が 5**(`/` が 5 つ)で、K = 3 を超える。`Task` は `*CreateTaskRequest` のポインタの先で、ポインタの値のセルは指す先と別名なので段にならない(`locs`)、と読んだが、**`/*` の数え方は、動物園のセルで実測してから**確定する(段階 0)。k を超える形では、**書き込みの道が `Task/MessageType/*` で切られる**(`pathTrunc`)ので、`Url` と `Body` を区別できない。この形では、`field_imprecision`(`path_limit`)で鳴る。おとりの(`Body` だけ汚れた)finding は、**消えない**。「消えた finding は本物でないことを示す」の対象は、k に収まる形だけ。深い形を細かくするのは、段階 4(7 節)。

**要約(`SinkReach`)で確かめたこと(コードを読んだ。段階 2 で、テストで固定する)**

- 関数 f の中の sink が、f のパラメータ p の下の道を読むとき、`reach` は `Param` の要素を `SinkReach{Elem, Site, Accepts, Steps, Guards}` に記録する(`noteSinkReach`)。`Elem` は、パラメータ番号と**道**を持つ(`ParamElem(idx, path)`)。指定した道は、`collect` が `Param` の要素に `rest`(上のセルが丸ごと `Param` のとき、道の残り)を足して(`ext.WithPath(pathTrunc(e.Path()+rest, K))`)、**要素の道に乗る**。
- 呼び元は、`expandParam` で `locs(実引数) + e.Path()` を `collect` する。**指定した道が、要素の道として、実引数の同じ道に当たる**。f が構造体を別の関数 g に渡して、g の中の sink が `Url` を読むときも、g の要約が `Param(p, "/Url")` の `SinkReach` を持ち、f の `Param` に変わる(今の道の合成)。
- つまり、**新しい種類の `SinkReach` はいらない**。足すのは 1 つ: `SinkReach` に、**不確実(`field_imprecision`)かどうか**の印(`mergeSinkReach` は、同じ `Elem` と site を、短い経路 1 つにまとめるので、不確実な到達が、確かな到達に潰されないよう、印は OR で残す。`signature` / `joinSummaries` の比べにも入れる)。経路の `sinkStep` の `Notes` に載せるだけでは、まとめるときに落ちうる。
- **k が効く所**: 要素の `Param` の道が `K` で切られる(上の `pathTrunc`)ので、深い道の `Param` は、浅い道のセルに当たる。呼び元は、広い(兄弟を含む)セルを読む。**過検出側(見逃しにはならない)**。切れたときは、印で `path_limit` を言う。

## 6. 最初に書く型(試金石)

| 型 | 道の形 | 試すこと |
| --- | --- | --- |
| `spanner.Statement{SQL}`(`txn.Query(ctx, stmt)`、`BatchUpdate`) | 浅い(1 段、`[]`) | 偽陽性(`Params` だけ汚れる)が消えること。`NewStatement(sql)` を通る形(丸ごとの汚れ)が、不確実性つきで鳴ること |
| `bigquery` の `Query` の `QueryConfig.Q`(`q.Run` / `q.Read`) | 埋め込み 1 段 + 1 | 今、関数の引数で書ける形(`client.Query(q)` の引数 0)との比較。**同じ流れを 2 つの sink にしない**: 実行する所(`Run` / `Read`)を sink にし、`client.Query(q)` は、`flows` で結果に運ぶ(下の注) |
| Cloud Tasks の `CreateTaskRequest.Task.MessageType.HttpRequest.Url` | 深い(5 段)+ oneof(インターフェース) | k = 3 に当たる。`/*` の読み飛ばし。おとり(`Body` だけ汚れ)は、段階 4 まで鳴る(不確実性つき) |

`exec.Cmd` は最初の対象にしない(`exec.Command(x)` が、すでに sink で、受け手の `Path` / `Args` も sink にすると、同じ流れが 2 件になる)。`tls.Config.InsecureSkipVerify` は対象外(汚れた値の流れでなく、定数の設定の話)。

注(BigQuery): `client.Query(q)` の `flows: [{from: arg0, to: result}]` は、結果を**丸ごと**汚す(`flows` の `to:` は、今、フィールドの道を言えない)。そのため、最初は、`Run` / `Read` の受け手の `QueryConfig.Q` が、上のセル(丸ごと)で当たり、`field_imprecision`(`whole_value`)つきで鳴る。見逃しではないが、確かな finding にならない。`flows` の `to:` に道を書けるようにして(`to: result.QueryConfig.Q`)、細かく言うのは、段階 5。

`googleapis` や `google-cloud` を、組み込み(`internal/model/builtin/`)に置くか、`examples/rules/`(`go-google-cloud.yaml`)に置くか: ADR の「組み込みは言語と標準ライブラリまで、フレームワークはユーザーのルール」の方針では、後者。**決めること**(11 節)。

## 7. 「プロジェクトのインターフェースの裏の sink」(ADR-068)との関係

**同じ道具で解ける。** ADR-068 は、動的な呼び出し(`CalleeDynamic`)の候補ごとに、`rules.Sinks[候補のシンボル]` を引き、sink のルールのある候補ごとに site を作る(`Site.Callee` を候補の ID に)。`Site.Sink` は `[]model.Sink` で、**`parts` は `Sink` の項目なので、そのまま site に付いて来る**。`checkSink` は、候補の site でも、同じ関数を通る(`cc.args` と `cc.recv` は、動的な呼び出しのオフセットのまま)。だから、プロジェクトの `Querier` インターフェースの裏の `(*spanner.ReadOnlyTransaction).Query` にも、`SQL` の指定が、そのまま効く。

- 構造体の値は、インターフェースの呼び出しでも、引数の位置で届く。受け手が `recv: true` の `parts` のときは、ポインタレシーバのインターフェース越しの呼び出し(ADR-060。`recvBoxed`)で、受け手が `/*` の下に包まれる。照合の `*` の読み飛ばしで、そこも拾う。段階 1 の試験に、ADR-068 の形(`testdata/go/cases/ownif`)の、`parts` つき版を入れる。
- ADR-068 の限界は、そのまま引き継ぐ: 候補が分からない(0 個)ときは、sink を当てない。呼び元が渡した型で絞るガード(ADR-031)は、候補の site には付かない(過大側)。

## 8. 他の言語に使えるか

**使える**(表現は、位置と、フィールド名の列で、言語に依らない)。照合は、IR のフィールド名と `*` の読み飛ばしだけで、型の言語に依らない。

- **Dart**(`googleapis` の生成クラス。`CreateTaskRequest(task: Task(httpRequest: HttpRequest(url: x)))`): 名前付きのコンストラクタ引数が、`field_store` として IR に出るかどうかで決まる(`this.url` の初期化の形式と、通常のコンストラクタの本体)。段階 6 で、zoo-dart の同じ 3 つのマスで測る。セッター(`req.url = x`)が、呼び出しとして出るなら、モデルの `flows` が要る(Dart の extractor が、フィールドの書き込みを、`field_store` にしているか、測る)。
- **Rust**(生成された構造体。`prost` / `tonic` のリクエスト): 構造体のフィールドは、`/name` か `/0`。`Option<T>` と `oneof` の enum は、`Some(..)` のペイロードや enum の値の道が、どう出るかが、Go の `/*` と違う。ビルダー(`Request::builder().url(x).build()`)は、ビルダーの内部のフィールドを通る(モデルの書き手は、内部の名前に頼れない)ので、**ビルダーの最後の呼び出し(`.send()`)の受け手に、道を書く**か、ビルダーのメソッドに `flows` を書く。段階 6 で測る。
- 3 言語とも、**道の名前を知る補助**(5 節の診断)が、モデルを書くのに要る。

## 9. 段階と、各段の受け入れ

精度を変える段は、**消えた finding が本物でないことを、`bench/compare.sh` の、消えたものの経路を読んで分類して示す**(本物が 1 件でも消えたら、push しない)。**増えたものは、sink の単位で分類する**(本物の流れか、不確実性つきの鳴りか)。

| 段 | 内容 | 受け入れ |
| --- | --- | --- |
| **0. 測る**(何も変えない) | (a) zoo の 3 つの形(浅い / 5 段 / インターフェースに包む)で、実際のセルの道を出す(診断の追加だけ)。`/*` の段の数え方と、k に当たる所を確定する。(b) go-scale と Gitea で、今の sink の位置に、**構造体が渡される**ものを数える(組み込みの sink で、`parts` を足せば偽陽性が減るもの。例: `net/http` の要求を渡すもの)。(c) `pathsOf` の走査の費用 | 数と道を、この文書に書き戻す。結果は変わらない |
| **1. 書式と、1 つの呼び出し** | `model.Sink.Parts`(読み込みの検査: 位置と `field` の整合、糖衣)。`sites.SinkArgs` と、`sinksAt` の整理。`checkSink` を、`parts` のある sink で、道の照合に。`field_imprecision`(sink の段の `Notes`)。**`parts` のない sink は、今と同じ**。zoo の新しいマス `conv/struct-field-at-call`(下) | `compare.sh`: **`parts` を使うルールがまだないので、finding の差 0**(本物と偽のどちらも)。zoo のマスが期待どおり |
| **2. 要約(`SinkReach`)** | `SinkReach` の不確実性の印。`mergeSinkReach` と `joinSummaries` / `signature`。試験: 構造体を別の関数に渡す、`*dst = *src` のコピー、ポインタ、ビルダー風の関数(中で詰める)、`flows` のあるモデルが詰める形 | 試験(`testdata/go/cases`)が、1 つずつ。差 0(まだルールがない) |
| **3. 最初のモデル** | `go-google-cloud.yaml`(6 節の 3 つ)。既存の sink に `parts` を足すもの(段階 0(b) で選ぶ)は、**1 つずつ**足す | **足すたびに `compare.sh`**: 消えた finding は、「危険でないフィールドだけが汚れていた」ものと、経路で示す(ソースが、`Body` や `Params` だけに入っている)。増えた finding は、sink の単位で分類。`go-scale`: 時間と不確実性の数 |
| **4. 要求された深さ**(k の、局所的な拡張) | 規則の `parts` が言う道(例: 5 段)の上だけ、切り詰めを `max(K, 道の段数)` にする(エンジンの全体の `K` は 3 のまま)。道の集合は規則で決まり有限なので、収束は変わらない。要約の `Param` の道の切り詰めも同じ | 先に測る(段階 0 の続き): 要素数、収束、`go-scale` の時間。**結果が変わる(おとりが消える)ので、compare で、消えたものを分類**。深い形の `field_imprecision`(`path_limit`)が、細かい finding に変わる |
| **5. `flows` の `to:` に道** | `to: result.QueryConfig.Q` のように、結果や引数の、フィールドを言う。`client.Query(q)`、`spanner.NewStatement(sql)` を細かくする | `whole_value` の `field_imprecision` が、細かい finding に変わる。差は、compare で分類 |
| **6. Dart と Rust** | zoo-dart、zoo-rust の同じ 3 マス。`googleapis` の生成クラス、Rust の生成された構造体 | マスの期待値。出ない形は、`@miss(理由)` で記録 |

**zoo の新しいマス `conv/struct-field-at-call`**(3 言語。`bench/zoo-go/cells/conv/struct_field_at_call/`)。形は 3 つ:

1. **浅い**: `req.SQL = src` → `mark.SinkStmt(req)`(`SQL` を sink とする、zoo 用のルール)。
2. **5 段**: `req.Task.MessageType` に oneof のラッパーを入れ、`HttpRequest.Url` に入れる → `mark.SinkTask(req)`。
3. **インターフェースに包む**: ポインタレシーバの値をインターフェースに入れて渡す(`any(&X{...})`、ADR-060 の形)。

おとりは、形ごとに 1 つ: **別のフィールドだけ**(`Params`、`Body`)に source を入れる(`@decoy`。`@nosink`)。期待値: 浅い = vuln は出る、おとりは**静か**(今は、`SinkAny` のように、出る)。5 段 = vuln は出る(`field_imprecision`、`path_limit` つき)、おとりは**段階 4 まで出る**(`@fp`、理由 `path_limit`、参照 = この文書)。包む形 = vuln は出る、おとりは、k に入れば静か。`@miss` は 1 つもなく、見逃しの形は、`vuln` のどの形にもない(実行で裏付ける)。

## 10. 案 A を、後で足す条件

案 B で解けないのは、**「その構造体を使う呼び出しが、読んだコードに現れない」形**: 構造体を作って、読んでいないコード(フレームワークのリフレクション、`json.Marshal` を通って別のプロセスが受ける、`context` に入れてミドルウェアが取る、設定ファイルの代わりの構造体)が、危険な処理をする形。このとき、sink になる**呼び出しがない**ので、案 B は書けない。条件: (1) **この形が、実コード(`go-scale`、Gitea、zoo の wild)か、実在の脆弱性で、1 件でも見つかった**とき。(2) 案 A は、**案 B と並べる**(置き換えない。同じフィールドを、案 A と案 B の両方に書かない: 同じ流れが 2 件になる)。(3) 案 A の見逃しの形(丸ごとのコピー、読んでいない関数が詰める)は、`field_imprecision` の考え方(上のセルの汚れ)で、同じく鳴らす + 不確実性にする。それまでは、作らない。

## 11. 決めること

1. 新しい不確実性の種類 `field_imprecision`(推す)か、`depth_limit`(`path_limit`)と `container_element_imprecision` の再利用か(5 節)。
2. 書式の `parts: [{arg|recv, field}]` と、糖衣 `args + field`(3 節)。
3. 照合で `*` を読み飛ばす(3 節。ずれたら拾う側)。
4. Google Cloud のモデルの置き場: `examples/rules/`(ADR の方針)か、組み込みか。
5. 段階 4(k の局所拡張)をやるか。やらなければ、深い形のおとりは、`field_imprecision` つきで鳴り続ける(見逃しはない)。測ってから決める(段階 0)。
6. BigQuery の sink を、実行する所(`Run` / `Read`)に置き、`client.Query(q)` は `flows` にする(同じ流れを 2 件にしない)。

## 確かめていないこと

- **アクセスパスの実際の形**(`/*` の段の数、Go の埋め込み、oneof のラッパー)は、コードを読んだ範囲。段階 0 で、zoo のセルで実測する。数え方が違えば、k に収まる形と収まらない形が入れ替わる。
- `pathsOf` を道の照合に使う費用(段階 0)。
- `SinkReach` の、不確実性の印を `signature` に入れたときの、収束への影響(段階 2)。
- Dart と Rust の、フィールドの書き込みと、`Option` / `oneof` の道の形(段階 6)。
- 既存の組み込みの sink で、構造体を渡されるものの数(段階 0(b))。
