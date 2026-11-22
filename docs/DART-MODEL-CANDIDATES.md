# Dart の組み込みモデルにする pub パッケージの候補

状態: 案(決めてもらうための資料)。組み込みモデルの方針は [DESIGN-model-authoring.md](DESIGN-model-authoring.md) に従う(ADR-097(ADR-096 を更新): 組み込みは Google Cloud、外部のサービスのクライアント、標準ライブラリ、代わりのないパッケージ、pub の人気の上位)。

## 1. 基準

選ぶ根拠は pub.dev の公開の指標だけ。特定の利用者やプロジェクトが何を使うかは考えていない。

- 取得日: 2026-11-22
- 順位: `https://pub.dev/api/package-name-completion-data`(20000 件)の並び順。この一覧は人気の順に並んでいるように見える(先頭は http、intl、uuid、path_provider、url_launcher)が、並びの基準を pub.dev は明記していないので、順位は「その一覧での位置」とする。
- ダウンロード数: `https://pub.dev/api/packages/<名前>/score` の `downloadCount30Days`(直近 30 日)。
- likes: 同じく `likeCount`。pub points: `grantedPoints`(160 が満点)。
- 依存される数(dependents)と popularity スコア: 公開 API で数値が取れなかったので **未取得**(推測の数字は書かない)。
- 取得したのはメタデータだけ。パッケージのコードは取得していない。API の境界の案は、公開のドキュメントと API の知識で書いたもので、コードで確認していない。

## 2. 優先の観点

1. 実装の多いインターフェース(例: `Client`)を持つもの。ネットワーク、ファイル、DB、認証など外との境界になるもの。sink、source、動的な呼び出しの合流が起きやすい。
2. 中を読むコストが大きいもの。

純粋な計算やデータ構造のパッケージは、中を読めば足りるので後回し(表に入れても「後回し」と書く)。

## 3. 進める順

1. `http`(進行中)。
2. Google Cloud(進行中)。`googleapis`、`googleapis_auth`、`gcloud`、`google_cloud_storage`、`dart_firebase_admin` に加えて、Firebase 系(`firebase_auth`、`cloud_firestore`、`firebase_messaging`、`cloud_functions`、`firebase_storage`、`firebase_ai`)も含める。
3. このあとは、4 節(サーバ側の基本のパッケージ)の上位と、5 節(外部のサービスのクライアント)の上位を、交互に進める。
   1. 4 節: `dio`。5 節: `supabase`(`supabase_flutter` と共通の層)。
   2. 4 節: `shelf` の残り、`shelf_router`。5 節: `sentry_flutter`(エラー報告)。
   3. 4 節: `logging`、`postgres`。5 節: `openai_dart` と `google_generative_ai`(AI の API)。
   4. 4 節: `grpc` のクライアント側、`web_socket_channel`。5 節: `aws_signature_v4` と `aws_client`(AWS)。
   5. 4 節: `dart_jsonwebtoken`、`process_run`。5 節: `flutter_stripe`、`purchases_flutter`(決済)、`mailer`(メール)。
   6. 4 節の残りと、5 節の残り(認証、検索、メッセージング、BaaS)。
4. 6 節(Flutter のクライアント側): `url_launcher`、`drift`、`shared_preferences` など。
5. 7 節(後回し)。

`dio` は組み込みにする(人気の順に組み込む、という決定)。Go と Rust の同じ種類の一覧(サーバ側の基本のパッケージ、外部のサービスのクライアント)は、Dart のあとで作る(今は作らない)。

## 4. サーバ側と、どこでも使う基本のパッケージ

pub.dev の公開の指標で、Flutter に依存しないパッケージ(最新版の `pubspec.yaml` の `dependencies` と `environment` に `flutter` が無い。2026-11-22 に `/api/packages/<名前>` で確認)のうち、種類ごとに上位のものを選んだ。理由は公開のものだけ: サーバ側の Dart(例: gRPC サーバ。`bench/dart-grpc`)で使え、Flutter を必要としない。DL は直近 30 日。

