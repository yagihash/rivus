# 139: Dart: 名前つき引数を指す sink の書式がない(`Query(kind: ...)` などの、位置でない引数が sink にできない)

- 状態: **open**
- 種類: 道具(重さ: 中)
- 言語: Dart
- ROADMAP の節: 1
- 守るもの: zoo `F/dart/gcloud-datastore-query-kind`
- 題: Dart: 名前つき引数を指す sink の書式がない(`Query(kind: ...)` などの、位置でない引数が sink にできない)
- 見つけた経緯: `dart-gcp.yaml`(Google Cloud の Dart のクライアントのモデル)の測定と zoo のセルで見つかった(`docs/DESIGN-deps-cost.md` 12 節)。

sink の行は位置(`args`、`recv`)か、リクエストの 1 つのフィールド(`parts`)でしか指せない。Dart の API は名前つき引数が多く(`gcloud` の `Query(kind: ...)`、`Bucket.list({prefix})`、`PubSub.createSubscription(..., endpoint:)`)、名前だけが入力になる呼び出しが、`dart-gcp.yaml` で拾えない。`named: kind` のような書式で、呼び出しの名前つき引数を指せるようにする。

zoo: `F/dart/gcloud-datastore-query-kind` の `vuln`(`@miss`)。モデルの側は `docs/GCP-MODELS.md` 8 節。
