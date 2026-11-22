# Google Cloud のモデル: 製品ごとの表(YAML を書く前に)

> **状態: 一部実装**(確認: 2026-11-22、9c26a39)
> - 実装済み: Spanner と BigQuery の SQL(`internal/model/builtin/go-gcp.yaml`、ADR-077)。
> - 未実装(YAML にしていない): Pub/Sub と Functions の表。後ろの 3 つ(Firestore など)は見出しだけ。
> - 注意: ADR-077 から、この文書へのリンクが無い。

ADR-077 の例外(メンテナの選択。ほかのベンダーの根拠にならない)で組み込む Google Cloud のクライアントを、**製品ごとに、書く前に、1 枚の表で決める**文書。ここは設計の置き場で、YAML はない。表をレビューしてから `internal/model/builtin/go-gcp.yaml` に行を足す。

順番: **Pub/Sub と Functions の入口(この版で書いた)** → Storage / BigQuery の残り → Firestore → Firebase Auth。後ろの 3 つは節の見出しだけ(まだ表がない)。Spanner と BigQuery の SQL は ADR-077 で済み。

## 0. 全製品で同じにする線

Google Cloud は、何を「外からの入力」と数えるかが製品ごとに違う。HTTP は外から来る。保存データ(DB の行、オブジェクトの中身)は、誰かが書いたもので、外から来たとは限らない。メッセージ(Pub/Sub、イベント)は、トピックが内部専用か、外から書けるかで違い、**rivus には分からない**。そこで、ラベルを 3 つに分けて、**利用者が選ぶ**形を全製品で同じにする。

| ラベル | 何 | 既定 | 出所 |
|---|---|---|---|
| `user_input` | 呼び出し元が直接決める(HTTP のリクエスト、gRPC) | **入る**(組み込み) | `*net/http.Request` ほか、今あるもの |
| `stored` | 保存されたデータの読み出し(Spanner の行、BigQuery の行、Firestore のドキュメント、Storage のオブジェクト) | **入らない**(利用者の選択) | `rules-stored.yaml` の形(`bench/gcp/rules-stored.yaml`、`examples/rules/go-database-sql.yaml`) |
| `queue_message` | プロデューサが書いて、基盤が届けるもの(Pub/Sub のメッセージ、Cloud Functions のイベント) | **入らない**(利用者の選択) | 同じ形のファイル |

* 判定役は、ラベルで区別できる。「`user_input` は外から。`stored` と `queue_message` は、書いた側が信頼できるかによる」。
* Go の sink は `accepts` を書かない(どのラベルでも検出される)。選んだ source は、そのまま、既存の sink に届く。
* **組み込みで持つのは、sink と、データを運ぶ `flows`(source を作らない行)だけ。`stored` と `queue_message` の source は、`examples/rules/` に 1 つの例のファイルとして置く**(`go-gcp-sources.yaml`。YAML を書く段階で)。「全部入り」の選択は、そのファイルを `-rules` で足すだけ。
* 道具: **`param_type`**(型による source。`*net/http.Request` のように、型の引数が入力)、**`calls:`**(ADR-071。コールバックを呼ぶ API)、**`parts`**(ADR-074。要求の中の 1 つのフィールドが sink)、**`flows`**(伝播)、**`approximate`**(推測の流れ。理由つき)。

## 1. Pub/Sub

### 1.1 Go のパッケージの版(確かめた)

| | v1 | v2 |
|---|---|---|
| モジュール | `cloud.google.com/go/pubsub`(v1.50.2。**非推奨**。2026-12-31 まで修正のみ) | `cloud.google.com/go/pubsub/v2`(v2.6.0。別のモジュール) |
| 手書きのクライアント | `Client.Subscription(id)` → `Subscription.Receive`、`Client.Topic(id)` → `Topic.Publish` | `Client.Subscriber(nameOrID)` → `Subscriber.Receive`、`Client.Publisher(nameOrID)` → `Publisher.Publish` |
| 生成されたクライアント(`apiv1`) | `PublisherClient.Publish`、`SubscriberClient.Pull` / `StreamingPull` / `Acknowledge`(データの面) | `TopicAdminClient.Publish`、`SubscriptionAdminClient.Pull` / `StreamingPull` / `Acknowledge`(管理のクライアントが、データの面も持つ。`PublisherClient` と `SubscriberClient` はない) |
| メッセージの型 | `pubsub.Message` は、ルートのモジュール `cloud.google.com/go` の `internal/pubsub.Message` の**別名** | 同じ型の別名 |
| **IR が示す型の名前** | `*cloud.google.com/go/pubsub.Message` | `*cloud.google.com/go/pubsub/v2.Message` |
| 生成された protobuf の型 | `cloud.google.com/go/pubsub/apiv1/pubsubpb` | `cloud.google.com/go/pubsub/v2/apiv1/pubsubpb`(別の型) |

