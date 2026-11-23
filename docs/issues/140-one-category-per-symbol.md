# 140: モデル: 1 つのシンボルに複数のカテゴリの sink の行を持てる仕組みは入った(PR #169)。`dart-gcp.yaml` の購読の名前の行(`resource_name`)の追加と、zoo のセルの `@miss` を外すことが残る

- 状態: **partial**
- 種類: 道具(重さ: 低)
- 言語: 全
- ROADMAP の節: 0
- 守るもの: zoo `F/dart/googleapis-pubsub-subscription-name`
- 題: モデル: 1 つのシンボルに複数のカテゴリの sink の行を持てる仕組みは入った(PR #169)。`dart-gcp.yaml` の購読の名前の行(`resource_name`)の追加と、zoo のセルの `@miss` を外すことが残る
- 補足(状態): **仕組みは直した**(PR #169。位置が違う行は、別のカテゴリの別の finding になる。同じ位置を読む別カテゴリの行は、従来どおり先頭の 1 つ)。残り: `dart-gcp.yaml` は、まだ購読の名前の行を置いていない(冒頭のコメントが「1 つのシンボルは 1 つのカテゴリ」のまま)ので、zoo `F/dart/googleapis-pubsub-subscription-name` は `@miss` のまま
- 見つけた経緯: `dart-gcp.yaml`(Google Cloud の Dart のクライアントのモデル)の測定と zoo のセルで見つかった(`docs/DESIGN-deps-cost.md` 12 節)。

`dart-gcp.yaml` の `ProjectsSubscriptionsResource.modifyPushConfig` / `create` / `patch` は、送り先(`pushConfig.pushEndpoint`、`ssrf`)と購読の名前(引数 1、`resource_name`)の両方が sink にしたい。同じシンボルに 2 つのカテゴリの行を書くと、1 つの sink にまとめられ、先頭の行のカテゴリで報告される(送り先の流れが `resource_name` で出た)。今は送り先(`ssrf`)だけを書き、購読の名前の行を置いていない。1 つのシンボルが、位置や `parts` ごとにカテゴリを持てるようにする(書式の課題)。

zoo: `F/dart/googleapis-pubsub-subscription-name` の `vuln`(`@miss`)。
