# #234 Rust: 実対象で、到達性のラベルが信用できない(finding が全部 `not_from_entry`)

- 状態: **open**
- 優先度: 中
- 種類: 取りこぼしに近い(`reachability` で絞り込む利用者から見ると、finding が出ていないのと同じ)
- 言語: Rust
- ROADMAP の節: B / C
- 守るもの: zoo `bench/zoo-rust-app`(再 export の 3 つのハンドラ `serve` / `upload` / `delete` は `from_entry` の期待で、`@miss` は外した。対照の `lookalike` と `unused` は `not_from_entry`、過大近似の `orphan` は `from_entry`、`hooked` は `from_entry`)と、`cmd/rivus/zoo_rust_reach_test.go`
- 進める完了条件: -

## 何が起きるか(事実)

Rust の実対象で、finding が全部 `reachability: not_from_entry` になる。`reachability` の内訳は `from_entry` / `from_framework_callback` / `not_from_entry`。2026-11-26 に、origin/main d90c9255 で取った既存の出力から:

| 対象 | from_entry | from_framework_callback | not_from_entry |
| --- | --- | --- | --- |
| rustypaste(pin) | 0 | 0 | 12 |
| rust-curated | 0 | 0 | 281 |
| zoo-rust | 292 | 0 | 0 |
| memos(Go) | 182 | 0 | 0 |
| Gitea(Go) | 342 | 0 | 17(全部テスト用ヘルパーが sink。下の「Gitea(Go)の `not_from_entry` 17 件」) |
| cocoon(Dart) | 56 | 0 | 0 |
| dart_services(Dart) | 22 | 0 | 0 |

注意:

- zoo-rust の IR に、entry の関数は 0 個。起点が無いので、`reach()` は「Exported かつ !Dep の関数を全部 root にする」ライブラリ扱いになり、その結果が `from_entry` になっている。**実対象の到達性を測っていない**。
- cocoon と dart_services は「ある時点の出力」で、最新の main での再測定はしていない。
- Gitea の `not_from_entry` 17 件は、中身を見た(下の「Gitea(Go)の `not_from_entry` 17 件」)。Rust の形(ハンドラ登録が切れる)とは別の形。

`reachability` で絞り込む利用者(エージェントを含む)から見ると、実対象の Rust の finding が全部「entry から届かない」と読める。

## entry の決まり方(事実)

- コア: `internal/sg/sg.go` の `reach()` は、Init でない entry が 1 つも無いと、Exported かつ !Dep の関数を全部 root にする。`internal/taint/engine.go:1786-1798` は、source、sink、path 上の全関数のどれかが reach に無ければ `not_from_entry` にする(1 つでも外れると、finding 全体が `not_from_entry`)。`internal/taint/result.go:326-332` の `NotFromEntryNote` は「フレームワークに登録したハンドラは entry に入っていない」と書く。
- Rust: `extractor/rust/src/main.rs:248-281`。`rustc_public::entry_fn()` と一致し、かつ !dep の関数だけが `entry: true`。つまり crate の `fn main` だけで、pub 関数やハンドラ登録は entry に入らない。
- Go: `goir.go:695`。main パッケージの `main` と `init`。
- Dart: `extractor/dart/lib/extract.dart:877`。!dep のトップレベルの `main`。加えて ADR-073 の `from_framework_callback`。

## rustypaste の切れ方(粗い BFS の事実)

これは、IR の `callee.func` と `candidates` を辿っただけの粗い BFS で、rivus の本物の `reach()` ではない。数は目安。IR は `~/tmp/rivus-developer/b2-var/rp.ir.jsonl`(関数 84,996、依存でないもの 471、`entry: true` は `rustypaste::main` の 1 つだけ)。

