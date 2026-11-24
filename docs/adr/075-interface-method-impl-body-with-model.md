# ADR-075: モデルのあるインターフェースのメソッドの dynamic な呼び出しで、プロジェクト自身の実装の本体も読む(モデルとの和)

- 状態: 一部覆った(→ADR-086)
- 関連: -
- 状態の詳細: sanitizer を対象外にした点を 086 が変更; 訂正・追記あり(書き方を保つ)。追記あり。覆した先・注意: 086
- 要旨: 和を取る。追記で `dependency_body_unread`

**背景:** `io.Writer`、`http.Handler`、`error`、`io.Closer` など、標準のインターフェースのメソッドにはモデルがある(`(io.Writer).Write` は「引数がレシーバに入る」、`(net/http.Handler).ServeHTTP` と `(io.Closer).Close` は `pure`、`(error).Error` は「レシーバが結果に入る」)。dynamic な呼び出しでモデルが一致すると、プロジェクト自身の実装の本体は読まれなかった(`read_impls` の行だけが例外で、Rust の標準ライブラリの 2 つの行集合)。そのため、`var out io.Writer = shellWriter{}; out.Write(src)` の `Write` の中の sink、ミドルウェアの連鎖の `next.ServeHTTP(w, r)` の先(`logging{next: inner{v: src}}` の `inner.ServeHTTP` の中の sink)、`error` の実装の `Error()` の中の sink が出なかった。uncertainty はモデルが「解決済み」に見せるので、`dynamic_dispatch` の不確実性だけが残っていた。Dart(`StringSink` を自前で実装して `out.write(src)`)と Rust(`impl Write` を `dyn Write` で呼ぶ、自前の trait にモデルを書いた場合)も同じ形。

**決めたこと:**
1. dynamic な呼び出しで呼び出し先にモデルがあるとき、モデルを適用したあと、**本体のある候補(依存でなく、プロジェクト自身のもの)の要約も適用する**(`internal/taint/calls.go` の `readsImpls`)。モデルと本体の**和**: モデルが与える流れは失われない(`pure` のモデルとの和は、本体そのもの)。
2. **候補の集合が閉じていないとき**(プロジェクトの外の実装がありうる)は、モデルがその分を示す。今までと同じで、本体のない候補は、モデルに任せる。
3. **sanitizer のモデルは除く**: sanitizer は「結果は無害だ」という宣言で、本体を読むと取り戻してしまう。sanitizer のあるシンボルは、今までどおり、本体を読まない。propagator だけのシンボルが対象(`read_impls` の行は、これまでどおり)。
4. 言語は区別しない(Go、Dart、Rust の dynamic な呼び出し)。

