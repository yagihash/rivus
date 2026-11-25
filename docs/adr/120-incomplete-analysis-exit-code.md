# ADR-120: 完全でない解析は、結果を出したうえで、専用の終了コード 4 で終わる(2026-11-23)

- 状態: 有効
- 関連: ADR-054(時計に依る結果)、`stats.timed_out`、`summary.incomplete`
- 要旨: 全体の時間切れ(`-timeout`)と未収束(`-max-passes`)は、結果を全部出したあと、終了コード 4 で終わる。理由は `summary.incomplete_reasons`(`timeout`、`not_converged`)。`-max-memory` の停止は、同じコードの別の理由として足す(この PR にはない)。

**背景:** 今まで、時間切れも未収束も終了コード 0 で、`stats` を見ないと完全かどうか分からなかった。利用者(ユーザーの決定)は、完全でない解析は 0 にしないことを選んだ。

**決めたこと:**
1. コードは 4(0 成功、1 エラー、2 使い方、3 Rust を断った、130 中断。extractor の 10/20/50/70 は `make model` の中のもので、`rivus analyze` の終了コードではない)。理由が複数でも 1 つのコード(呼ぶ側が知りたいのは、完全かどうか)。
2. 出力は全部出してから終わる。`summary.incomplete`(bool)と `summary.incomplete_reasons`(配列)は、いつも出る。`stats` の項目は変えない。
3. 関数ごとの切り(`stats.func_timeouts`、`budget_exhausted`)は、実行が収束していれば、今はこの PR では不完全に数えない(`summary` と `stats` に出るだけ)。段階を踏む: 先に全体の限界だけ。関数ごとの切りも数える案は、頻度を測ってから(下の数字)。
4. `analyze`、`index -taint`、`query`、`resolve`、`models coverage` が同じ。保存した index を読むだけの `-index` は、今の解析ではないので 0。

**頻度(関数ごとの切り):** `.claude/work/bench-out` の結果では、収束した実行(cve、memos-aws)に `func_timeouts`、`budget_exhausted` は無かった(出力に項目なし)。全体の時間切れは、memos と dartpad のパスラインの実行で起きている。軽い対象での測定は未完。

**呼ぶ側と道具への影響:** 時間切れ(`-timeout`)で、今まで 0 で終わっていた実行が 4 になる。終了コードが 0 でないことを失敗として扱うスクリプトは、4 を許す形にする。`bench/deps_bench.py`、`bench/compare.py`(`rc` は 0 のまま、`exit_code` に 4 を記録し、完全でない実行はキャッシュに入れない)、`bench/go-scale`、`bench/go-synth`、`bench/model-diff` は 4 を許す。

**zoo:** zoo の `F/state/global-chain`(13 段と 30 段の鎖)は、周回の上限で止まるように作ってある(`recursion_approximation` の `@miss`。Go、Dart、Rust に同じ)。それを含む実行(zoo 全体、`TestScaleSynth` の chain13 と chain30)は 4 で終わるのが正しく、テストはその実行だけ 4 を許す(`notFailed`、`notFailedIf`)。ほかの実行は、4 なら失敗で、zoo の残りのセルが収束することは `TestZoo*` の `zooSplit` が確かめる。

## 追記: 問い合わせの出力(2026-11-23、相談役の指示。#195)

**背景(測定):** 合成の対象で `rivus index -taint -timeout 1ns` は終了コード 4 で、index の `summary` に `incomplete: true`、`incomplete_reasons: ["timeout"]` が入る。ところが `rivus query -index <その index> -sink <symbol>` の出力には `summary` も `incomplete` もなく、終了コードは 0 だった。`matched` が 1 件以上で `groups` と `findings` が 0 件は、「届く source は無い」と読める、完全な答えに見えた(探していないのに「無い」と答える)。`stats` には同じ意味の情報があったが、名前のある項目ではない。`resolve -index` も同じ index を読む。

