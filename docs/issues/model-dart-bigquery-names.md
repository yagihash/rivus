# BigQuery のプロジェクト、データセット、テーブルの名前が、resource_name の行にない

- 状態: **deferred**(型を単位に一括採用する形が決まってから)
- 種類: サイレントな見逃し
- 言語: Dart
- ROADMAP の節: -
- 守るもの: -

## 何が見逃されるか(または、何が誤って報告されるか)

`googleapis` の `bigquery/v2` で、`DatasetsResource` と `TablesResource` などの名前の引数(`datasetId`、`tableId`、`projectId`)に、利用者が決めた値が入っても、行がないので、見逃される。`JobsResource` の `jobId` と、SQL の文字列(`JobsResource.query`、`JobsResource.insert`)は、行がある。

## 原因

`make model PKG=googleapis LANG=dart` が候補として出した 44 件を、`dart-gcp.yaml` の `deferred:` に置いてある。`docs/GCP-MODELS.md` の 8.4 節にあるとおり、名前の API が多いため、一般の形(`JobsResource`)だけを先に行にした。型を単位に一括採用する形が決まってから、行にするかを決める。

## 再現するセル

なし。行を足す PR で、`bench/zoo-dart` にセルを足す。足したら、`deferred:` の該当の項目を消す。