| 順位 | パッケージ | 説明 | モデルにする API の境界の案 | 種類 | 組み込み | 優先度と理由 |
|---|---|---|---|---|---|---|
| 10 | dio | HTTP クライアント(8.4k likes、DL 5.6M) | sink: `Dio.get/post/request/download` の URL とデータ。source: `Response.data`。flow: `Options`、`BaseOptions`。`Interceptor` は実装の多いインターフェース | HTTP クライアント | なし | 高: 実装の多い `Interceptor` と `HttpClientAdapter` で呼び出しが合流する |
| 794 | http2 | HTTP/2 の実装(DL 1.0M) | source: `ClientTransportConnection` / `ServerTransportConnection` の `incomingStreams` と `StreamMessage`。sink: `makeRequest` のヘッダ | HTTP / ネットワーク | なし | 中: `grpc` の下の層。ストリームのコールバックの合流 |
| 99 | http_parser | HTTP のヘッダや日付のパーサ(DL 14.1M) | propagator: `MediaType.parse`、`parseHttpDate` の入力から戻り値へ | HTTP | なし | 中: DL が多い。`http` と `shelf` の中で使われる |
| 48 | retry | リトライ(DL 3.1M) | `RetryOptions.retry(fn)` の `fn` の戻り値が戻り値へ。呼び戻し(`fn`)を呼ぶ | 非同期 / HTTP | なし | 中: 呼び戻しの呼び出しを繋ぐ(HTTP の呼び出しを包むことが多い) |
| 504 | grpc(クライアント側) | gRPC の Dart 実装(DL 0.7M) | sink: `ClientChannel.new` のホスト(済み)、`Client.$createCall` の呼び出しのメタデータ。source: `ResponseFuture`/`ResponseStream` の中身 | RPC | 一部(`dart-grpc.yaml` はサーバ側と `ClientChannel.new`) | 高: 生成コードの `Client` から呼ぶ。実装の多い `ClientChannel`/`ClientTransportConnector` |
| 535 | protobuf | Protocol Buffers のランタイム(DL 3.2M) | propagator: `GeneratedMessage.fromBuffer/fromJson/writeToBuffer/toProto3Json` と、生成コードのフィールドの getter/setter の flow | RPC / シリアライズ | なし | 高: gRPC の入力のメッセージはここを通る。生成コードの中を読むコストが大きい |
| 340 / 21 | json_annotation / json_serializable | JSON の注釈とジェネレータ(DL 16.2M / 4.5M) | 生成コードの `fromJson`/`toJson` の flow。ランタイムの関数は少ない(`$checkedCreate` など) | シリアライズ | なし | 中: 生成コードを読めば足りる面が大きい。DL は多い |
| 549 | googleapis_auth | Google API の認証(DL 2.3M) | source: `AccessCredentials`、`clientViaServiceAccount` などが返す `AuthClient`(`http.Client` の実装)。sink: 資格情報の JSON | 認証 / クラウド | なし | 高(Google Cloud の範囲): `googleapis` の前提 |
| 139 | oauth2 | OAuth2 のクライアント(DL 0.6M) | source: `Credentials`、`Client` が返す `http.Response`。sink: `authorizationUrl` の引数。`http.BaseClient` を継承 | 認証 | なし | 中: `http` の `Client` の実装でもある |
| 105 | dart_jsonwebtoken | JWT の生成と検証(DL 2.2M) | source: `JWT.verify` の `payload`(検証後も外から来たデータ)。sanitizer の候補: 検証の成功。sink: `JWT.sign` の key | 認証 | なし | 高: 認証の判断に関わる |
| 121 | postgres | PostgreSQL のドライバ(DL 0.7M) | sink: `Connection.execute` と `Session.execute` の SQL。source: `Result` の行 | DB | なし | 高: 純 Dart のドライバで、中を読むコストが大きい |
| 408 / 1917 | mysql1 / mysql_client | MySQL のドライバ(DL 11k / 14k) | sink: `MySqlConnection.query`、`prepared` の SQL 文字列。source: 結果の `Row` | DB | なし | 中: DL は少ない(公開の指標では低い) |
| 2775 | redis | Redis のクライアント(DL 0.3M) | sink: `Command.send_object` のコマンドと引数。source: 戻り値 | DB | なし | 低: 順位が低い |
| 645 | mongo_dart | MongoDB のドライバ(DL 0.1M) | sink: `DbCollection.find/insert/update/deleteOne` の selector(NoSQL 注入)。source: 戻り値の `Map` | DB | なし | 中: クエリを `Map` で渡す形が注入の経路になる |
| 435 | sqlite3 | SQLite の FFI のバインディング(DL 3.9M) | sink: `Database.execute/select`、`prepare` の SQL。source: `ResultSet` の行 | DB | なし | 高: SQL の sink が明確。FFI の本体は読みにくい(中を読むコストが大きい) |
| 26 | shelf | サーバのミドルウェア(DL 16.6M) | source: `Request`(済み)。sink: `Response` のボディ、`Response.found`/`movedPermanently` の URL(リダイレクト) | Web フレームワーク | 一部(9 節) | 高: 残りは `Response` と `Handler`/`Middleware` の合流 |
| 146 | shelf_router | shelf のルーター(DL 0.6M) | source: `Request.params`。`Router.get/post` に登録したハンドラが入口 | Web フレームワーク | なし | 高: 入口の登録が動的な呼び出し |
| 163 | shelf_web_socket | shelf で WebSocket(DL 14.3M) | source: `webSocketHandler` のコールバックに渡るメッセージ | Web フレームワーク | なし | 中: 入口が 1 つ |
| 132 | dart_frog | サーバのフレームワーク(DL 0.2M) | source: `RequestContext.request`、`Request.json/body`。`onRequest(RequestContext)` が入口 | Web フレームワーク | なし | 中: 入口の規約(ファイルの配置)が特別 |
| 32 | logging | ロギング(DL 14.0M) | sink: `Logger.info/warning/severe` のメッセージ(ログへの注入)。`Logger.onRecord` と `LogRecord` | ロギング | なし | 中: ダウンロード数が多い。実装の多いリスナへ合流 |
| 171 | process_run | プロセスの実行(DL 0.3M) | sink: `run`、`runExecutableArguments` のコマンドと引数(コマンド注入) | プロセス | なし | 高: コマンドの sink で、組み込みで見える範囲が広がる |
| 458 | file | `dart:io` のファイルシステムの抽象(DL 14.1M) | `FileSystem` のメソッドは `dart:io` の `File`/`Directory` と同じ境界(`dart-io.yaml` を参照する形で同じ sink/source) | ファイル | なし | 高: `FileSystem` は実装の多いインターフェース(Local、Memory、Chroot) |
| 12 | path | パスの操作(DL 14.0M) | propagator: `join`、`normalize` など | ファイル | 済み(`dart-path.yaml`) | 済み |
| 298 | io | `dart:io` の補助(DL 17.9M) | `copyPath`、`ProcessManager.spawn`、`sharedStdIn`。sink: プロセスの引数 | プロセス / ファイル | なし | 中: DL が多い。`dart:io` の上の薄い層 |
| 378 | async | `dart:async` の補助(DL 16.9M) | propagator: `StreamGroup`、`StreamQueue`、`FutureGroup`、`AsyncCache`、`CancelableOperation` の入力から出力へ | 非同期の基盤 | なし | 高: 呼び戻しと Future/Stream の合流が多い。中を読むコストが大きい |
| 1542 | stream_channel | 2 方向のストリーム(DL 13.4M) | propagator: `StreamChannel.stream`/`sink` の入出力、`StreamChannelController` の `local`/`foreign` の対 | 非同期の基盤 | なし | 中: `web_socket_channel` と `shelf_web_socket` の下の層 |
| 333 | web_socket_channel | WebSocket(DL 13.9M) | source: `WebSocketChannel.stream`。sink: `WebSocketChannel.sink.add`、`connect` の URL | ネットワーク | なし | 高: ダウンロードが多い。sink と source が両方ある |

