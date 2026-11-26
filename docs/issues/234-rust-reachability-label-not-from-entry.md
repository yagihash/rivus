# #234 Rust: 実対象で、到達性のラベルが信用できない(finding が全部 `not_from_entry`)

- 状態: **open**
- 優先度: 中
- 種類: 取りこぼしに近い(`reachability` で絞り込む利用者から見ると、finding が出ていないのと同じ)
- 言語: Rust
- ROADMAP の節: B / C
- 守るもの: zoo-rust-app(`bench/zoo-rust-app`、`TestZooRustReach`。再現するセルは `// from_entry @miss(not_from_entry)` の 3 つ)
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

entry は `curated::main` の 1 つで、`main` に callee が 0 個。own 510 のうち、main から届くのは main 自身の 1 つ(粗い BFS)。281 件の source は、`curated::...` の別の関数。

PR #620(`feat/rust-trait-identity-by-def-path`。extractor が trait の同一性を「クレート名 + 定義パス」で比べる)の測定(2026-11-27)で、**281 件の `not_from_entry` は、#620 で変わらなかった**(graph-check で finding の JSON 全体が `same`、`reachability` も同じ)。この対象の `not_from_entry` は別の原因で、`curated::main` の IR に callee が 0 個(上の BFS のとおり)。#620 では直らない。**次に調べる対象**。この課題は close にしない。

rustypaste は、#620 で `from_entry` になった(12 件)。ただし #620 は未マージで、これは「#620 がマージされたら」の条件つき。

ヘッダの「守るもの」の行(`@miss` の 3 つ)は、#620 がマージされたら直す(#620 の中で `@miss` を外したため。この PR では直さない)。

同じ測定で、dyn 呼びの candidates が約 9 倍に増えた。課題 #236。

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