* **別名は、使った名前で IR に出る**が、照合は別名をほどいて行う(ADR-083)。別名の名前(`.../pubsub.Message`)でも、実体の名前(`.../internal/pubsub.Message`)でも当たる。v1 と v2 は別の型なので、`param_type` の行は別の 2 行のまま。メソッドのシンボルは実体の型の側(別名の名前では当たらない)。
* 2026-11-21 の時点で、v1 のサポートは 6 週間で終わる。それでも、既存のコードの多くが v1 なので、**両方の版に行を置く**(形は同じ。行は安い)。v2 を先に書く。
* `bench/gcp` に、どちらの版も、まだない(YAML の段で足す)。

### 1.2 表

「**確認**」の列: 実際に確かめたもの(`rivus analyze` を、Pub/Sub、Functions Framework、CloudEvents SDK を使う小さい試験用のプロジェクトに、`-rules` の source を足して走らせた。実際の Google Cloud には接続していない。プロジェクトは、まだリポジトリにない)。

| # | 何 | source / sink / 安全 | 道具 | 確認、注意 |
|---|---|---|---|---|
| P1 | `sub.Receive(ctx, func(ctx, m *pubsub.Message){…})` のコールバックの引数 `m` | **source**、`queue_message` | `param_type`(v1 と v2 の 2 行)。`calls:` は使わない(下) | 確認: v1(Data、Attributes)と v2 のコールバックで検出される。到達は `from_entry`。**ノイズ**: 型が決めるので、`func send(m *pubsub.Message)` のような**送る側の関数**の引数も source になる(確認: 検出された) |
| P2 | `m.Data`(`[]byte`) | 危険(トピックに外から書けるなら、攻撃者が決める) | P1 の一部(別の行は要らない) | `exec.Command("sh","-c",string(m.Data))` で検出される |
| P3 | `m.Attributes`(`map[string]string`) | 危険(キーも値も、プロデューサが決める) | P1 の一部 | 検出される。経路に `container_element_imprecision`(map の要素は区別しない) |
| P4 | `m.OrderingKey` | 危険(プロデューサが決める) | P1 の一部 | P1 と同じ |
| P5 | `m.ID`、`m.PublishTime`、`m.DeliveryAttempt` | 安全(基盤が付ける) | **分けない**(`param_type` は、構造体の全体を汚す) | 安全なフィールドを除く道具がない。許す(opt-in のラベルなので、判定役が見る) |
| P6 | `m.Ack()`、`m.Nack()` | 安全、流れなし | 行なし | 依存の本体が読まれる。効果なし |
| P7 | `Topic.Publish(ctx, msg)` / `Publisher.Publish(ctx, msg)` の `Message.Data` / `Attributes` / `OrderingKey` | **sink にしない** | 行なし | 宛先は topic の名前で、メッセージの中身は**データ**(読む側が危険になるが、読む側は rivus に見えない。6 つのカテゴリのどれにも当たらない) |
| P8 | `Client.Topic(id)` / `Client.Publisher(nameOrID)` / `Subscriber(nameOrID)` の名前 | **今は sink にしない**(下の判断 E) | 行なし | v2 の `Publisher` は、`/` で区切って 4 つなら、名前を**そのまま**使う(`projects/<別のプロジェクト>/topics/x`)。入力が名前なら、別のプロジェクトを指せる。カテゴリがない |
| P9 | Pull の形: `SubscriberClient.Pull`(v1)、`SubscriptionAdminClient.Pull`(v2)の結果(`ReceivedMessages[].Message`)、`StreamingPull().Recv()` | **source**、`queue_message` | `call_result`(結果が source)。`param_type` や `kind: field` は使えない | `pubsubpb.PubsubMessage` は送る側(`PublishRequest.Messages`)でも組み立てる型で、型やフィールドで source にすると、送る側が全部検出される。**未確認**(YAML の段で、ケースを足す) |
| P10 | `Publish` の生成されたクライアント: `PublishRequest.Messages[].Data` | sink にしない | 行なし | P7 と同じ |
| P11 | 管理: `CreateSubscription` / `ModifyPushConfig` / `UpdateSubscription` の `PushConfig.PushEndpoint`(v1 の手書きは `SubscriptionConfig.PushConfig.Endpoint`) | **sink**、`ssrf`(基盤が、その URL に、メッセージを送る。攻撃者の URL に流れる) | `parts`(`pubsubpb.Subscription` と `ModifyPushConfigRequest` の `PushConfig.PushEndpoint`。v1 と v2 で別のシンボル) | 低い優先。メンテナの使い方では少ない。**未確認** |
| P12 | **依存の中の時間**(grpc と gax の `invoke`) | source でも sink でもない。**費用の行** | なし(**モデルで依存の中を切らない**。ただし、ADR-092 の条件(費用の測定の記録、本体を読んだ結果との突き合わせの検査、`calls:`、注記つきの保守的な扱い、切ったことの表示)を満たせば、切ってよい。今は、パッケージごとに切る仕組みも、突き合わせの検査もない) | Spanner と BigQuery の `bench/gcp`: 42〜45 秒、3.4〜4.1 GB、時間の 99.7% が依存。grpc が 10.5 秒(2,259 回の再解析)、gax の `invoke` が 356ms。Pub/Sub を足したときの数字は、別のエージェントが測る。**速くする作業は、結果が同一であること**(`pure` を、実行で確かめずに足して、依存の本体を飛ばさない) |

