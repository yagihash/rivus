# ADR-082: ライターのモデルは、書かれたデータをレシーバの全体でなく「中身」の要素(`.content`)に置く

- 状態: 有効
- 関連: MODELING に記載なし
- 状態の詳細: 訂正が本文内。090 が続き; 訂正・追記あり(書き方を保つ)

**背景:** ADR-075 で、`(io.Writer).Write` の類のモデルが「引数がレシーバに入る」と宣言し、本体も読むようになった。レシーバの**全体**に汚染が載るので、自前の実装の本体が自分のフィールド(ファイル名、設定)を読んで sink に渡すと、書かれたデータとは関係のない流れが出た(Gitea の追加 20 件の主な根)。

**決めたこと:** モデルの `flows` の `to:` に `recv.content` / `argN.content` を書けるようにした(`model.ParseDest`。`from:` と結果には書けない)。エンジンでは、書き込み先の場所(Loc)のアクセスパスに、どの言語のフィールド名にもならない 1 段 `#content`(`model.ContentStep`)を足した場所に載せる。値の**フィールドとは別の要素**なので、ライターのフィールドの読み出しには届かず、ライターの全体(`from: recv`)の読み出し、`String()` などの中身の読み出しは、今までどおり汚染を見る(全体の読み出しは部分の汚染を含むため)。sink の `parts` の照合では `#content` を透過の段として飛ばす(`transparentStep`)。対象は、`io.Writer` の類に限る: `Write`、`WriteString`、`WriteByte`、`WriteRune`、`ReadFrom`、`fmt.Fprint*` と `io.WriteString` の先、`bytes.Buffer` と `strings.Builder` の書き込み(`go-stdlib.yaml`、`go-stdlib-more.yaml`)。Dart と Rust のモデルは変えない。

**確かめたこと:** `testdata/go/cases/writer`(ライターのフィールドを読む本体は検出されない。書いたデータを読む本体と、ライター全体の読み出しは検出される)。zoo の `dyn/stdlib-writer-own`、`dyn/http-middleware-chain`、`conv/buffer-writestring` は出たまま。Gitea(`bench/compare.sh --heavy`、base は ADR-078 の main): 367 → 358、追加 3、削除 12、7 パスで収束(再解析 112,695 → 112,353)。削除 12 は、1 件ずつ経路を読んだ: 9 件は、ログのファイルのライター(`rotatingfilewriter`)の 3 つの sink に、書いたデータでなくライターのファイル名を通って届いていたノイズ(経路 100 以上、source の関数で始まらない)。3 件(`UploadPackage` と `PutManifest` を source にしたもの)は、同じ sink が別の source(同じ形の `Context`)で見つかっており、代表の source が変わっただけで、sink は失われていない(追加の 3 件も同じ入れ替わり)。本物の finding が消えた例はない。

**限界:** ADR-075 の追加 20 件のうち、残るのは 8 件(sink は 4 か所)。この変更の根(書き込みの置き先)ではない: (1) 汚染されたリクエストの context そのものを `io.Writer` として渡した先(`event.WriteTo(ctx)`)で、`w.Write` の候補に別の実装(`RotatingFileWriter`)が入り、そのレシーバが汚染される。これはレシーバの同一視(XTALK)の根で、呼び出し元が渡した型で候補を絞る作業(`narrow.go`)の範囲。(2) 共有状態を通る長い経路(`shared_state`)。`ctx.Resp` を包む形(XSS 2 件)は、sink 単位では、変更の前後で件数が同じで、この変更の効果は区別できない。


**訂正(サイレントな見逃しを直した):** `.content` に置くだけでは、`fmt.Fprint*`、`io.WriteString`、`io.Copy`、`template.Execute*`、`pem.Encode`、`png.Encode` のように**ライターの `Write` の本体を適用しない**モデルで、ライターの本当のフィールド(`w.data`、`w.buf`、`httptest.ResponseRecorder.Body`)に流れが届かず、本物の流れが何も報告せずに消えた(自前のライター `Fprintf(w, …)` のあと `string(w.data)` を sink へ、`Fprint(rec, …)` のあと `rec.Body.String()` を sink へ)。テストは `w.Write` を直接呼ぶ形だけだった。**今の決定**(`contentOnly`): `.content` の段だけに置くのは、(1) `recv.content`(ライター自身のメソッド。動的な呼び出しでは本体を適用する)、(2) レシーバの型が `bytes.Buffer` / `strings.Builder`(モデルが読み戻す)のときだけ。それ以外(プロジェクトの型、インターフェース、他のライブラリの型)の `argN.content` は、`.content` の段に加えて**ライターの全体にも**置く(ADR-082 以前と同じ)。代わりに、本体を適用しない呼び出し(`fmt.Fprint(w, x)`)で、ライターのフィールド(ファイル名)を読む本体の sink は、書いたデータと無関係に検出される(ノイズ。zoo の `dyn/fprint-writer-own` の decoy は `@fp`)。本体を適用する直接の `w.Write` / インターフェースの `Write` では、ADR-082 のとおり検出されない(`bench/go-curated/api/rotate.go`)。根の修正(`Fprint` の先の `Write` の本体を適用する)は未着手。テスト: `testdata/go/cases/writer`(自前のライター、`httptest.NewRecorder`、`io.Copy`、`bytes.Buffer` の `Fprint`)、zoo の `dyn/fprint-writer-own`。なお、インターフェース型のパラメータで受けたヘルパー(`func h(w io.Writer){ fmt.Fprintf(w, …) }`)に自前のライターを渡す形は、ADR-082 より前から出ない(別の見逃し。未着手)。