**確かめたこと:**
- zoo(Go): `dyn/stdlib-writer-own` と `dyn/http-middleware-chain` の `@miss`(vuln と decoy、4 か所)が見つかる側に変わった(`@miss` を外した)。修正済みのケース(`fixed`)の sink は検出されないまま、デコイの `@nosink` も検出されないまま。zoo(Rust)は、`dyn/stdlib-writer-own` が**まだ見つからない**: `out.write_all(..)` は標準ライブラリの既定のメソッドで、候補は `ShellWriter::write` でなく、`write_all` の本体はプログラムにない(`@ref(PARITY)` に直した)。Dart は、zoo に同じ形のセルがない。作業用に、`StringSink` の自前の実装を `dart:io` の `Process.run` に渡す 1 ファイルで測った: 変更前 0 件、変更後 1 件。
- `rust-curated/src/modelled.rs` の `model_body`(自前の trait のモデルが、実装の本体の source を隠していた)が見つかる側に変わった(`@miss` を外した。`model_drop` の `@fp` は残る: モデルが「レシーバが結果に入る」と宣言し続けるので、和でも消えない)。
- `bench/compare.sh 7e276b5 58c3e0f`(軽い層。base は変更の前の main): **削除 0**。追加は 5 件、sink の単位で、経路を読んで分類した: zoo(Go)の 4 件と `rust-curated` の 1 件。5 件とも、本物(source が、呼び出しの引数や、実装の本体の中にあり、`@miss` の uncertainty が「あるはずの流れ」としていたもの)。**実コード(gogs 2 つ、gin-vue-admin、go-test-bench、gophish、casdoor、dart-pub、parseable)は、finding の追加 0・削除 0**: ノイズはない(基準の 1 割どころか 0)。変わったのは不確実性の数(`dynamic_dispatch`: gogs-9a9388ac 703 → 728、dart-pub 3,573 → 3,643、go-test-bench 105 → 106、gophish 28 → 32。本体を読む候補が増えた分)と、再解析の数(gin-vue-admin 25,651 → 26,362、+2.8%。収束は同じ)、メモリ(gin-vue-admin 5,242 → 5,688 MB)。
- Gitea(`bench/compare.sh --heavy 7e276b5 58c3e0f`): finding 350 → 367、**削除 0**(3 件は、ほかの finding の `other_sources` に畳まれた、失われていない流れ)、**追加 20 件(sink の単位で 7 か所)**、パスが変わったのが 35、`other_sources` の変化が 24。7 パスのまま収束、再解析 110,120 → 112,688(+2.3%)、メモリ 12,291 → 11,873 MB。時間 593 → 949 秒は、同じ時間に別の解析が実行されていた(ほかの比較)ので、基準にしない(`func_timeout` の 8 → 11 も、同じ時計の影響)。**追加の分類(経路を読んだ): 20 件ともノイズ。** (a) ログのファイルに書く `io.Writer` の実装(回転するライター)の、ファイル名を開く・名前を変える・消す sink、5 か所 18 件: 呼び出し側が `w.Write(data)` を呼び、モデル(「引数がレシーバに入る」)がライターの**オブジェクト全体**に汚染を置き、読むようになった本体が、ライターの**ファイル名のフィールド**(`file.Name()`)を読んで sink に渡す。書いたデータは、ライターのファイル名を決めない。(b) 圧縮のライターに `ctx.Resp` を包んで `Close` → `Flush` → 自前の `Response.Write` に至る、`ResponseWriter.Write` の XSS 2 か所 2 件: source は、リクエストの context そのもの(包まれたライターのオブジェクト)で、書かれる圧縮データではない。(a)(b) とも、同じ根: **モデルがライターの中身を、オブジェクト全体の汚染として持つ**(ライターのフィールドの区別がない)。追加 20 件は、基準 350 件の 5.7%(ノイズの上限 1 割 = 35 件の内)で、収束も変わらない。受け入れの基準は満たすが、ノイズの根は残る(下の限界)。

**追記(読まなかった候補を示す):** 依存の本体は読まない(上の 2。gin-vue-admin で 24 秒が 15 分以上になる)ので、`io.Writer` の動的な呼び出しの候補に、依存の `Write`(受け取った値を、包んだ先の `Write` に渡す)があると、その先の自前の実装の中の sink は見つからず、`dynamic_dispatch` だけが出て、区別がつかなかった。種類 `dependency_body_unread`(`attributable_to: rivus`)を足した(足すだけ。スキーマバージョンはそのまま): モデルのある呼び出し先の候補に、本体のある依存の関数があるとき、その関数を `candidates` に並べて記録する。zoo `dyn/through-dependency`(Go。`bench/zoo-go/dep` は依存の代わりの別モジュール)の `@miss(dependency_body_unread)` で、見逃しの uncertainty として確かめる。

**限界:** (1) Rust の標準の既定のメソッド(`write_all`、`Iterator::map` など)は、候補が実装の `write` / `next` でなく、本体のない標準のメソッドなので、この修正の対象外(`write_all` のモデルが `write` を呼ぶ、という行が必要。`calls:` の延長: ADR-080 で `write_all` を足した)。(2) **ライターのフィールドの区別がない**: `(io.Writer).Write` のモデルは、書いたデータをレシーバの全体に置く。本体を読むと、ライターの別のフィールド(ファイル名、ロガーの設定)が汚染されて見える。Gitea の追加 20 件は、これだった。修正方法の案: モデルの置き先を、レシーバの「中身」の要素にする(フィールドとは別の要素)。この変更の対象外。(3) 同じインターフェースの実装が複数あるときの絞り込みは、Go と Dart で違う。Go の zoo(実装が 4 つ以上)は、デコイの `@nosink` が検出されないまま。Dart で、`StringSink` の実装 4 つ(vuln、fixed、デコイ用の 2 つ)を 1 ファイルに置いて試すと、ある実装に渡した source が、他の実装の sink にも届いた(レシーバの静的な型の全実装が候補。XTALK と同じ根)ので、Dart の zoo のセルにはしていない。Dart の実コード(dart-pub)に、finding の増減はない。


