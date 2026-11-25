# 組み込みモデルを作る順番の一覧

状態: 決定(ユーザーと相談役。2026-11-23)。クラウドの定期実行 `rivus-model-author` が、モデルを作る順番として読む。手順は [MODEL-RUNBOOK.md](MODEL-RUNBOOK.md)、方針は ADR-096、ADR-097。

候補に入れる基準は公開の指標。順番はメンテナが決める。

**この一覧の案(境界の案)は、コードを読まずに書いたもの。モデルを作るときに、公開のソースを読んで、案と違う判断をしてよい(その理由を、PR の本文に書く)。**

パッケージ単位の宣言(段階 2)の最初の適用先は、`dart-http`、Dart と Go の Google Cloud。`bench/go-s3` は、仕組みの効果を示すベンチで、AWS SDK の組み込みモデルを本格的に作るのは後回し。

**Rust の行の注意(2 行)。** rivus の Rust extractor は、読むときに `build.rs` と proc macro を実行する(隔離の下でだけ許される)。隔離は、macOS(Seatbelt)と Linux(Landlock + seccomp。クラウドのセッションでも動く: ADR-128)にある。隔離が作れない環境では、Rust は「モデルは書くが、extractor を使う確認はしない」にして、取得したクレートのコードを、隔離なしで実行しない(線の詳細は MODEL-RUNBOOK.md の 0 節)。

## 0. 取り方(セッションが最初に読む)

1. この一覧の 1 節の表を、上から見る。**順番が正**である。[DART-MODEL-CANDIDATES.md](DART-MODEL-CANDIDATES.md) は調査の記録で、順番を決めるのは、この一覧。
2. 次の 2 つのどちらにも当てはまらない、最初の 1 行を取る(どちらかに当てはまると「取られている」)。
   - モデルのファイルが main にある: `ls internal/model/builtin/` に `<言語>-<パッケージ>*.yaml`。(作業の種類は、この列でなく、このファイルがあるかで決める(MODEL-RUNBOOK.md の 1 節)。一覧が「広げる」でも、ファイルがなければ新規として扱う。ファイルがあり、種類が「広げる」の行は、このファイルの有無でなく、PR の題(`のモデルを広げる`)だけで取られたかを判定する)
   - そのパッケージの PR が、開いている / マージ済み: `gh pr list --state all --search "<パッケージ> in:title" --limit 20`。題の形は `feat(model): <言語> <パッケージ> の組み込みモデル`(広げるときは `feat(model): <言語> <パッケージ> のモデルを広げる`、例から昇格するときは `feat(model): <言語> <パッケージ> の組み込みモデル(例から昇格)`)。MODEL-RUNBOOK.md の 1 節の例外(閉じてある PR、24 時間以上更新のないドラフトは、取られていない)に従う。