`http` と Google Cloud は 8 節の「すでに決まっているもの」に書いた。`dart_firebase_admin`(サーバ側の Firebase)は、5 節に置いた(Google Cloud の範囲)。

## 5. 外部のサービスのクライアント

クラウド、認証、決済、メールと通知、検索、監視とエラー報告、メッセージング、ストレージ、BaaS、AI の API のクライアントを、pub.dev の公開の指標(DL、likes、一覧での位置)で、種類ごとに上位のものを選んだ。選び方は 1 節と同じで、特定の利用者が使うものを推測していない。取得日 2026-11-22。

- 「公式」は、pub.dev の publisher(検証済みの発行者)がサービスの運営元か、説明がそれを明記しているもの。publisher の表示がなく、説明からの推定のものは、そう書いた。「コミュニティ」は運営元以外が出しているもので、「事実上の標準」は、その種類の中で DL と likes が最も多いもの。
- 「Flutter 依存」は、最新版の `pubspec.yaml` の `dependencies` と `environment` に `flutter` があるもの。「Dart のみ」は無いもの(`/api/packages/<名前>` で確認)。
- 「一覧に無い」は、1 節の 20000 件の一覧に載っていないもの(DL は取れている)。順位は「未取得」と書いた。
- Kafka は、pub.dev に DL の多いクライアントが見つからなかった(`kafka` は 12 DL)ので入れていない。Azure は `azblob`(DL 1.7k、コミュニティ)が見つかっただけで、公開の指標が低いので入れていない。
- `dart_firebase_admin` は Google Cloud の範囲(3 節の 2)。ADR-097 では、Google Cloud はこの範囲の 1 つ目。

