# 140: モデル: 1 つのシンボルに複数のカテゴリの行を持てない(同じシンボルの行は足し合わされ、先頭のカテゴリになる)

- 状態: **open**
- 種類: 道具(重さ: 低)
- 言語: 全
- ROADMAP の節: 0
- 守るもの: zoo `F/dart/googleapis-pubsub-subscription-name`
- 題: モデル: 1 つのシンボルに複数のカテゴリの行を持てない(同じシンボルの行は足し合わされ、先頭のカテゴリになる)
- 見つけた経緯: `dart-gcp.yaml`(Google Cloud の Dart のクライアントのモデル)の測定と zoo のセルで見つかった(`docs/DESIGN-deps-cost.md` 12 節)。

`dart-gcp.yaml` の `ProjectsSubscriptionsResource.modifyPushConfig` / `create` / `patch` は、送り先(`pushConfig.pushEndpoint`、`ssrf`)と購読の名前(引数 1、`resource_name`)の両方が sink にしたい。同じシンボルに 2 つのカテゴリの行を書くと、1 つの sink にまとめられ、先頭の行のカテゴリで報告される(送り先の流れが `resource_name` で出た)。今は送り先(`ssrf`)だけを書き、購読の名前の行を置いていない。1 つのシンボルが、位置や `parts` ごとにカテゴリを持てるようにする(書式の課題)。

zoo: `F/dart/googleapis-pubsub-subscription-name` の `vuln`(`@miss`)。