**P1 の `calls:` の行(原因が分かった。確かめた)。** `Receive` の `calls:` 行を `fn: arg1`、`with: [recv]` と書くと、実際の Pub/Sub(v1、v2)で当たらなかった。**原因は、エンジンでもシンボルでもなく、行の書き方**: `with` は、コールバックの**パラメータを先頭から順に**埋める。コールバックは `func(ctx context.Context, m *Message)` で、`[recv]` は第 1 パラメータの `ctx` に入り、`m` には何も入らない。小さい自前のライブラリのコールバックは、パラメータが 1 つだったので当たった。`with: [arg0, recv]`(`arg0` は `ctx`)と書くと、v1 の `Subscription.Receive`、v2 の `Subscriber.Receive` ともに当たる(確認: 試験用のプロジェクトで、コールバックの中の `exec.Command("sh", "-c", string(m.Data))` と `m.Attributes[...]` に finding が出て、送る側の関数 `send(m *Message)` の `m.Data` には出ない = `param_type` のノイズがない)。モデルと本体は、`calls:` の行がある関数では、**モデルが勝ち**、依存の本体は読まれない(`Receive` の本体のゴルーチンやチャネルは関係しない)。組み込みの `calls:` の行(Go、Dart、Rust)は、どれもコールバックのパラメータが 1 つか、`with` が全部を埋めており、同じ書き間違いはない。**YAML の段で、`calls:` に乗り換える**(判断 B)。試験: `testdata/go/cases/callbacks` の `SecondParameter`、`FirstParameterDecoy`。

## 2. Functions と Cloud Run(イベントと HTTP の入口)

### 2.1 表