| 順位 | パッケージ | 説明 | モデルにする API の境界の案 | 種類 | 組み込み | 優先度と理由 |
|---|---|---|---|---|---|---|
| 443 | supabase | Supabase の Dart クライアント(公式。Dart のみ。DL 1.3M、615 likes) | sink: `from(t).select/insert/update/delete/eq` のフィルタの文字列、`rpc` の引数、`auth.signIn*` の資格情報。source: 戻り値の行、`auth.currentUser` | BaaS | なし | 高: BaaS の中で DL が最も多い。`supabase_flutter` と共通 |
| 414 | supabase_flutter | Supabase の Flutter 版(公式。Flutter 依存。DL 1.3M) | `supabase` と同じ。`Supabase.instance.client` から入る | BaaS | なし | 高: `supabase` の後ろに足す(同じ API) |
| 1031 | sentry_flutter | Sentry の Flutter 版(公式。Flutter 依存。DL 2.1M) | sink: `Sentry.captureException/captureMessage`、`addBreadcrumb` のデータ(外へ出る) | 監視とエラー報告 | なし(`sentry` も無し) | 中: DL が多いが、外へ出るだけ |
| 356 | openai_dart | OpenAI の REST のクライアント(コミュニティ。Dart のみ。DL 45k) | sink: `createChatCompletion` などのプロンプト(プロンプトの注入)。source: 応答の `choices` の文字列 | AI の API | なし | 高: 応答が次の処理に流れる(間接の注入)。DL は少なめ |
| 307 | dart_openai | OpenAI のクライアント(コミュニティ。Dart のみ。DL 17k) | 同上。`OpenAI.instance.chat.create` | AI の API | なし | 中: `openai_dart` と同じ種類 |
| 未取得(一覧に無い) | google_generative_ai | Google の Gemini の Dart SDK(公式、publisher: google.dev。Dart のみ。DL 89k) | sink: `GenerativeModel.generateContent` のプロンプト。source: `GenerateContentResponse.text` | AI の API | なし | 高: Google Cloud の範囲に近い |
| 866 | firebase_ai | Firebase AI Logic(公式。Flutter 依存。DL 127k) | 同上(`GenerativeModel` と `LiveGenerativeModel`) | AI の API | なし | 高(Google Cloud の範囲): 3 節の 2 |
| 1784 | anthropic_sdk_dart | Anthropic の API のクライアント(コミュニティ。Dart のみ。DL 27k) | sink: `messages.create` のプロンプト。source: 応答の内容 | AI の API | なし | 中: 同じ種類 |
| 1106 | aws_signature_v4 | AWS の署名 V4(公式 aws-amplify.com。Dart のみ。DL 192k) | sink: `AWSSigV4Signer.sign` の要求(宛先とヘッダ)。`AWSHttpRequest` の流れ | クラウド(AWS) | なし | 高: AWS のクライアントが通る共通の層 |
| 220 | amplify_flutter | AWS Amplify(公式。Flutter 依存。DL 165k) | sink/source: `Amplify.Auth.signIn*`、`Amplify.API.*`、`Amplify.Storage.*` の引数と戻り値 | クラウド(AWS)/ BaaS | なし | 中: 入口が多い(カテゴリごとのプラグイン)。登録して後で呼ぶ形 |
| 未取得(一覧に無い。DL 89k) | aws_client | AWS の API のクライアント集(コミュニティの事実上の標準。Dart のみ。generated。pub points 0) | sink: 各サービスの要求(`S3.putObject` の key、`SQS.sendMessage` の本文、`DynamoDB.query` の式)。source: 応答 | クラウド(AWS) | なし | 高: 生成コードが大きく、中を読むコストが大きい。pub points が 0 なので扱いに注意 |
| 1694 | minio | S3 互換のストレージ(コミュニティ。Dart のみ。DL 69k) | sink: `getObject/putObject/presignedGetObject` のバケットと key。source: オブジェクトの `Stream` | ストレージ | なし | 中: S3 互換の API 1 つに集まる |
| 1782 | auth0_flutter | Auth0(公式。Flutter 依存。DL 210k) | source: `Credentials.accessToken/idToken`、`UserProfile`。sink: `login`/`webAuthentication` の `audience` と `parameters` | 認証 | なし | 中: 認証の結果が source |
| 1287 | openid_client | OpenID Connect のクライアント(コミュニティの事実上の標準。Dart のみ。DL 16k) | source: `Credential.getTokenResponse`、`idToken.claims`。sink: `Flow.authorizationUrl`、`Client.discover` の URL | 認証 | なし | 中: OIDC の標準。DL は少なめ |
| 433 | flutter_stripe | Stripe(コミュニティ。Stripe 公式ではない。Flutter 依存。DL 514k) | sink: `Stripe.instance.confirmPayment` などの `clientSecret` と支払い情報。source: `PaymentIntent` | 決済 | なし | 中: 支払い情報は外へ出る。サーバ側の Stripe のクライアントは別 |
| 1107 | purchases_flutter | RevenueCat(公式。Flutter 依存。DL 720k) | source: `CustomerInfo`、`Offerings`。sink: `Purchases.logIn` の ID、`purchasePackage` | 決済 | なし | 中: DL が多い |
| 566 | mailer | メールの送信 SMTP(コミュニティの事実上の標準。Dart のみ。DL 287k) | sink: `send(Message, SmtpServer)` の宛先、件名、本文、ヘッダ(メールヘッダの注入)。`SmtpServer` のホスト | メール | なし | 高: ヘッダと本文の sink が明確。Dart のみ |
| 401 | enough_mail | IMAP/POP3/SMTP(コミュニティ。Dart のみ。DL 34k) | source: `fetchMessages` の `MimeMessage`(本文、ヘッダ)。sink: `sendMessage` | メール | なし | 中: 受信の source がある |
| 2478 | onesignal_flutter | OneSignal の通知(公式と思われる。publisher の表示なし。Flutter 依存。DL 202k) | sink: `OneSignal.login` の ID、`User.addTags`。source: 通知のクリックのデータ | 通知 | なし | 低: 外へ出るデータが中心 |
| 923 | algoliasearch | Algolia(公式。Dart のみ。DL 38k) | sink: `searchSingleIndex` の `query`、`filters`(フィルタの注入)。source: 応答の `hits` | 検索 | なし | 中: 検索の中では公式で、Dart のみ |
| 2669 | meilisearch | Meilisearch(公式。Dart のみ。DL 5.8k) | sink: `index.search` の query、filter。source: 応答 | 検索 | なし | 低: DL が少ない |
| 3047 | elastic_client | Elasticsearch の HTTP API(コミュニティ。Dart のみ。DL 1.3k) | sink: `Client.search` の `Query` と、`Doc` | 検索 | なし | 低: DL が少ない。ただし Elasticsearch の選択肢が少ない |
| 518 | mqtt5_client | MQTT 5(コミュニティの事実上の標準。Dart のみ。DL 66k) | source: `updates` の `MqttPublishMessage`。sink: `publishMessage`、`connect` のホスト | メッセージング | なし(`mqtt_client` も無し) | 中: MQTT は 7 節の `mqtt_client` と同じ種類 |
| 1343 | dart_nats | NATS のクライアント(コミュニティ。Dart のみ。DL 6.4k) | source: `Subscription.stream` の `Message`。sink: `pub`/`request` の subject とデータ | メッセージング | なし | 低: DL が少ない |
| 1684 | dart_amqp | AMQP 0.9.1(コミュニティ。Dart のみ。DL 9.9k) | source: `Queue.consume` の `AmqpMessage`。sink: `Exchange.publish` | メッセージング | なし | 低: DL が少ない |
| 460 | socket_io_client | socket.io(コミュニティの事実上の標準。Dart のみ。DL 336k、1.5k likes) | source: `socket.on(event, handler)` のデータ。sink: `socket.emit`、`io(url)` の URL | メッセージング / ネットワーク | なし | 高: DL と likes が多い。呼び戻しの登録 |
| 2020 | dart_appwrite | Appwrite のサーバ側 SDK(公式。Dart のみ。DL 20k) | sink: `Databases.listDocuments` の `queries`、`createDocument` のデータ。source: 戻り値 | BaaS | なし | 中: 公式の SDK。Flutter 版 `appwrite`(3423 位、DL 17k、Flutter 依存)も同じ API |
| 344 | pocketbase | PocketBase(公式。Dart のみ。DL 20k) | sink: `collection(c).getList/create/update` の `filter`、`body`。source: `RecordModel` | BaaS | なし | 中: 公式の SDK |
| 未取得(一覧に無い) | dart_firebase_admin | Firebase の Admin SDK(コミュニティ、publisher: invertase.io。Dart のみ。DL 17.5k) | source: `Auth.verifyIdToken` の `DecodedIdToken`。sink: `Firestore` の操作、`Messaging.send` | クラウド(Google Cloud の範囲) | なし | 高(Google Cloud の範囲): サーバ側の Firebase |

