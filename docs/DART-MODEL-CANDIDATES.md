# Dart の組み込みモデルにする pub パッケージの候補

状態: 案(決めてもらうための資料)。組み込みモデルの方針は [DESIGN-model-authoring.md](DESIGN-model-authoring.md) に従う(組み込みは標準ライブラリと、代わりのないパッケージまで)。

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

## 3. 候補の表

順位は 2026-11-22 時点。DL は直近 30 日(M は百万)。優先度は観点 1 と 2 による。

| 順位 | パッケージ | 説明 | モデルにする API の境界の案 | 種類 | 組み込み | 優先度と理由 |
|---|---|---|---|---|---|---|
| 10 | dio | HTTP クライアント(8.4k likes、DL 5.6M) | sink: `Dio.get/post/request/download` の URL とデータ。source: `Response.data`。flow: `Options`、`BaseOptions`。`Interceptor` は実装の多いインターフェース | HTTP クライアント | なし | 高: 実装の多い `Interceptor` と `HttpClientAdapter` で呼び出しが合流する |
| 11 | flutter_secure_storage | キーと値の安全な保存(DL 6.6M) | source: `read/readAll`。sink: `write`。flow: 保存したキーと値 | ファイル / 保存 | なし | 中: 値が保存を経由して戻る流れを繋げる |
| 4 | path_provider | 標準的なディレクトリのパスを返す(DL 11.1M) | source(信頼済み): `getApplicationDocumentsDirectory` などの戻り値。パスの構築の起点 | ファイル | なし | 中: ファイルの sink に至る経路の起点 |
| 6 | shared_preferences | 小さな設定の保存(DL 9.7M、10.6k likes) | source: `getString/get`。sink: `setString/set`。flow: キーと値 | 保存 | なし | 中: 保存を経由する流れ |
| 5 | url_launcher | URL やアプリを開く(DL 10.2M) | sink: `launchUrl`、`launchUrlString` の URL | 外部の呼び出し | なし | 高: URL が外へ出る sink が 1 つに集まる |
| 31 | firebase_auth | Firebase の認証(DL 2.4M) | source: `User.getIdToken`、`currentUser`、`signIn*` の戻り値。sink: `signIn*` の資格情報 | 認証 / クラウド | なし | 高: 認証の結果が source になる |
| 18 | firebase_messaging | プッシュ通知(DL 4.7M) | source: `onMessage` と `getInitialMessage` の `RemoteMessage.data` | クラウド | なし | 中: 外から来るデータの source |
| 369 | cloud_firestore | Firestore のクライアント(DL 1.8M) | source: `DocumentSnapshot.data`。sink: `set/add/update` のデータ、`where` の値 | DB / クラウド | なし | 中: 実装の多いクエリの型を持つ |
| 101 | cloud_functions | Cloud Functions の呼び出し(DL 0.8M) | sink: `HttpsCallable.call` の引数。source: `HttpsCallableResult.data` | クラウド | なし | 低: 呼び出しの範囲が狭い |
| 38 | drift | リアクティブな SQLite(DL 2.1M) | sink: `customSelect`、`customStatement`、`customUpdate` の SQL 文字列。source: 行の `QueryRow.data` | DB | なし | 高: 生の SQL の sink が明確 |
| 121 | postgres | PostgreSQL のドライバ(DL 0.7M) | sink: `Connection.execute` と `Session.execute` の SQL。source: `Result` の行 | DB | なし | 高: 純 Dart のドライバで、中を読むコストが大きい |
| 22 | sqflite | SQLite(DL 3.9M) | sink: `rawQuery/rawInsert/rawUpdate/rawDelete/execute`(`sqflite_common` 側) | DB | 一部(下の 5 節) | 低: 主要部分は済み。`query` の `where` 文字列と `Batch` が残り |
| 144 | sqflite_common_ffi | デスクトップ用の sqflite(DL 0.7M) | `databaseFactoryFfi` から `sqflite_common` の API に入る | DB | 間接(`sqflite_common`) | 低: 入口だけで済む可能性 |
| 87 | sembast | Dart だけで書いた NoSQL の DB(DL 0.5M) | sink: `store.add/put`。source: `find`、`get` の戻り値。`Finder` の filter | DB | なし | 低: 注入の sink が少ない |
| 84 | hive_ce | ローカルのキー値の DB(DL 1.4M) | source: `Box.get`。sink: `Box.put` | DB | なし | 低: 保存を経由する流れ |
| 105 | dart_jsonwebtoken | JWT の生成と検証(DL 2.2M) | source: `JWT.verify` の `payload`(検証後も外から来たデータ)。sanitizer の候補: 検証の成功。sink: `JWT.sign` の key | 認証 | なし | 高: 認証の判断に関わる |
| 106 | jwt_decoder | JWT のデコード(DL 0.7M) | source: `JwtDecoder.decode` の戻り値(検証なし) | 認証 | なし | 低: 範囲が狭い |
| 139 | oauth2 | OAuth2 のクライアント(DL 0.6M) | source: `Credentials`、`Client` が返す `http.Response`。sink: `authorizationUrl` の引数。`http.BaseClient` を継承 | 認証 | なし | 中: `http` の `Client` の実装でもある |
| 133 | flutter_appauth | OAuth / OpenID Connect(DL 0.5M) | source: `TokenResponse` の各トークン。sink: `AuthorizationTokenRequest` の URL | 認証 | なし | 低: 範囲が狭い |
| 36 | local_auth | 生体認証(DL 2.1M) | 判定の戻り値(`authenticate` の bool)が sanitizer のように使われる | 認証 | なし | 低: データの流れにほぼ関わらない |
| 32 | logging | ロギング(DL 14.0M) | sink: `Logger.info/warning/severe` のメッセージ(ログへの注入)。`Logger.onRecord` と `LogRecord` | ロギング | なし | 中: ダウンロード数が多い。実装の多いリスナへ合流 |
| 74 | sentry | エラー報告(DL 2.3M) | sink: `Sentry.captureException/captureMessage` | ロギング / クラウド | なし | 低: 外へ出るデータだけ |
| 26 | shelf | サーバのミドルウェア(DL 16.6M) | source: `Request`(済み)。sink: `Response` のボディ、`Response.found`/`movedPermanently` の URL(リダイレクト) | Web フレームワーク | 一部(5 節) | 高: 残りは `Response` と `Handler`/`Middleware` の合流 |
| 146 | shelf_router | shelf のルーター(DL 0.6M) | source: `Request.params`。`Router.get/post` に登録したハンドラが入口 | Web フレームワーク | なし | 高: 入口の登録が動的な呼び出し |
| 163 | shelf_web_socket | shelf で WebSocket(DL 14.3M) | source: `webSocketHandler` のコールバックに渡るメッセージ | Web フレームワーク | なし | 中: 入口が 1 つ |
| 132 | dart_frog | サーバのフレームワーク(DL 0.2M) | source: `RequestContext.request`、`Request.json/body`。`onRequest(RequestContext)` が入口 | Web フレームワーク | なし | 中: 入口の規約(ファイルの配置)が特別 |
| 333 | web_socket_channel | WebSocket(DL 13.9M) | source: `WebSocketChannel.stream`。sink: `WebSocketChannel.sink.add`、`connect` の URL | ネットワーク | なし | 高: ダウンロードが多い。sink と source が両方ある |
| 175 | mqtt_client | MQTT(DL 0.1M) | source: 購読した `MqttPublishMessage`。sink: `publishMessage`、`connect` のホスト | ネットワーク | なし | 低: 利用が少ない |
| 143 | chopper | HTTP クライアントのジェネレータ(DL 0.2M) | `ChopperClient` の呼び出しと、生成コードの `Response.body` | HTTP クライアント | なし | 低: 生成コードに依存 |
| 190 | retrofit_generator | HTTP クライアントのジェネレータ(DL 0.6M) | 生成コードが `dio` の呼び出しに行き着くので、`dio` のモデルで足りる可能性 | HTTP クライアント | なし | 低: `dio` が先 |
| 171 | process_run | プロセスの実行(DL 0.3M) | sink: `run`、`runExecutableArguments` のコマンドと引数(コマンド注入) | プロセス | なし | 高: コマンドの sink で、組み込みで見える範囲が広がる |
| 86 | open_file | 外部のアプリでファイルを開く(DL 0.5M) | sink: `OpenFile.open` のパス | ファイル | なし | 低: 範囲が狭い |
| 8 | image_picker | 画像の選択(DL 6.4M) | source: `ImagePicker.pickImage` の `XFile.path/name` | ファイル | なし | 低: 利用者の操作が起点 |
| 104 | file_selector | ファイルの選択(DL 1.2M) | source: `openFile` の `XFile`。sink: `getSaveLocation` に続く書き込み | ファイル | なし | 低: 同上 |
| 187 | cross_file | `XFile`(DL 0.2M) | source: `XFile.path`、`readAsString`、`readAsBytes`。sink: `saveTo` のパス | ファイル | なし | 中: `image_picker`/`file_selector` の戻り値の型 |
| 49 | flutter_dotenv | `.env` の読み込み(DL 1.0M) | source: `dotenv.env[...]`(設定の値) | 設定 | なし(`dart-config.yaml` は別) | 低: 設定は信頼済みとして扱うことが多い |
| 53 | xml | XML のパース(DL 19.2M) | source: `XmlDocument.parse` の戻り値へ flow。propagator: `XmlElement.getAttribute`、`innerText` | シリアライズ | なし | 後回し: 中を読めば足りる。ただし DL が多く、flow モデルで解析のコストは下がる |
| 88 | yaml | YAML のパース(DL 21.3M) | propagator: `loadYaml` の入力から戻り値へ | シリアライズ | なし | 後回し: 純粋なパース |
| 7 | crypto | ハッシュ(DL 21.4M) | propagator: `sha256.convert`、`Hmac.convert` の入力から戻り値へ。sanitizer の候補ではない | 暗号 | なし | 後回し: 純粋な計算。ただし flow を切れば解析が軽くなる |
| 100 | pretty_dio_logger | `dio` のログ(DL 0.4M) | `Interceptor` の実装。`dio` の後 | ロギング | なし | 低: `dio` の後 |
| 156 | dio_cache_interceptor | `dio` のキャッシュ(DL 0.3M) | `Interceptor` の実装。`dio` の後 | HTTP クライアント | なし | 低: `dio` の後 |
| 158 | puppeteer | ヘッドレスブラウザの操作(DL 0.3M) | sink: `Page.goto` の URL、`Page.evaluate` のコード | ネットワーク / 動的実行 | なし | 低: コード実行の sink は明確 |