| # | 何 | source / sink / 安全 | 道具 | 確認、注意 |
|---|---|---|---|---|
| F1 | HTTP 関数 `func(w http.ResponseWriter, r *http.Request)`、`functions.HTTP(name, fn)` の登録 | **source**、`user_input` | **すでに入る**(`*net/http.Request` の `param_type`)。新しい行なし | 確認: `functions.HTTP` で登録した関数が、`from_entry` で検出される。`param_type` は「登録のしかたによらない」(型で入口になる) |
| F2 | CloudEvent 関数 `func(ctx, e cloudevents.Event) error`、`functions.CloudEvent(name, fn)`、`cloudevents.NewClientHTTP()` の `StartReceiver(ctx, fn)` の `fn` | **source**、`queue_message` | `param_type`: `github.com/cloudevents/sdk-go/v2.Event`(別名の名前。値の型で、`*` なし) | 確認: 検出される。`from_entry`。**ノイズ**: SDK の `Event` を渡す関数の引数が、全部 source(P1 と同じ) |
| F3 | `e.Data()` | 危険(F2 の一部) | F2 の一部。依存の本体が読まれる | 確認: 検出される |
| F4 | `e.DataAs(&v)` | 危険。`v` に入る | **`flows`**: `(github.com/cloudevents/sdk-go/v2/event.Event).DataAs` の `recv` から `arg0` | 確認: 行がないと**見逃す**(コーデックが関数の表を通る)。行を足すと検出される。メソッドのシンボルは `event.Event` の側(型の別名の名前とは別) |
| F5 | `e.Subject()`、`e.Source()`、`e.Type()`、`e.ID()`、`e.Extensions()` | 危険(Firestore のトリガの `Subject` は、ドキュメントのパスで、利用者が決める ID を含む) | F2 の一部(`Event` の全体が汚染される) | 分けない(P5 と同じ) |
| F6 | Pub/Sub のトリガのペイロード(`MessagePublishedData{Message{Data, Attributes}}`) | 危険(P2、P3 と同じ。`Data` は base64) | F2 と F4 の一部。型の行は要らない(ペイロードの型は利用者が書くか、`DataAs` の先) | 確認: `DataAs(&d)` で `d.Message.Data` に着く。base64 の復号は標準ライブラリのモデル |
| F7 | `functions.Typed(name, fn)`、第 1 世代の背景関数(`func(ctx, m PubSubMessage) error` を `RegisterEventFunctionContext` で登録) | 危険(`Typed` の引数は、リクエストの JSON から復号される) | **今の道具では、組み込みで書けない**: 引数の型が、利用者が決める型で、`fn` は `any` | 確認: **見逃す**(`Typed` の `fn` の引数)。利用者は、自分の型を `param_type` で書ける(自分のルールで)。組み込みには、「関数を登録する呼び出しが、渡された関数の引数を入口にする」道具がいる(判断 C) |
| F8 | `functions.HTTP` / `functions.CloudEvent` の登録そのもの(`Reachable`) | 流れなし | 行なし | 登録は、関数を内部のレジストリに入れる呼び出し。**入口は、登録ではなく、型で決まる**(F1、F2)。登録されていない関数でも、型が合えば入口(過検出側)。確認はしていない(`param_type` の定義から) |
| F9 | **Cloud Run** | **足すものはない** | なし | 素の `net/http`(`$PORT` で待つ)。Pub/Sub の push、Eventarc、Cloud Scheduler、Cloud Tasks の配信は、**ふつうの HTTP のリクエスト**として届くので、`user_input`(`queue_message` の区別はできない)。封筒(`{"message":{"data":…}}`)は、リクエストの本体の JSON |
| F10 | **依存の中の時間** | 費用の行 | なし | functions-framework は zap と CloudEvents だけを引く(軽い)。Pub/Sub と組み合わせたときは、P12 の grpc の費用 |

## 3. Storage / BigQuery の残り(予定)

表は、まだない。範囲(ユーザーの指定): Storage のオブジェクト名(`parts` の `Object.Name`、`path_traversal` ではなく、バケットの中の名前)、読み出し(`stored`)、BigQuery の `Job` の結果の読み出し、`Client.Dataset` / `Table` の名前、Spanner の `apiv1` の `ExecuteSqlRequest.Sql`。

## 4. Firestore(予定)

読み出し = `stored`(`database/sql` と同じ選択)。名前(`Doc` のパス)と `Where` のフィールド名が、sink の候補。

## 5. Firebase Auth(予定)

ID トークンの検証と、復号したクレーム。ラベルを決める(署名されたクレームの `uid` と `email` は、攻撃者が決める値ではない。カスタムクレームと、利用者が書いたプロフィールの欄は、信頼する値ではない)。Admin SDK のユーザー作成・更新が sink の候補。