**決めたこと:**
1. `query` と `resolve` の出力の最上位に、`incomplete`(bool)と `incomplete_reasons`(配列。完全なら空の配列で、null ではない)を、**いつも**出す(`-index` のときも、今の解析から答えるときも)。名前と意味は `summary.incomplete` と同じ(`report.IncompleteReasons`)。既存の項目は変えず、足すだけなので、`schema_version` は上げない(ADR-008)。
2. **終了コードは変えない**: `-index` の問い合わせは 0 のまま(今の解析ではない)。今の解析から答える問い合わせは、4 のまま。
3. **古い index の扱い:** `summary.incomplete_reasons` がある index は、その値をそのまま引き継ぐ。ない(古い形の)index は、保存した `stats` から `report.IncompleteReasons` で作る。`stats` もない index は、完全とは言えないので、`incomplete: true`、`incomplete_reasons: ["unknown"]`(分からない)にする。
4. `models coverage -index` の出力は、届くかどうかの答えではなく、モデルの足りない所の一覧なので、この追記では変えない。`graph` は、別の層(呼び出しの関係。`index` が書くグラフのファイル)で、taint の JSON の index を読まない。

## 追記: 関数ごとの打ち切りも「完全でない」に数える(2026-11-23。決定の 3 を改める)

**決定の 3 は、次に置き換える。** 関数ごとの打ち切り(1 関数の時計 `-func-timeout`: `stats.func_timeouts`。作業量の上限: `stats.budget_exhausted`)が起きた実行は、収束していても、完全でない解析として、終了コード 4 で終わる。理由は `summary.incomplete_reasons` に、`func_timeout` と `budget` として、分けて出る(終了コードは 1 つのまま。決定の 1)。`summary.incomplete`、`query` と `resolve` の最上位の `incomplete`、`incomplete_reasons` は、同じ `report.IncompleteReasons` で決まるので、そのまま効く。

**理由:**
- 基準は「計算機に依るか」ではなく、「流れを落としたか」である。打ち切った関数の要約は流れを欠く(uncertainty の文面も「its summary may miss flows」)。その関数を通る問い合わせが「届く source はない」と答えても、探し尽くした答えではない。作業量の上限は、同じ計算機なら同じ所で止まるが、流れを落とすことは変わらない。
- #195(時間切れの index への問い合わせで、完全でないことが出なかった)と同じ型である。`stats` と `depth_limit` には出ていたが、`stats` を読まない呼び出し側には届かなかった。
- 毎回 4 になる対象があれば、それは正しい知らせで、直すのは打ち切りの側(止める所を粗くする、時計を作業量の規則に置き換える。ADR-118 の表)である。

**打ち切った関数の一覧:** 関数ごとに 1 件の `depth_limit` の uncertainty(`func` に関数、`detail` の先頭が `func_timeout:` か `budget:`)が、トップレベルの `uncertainties` と、その関数を通る finding の `uncertainties` に出る。`candidates` に一覧を持つのは、`out_limit`(#332)と `path_limit` の uncertainty で、これは別の記録である。

**根拠:** 外部の設計者の返事 `2026-11-23-pending-decisions-reply.md`(問 1: 両方入れる)。ユーザーの決定(2026-11-23): 「architect に同意」。

**影響:** 関数ごとの打ち切りが起きていた実行は、0 から 4 になる。古い index(`summary.incomplete_reasons` がないもの)は、保存した `stats` から作るので、`func_timeouts` か `budget_exhausted` が 0 でなければ、問い合わせの `incomplete` は true になる。

## 追記: 読む対象が無いときは、エラー(1)(2026-11-25)

**背景(測定):** 存在しないパス、`go.mod` だけで `.go` が無いディレクトリ、`pubspec.yaml` だけのディレクトリは、終了コード 0 で、`findings: 0` の結果を出していた。「何も出ない」が「安全」と読める形で、読む側が気づけない。

**決めたこと:** 解析の対象が 0 のときは、結果を出さず、終了コード 1(入力のエラー。新しいコードは足さない)で終わる。対象は、Go はファイル(`-tests` なしのときの `_test.go` は数えない。テストのファイルだけなら `-tests` を示す)、Dart と Rust は `.dart`、`.rs` のファイル。ファイルが 1 つ以上あれば、関数が 0 でも成功(型だけ、コメントだけのファイルは、正当な空)。Go、Dart、Rust のどの目印も無い対象は、その旨と、別の言語の目印(`package.json` など。表は `cmd/rivus/emptyinput.go`)を言う。言語の推測はしない。`analyze`、`sites`、`index`、`query`、`resolve`、`models coverage` は、同じ読み込みを通るので同じ。`-ir` は対象外。フラグは足さない。
