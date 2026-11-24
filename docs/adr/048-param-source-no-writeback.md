# ADR-048: `kind: param` の source の汚染を、呼び出し元へ書き戻さない（C-1）

- 状態: 有効
- 関連: -
- 要旨: 「シード」だけ書き戻さない

**背景:** `kind: param`（`param_type`、`marker`、`extends` も同じ）の source は、そのパラメータが指すオブジェクトを汚染する。この汚染は、要約の `OutParam`（そのパラメータ）と `OutGlobal`（グローバルへの書き込み。グローバルのストアにも公開される）として、呼び出し元へ書き戻っていた。呼び出し元が同じオブジェクト（`main` が `os.Args[2:]` を、複数の `run*` に渡す形、ルータが同じリクエストを複数のハンドラに渡す形）を別の関数に渡すと、「source = runB のパラメータ、sink = runA の中」という finding が出た。runB が sink に届かなくても、である（`testdata/go/cases/paramshare`、`cmd/rivus/paramshare_test.go` の CORE-A）。

**決めたこと:** **`kind: param` の source は、その関数が受け取った値についての宣言**として読む。**書き戻さないのは、「シード」だけ**: `seedParam` が、パラメータのセルに置いた要素（理由が `source: parameter ...` で、`from` を持たないもの）を、要約の `OutParam`（そのパラメータ自身）に出さない（`summarize`、`why.isParamSeed`）。関数が命令で行った書き込み（store、チャネルへの送信、呼び出し先の要約が書いたもの）で着いた要素は、命令の理由と `from` を持つので、**外さない**。つまり、
- `shared = r.Path`、`save(r.Path)`（ヘルパーがグローバルに書く）、`queue <- r.Path`（グローバルのチャネル）、`c.App.Last = c.Query`（同じパラメータの先の共有の状態）は、`OutGlobal` / `OutParam` として出て、グローバルのストアにも公開される。別の関数がそれを読めば、finding になる（`testdata/go/cases/paramshare/gwrite`、`gwritehelper`、`gqueue`、`ctxstate`）。
- シードそのものは、呼び出し元に書き戻らない。`runB(r)` の `r` が、呼び出し元で、`runB` の source として汚染されることはない。

最初の実装（623b620）は、`OutGlobal` を、シードかどうかを見ずに外し、`compute` のグローバルへの公開も止めていた。そのため、関数自身がグローバルに書いた値まで消え（上の 4 つの形が、変更前は 1 件、623b620 では 0 件）、gogs の `InstallPost` からの流れ 11 件と、gin-vue-admin の 3 件が消えた。この規則に直した。`os.Args` の再現（`global`）は、`OutGlobal` を外さなくても直る（`runB` の `OutParam` のシードを呼び出し元に書き戻さないだけで、呼び出し元の実引数（グローバル）に一致しない）。

**この退行(623b620)は、実コードのベンチ(gogs、gin-vue-admin)の finding の差分で見つかった。コアの精度の変更は、その手順(前後のバイナリ、`measure.py compare`、実コードの finding の差分を 1 件ずつ分類、見逃しが 1 件でもあれば push しない、実行できなければ push しない)を `bench/README.md` の「Changing the accuracy of the core: the procedure」に置いた。**

止めるのは**これだけ**: シード以外の要素（ほかのパラメータへの書き込み、同じパラメータの別の場所への書き込み、戻り値、例外、通常の source の書き戻し、グローバルへの書き込み）は、そのまま。パラメータ自身から、その関数の中で sink に届く流れ（`direct`）は残る。複数の source が同じオブジェクトを受け取って、それぞれが sink に届く形（`repro`）は、finding の数が変わらない。

**理由（実測）:**
- rivus 自身（dogfood のルール）: finding 88 → 39（`OutGlobal` を外さない規則でも同じ 39 件。書き戻しの偽の組み合わせは、`OutParam` のシードで起きていた）（8 つの入口 × 11 の sink の組み合わせのうち、source の関数に呼び出しで届かない、偽の組み合わせ 43 件（キーで数えて）が消えた。残りの 39 件は、すべて経路に source の関数を含む）。経路に source の関数が出ない finding は 77 → 0。`runExtract` は、規則の sink のどれにも届かない（11 件がすべて偽だった）。再解析は、シードだけを外す規則では 10,151 回（変更前 10,155。`OutGlobal` まで外した 623b620 の 8,168 は、グローバルへの書き込みを止めた効果で、書き込みの消失を含んでいた）。
- `other_sources` の誤った uncertainty: `param_type` で、同じリクエスト型を受け取るハンドラ 2 つ + 共通の補助関数に sink（`ptype`）で、届かないハンドラが、届くハンドラの finding の `other_sources` に入っていた。finding ごとに判定する LLM は `other_sources_count` を見て「ほかの入口からも届く」と読むので、誤った uncertainty だった。直った（`other_sources_count` が 0）。
- go-synth S / M、Dart・Rust のサンプル、Dart の pub と serverpod、3 つの curated（`@miss` / `@fp` / `@noisy` の数）は、finding が同一。Rust の parseable は 94 件のまま、1 件の source が入れ替わった（書き戻しの偽の経由先だった `get_user_from_request` から、実際に tenant を読む `get_tenant_id_from_request` へ）。
- Gitea（`./...`、実験のとき。再実行していない）: 収束（7 パス）、**最大の SCC は 3370 のまま**（書き戻しは SCC を作っていない。案 2 の領分）。finding は 37 → 41（off にだけある 8 件はすべて、経路が source の関数から始まらない。on にだけある 12 件のうち 5 件は、経路が source の関数から始まる）。書き戻しは、収束の問題としては部分的で、精度の問題が主。

**失うもの:** `func runB(r *Req){ _ = r }` が source で、呼び出し元が `runB(r); os.Create(r.Path)` と書くとき、「runB が受け取った値が、呼び出し元の sink に届く」流れは、**出なくなる**（`testdata/go/cases/paramshare/lost`、`TestParamSourceNotWrittenBackIsLost`。既知の見逃し。uncertainty もない）。source が受け取った値は、その関数の中で使われたときだけ追う。呼び出し元の側でも追いたいなら、呼び出し元を入口にする（`main` や、リクエストを作る関数に、`kind: param` や `call_result` の source を宣言する）。

**限界:**
- 別のオブジェクト経由の書き戻し（source のパラメータが指す値が、ほかのパラメータやグローバルが指すオブジェクトに入れられて、それが書き戻る）は、止めていない。止めるのは、param source のサイトが起点の要素が、そのパラメータ自身（`OutParam`）と `OutGlobal` に出る場合だけ。
- Gitea の on / off の差は、実験の保存された出力を読んだもので、呼び出しの到達の確認ではない。
- 巨大 SCC（Gitea の 3370）は、この変更では小さくならない。

