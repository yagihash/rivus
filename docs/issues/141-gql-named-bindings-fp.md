# 141: Dart: 定数の GQL の `namedBindings` に入力を入れる形が、`gqlQuery.queryString` の sink で報告される(`field_imprecision`)

- 状態: **open**
- 種類: 精度(誤検出。重さ: 低)
- 言語: Dart
- 見つけた経緯: `dart-gcp.yaml`(Google Cloud の Dart のクライアントのモデル)の測定と zoo のセルで見つかった(`docs/DESIGN-deps-cost.md` 12 節)。

`ProjectsResource.runQuery` の sink は `parts` の `gqlQuery.queryString`。リクエストの別のフィールド(`gqlQuery.namedBindings`、束縛する値)にだけ入力を入れても、`namedBindings` の Map の要素を区別しないため、汚染が `/gqlQuery/*` に広がり、`queryString` の sink が `field_imprecision`(`whole_value`)と `container_element_imprecision` つきで報告される。注記があるので判定役が見るが、誤検出になる。Map の要素と、構築した値のフィールドを区別するか、この形を `field_imprecision` の既知の誤検出として扱うかを決める。

zoo: `F/dart/googleapis-datastore-run-query` の `decoy`(`@fp`)。