3. 済みかどうかを、この一覧に書き込まない(PR のたびに同じファイルをさわらない)。
4. 「新規 / 広げる / 例から昇格」の列で、作業の種類が決まる(MODEL-RUNBOOK.md の 1 節に、広げる、例から昇格の手順がある)。
5. **残りが 5 つ以下になったとき**(上の判定で、取れる行が 5 つ以下)は、モデルを作る代わりに、「次の候補を 10 個、公開の指標から提案する PR」を立てて終わる。提案は、この一覧の形(順番、言語、パッケージ、境界の案、指標、種類、理由)で、公開の指標(Dart は pub.dev、Go は pkg.go.dev、Rust は crates.io)だけで選ぶ。
6. 並びは、Dart、Go、Rust の順に 1 つずつを基本にする(Rust は 3 行に 1 行)。最初の 3 行(shelf_router、gin、actix-web)は検証用。そのあとの最初の本番は、Dart の dio と、Go の Google Cloud の残り(Storage、Pub/Sub、Firestore、Datastore。`resource_name` の追加の課題 #142 も一緒に)。そのあとに、GitHub のクライアント、Google Cloud と Firebase、DB のライブラリ、監視とエラー報告の順で前に出した。AWS は、3 言語とも後ろに下げた。種類の都合で、交互の並びを崩した所がある。
7. **モデルにする範囲の指針。** 監視、トレース、エラー報告、ログのクライアントは、外への送信の sink と、「中を読まない」効果(多くの関数から呼ばれて値が合流しやすい)が主。GitHub などの API のクライアントは、操作するリソースを決める引数(所有者、リポジトリ名、パス、参照)を `resource_name`、返ってくる本文やファイルの中身を source にする。

## 1. 順番の表

| 順番 | 言語 | パッケージ | 新規 / 広げる / 例から昇格 | モデルにする API の境界の案 | 指標(2026-11-23 取得) | 種類 | 理由(公開の指標だけ) |
|---|---|---|---|---|---|---|---|
| 1 | Dart | shelf_router | 新規 | source: `Request.params`、`Router.get/post/...` に登録したハンドラが入口 | 30 日 DL 637k、325 likes。最終公開 2026-10-16。Dart のみ | Web フレームワーク | 検証用。shelf の上の層で、API の境界が明確 |
| 2 | Go | gin | 例から昇格 | source: `*gin.Context` の `Query/Param/PostForm/ShouldBind*/GetHeader`、sink: `Context.File`/`Redirect` など(`examples/rules/go-gin.yaml` の 103 行が材料) | imported by 80.8k モジュール / 191.4k パッケージ。v1.12.0(2026-02-28) | Web フレームワーク | 検証用。Go の Web フレームワークで imported by が最多 |
| 3 | Rust | actix-web | 例から昇格 | source: `HttpRequest`、`web::Query/Path/Json/Form/Bytes`。`examples/rules/rust-actix-web.yaml` の 58 行と、`bench/parseable/rules.yaml` の actix-web の 14 行が材料 | 直近 90 日 13.9M / 全期間 92.1M。4.20.0(2026-11-17) | Web フレームワーク | 検証用。Web フレームワークの中でも全期間 92.1M |
| 4 | Dart | dio | 新規 | sink: `Dio.get/post/request/download` の URL とデータ。source: `Response.data`。`Interceptor`、`HttpClientAdapter` が合流点(**`Response.data` は、モデルの側が、`dart-http.yaml` の前例に合わせて source にしないと判断した。#202**) | 30 日 DL 5.6M、8.4k likes。最終公開 2026-09-04。Dart のみ | HTTP クライアント | 最初の本番。Dart の HTTP で DL が 2 番目(http は進行中) |
| 5 | Go | cloud.google.com/go/storage | 新規 | resource_name: `Bucket/Object` の名前。source: `Reader.Read`、sink: `Writer.Write` | imported by 5.7k モジュール / 6.6k パッケージ。v1.71.0(2026-11-16) | クラウド(Google Cloud) | Google Cloud の残りで最多 |
| 6 | Rust | octocrab | 新規 | resource_name: owner、repo、path、ref。source: 戻り値の本文とファイル | 直近 90 日 3.5M / 全期間 20.3M。0.54.9(2026-10-31) | API クライアント(GitHub) | GitHub のクライアントで最多 |
| 7 | Dart | github(package:github) | 新規 | resource_name: `RepositorySlug`、パス、ref。source: 戻り値の本文とファイル | 30 日 DL 295k、165 likes。最終公開 2026-07-28。Dart のみ | API クライアント(GitHub) | GitHub のクライアントで最多 |
| 8 | Go | google/go-github | 新規 | resource_name: `Repositories.*`、`Issues.*` の owner、repo、path、ref。source: 戻り値の本文とファイルの中身 | 最新の major v93 は 0(新しい major に移行中)。v84 は 73 モジュール。major の合計は未取得。v93 は 2026-10-15 | API クライアント(GitHub) | GitHub の API のクライアントで事実上の標準(major ごとに数が分かれる) |
| 9 | Dart | googleapis_auth | 新規 | source: `clientViaServiceAccount` などの `AuthClient`、`AccessCredentials`。sink: 資格情報の JSON | 30 日 DL 2.3M、244 likes。最終公開 2026-11-07。Dart のみ | 認証 / クラウド(Google Cloud) | Google Cloud の前提 |
| 10 | Go | shurcooL/githubv4 | 新規 | sink: `Query/Mutate` の変数。resource_name: owner、name、path | imported by 684 モジュール / 931 パッケージ。最新(proxy で未取得) | API クライアント(GitHub) | GraphQL 側。go-github の後 |
| 11 | Rust | google-cloud-storage | 新規 | resource_name: bucket、object。source: ダウンロードの本文、sink: アップロード | 直近 90 日 13.1M / 全期間 31.1M。1.22.0(2026-11-11) | クラウド(Google Cloud) | Google Cloud のクレートで最多 |
| 12 | Dart | firebase_messaging | 新規 | source: `onMessage`、`getInitialMessage` の `RemoteMessage.data` | 30 日 DL 4.7M、4.0k likes。最終公開 2026-11-16。Flutter 依存 | クラウド(Firebase) | Firebase で DL が最多 |
| 13 | Go | cloud.google.com/go/pubsub | 新規 | sink: `Topic.Publish` の `Message`。source: `Subscription.Receive` のコールバックの `Message` | imported by 2.1k モジュール / 3.0k パッケージ。v1.52.0(2026-11-04) | クラウド(Google Cloud) | 2 番目 |
| 14 | Rust | axum | 広げる | source: `Query/Path/Json/Form/Multipart/State/HeaderMap`(既存の 9 行を残す)。sink: `Redirect`、`Response` | 直近 90 日 175.0M / 全期間 613.2M。0.8.9(2026-04-14) | Web フレームワーク | Web フレームワークで直近のダウンロードが最多 |
| 15 | Dart | firebase_auth | 新規 | source: `getIdToken`、`currentUser`。sink: `signIn*` の資格情報 | 30 日 DL 2.4M、4.3k likes。最終公開 2026-11-16。Flutter 依存 | 認証 / クラウド(Firebase) | 2 番目 |
| 16 | Go | cloud.google.com/go/firestore | 新規 | sink: `Collection/Doc` のパス、`Where` の値。source: `DocumentSnapshot.Data` | imported by 2.0k モジュール / 2.4k パッケージ。v1.29.2(2026-10-29) | クラウド(Google Cloud) | 3 番目 |
| 17 | Rust | rusqlite | 広げる | sink: `Connection::execute/prepare/query_row` の SQL。source: `Row::get` | 直近 90 日 66.7M / 全期間 163.3M。0.43.0(2026-11-15) | DB | DB のドライバで直近が最多 |
| 18 | Dart | sqlite3 | 新規 | sink: `Database.execute/select/prepare` の SQL。source: `ResultSet` | 30 日 DL 3.9M、467 likes。最終公開 2026-11-08。Dart のみ | DB | DB のドライバで DL が最多 |
| 19 | Go | cloud.google.com/go/datastore | 新規 | sink: `Query.Filter`、`Get/Put` の Key。source: 戻り値 | imported by 0.9k モジュール / 1.3k パッケージ。v1.28.1(2026-10-28) | クラウド(Google Cloud) | 4 番目 |
| 20 | Rust | sqlx | 広げる | sink: `query/query_as/query_scalar`(既存の 3 行)に加えて、`Executor::execute/fetch*`、`QueryBuilder::push`。source: `Row::get` | 直近 90 日 61.4M / 全期間 196.5M。0.9.1(2026-11-18) | DB | 2 番目 |
| 21 | Dart | cloud_firestore | 新規 | source: `DocumentSnapshot.data`。sink: `set/add/update`、`where` の値 | 30 日 DL 1.8M、3.8k likes。最終公開 2026-11-16。Flutter 依存 | DB / クラウド(Firebase) | 3 番目 |
| 22 | Go | gorm.io/gorm | 新規 | sink: `DB.Raw/Exec/Where/Order` の文字列の引数。source: `Find/First/Scan` の結果 | imported by 44.7k モジュール / 86.9k パッケージ。v1.31.2(2026-06-22) | DB / ORM | DB 層で imported by が最多 |
| 23 | Rust | sentry | 新規 | sink: `capture_message/capture_error/configure_scope` の値 | 直近 90 日 20.7M / 全期間 69.1M。0.49.6(2026-11-16) | エラー報告 | エラー報告で最多 |
| 24 | Dart | sentry | 新規 | sink: `Sentry.captureException/captureMessage/addBreadcrumb` の値(外への送信) | 30 日 DL 2.3M、583 likes。最終公開 2026-11-19。Dart のみ(`sentry_flutter` は Flutter 依存、DL 2.1M) | エラー報告 | エラー報告で最多。送信の sink と中を読まない効果が主 |
| 25 | Go | getsentry/sentry-go | 新規 | sink: `CaptureException/CaptureMessage/Scope.SetTag` の値 | imported by 2.6k モジュール / 3.6k パッケージ。v0.50.0(2026-10-08) | エラー報告 | エラー報告で最多 |
| 26 | Rust | opentelemetry | 新規 | sink: `Tracer::start`、`Span::set_attribute` の値(外への送信) | 直近 90 日 94.9M / 全期間 341.9M。0.33.1(2026-10-09) | 監視 / トレース | 監視で直近のダウンロードが最多 |
| 27 | Dart | opentelemetry | 新規 | sink: `Tracer.startSpan` の名前と属性(外への送信) | 30 日 DL 270k、47 likes。最終公開 2026-10-14。Dart のみ | 監視 / トレース | 指標で確認: DL 270k で、この種類で最多 |
| 28 | Go | go.opentelemetry.io/otel | 新規 | sink: `Tracer.Start` の名前、`Span.SetAttributes/RecordError` の値。外への送信が主 | imported by 13.5k モジュール / 16.1k パッケージ。v1.47.0(2026-10-02) | 監視 / トレース | トレースで最多 |
| 29 | Go | DataDog/datadog-go statsd | 新規 | sink: `Client.Gauge/Count/Incr` のタグ、名前 | imported by 204 モジュール / 276 パッケージ。(パッケージ `statsd` で取得) | 監視 | Datadog の公式クライアント。値は低いが、指標で他の候補を特定できなかった |
| 30 | Dart | drift | 新規 | sink: `customSelect/customStatement/customUpdate` の SQL。source: `QueryRow.data` | 30 日 DL 2.1M、2.5k likes。最終公開 2026-11-08。Dart(ドキュメントの上では Flutter でも使う) | DB / ORM | 2 番目 |
| 31 | Go | jmoiron/sqlx | 新規 | sink: `Get/Select/Query*/Exec/NamedExec` の SQL。`database/sql` の既存のモデルを継ぐ | imported by 17.3k モジュール / 27.2k パッケージ。v1.4.0(2024-04-15) | DB | 既存の `database/sql` の行と重複を避けて広げる |
| 32 | Rust | warp | 広げる | source: `warp::query`、`path::param`、`body::json`(既存の 1 行) | 直近 90 日 6.9M / 全期間 53.6M。0.4.4(2026-11-13) | Web フレームワーク | Web フレームワークの 3 番目 |
| 33 | Dart | firebase_storage | 新規 | resource_name: `ref(path)`。sink: `putData/putFile`。source: `getData` | 30 日 DL 981k、1.8k likes。最終公開 2026-11-16。Flutter 依存 | クラウド(Firebase) | 4 番目 |
| 34 | Go | jackc/pgx/v5 | 新規 | sink: `Conn/Pool` の `Exec/Query/QueryRow/CopyFrom` の SQL。source: `Rows.Scan` | imported by 12.1k モジュール / 12.1k パッケージ。v5.13.0(2026-11-11) | DB | PostgreSQL のドライバで imported by が最多 |
| 35 | Rust | rocket | 広げる | source: `FromRequest`、`Form/Json/Query` の入力(既存の 2 行) | 直近 90 日 2.2M / 全期間 15.1M。0.6.0-rc.1(2026-11-21) | Web フレームワーク | Web フレームワークの 4 番目 |
| 36 | Dart | cloud_functions | 新規 | sink: `HttpsCallable.call` の引数。source: `HttpsCallableResult.data` | 30 日 DL 809k、513 likes。最終公開 2026-11-16。Flutter 依存 | クラウド(Firebase) | 5 番目 |
| 37 | Go | gorilla/mux | 新規 | source: `mux.Vars(r)`。ハンドラは `net/http` の既存のモデルで入る | imported by 65.2k モジュール / 99.5k パッケージ。v1.8.1(2023-10-18) | ルーター | imported by が多い。最終公開は古い(保守の状態は着手時に確認) |
| 38 | Rust | reqwest | 広げる | sink: `Client::get/post/request`、`RequestBuilder::send/json/body` の URL。source: `Response::text/json/bytes`(既存の 6 行を残す) | 直近 90 日 280.3M / 全期間 944.4M。0.13.5(2026-09-08) | HTTP クライアント | HTTP クライアントで最多 |
| 39 | Dart | dart_firebase_admin | 新規 | source: `Auth.verifyIdToken` の `DecodedIdToken`。sink: `Firestore` の操作、`Messaging.send` | 30 日 DL 17.5k、61 likes。最終公開 2025-03-21。Dart のみ | クラウド(Firebase) | サーバ側の Firebase。DL は小さい |
| 40 | Go | go-chi/chi/v5 | 新規 | source: `chi.URLParam`、`chi.RouteContext` | imported by 17.3k モジュール / 17.3k パッケージ。v5.5.0(2026-10-14) | ルーター | ルーターで imported by が 2 番目 |
| 41 | Rust | hyper | 広げる | source: `Request<Incoming>` の本文(既存の 1 行)。sink: `client::conn` の送信 | 直近 90 日 301.3M / 全期間 1163.3M。1.12.0(2026-10-06) | HTTP | HTTP の基盤で全期間が最多 |
| 42 | Dart | postgres | 新規 | sink: `Connection.execute`、`Session.execute` の SQL。source: `Result` の行 | 30 日 DL 697k、416 likes。最終公開 2026-11-22。Dart のみ | DB | 純 Dart のドライバで、中を読むコストが大きい |
| 43 | Go | labstack/echo/v4 | 新規 | source: `echo.Context` の `QueryParam/Param/FormValue/Bind/Cookie`、sink: `Redirect/File` | imported by 14.8k モジュール / 32.2k パッケージ。v4.16.6(2026-11-20) | Web フレームワーク | Web フレームワークで packages が 3 番目(32.2k) |
| 44 | Rust | tokio-postgres | 新規 | sink: `Client::query/execute/prepare` の SQL。source: `Row::get` | 直近 90 日 29.1M / 全期間 91.5M。0.7.18(2026-06-12) | DB | 3 番目 |
| 45 | Dart | shelf | 広げる | sink: `Response` のボディ、`Response.found` などの URL。既存の 14 行を残す。`Handler`、`Middleware` の合流は、行にしない(本体が数行で読まれ、行にすると包んだハンドラへの呼び出しを切って流れを失う) | 30 日 DL 16.6M、1.0k likes。最終公開 2026-10-16。Dart のみ | Web フレームワーク | サーバ側で DL が最多 |
| 46 | Go | gofiber/fiber/v2 | 新規 | source: `*fiber.Ctx` の `Query/Params/FormValue/Body/BodyParser` | imported by 13.3k モジュール / 30.9k パッケージ。v2.52.16(2026-10-23) | Web フレームワーク | Web フレームワークで packages が 4 番目 |
| 47 | Rust | diesel | 新規 | sink: `sql_query`、`sql` の文字列。source: ロードした行 | 直近 90 日 13.3M / 全期間 47.2M。2.4.0(2026-11-13) | DB / ORM | ORM で最多 |
| 48 | Dart | shelf_web_socket | 新規 | source: `webSocketHandler` のコールバックのメッセージ | 30 日 DL 14.3M、158 likes。最終公開 2025-01-30。Dart のみ | Web フレームワーク | DL が多い |
| 49 | Go | go.mongodb.org/mongo-driver/mongo | 新規 | sink: `Collection.Find/Aggregate/UpdateOne` のフィルタ(`bson.M` の値)。source: `Cursor.Decode` | imported by 22.0k モジュール / 32.6k パッケージ。(パッケージのパスで取得。module の最新は未取得) | DB | NoSQL で imported by が最多 |
| 50 | Rust | sea-orm | 新規 | sink: `Statement::from_string`、`Condition` の生の文字列。source: ロードした行 | 直近 90 日 6.6M / 全期間 30.3M。2.0.4(2026-09-27) | DB / ORM | ORM の 2 番目 |
| 51 | Dart | dart_frog | 新規 | source: `RequestContext.request`、`Request.json/body`。`onRequest` が入口 | 30 日 DL 245k、864 likes。最終公開 2025-11-03。Dart のみ | Web フレームワーク | サーバ側のフレームワーク |
| 52 | Go | redis/go-redis/v9 | 新規 | sink: `Client.Do/Eval/Set/...` のキーと引数。source: 戻り値の `Cmd.Result` | imported by 16.7k モジュール / 17.4k パッケージ。v9.23.0(2026-10-05) | DB / キャッシュ | Redis のクライアントで最多 |
| 53 | Rust | redis | 新規 | sink: `Commands`、`cmd(..).arg(..)` のキーと引数。source: 戻り値 | 直近 90 日 38.8M / 全期間 134.7M。2.0.0-rc.3(2026-11-14) | DB / キャッシュ | Redis のクライアントで最多 |
| 54 | Dart | redis | 新規 | sink: `Command.send_object` のコマンドと引数。source: 戻り値 | 30 日 DL 280k、146 likes。最終公開 2026-03-16。Dart のみ | DB / キャッシュ | Redis のクライアントで最多 |
| 55 | Go | google.golang.org/grpc | 広げる | クライアント側: `ClientConn.Invoke/NewStream`、`grpc.Dial/NewClient` の target。既存の 8 行を残して広げる | imported by 87.1k モジュール / 270.1k パッケージ。v1.85.0(2026-10-27) | RPC | imported by が全体で最多。サーバ側は組み込み済み |
| 56 | Rust | mongodb | 新規 | sink: `Collection::find/aggregate/update_one` のフィルタ。source: `Cursor` | 直近 90 日 5.2M / 全期間 21.4M。3.10.2(2026-11-19) | DB | NoSQL で最多 |
| 57 | Dart | mongo_dart | 新規 | sink: `DbCollection.find/insert/update` の selector。source: 戻り値の `Map` | 30 日 DL 101k、506 likes。最終公開 2026-10-11。Dart のみ | DB | NoSQL で最多 |
| 58 | Go | google.golang.org/protobuf | 新規 | propagator: `proto.Marshal/Unmarshal/Clone`、`protojson`。メッセージのフィールドの流れ | imported by 23.1k モジュール / 39.8k パッケージ。(パッケージ `proto` で取得) | シリアライズ / RPC | gRPC の入力のメッセージが通る |
| 59 | Rust | prometheus | 新規 | sink: `Counter/Gauge/Histogram` のラベル値 | 直近 90 日 31.8M / 全期間 169.7M。0.14.0(2025-03-27) | 監視 | 2 番目(最終公開は古い) |
| 60 | Dart | mysql_client | 新規 | sink: `MySQLConnection.execute`、`prepare` の SQL。source: 結果の行 | 30 日 DL 14k、186 likes。最終公開 2022-10-04。Dart のみ | DB | MySQL で DL が最多(最終公開が古い。着手時に保守の状態を確認) |
| 61 | Go | grpc-ecosystem/grpc-gateway/v2 | 新規 | source: `runtime.PathParams` と、HTTP から gRPC に変わる入力。`runtime.ServeMux` | imported by 5.5k モジュール / 12.1k パッケージ。(パッケージ `runtime` で取得) | RPC / Web | gRPC と HTTP の合流 |
| 62 | Rust | metrics | 新規 | sink: `counter!/gauge!/histogram!` のラベル | 直近 90 日 31.4M / 全期間 133.0M。0.24.6(2026-05-13) | 監視 | 3 番目 |
| 63 | Dart | protobuf | 新規 | propagator: `GeneratedMessage.fromBuffer/fromJson/writeToBuffer`、生成コードのフィールド | 30 日 DL 3.2M、256 likes。最終公開 2026-09-11。Dart のみ | RPC / シリアライズ | gRPC の入力のメッセージが通る |
| 64 | Go | connectrpc.com/connect | 新規 | source: ハンドラの `*connect.Request[T]` の `Msg`、`Header()`。sink: クライアントの呼び出し | imported by 2.2k モジュール / 7.0k パッケージ。v1.21.1(2026-10-14) | RPC | 新しい RPC のライブラリで packages が 7.0k |
| 65 | Rust | jsonwebtoken | 新規 | source: `decode` の `claims`(検証後も外から来たデータ)。sink: `encode` の鍵 | 直近 90 日 78.7M / 全期間 253.6M。12.0.0(2026-11-13) | 認証 | JWT で最多 |
| 66 | Dart | grpc(クライアント側) | 広げる | sink: `ClientChannel.new` のホストと、`Client` の呼び出し(サーバ側と `ClientChannel.new` は既存の 5 行) | 30 日 DL 653k、465 likes。最終公開 2026-10-15。Dart のみ | RPC | サーバ側の拡張 |
| 67 | Go | go-resty/resty/v2 | 新規 | sink: `Request.Get/Post/SetBody/SetQueryParam` の URL と本文。source: `Response.Body/String` | imported by 6.9k モジュール / 8.2k パッケージ。v2.17.2(2026-02-14) | HTTP クライアント | HTTP クライアントのライブラリで最多 |
| 68 | Rust | askama | 新規 | sink: テンプレートのコンテキストと `render`(エスケープの扱い、safe フィルタ) | 直近 90 日 18.7M / 全期間 63.8M。0.16.1(2026-09-04) | テンプレート | テンプレートで直近が最多 |
| 69 | Dart | supabase | 新規 | sink: `from(t).select/insert/eq` のフィルタ、`rpc`、`auth.signIn*`。source: 戻り値の行 | 30 日 DL 1.3M、616 likes。最終公開 2026-10-12。Dart のみ(`supabase_flutter` は Flutter 依存、DL 1.3M) | BaaS | BaaS で DL が最多 |
| 70 | Go | prometheus/client_golang | 新規 | sink: `Counter/Gauge/Histogram` のラベル値(外への送信)。`promhttp.Handler` | imported by 38.4k モジュール / 71.7k パッケージ。(パッケージ `prometheus` で取得) | 監視 | 監視の中で imported by が最多。主に外への送信の sink と中を読まない効果 |
| 71 | Rust | tera | 新規 | sink: `Tera::render` と `Context::insert`、`safe` フィルタ | 直近 90 日 8.2M / 全期間 44.4M。2.4.0(2026-09-11) | テンプレート | 2 番目 |
| 72 | Dart | datadog_flutter_plugin | 新規 | sink: `DatadogSdk` のログ、トレース、RUM の属性(外への送信) | 30 日 DL 410k、41 likes。最終公開 2026-10-06。Flutter 依存 | 監視 | Datadog の公式 |
| 73 | Go | DataDog/dd-trace-go | 新規 | sink: `tracer.StartSpan` と `Span.SetTag` の値 | imported by 186 モジュール / 245 パッケージ。(パッケージ `ddtrace/tracer` で取得) | トレース | Datadog の公式。値は低い |
| 74 | Rust | lettre | 新規 | sink: `Message::builder` の宛先、件名、本文(ヘッダの注入)、`Transport::send` | 直近 90 日 9.6M / 全期間 25.4M。0.11.24(2026-11-17) | メール | メールで最多 |
| 75 | Dart | dart_jsonwebtoken | 新規 | source: `JWT.verify` の `payload`(検証後も外から来たデータ)。sink: `JWT.sign` の鍵 | 30 日 DL 2.2M、360 likes。最終公開 2026-04-19。Dart のみ | 認証 | JWT で最多 |
| 76 | Go | sirupsen/logrus | 新規 | sink: `Info/Warn/Error/WithField(s)` のメッセージと値(ログへの注入)。`Entry` が流れを持つ | imported by 69.2k モジュール / 240.0k パッケージ。v1.10.2(2026-08-25) | ロギング | ロギングで packages が最多 |
| 77 | Rust | elasticsearch | 新規 | sink: `Search::body`、`Index::body` | 直近 90 日 2.8M / 全期間 25.4M。9.1.0-alpha.1(2025-08-08) | 検索 | 検索の公式。最終公開が古く、alpha のみ。着手時に確認 |
| 78 | Dart | oauth2 | 新規 | source: `Credentials`、`Client` の応答。sink: `authorizationUrl`。`http.BaseClient` を継承 | 30 日 DL 611k、382 likes。最終公開 2025-10-20。Dart のみ | 認証 | OAuth2 で最多 |
| 79 | Go | go.uber.org/zap | 新規 | sink: `Logger.Info/Error`、`zap.String/Any` のフィールド。`Sugar` の `Infof` | imported by 46.8k モジュール / 119.7k パッケージ。v1.28.0(2026-04-28) | ロギング | 2 番目 |
| 80 | Rust | async-openai | 新規 | sink: `Chat::create` のプロンプト。source: 応答の `choices` | 直近 90 日 3.7M / 全期間 11.2M。0.43.0(2026-11-01) | AI の API | AI の API で最多 |
| 81 | Dart | logging | 新規 | sink: `Logger.info/warning/severe` のメッセージ。`onRecord` と `LogRecord` が合流点 | 30 日 DL 14.0M、1.0k likes。最終公開 2024-10-17。Dart のみ | ロギング | DL が全体の上位 |
| 82 | Go | rs/zerolog | 新規 | sink: `Event.Str/Msg/Err` の値 | imported by 18.4k モジュール / 30.5k パッケージ。v1.35.1(2026-04-20) | ロギング | 3 番目 |
| 83 | Rust | tracing | 広げる | sink: `info!/warn!/error!` の値とフィールド(既存の 1 行を広げる)。`Subscriber` が外に出す | 直近 90 日 290.0M / 全期間 1083.3M。0.1.44(2025-12-18) | ロギング | ログで全期間が最多 |
| 84 | Dart | web_socket_channel | 新規 | source: `WebSocketChannel.stream`。sink: `sink.add`、`connect` の URL | 30 日 DL 13.9M、1.6k likes。最終公開 2025-04-17。Dart のみ | ネットワーク | DL が多い |
| 85 | Go | golang.org/x/oauth2 | 新規 | source: `Config.Exchange` の `Token`。sink: `AuthCodeURL` の state、`Client` の HTTP の呼び出し | imported by 28.6k モジュール / 51.4k パッケージ。v0.38.0(2026-11-06) | 認証 | 認証で imported by が最多 |
| 86 | Rust | clap | 新規 | source: `ArgMatches::get_one/get_many`、`Parser` の派生のフィールド(コマンドラインの入力) | 直近 90 日 300.7M / 全期間 1371.5M。4.6.9(2026-11-20) | CLI | 全期間が全体で最多 |
| 87 | Dart | process_run | 新規 | sink: `run`、`runExecutableArguments` のコマンドと引数 | 30 日 DL 258k、354 likes。最終公開 2026-11-16。Dart のみ | プロセス | コマンドの sink が明確 |
| 88 | Go | golang-jwt/jwt/v5 | 新規 | source: `Parse/ParseWithClaims` の `Claims`(検証後も外から来たデータ)、sanitizer の候補: 検証の成功。sink: `Token.SignedString` の鍵 | imported by 19.2k モジュール / 18.0k パッケージ。v5.3.1(2026-01-28) | 認証 | JWT で最多 |
| 89 | Rust | tokio | 広げる | source/sink: `fs::read/write`、`net::TcpStream`、`process::Command`、`io::AsyncRead*`(既存の 33 行を残す) | 直近 90 日 324.4M / 全期間 1229.6M。1.54.1(2026-10-21) | 非同期の基盤 | 全体で最多 |
| 90 | Dart | file | 新規 | `FileSystem` のメソッドは `dart:io` の `File`/`Directory` と同じ境界。`LocalFileSystem`、`MemoryFileSystem` の実装 | 30 日 DL 14.1M、299 likes。最終公開 2024-10-08。Dart のみ | ファイル | `FileSystem` は実装の多いインターフェース |
| 91 | Go | coreos/go-oidc/v3 | 新規 | source: `IDToken.Claims`、`Verifier.Verify` の結果。sink: `NewProvider` の issuer の URL | imported by 2.1k モジュール / 1.3k パッケージ。(パッケージ `oidc` で取得) | 認証 | 認証のサービスのクライアントで特定のサービスに依らない標準。Auth0 の `go-auth0` は 7 モジュールで入れない |
| 92 | Rust | async-stripe | 新規 | sink: 決済の Params。source: 戻り値、Webhook の本文 | 直近 90 日 1.0M / 全期間 6.0M。1.0.0-rc.10(2026-10-26) | 決済 | 決済で最多(公式の `stripe-rs` は 2015 年のまま。直近 12) |
| 93 | Dart | async | 新規 | propagator: `StreamGroup`、`StreamQueue`、`FutureGroup`、`AsyncCache`、`CancelableOperation` | 30 日 DL 16.9M、638 likes。最終公開 2026-03-26。Dart のみ | 非同期の基盤 | DL が全体の上位。呼び戻しと Future の合流が多い |
| 94 | Go | slack-go/slack | 新規 | sink: `PostMessage` の本文とチャンネル、`OpenView`。source: イベントの `MessageEvent.Text` | imported by 2.2k モジュール / 3.0k パッケージ。v0.36.0(2026-11-14) | チャット | チャットで最多 |
| 95 | Rust | slack-morphism | 新規 | sink: 投稿の本文とチャンネル。source: イベントのテキスト | 直近 90 日 1.9M / 全期間 6.1M。2.34.0(2026-11-16) | チャット | チャットで最多 |
| 96 | Dart | http2 | 新規 | source: `incomingStreams` の `StreamMessage`。sink: `makeRequest` のヘッダ | 30 日 DL 967k、97 likes。最終公開 2026-11-16。Dart のみ | HTTP | `grpc` の下の層 |
| 97 | Go | sendgrid/sendgrid-go | 新規 | sink: `Send` の宛先、件名、本文(メールヘッダの注入) | imported by 1.1k モジュール / 2.5k パッケージ。v3.16.1+incompatible(2025-05-29) | メール | メールで packages が 2.5k |
| 98 | Rust | datadog-api-client | 新規 | sink: メトリクス、ログ、イベントの送信の値 | 直近 90 日 1.6M / 全期間 3.5M。0.38.0(2026-10-13) | 監視 | Datadog の公式のクライアント(監視の中では 1 桁小さい) |
| 99 | Dart | flutter_stripe | 新規 | sink: `Stripe.instance.confirmPayment` の `clientSecret` と支払い情報。source: `PaymentIntent` | 30 日 DL 514k、1.5k likes。最終公開 2026-11-13。Flutter 依存 | 決済 | 決済で最多(Stripe の公式ではない) |
| 100 | Go | twilio/twilio-go | 新規 | sink: `Api.CreateMessage` の宛先と本文 | imported by 445 モジュール / 387 パッケージ。v1.33.1(2026-11-17) | SMS | SMS で最多 |
| 101 | Rust | gitlab | 新規 | resource_name: プロジェクト、パス、ref。source: ファイルの内容 | 直近 90 日 1.9M / 全期間 6.2M。0.1905.0(2026-10-23) | API クライアント(GitLab) | GitLab のクライアントで最多 |
| 102 | Dart | mailer | 新規 | sink: `send(Message, SmtpServer)` の宛先、件名、本文、ヘッダ | 30 日 DL 287k、450 likes。最終公開 2026-11-08。Dart のみ | メール | メールで最多 |
| 103 | Go | stripe/stripe-go | 新規 | sink: `paymentintent.New` などの Params(金額、顧客)。source: 戻り値、`webhook.ConstructEvent` の本文 | v87 のみで 31 モジュール(major ごとに分かれる。最新 v87.2.0、2026-11-19)。旧 major の合計は未取得 | 決済 | 決済のクライアント。major ごとに数が分かれ、確認が要る |
| 104 | Rust | azure_storage_blobs | 新規 | resource_name: コンテナ、ブロブ。source: ダウンロード、sink: アップロード | 直近 90 日 2.1M / 全期間 21.1M。0.21.0(2024-10-15) | クラウド(Azure) | Azure の中で最多。最終公開が古いので、着手時に保守の状態を確認 |
| 105 | Dart | openai_dart | 新規 | sink: `createChatCompletion` のプロンプト。source: 応答 | 30 日 DL 45k、139 likes。最終公開 2026-11-18。Dart のみ | AI の API | AI の API の中で最近の更新が多い |
| 106 | Go | elastic/go-elasticsearch/v9 | 新規 | sink: `Search` の本文、`Index` の文書。source: 応答の `Body` | imported by 113 モジュール / 66 パッケージ。v9.5.3(2026-11-17) | 検索 | 検索の公式クライアント。最新の major の数。旧 major は未取得 |
| 107 | Rust | aws-sdk-s3 | 新規 | resource_name: `bucket`、`key`。source: `get_object` の `body`。sink: `put_object` の `body` | 直近 90 日 33.7M / 全期間 114.1M。1.161.0(2026-11-18) | クラウド | AWS のクレートで最多(config は 49.8M) |
| 108 | Dart | google_generative_ai | 新規 | sink: `GenerativeModel.generateContent` のプロンプト。source: `text` | 30 日 DL 89k、913 likes。最終公開 2025-04-17。Dart のみ | AI の API | Google の公式(Google Cloud の範囲に近い) |
| 109 | Go | opensearch-project/opensearch-go/v4 | 新規 | sink: `Search` の本文、`Index` | imported by 93 モジュール / 56 パッケージ。v4.8.1(2026-10-09) | 検索 | OpenSearch の公式 |
| 110 | Rust | aws-config | 新規 | 設定の読み込み。`load_defaults` の資格情報の source | 直近 90 日 49.8M / 全期間 163.2M。1.13.3(2026-11-17) | クラウド | S3 の前提 |
| 111 | Dart | url_launcher | 新規 | sink: `launchUrl`、`launchUrlString` の URL | 30 日 DL 10.2M、8.2k likes。最終公開 2026-10-02。Flutter 依存 | 外部の呼び出し | Flutter 側で DL が最多 |
| 112 | Go | anthropics/anthropic-sdk-go | 新規 | sink: `Messages.New` のプロンプト(プロンプトの注入)。source: 応答の `Content` | imported by 1.0k モジュール / 0.4k パッケージ。v1.84.0(2026-10-20) | AI の API | AI の API で imported by が最多(modules) |
| 113 | Rust | aws-sdk-dynamodb | 新規 | sink: `query/scan/put_item` の式と値 | 直近 90 日 7.9M / 全期間 30.9M。1.140.0(2026-11-17) | クラウド / DB | AWS の次の候補 |
| 114 | Dart | shared_preferences | 新規 | source: `getString/get`。sink: `setString/set` | 30 日 DL 9.7M、10.6k likes。最終公開 2026-10-05。Flutter 依存 | 保存 | 2 番目 |
| 115 | Go | openai/openai-go/v3 | 新規 | sink: `Chat.Completions.New` のプロンプト。source: 応答の `Choices` | imported by 591 モジュール / 697 パッケージ。v3.105.0(2026-11-20) | AI の API | 2 番目 |
| 116 | Dart | flutter_secure_storage | 新規 | source: `read/readAll`。sink: `write` | 30 日 DL 6.6M、4.5k likes。最終公開 2026-09-16。Flutter 依存 | 保存 | 3 番目 |
| 117 | Go | Azure/azure-sdk-for-go の azblob | 新規 | resource_name: コンテナとブロブの名前。source: `DownloadStream`、sink: `UploadStream` | imported by 365 モジュール / 246 パッケージ。(パッケージ `sdk/storage/azblob/blob` で取得) | クラウド(Azure) | Azure のクライアントの候補。ほかの Azure のサービスは未取得 |
| 118 | Dart | aws_signature_v4 | 新規 | sink: `AWSSigV4Signer.sign` の要求 | 30 日 DL 192k、24 likes。最終公開 2026-07-31。Dart のみ | クラウド(AWS) | AWS の共通の層 |
| 119 | Go | spf13/cobra | 新規 | source: `Command.Flags()` の値、`Args`(コマンドラインの入力)。`RunE` が入口 | imported by 99.3k モジュール / 195.9k パッケージ。v1.10.2(2025-12-03) | CLI | imported by が全体の上位 |
| 120 | Dart | aws_client | 新規 | sink: 各サービスの要求。source: 応答 | 30 日 DL 89k、42 likes。最終公開 2026-08-10。Dart のみ。pub points 0 | クラウド(AWS) | AWS のコミュニティのクライアント。保守の状態は着手時に確認 |
| 121 | Go | spf13/viper | 新規 | source(config): `viper.Get*` の値。設定は信頼済みで、config_read の終端 | imported by 65.7k モジュール / 103.3k パッケージ。v1.21.0(2025-09-08) | 設定 | 設定の読み込みで最多。`go-config.yaml` に合わせる |
| 122 | Go | nats-io/nats.go | 新規 | source: `Subscribe` のコールバックの `Msg.Data`。sink: `Publish/Request` の subject とデータ | imported by 5.8k モジュール / 8.6k パッケージ。v1.56.0(2026-11-17) | メッセージング | メッセージングで最多 |
| 123 | Go | segmentio/kafka-go | 新規 | source: `Reader.ReadMessage` の `Message`。sink: `Writer.WriteMessages` | imported by 3.8k モジュール / 4.5k パッケージ。v0.4.51(2026-04-23) | メッセージング | 2 番目 |
| 124 | Go | gitlab.com/gitlab-org/api/client-go | 新規 | resource_name: プロジェクト、パス、ref。source: ファイルの内容 | imported by 337 モジュール / 391 パッケージ。v1.46.0(2026-03-01) | API クライアント(GitLab) | GitLab のクライアント。指標は低め |
| 125 | Go | aws-sdk-go-v2 の service/s3 と config | 新規 | sink: `PutObject/GetObject` の Bucket、Key(resource_name)、source: `GetObjectOutput.Body`。`config.LoadDefaultConfig` は設定の読み込み。`smithy-go` は宣言の対象 | s3: 6.4k / 6.7k。config: 11.7k / 10.4k。s3 v1.116.0(2026-11-18) | クラウド | AWS の中で imported by が最多。config は imported by が 1 万超 |
| 126 | Go | aws-sdk-go-v2 の service/dynamodb | 新規 | sink: `Query/Scan/PutItem` の式と Key。source: 戻り値の `Items` | imported by 1.9k モジュール / 2.3k パッケージ。v1.74.1(2026-11-17) | クラウド / DB | AWS の次の候補(S3 の次に imported by が多い) |
| 127 | Go | aws-sdk-go-v2 の service/sqs | 新規 | sink: `SendMessage` の本文。source: `ReceiveMessage` の `Messages` | imported by 1.2k モジュール / 1.7k パッケージ。v1.53.0(2026-11-10) | クラウド / メッセージング | AWS の次の候補 |
| 128 | Go | aws-sdk-go-v2 の service/secretsmanager | 新規 | source: `GetSecretValue` の `SecretString`(秘密)。sink: `PutSecretValue` | imported by 1.2k モジュール / 1.1k パッケージ。v1.51.0(2026-11-12) | クラウド | AWS の次の候補 |
| 129 | Dart | tar(package:tar) | 新規 | source: `TarEntry.name`、`TarHeader.name`、`TarHeader.linkName`(アーカイブのエントリ名と、リンク先の名前) | 未取得(指標は取得していない) | アーカイブ | 広く使われるアーカイブのライブラリ。エントリ名は zip slip の入口 |
| 130 | Dart | archive(package:archive) | 新規 | source: `ArchiveFile.name`(アーカイブのエントリ名) | 未取得(指標は取得していない) | アーカイブ | 広く使われるアーカイブのライブラリ。エントリ名は zip slip の入口 |
| 131 | Rust | tar | 新規 | source: `Entry::path`、`Entry::header().path()`、`link_name`(エントリ名と、リンク先の名前) | 未取得(指標は取得していない) | アーカイブ | 広く使われるアーカイブのクレート。エントリ名は zip slip の入口 |
| 132 | Rust | zip | 新規 | source: `ZipFile::name`、`mangled_name`(元のままの名前)。`enclosed_name` は安全な形に直した名前なので source にしない | 未取得(指標は取得していない) | アーカイブ | 広く使われるアーカイブのクレート。エントリ名は zip slip の入口 |

## 2. 指標で確認してから入れたもの、入れないもの

上位のものを特定できなかった種類を、公開の指標で確認した。基準: **同じ種類で最多のものが特定でき、数字が 0.5M 以上(Rust は直近 90 日、Dart は 30 日の DL。Go は imported by が 300 モジュール以上)のものは、1 節に入れた**。そうでないものは入れない。

| 言語 | 種類 | 入れる / 入れない | 指標と理由 |
|---|---|---|---|
| Rust | Datadog | 入れた(`datadog-api-client`) | 直近 90 日 1.6M。監視の中では 1 桁小さいが、Datadog の公式のクライアント |
| Rust | 決済 | 入れた(`async-stripe`) | 直近 90 日 0.99M で最多。`stripe-rs` は 2015 年のまま(直近 12) |
| Rust | チャット | 入れた(`slack-morphism`) | 直近 90 日 1.9M |
| Rust | Google Cloud | 入れた(`google-cloud-storage`) | 直近 90 日 13.1M(Google の公式) |
| Rust | Azure | 入れた(`azure_storage_blobs`) | 直近 90 日 2.1M。最終公開が 2024-10 で古い(着手時に確認)。`azure_core` は 11.1M(土台。モデルの対象は blobs) |
| Rust | GitLab | 入れた(`gitlab`) | 直近 90 日 1.9M |
| Go | 認証のサービスのクライアント | 入れた(`go-oidc` のみ) | 特定のサービスに依らない OIDC は imported by 2.1k モジュール。Auth0 の `go-auth0` は 7 モジュールで、入れない |
| Go | GitLab | 入れた(`gitlab-org/api/client-go`) | imported by 337 モジュール |
| Dart | OpenTelemetry | 入れた(`opentelemetry`) | 30 日 DL 270k で最多 |
| Dart | GitLab | **入れない**(`gitlab`) | 30 日 DL 44、最終公開 2021-10。ほかに上位が見つからない |

## 3. 入れないもの

- macaron(`examples/rules/go-macaron.yaml`)。例のまま残す。
- `examples/rules/go-database-sql.yaml`(DB から読んだ値を source にする規則)。既定で有効にすると finding が増える種類なので、組み込みにするかは別に判断する。例のまま。

## 4. カバー済みのもの

今ある組み込みモデルは `internal/model/builtin/` が正で、この文書には写さない(`ls internal/model/builtin/`)。

## 5. 指標の取得

- 取得日: 2026-11-23。メタデータだけで、パッケージのコードは取得していない。リクエストは、間隔をあけて(1〜2 秒)、同じページを繰り返さずに行った。
- Go: pkg.go.dev の「imported by」(`?tab=importedby`)の「importing modules」と「importing packages」、proxy.golang.org の `@latest`(最新の version と日付)。
- Rust: crates.io の公開 API(`/api/v1/crates/<名前>`)の `recent_downloads`(直近 90 日)、`downloads`(全期間)、`max_version`、`updated_at`。
- Dart: pub.dev の公開 API(`/api/packages/<名前>`、`/score`)の `downloadCount30Days`、`likeCount`、`latest.published`。Flutter 依存は `latest.pubspec` の `dependencies` と `environment` に `flutter` があるか。
- 取れなかったもの(未取得): deps.dev の依存される数(Go のモジュールで見つからない)、Dart の dependents と popularity スコア、`google/go-github` の major をまたぐ合計(major ごとに分かれ、最新の v93 は 0)、`stripe-go`、`go-elasticsearch`、`opensearch-go` の旧 major の合計。最新の version を proxy で取れなかったもの(パスがモジュールの途中のもの)は、「(パッケージで取得)」と書いた。
