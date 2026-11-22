# 142: Go の Google Cloud のモデルに、名前の sink(`resource_name`)を足す(Storage のバケットとオブジェクト、Pub/Sub のトピックと購読)

- 状態: **open**
- 種類: モデル(重さ: 中)
- 言語: Go
- 見つけた経緯: `dart-gcp.yaml`(Google Cloud の Dart のクライアントのモデル)の測定と zoo のセルで見つかった(`docs/DESIGN-deps-cost.md` 12 節)。

Dart のモデル(`dart-gcp.yaml`)で、バケット、オブジェクト、トピック、購読、kind の名前を、新しいカテゴリ `resource_name` の sink にした(`docs/GCP-MODELS.md` 8 節。判断 E は、このカテゴリを作ったので、名前を sink にする)。Go の `go-gcp.yaml` にも、同じカテゴリで、名前の sink を足す: `cloud.google.com/go/storage` の `Client.Bucket`、`BucketHandle.Object`、`pubsub` の `Client.Topic`、`Subscription`(v1 と v2。`Client.Publisher`、`Client.Subscriber`)など。`GCP-MODELS.md` の 1 節の P8 と、3 節。Go の `bench/gcp` に、名前の流れの a / b のケースを足して確認する。