表は 42 行(優先度の観点で幅を持たせた。外すものは決めてもらう)。優先度「高」だけ先に取る場合は、`dio`、`url_launcher`、`firebase_auth`、`drift`、`postgres`、`dart_jsonwebtoken`、`shelf` の残り、`shelf_router`、`web_socket_channel`、`process_run`。

## 4. すでに決まっているもの(別の作業で進行中)

一覧の候補には入れない。

| 対象 | 状態 | 順位(2026-11-22)、DL |
|---|---|---|
| `package:http`(`Client` を含む) | 進行中。ただし `dart-packages.yaml` には `get/post/put/delete/head/patch/read`、`Client.get/post/send` が既にある | 1、DL 15.4M |
| Google Cloud: `googleapis` | 進行中 | 219、DL 1.5M |
| Google Cloud: `gcloud` | 進行中 | 1552、DL 0.8M |
| Google Cloud: `google_cloud_storage` | 進行中 | 3476、DL 29k |
| `grpc` | 部分的に済み(`dart-grpc.yaml`) | 504、DL 0.7M |

## 5. カバー済みの確認

`internal/model/builtin/dart-*.yaml` を読み、`package:` で始まる参照を数えた(2026-11-22)。

| パッケージ | ファイル | `package:` の参照数 | 範囲 |
|---|---|---|---|
| shelf | `dart-packages.yaml`(140 行) | 15 | `Request` を source に(`param_type`)、`Request` の getter と `read/readAsString` の flow。`Response` と `Handler` は無し |
| http | `dart-packages.yaml` | 21 | `Response.body/bodyBytes/headers`、`StreamedResponse.stream` の flow。`get/post/put/delete/head/patch/read` と `Client.get/post/send` の sink |
| sqflite_common | `dart-packages.yaml` | 10(`sqflite` が 1) | `rawQuery/rawInsert/rawUpdate/rawDelete/execute` の sink(`SqfliteDatabaseMixin` と `DatabaseExecutor`) |
| grpc | `dart-grpc.yaml`(32 行) | 6 | サーバ側のハンドラを入口に(`Service` の継承と `ServiceCall`)、`ServiceCall.clientMetadata`、`ClientChannel.new` |
| path | `dart-path.yaml`(34 行) | 13 | パスの操作の flow |

`dart-io.yaml`(169 行)、`dart-core.yaml`(337 行)、`dart-async.yaml`(140 行)、`dart-misc.yaml`(173 行)、`dart-pure.yaml`(96 行)、`dart-config.yaml`(13 行)は標準ライブラリ(`dart:*`)で、パッケージの参照は無い。したがって、3 節の表のうち shelf、sqflite、http、grpc 以外は「なし」。