- finding 12 件の source は、すべて `<rustypaste::server::{upload,serve,delete} as actix_web::dev::HttpServiceFactory>::register::<名前>`(ハンドラ本体。`upload` は `src/server.rs:324`)。
- sink 側(`std::fs::File::open` を含む `Directory::try_from::{closure#2}` など)は、main から届く。main から届く関数は 45,242、own 471 のうち 91。
- source のハンドラは、main から届かない(`Paste::store_file` なども)。
- 切れる所: `main` → `main::{closure#0}` → `HttpServer::new/bind/run` → `main::{closure#0}::{closure#0}`(`App` を作る closure、`src/main.rs:189` 付近)→ `App::configure(rustypaste::server::configure_routes)` → `ServiceConfig::service`。
- `<upload as HttpServiceFactory>::register` を指す edge(callee、candidates、make_closure)は、IR 全体で `register` 族の自分自身からの参照以外に 0 件。
- IR にある `HttpServiceFactory::register` の dynamic 呼びは、actix 側のタプル impl の中だけ。candidates は 17 件で、全部 `(A, B, ...)` のタプル impl。rustypaste の impl は 1 つも入っていない。callee は `actix_web::service::HttpServiceFactory::register`、impl は `actix_web::dev::HttpServiceFactory` と書かれていて、ADR-058 の「再 export された trait の dyn 呼び」と同じ形。
- **未確認**: `configure_routes` から `ServiceConfig::service` が `register` を呼ぶ所の edge。

## rust-curated

調べ終わった。**原因は extractor ではなく、対象の `main` が空だから**。

- `bench/rust-curated/src/main.rs:4` は `fn main() {}` のみ。`#[tokio::main]`、async、マクロ、spawn、loop は無い。`main.rs` のコメントは「`main` があれば closed world として読む(`closed_world_entry`)」と言っている(意図)。
- 15 モジュールは `src/lib.rs:1-16` が `pub mod` で持つ。bin の `main` は lib の関数を呼ばない。
- IR(`rust-curated.ir.jsonl`、2026-11-27 に読んだもの): `curated::main` は `entry: true`、`exported: true`、`private: true`、`file: src/main.rs`、pos 4:1。ブロックは 1 つ(`const`(nil)と `copy`、terminator は `return`)で、callee は 0 個。`"entry":true` は IR 全体で 1 個。ソースが空の関数を、そのまま IR にしている(`Terminator::Call`、async、マクロ展開の取りこぼしではない)。
- entry の決まり方: `extractor/rust/src/main.rs:248`(`rustc_public::entry_fn()` の名前)と `:277-279`(`!dep` かつ名前が一致する関数に `entry: true`)。
- `entry: true` が 1 個あるので、`reach()`(`internal/sg/sg.go`)は「起点がある」とみなす。ライブラリ扱い(Exported かつ !Dep を全部 root)にならない。lib の関数 281 件の finding は、`main` から届かない、つまり `not_from_entry`。**エンジンの規則どおりの、正しい出力**。
- 比較: rustypaste の `#[actix_web::main] async fn main` は、展開された結果(`actix_web::rt::System::new`、`{async block}` の alloc 2 回、`rustypaste::main::{closure#0}` の static 呼び、`SystemRunner::block_on`)が callee として IR に出ている。

判断(advisor、2026-11-27):

- rust-curated の `not_from_entry` は、規則どおりの正しい出力として、そのままにする。curated はモデルの検証用で、`main` が空なのは意図。
- `main` に呼び出しを足す案(ベンチ側)は進めない。closed world の結果と 281 件の finding の真偽が動く恐れがあり、得るものが小さい。
- エンジン側で「`bin` と `lib` が同居する crate で、lib の `pub` 関数も root に入れる」案は、課題に立てない。`main` が lib を呼ぶ形なら、lib の関数は `main` から届く(規則どおり)。届かない形は、フレームワークへの登録(この課題の `register` の edge)で、#620 で直した種類。実対象で、`main` が lib を呼ばないのに lib が解析対象にある形が見つかったとき、課題にする(今は見つかっていない)。
- 台帳の `reach_miss`(`bench/maturity_ledger.py` の列。別の PR で足す)の数え方は、rust-curated を含めない。到達性を測るのは `zoo-rust-app` と実対象だけ。

PR #620(`feat/rust-trait-identity-by-def-path`。extractor が trait の同一性を「クレート名 + 定義パス」で比べる)の測定(2026-11-27)でも、281 件の `not_from_entry` は変わらなかった(graph-check で finding の JSON 全体が `same`)。上の原因(空の `main`)と合っている。

rustypaste は、#620 でマージ済み(2026-11-27)。finding 12 件が全部 `not_from_entry` から `from_entry` になった。測定の表は #620 の本文を指す。ヘッダの「守るもの」の行は、#620 で `@miss` を外した状態に直した。

同じ測定で、dyn 呼びの candidates が約 9 倍に増えた。課題 #236。

## 未確認の点

この課題は open のまま。次の点が残っている。