## 6. 判断が要る点

| | 問い | 案 |
|---|---|---|
| A | `queue_message` を、組み込みの既定で入れるか | **入れない**(全製品で `stored` と同じ選択。ファイル 1 つ)。理由: トピックが内部専用なら、全部が偽陽性。組み込みには、sink と `flows` だけ |
| B | Pub/Sub の入口を、`param_type`(確認済み、ノイズあり)にするか、`calls:`(正確。`with: [arg0, recv]` と書けば実際の Pub/Sub で当たる。P1 の注)にするか | `calls:` に乗り換える(ノイズが消える)。YAML の段 |
| C | 「関数を登録する呼び出しが、渡された関数の引数を入口にする」道具(`functions.Typed`、第 1 世代の関数、`Receive` のような API を、利用者の型でも) | エンジンの変更(`flows` の `to:` のアクセスパスと並ぶ大きさ)。ADR が要る。入れるまで、利用者が自分の型を `param_type` で書く |
| D | **エンジンの見逃し**: 構造体を、**ポインタで**、読んでいない関数に渡し、あとで、**値で** sink に渡すと、何も報告しなかった | **直した**(ADR-084。7 節) |
| E | トピック、バケットなどの**名前**が入力のとき(別のプロジェクトのリソースを指せる)のカテゴリ | 今のカテゴリ(6 つ)に当たらない。新しいカテゴリ(`resource_name`)を作るか、作らないか。**作るまで sink にしない** |
| F | CloudEvent を `queue_message` にする(Eventarc は、Pub/Sub、Storage、Firestore と、HTTP の呼び出し元のどれでも運ぶ) | 同じラベルで始める。細かく分ける必要が出たら、ラベルを足す |

## 7. `bench/gcp` の今の出方(固定したもの)

`TestGCPRows`(`cmd/rivus/gcp_test.go`)と、`go test -tags gcpexec ./...`(`bench/gcp/exec_test.go`。実行で、値が本当に SQL に届くことを確かめる)が持つ。

実行の検査は、実際の Google の端点に接続しない。Spanner のクライアントを放っておくと、組み込みのメトリクスが `monitoring.googleapis.com:443` へ送り、GCP のリソース検出がメタデータサーバを探る。試験は、メトリクスを切り(`DisableNativeMetrics`、`SPANNER_DISABLE_BUILTIN_METRICS`)、検出(ライブラリは、切っても行う)の問い合わせ先を手元の待ち受け(`GCE_METADATA_HOST`)にし、`TestMain` が、ループバック以外への名前解決と HTTP の接続を、拒否して数え、1 つでもあれば失敗にする。

| ケース | 形 | 今の出方 |
|---|---|---|
| `VNewStatement`、`VNewStatementBare` | `spanner.NewStatement(src)`(連結あり、なし) | **`SQL` に細かく入って検出される。`field_imprecision` なし**(`no_uncertainty_on_path`)。依存の本体が読まれる。**`whole_value` ではない**。ビルダーが読まれない(本体がない)ライブラリでは `whole_value` になる。`flows` の `to:` にアクセスパス(段階 5)が入っても、これは変わらないことが、受け入れ |
| `URewrite` | `Statement` を値で、読まない関数に渡して、結果を使う | 検出される。`field_imprecision`(`whole_value`) |
| `UOpaque`、`UOpaqueAssign` | `Params` だけが汚染された `Statement` の**アドレス**を、読まない関数に渡し、`Statement` を値で `Query` に渡す。**実行では、値が SQL に届く** | 検出される。`field_imprecision`(`opaque_callee`)。もとはサイレントな見逃しだった(ADR-084: マークは変数に付き、値のコピーと要約を通して運ぶ)。同じ 8 形が `testdata/go/cases/fieldsink` にある |

見逃しの確認方法: 小さいプロジェクトで、依存なしに、同じ形を書いた(`testdata/go/cases/fieldsink` の `ValAfterPtr*`、`ValRewrite`、デコイ 2 つ)。今は、ポインタを渡して値で sink = `opaque_callee`、値を渡して値で返す = `whole_value`、デコイ(読まない関数を呼ばない、別の変数に渡す)= 検出されない。