## 6. Flutter のクライアント側の候補の表

Flutter に依存するパッケージ(`flutter_secure_storage`、`path_provider`、`url_launcher`、`shared_preferences`、`firebase_*` など)と、Flutter を使うアプリで主に使う `drift`、`sqflite`。`firebase_auth`、`firebase_messaging`、`cloud_firestore`、`cloud_functions` は Google Cloud の範囲(3 節の 2)なので、先に進める。

| 順位 | パッケージ | 説明 | モデルにする API の境界の案 | 種類 | 組み込み | 優先度と理由 |
|---|---|---|---|---|---|---|
| 5 | url_launcher | URL やアプリを開く(DL 10.2M) | sink: `launchUrl`、`launchUrlString` の URL | 外部の呼び出し | なし | 高: URL が外へ出る sink が 1 つに集まる |
| 31 | firebase_auth | Firebase の認証(DL 2.4M) | source: `User.getIdToken`、`currentUser`、`signIn*` の戻り値。sink: `signIn*` の資格情報 | 認証 / クラウド | なし | 高: 認証の結果が source になる |
| 18 | firebase_messaging | プッシュ通知(DL 4.7M) | source: `onMessage` と `getInitialMessage` の `RemoteMessage.data` | クラウド | なし | 中: 外から来るデータの source |
| 369 | cloud_firestore | Firestore のクライアント(DL 1.8M) | source: `DocumentSnapshot.data`。sink: `set/add/update` のデータ、`where` の値 | DB / クラウド | なし | 中: 実装の多いクエリの型を持つ |
| 101 | cloud_functions | Cloud Functions の呼び出し(DL 0.8M) | sink: `HttpsCallable.call` の引数。source: `HttpsCallableResult.data` | クラウド | なし | 低: 呼び出しの範囲が狭い |
| 38 | drift | リアクティブな SQLite(DL 2.1M) | sink: `customSelect`、`customStatement`、`customUpdate` の SQL 文字列。source: 行の `QueryRow.data` | DB | なし | 高: 生の SQL の sink が明確 |
| 6 | shared_preferences | 小さな設定の保存(DL 9.7M、10.6k likes) | source: `getString/get`。sink: `setString/set`。flow: キーと値 | 保存 | なし | 中: 保存を経由する流れ |
| 11 | flutter_secure_storage | キーと値の安全な保存(DL 6.6M) | source: `read/readAll`。sink: `write`。flow: 保存したキーと値 | ファイル / 保存 | なし | 中: 値が保存を経由して戻る流れを繋げる |
| 4 | path_provider | 標準的なディレクトリのパスを返す(DL 11.1M) | source(信頼済み): `getApplicationDocumentsDirectory` などの戻り値。パスの構築の起点 | ファイル | なし | 中: ファイルの sink に至る経路の起点 |
| 22 | sqflite | SQLite(DL 3.9M) | sink: `rawQuery/rawInsert/rawUpdate/rawDelete/execute`(`sqflite_common` 側) | DB | 一部(9 節) | 低: 主要部分は済み。`query` の `where` 文字列と `Batch` が残り |