- tokio と axum の `main` の形
- `web::scope("").service(...)` 経由の形(zoo は `ServiceConfig::service` を直接 6 回呼ぶ形にした)
- `ServiceConfig::service` が `ServiceFactoryWrapper` 経由の dyn 呼びで `register` を呼ぶ形(zoo は `factory.register(..)` を直接呼ぶ形)
- parseable の ADR-058 の確認(sandbox の下では取れない)

## Gitea(Go)の `not_from_entry` 17 件(事実)

出典は `gitea-pin.json`(359 件、`from_entry` 342、`not_from_entry` 17。17 件とも `reachability_note` が付く)と、pin のソース(95ba37d9)の `git grep`(2026-11-26)。

- 17 件の source は全部 `modules/httplib.getRequestScheme`(`modules/httplib/url.go:47`)。
- sink は 3 つの関数だけ。
  - `models/migrations/base.deleteDB`: 10 件(path_traversal 1、sql_injection 9、path の関数 33)。
  - `tests.InitTest`: 6 件(sql_injection、path の関数 33)。
  - `models/unittest.DumpQueryResult`: 1 件(sql_injection、path の関数 47)。
- sink の位置は `models/migrations/base/tests.go`、`tests/test_utils.go`、`models/unittest/unit_tests.go` の 3 ファイルだけ。
- 3 つの sink 関数の呼び元:
  - `deleteDB`: `PrepareTestEnv`(`tests.go:139,153`。`_test.go` から使われる)だけ。
  - `tests.InitTest`: `tests/integration/integration_test.go:90` だけ。
  - `DumpQueryResult`: 定義のほかに呼び元なし。
- `from_entry` の finding に、sink がテストヘルパーのものは 0 件。
- source の `getRequestScheme` と、path の入口 `routers/install.SubmitInstall`(`install.Routes()` の `r.Post("/", ..., SubmitInstall)` で登録されたハンドラ。`cmd/web.go:150` から呼ばれる)は reach にある(同じ関数が `from_entry` の finding の source と path に入っているため)。つまり Go では、ハンドラ登録の形(`web.Route`)は reach に入っている。
- 見つかった形は「テスト用ヘルパーが sink で、呼び元がテストだけ」の 17/17。ハンドラ登録が切れた件、interface の dyn 呼びが切れた件、`init`、goroutine、reflect、`go:generate`、別の `main` は 0 件。
- 読み(推定): この `not_from_entry` は、ラベルとして正しい(entry から届かないテストヘルパーの sink)と読める。
- **未確認**: `reach` の集合そのもの。Gitea 分のグラフファイルが無く、`rivus graph` を使えなかった。「3 つの sink 関数が `reach` に無い」は、呼び元がテストだけ、という読みからの推定。
- **未確認**: `DumpQueryResult` の path の後半(`db.InitEngine`、`setting.DBConnStr` など)が `reach` にあるか。

## 関連

- Gitea(Go)の 17 件は、Rust の形(ハンドラ登録が切れる)とは別で、テスト用ヘルパーが sink の形(上の節)。この課題の対象は Rust のまま
- ADR-052(`not_from_entry`)
- ADR-058(Rust の parseable で、finding 159 件の 100% が `not_from_entry` だった過去の記録。dyn の候補が空だったことと、環境 `alloc` の closure の 2 点を直した)
- ADR-073(`from_framework_callback`。一部は ADR-125 に覆われた)、ADR-125
- 課題 #209(Dart: 登録表から引いて呼ぶ関数がグラフの edge にならない)
- `docs/issues/model-actix_web-open-points.md`、課題 #149(actix に触れる)

## 進め方

原因は、この課題では決めていない(上の BFS は事実の記録)。順は次のとおり。

1. 先に、zoo で到達性を測る仕組み。今の zoo-rust は entry を持たないので、`main` を持つ別のアプリ `bench/zoo-rust-app` を足した(`TestZooRustReach`。rustypaste と同じ形で、`not_from_entry` になることを再現する。ハンドラの `<X as HttpServiceFactory>::register` を指す edge が無く、trait が再 export の形のとき)。
   `zoo-rust-app` の期待は「`not_from_entry` でない」で、修正の形に依らない。直ったら `@miss(not_from_entry)` を外す。
2. 直す形は、その次。エンジンの変更になる見込みで、設計者への相談の候補。