## 7. 後回し

優先度が低い行(外との境界の範囲が狭い、利用者の操作が起点、順位や DL が低い)と、純粋な計算のパッケージ。表から消さずにここに置く。

xml、yaml、crypto は後回しのまま。流れだけのモデルで前に出すのは、公開の対象で「中を読むコストが大きい」数字が出たときだけ。

| 順位 | パッケージ | 説明 | モデルにする API の境界の案 | 種類 | 組み込み | 優先度と理由 |
|---|---|---|---|---|---|---|
| 53 | xml | XML のパース(DL 19.2M) | source: `XmlDocument.parse` の戻り値へ flow。propagator: `XmlElement.getAttribute`、`innerText` | シリアライズ | なし | 後回し: 中を読めば足りる。ただし DL が多く、flow モデルで解析のコストは下がる |
| 88 | yaml | YAML のパース(DL 21.3M) | propagator: `loadYaml` の入力から戻り値へ | シリアライズ | なし | 後回し: 純粋なパース |
| 7 | crypto | ハッシュ(DL 21.4M) | propagator: `sha256.convert`、`Hmac.convert` の入力から戻り値へ。sanitizer の候補ではない | 暗号 | なし | 後回し: 純粋な計算。ただし flow を切れば解析が軽くなる |
| 36 | local_auth | 生体認証(DL 2.1M) | 判定の戻り値(`authenticate` の bool)が sanitizer のように使われる | 認証 | なし | 低: データの流れにほぼ関わらない |
| 74 | sentry | エラー報告(DL 2.3M) | sink: `Sentry.captureException/captureMessage` | ロギング / クラウド | なし | 低: 外へ出るデータだけ |
| 175 | mqtt_client | MQTT(DL 0.1M) | source: 購読した `MqttPublishMessage`。sink: `publishMessage`、`connect` のホスト | ネットワーク | なし | 低: 利用が少ない |
| 86 | open_file | 外部のアプリでファイルを開く(DL 0.5M) | sink: `OpenFile.open` のパス | ファイル | なし | 低: 範囲が狭い |
| 8 | image_picker | 画像の選択(DL 6.4M) | source: `ImagePicker.pickImage` の `XFile.path/name` | ファイル | なし | 低: 利用者の操作が起点 |
| 104 | file_selector | ファイルの選択(DL 1.2M) | source: `openFile` の `XFile`。sink: `getSaveLocation` に続く書き込み | ファイル | なし | 低: 同上 |
| 187 | cross_file | `XFile`(DL 0.2M) | source: `XFile.path`、`readAsString`、`readAsBytes`。sink: `saveTo` のパス | ファイル | なし | 中: `image_picker`/`file_selector` の戻り値の型 |
| 49 | flutter_dotenv | `.env` の読み込み(DL 1.0M) | source: `dotenv.env[...]`(設定の値) | 設定 | なし(`dart-config.yaml` は別) | 低: 設定は信頼済みとして扱うことが多い |
| 133 | flutter_appauth | OAuth / OpenID Connect(DL 0.5M) | source: `TokenResponse` の各トークン。sink: `AuthorizationTokenRequest` の URL | 認証 | なし | 低: 範囲が狭い |
| 106 | jwt_decoder | JWT のデコード(DL 0.7M) | source: `JwtDecoder.decode` の戻り値(検証なし) | 認証 | なし | 低: 範囲が狭い |
| 87 | sembast | Dart だけで書いた NoSQL の DB(DL 0.5M) | sink: `store.add/put`。source: `find`、`get` の戻り値。`Finder` の filter | DB | なし | 低: 注入の sink が少ない |
| 84 | hive_ce | ローカルのキー値の DB(DL 1.4M) | source: `Box.get`。sink: `Box.put` | DB | なし | 低: 保存を経由する流れ |
| 143 | chopper | HTTP クライアントのジェネレータ(DL 0.2M) | `ChopperClient` の呼び出しと、生成コードの `Response.body` | HTTP クライアント | なし | 低: 生成コードに依存 |
| 190 | retrofit_generator | HTTP クライアントのジェネレータ(DL 0.6M) | 生成コードが `dio` の呼び出しに行き着くので、`dio` のモデルで足りる可能性 | HTTP クライアント | なし | 低: `dio` が先 |
| 100 | pretty_dio_logger | `dio` のログ(DL 0.4M) | `Interceptor` の実装。`dio` の後 | ロギング | なし | 低: `dio` の後 |
| 156 | dio_cache_interceptor | `dio` のキャッシュ(DL 0.3M) | `Interceptor` の実装。`dio` の後 | HTTP クライアント | なし | 低: `dio` の後 |
| 158 | puppeteer | ヘッドレスブラウザの操作(DL 0.3M) | sink: `Page.goto` の URL、`Page.evaluate` のコード | ネットワーク / 動的実行 | なし | 低: コード実行の sink は明確 |
| 144 | sqflite_common_ffi | デスクトップ用の sqflite(DL 0.7M) | `databaseFactoryFfi` から `sqflite_common` の API に入る | DB | 間接(`sqflite_common`) | 低: 入口だけで済む可能性 |

## 8. すでに決まっているもの(別の作業で進行中)

一覧の候補には入れない。

| 対象 | 状態 | 順位(2026-11-22)、DL |
|---|---|---|
| `package:http`(`Client` を含む) | 進行中。ただし `dart-packages.yaml` には `get/post/put/delete/head/patch/read`、`Client.get/post/send` が既にある | 1、DL 15.4M |
| Google Cloud: `googleapis` | 進行中 | 219、DL 1.5M |
| Google Cloud: `gcloud` | 進行中 | 1552、DL 0.8M |
| Google Cloud: `google_cloud_storage` | 進行中 | 3476、DL 29k |
| `grpc` | 部分的に済み(`dart-grpc.yaml`) | 504、DL 0.7M |

## 9. カバー済みの確認

`internal/model/builtin/dart-*.yaml` を読み、`package:` で始まる参照を数えた(2026-11-22)。

| パッケージ | ファイル | `package:` の参照数 | 範囲 |
|---|---|---|---|
| shelf | `dart-packages.yaml`(140 行) | 15 | `Request` を source に(`param_type`)、`Request` の getter と `read/readAsString` の flow。`Response` と `Handler` は無し |
| http | `dart-packages.yaml` | 21 | `Response.body/bodyBytes/headers`、`StreamedResponse.stream` の flow。`get/post/put/delete/head/patch/read` と `Client.get/post/send` の sink |
| sqflite_common | `dart-packages.yaml` | 10(`sqflite` が 1) | `rawQuery/rawInsert/rawUpdate/rawDelete/execute` の sink(`SqfliteDatabaseMixin` と `DatabaseExecutor`) |
| grpc | `dart-grpc.yaml`(32 行) | 6 | サーバ側のハンドラを入口に(`Service` の継承と `ServiceCall`)、`ServiceCall.clientMetadata`、`ClientChannel.new` |
| path | `dart-path.yaml`(34 行) | 13 | パスの操作の flow |

`dart-io.yaml`(169 行)、`dart-core.yaml`(337 行)、`dart-async.yaml`(140 行)、`dart-misc.yaml`(173 行)、`dart-pure.yaml`(96 行)、`dart-config.yaml`(13 行)は標準ライブラリ(`dart:*`)で、パッケージの参照は無い。したがって、4 節から 7 節の表のうち shelf、sqflite、http、grpc 以外は「なし」。
