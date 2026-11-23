# ADR-077: Google Cloud のクライアントを、メンテナの選択として組み込む。最初の範囲は Go の Cloud Spanner と BigQuery の SQL

- 状態: 一部覆った(→ADR-097)
- 関連: GCP-MODELS
- 状態の詳細: 「例外」という位置づけは、ADR-097 が、範囲の 1 つ目に置き換えた
- 要旨: Spanner / BigQuery の SQL

**背景:** 組み込みの範囲は、言語の標準ライブラリと、代わりのないパッケージまで(README「組み込みで持つ範囲」)。Google Cloud のクライアントは、そのどちらでもない。メンテナ(ユーザー)が Google Cloud を多く使うので、明示の**例外**として組み込む。**これは基準ではなく、ほかのライブラリを組み込む根拠にはならない**(ほかのベンダーのクライアントは、これまでどおり、ユーザーのルール)。要求を構造体で組み立てて 1 回の呼び出しで渡す形なので、`parts`(ADR-074)の最初の実例でもある(docs/DESIGN-field-sinks.md の段階 3)。

**決めたこと:**
1. 組み込みの `internal/model/builtin/go-gcp.yaml`(ルールの組 `rivus/go/gcp@1`)に置く。**ファイルを分けて、境界を見えるようにする**。冒頭のコメントと README に、例外であることと理由を書く。
2. **Cloud Spanner**(`cloud.google.com/go/spanner` v1.96.1、`/v2` はない): sink は、`Statement` を取る呼び出しの、`SQL` フィールドだけ(`parts`)。`txReadOnly` の `Query` / `QueryWithOptions` / `QueryWithStats` / `AnalyzeQuery`(`Single()`、`ReadOnlyTransaction()`、`ReadWriteTransaction` から呼ぶと、IR は宣言した型の `txReadOnly` で示す)、`ReadWriteTransaction` の `Update(WithOptions)` / `BatchUpdate(WithOptions)`(`[].SQL`)、`Client.PartitionedUpdate(WithOptions)`、`BatchReadOnlyTransaction.PartitionQuery(WithOptions)`。**`Params` は sink にしない**(バインド変数)。`spanner.NewStatement(sql)` は、行を足さない(依存の本体が読まれ、`SQL` に細かく入る)。読み出し(`Row.Columns` / `ColumnByName` / `ToStruct` / `ToStructLenient`)は、レシーバから出力引数への flows だけ。保存データの境界(`boundary: stored_data`)は、`database/sql` と同じく選択式で、入れない。
3. **BigQuery**(`cloud.google.com/go/bigquery` v1.86.3。`cloud.google.com/go/bigquery/v2` は別のモジュール(生成された低水準の API)で、このバージョンの対象外): `Client.Query(q)` の引数 0 と、`Query.Run` / `Query.Read` のレシーバの `QueryConfig.Q`(埋め込み 1 段 + 1)の**両方を sink にする**(設計の判断: `q.Q = x` は `Client.Query` を通らないので、`Run` / `Read` の側にしか見えない。`Run` に届く経路をエンジンが落としたときに何も報告しないより、重複のほうが軽い)。`Parameters` と `Labels` は安全な側。`Job` の結果の読み出しは入れない。
4. Dart と Rust の Google Cloud(`googleapis` など)、Cloud Tasks、Pub/Sub、Storage、Firestore、Firebase Auth は、この ADR の範囲外(あとの段階で、1 つの API 項目の表から作る)。

**確かめたこと(`bench/gcp`、`TestGCPRows`):**
- 公式のモジュールだけに依存する小さいプロジェクト(`go.mod` と `go.sum` をコミット)。**実際の Google Cloud には接続しない**(呼び出しを組み立てるだけで、送らない)。脆弱(`V*`)、直した(`F*`: `Params` / `Parameters`)、デコイ(`D*`: 別のフィールドだけが汚染される)。
- **死んだ行のゲート**: go-gcp.yaml の sink の全部の行(15)に、脆弱なケースの finding が 1 つ以上一致する。出ない名前のアクセスパス(`field_unresolved`)は 0。V の finding に `field_imprecision` はない(`BatchUpdate` の `[]` の要素は `container_element_imprecision`)。F と D は、finding が 0(`parts` がなければ、D は検出される)。
- zoo の「一致する行」のゲート(`TestBuiltinRowsAreHitByTheZoo`)には、go-gcp.yaml の行(19)を `bench/parity/dead-allow.txt` に、理由 `dependency`(zoo にはこの依存がない)で載せ、一致することの検査は `TestGCPRows` が持つ。
- 行の flows(`ColumnByName`、`ToStruct`)は、保存データを入れたときの finding の経路に、propagator として載る。
- **重複**: `Client.Query(q)` で組み立てて `Run` / `Read` するケース(2)は、1 つの流れが 2 件(`Client.Query` と `Run` / `Read`)。`q.Q = x` は `Run` の 1 件。まとめる報告の側の作業は、数が多くなってから(DESIGN-field-sinks.md の 6 節)。
- 時間とメモリ(`rivus analyze`、1 つの解析の間に別の解析が実行されていた Mac で、2 回): 42 から 45 秒、最大 3.4 から 4.1 GB。**依存の中の時間が 99.7%**(`dep_spent_ms` 106 秒に対して、プロジェクト自身は 0.27 秒)。20,785 関数(spanner と bigquery が、grpc、xds、monitoring、arrow などを引く)。上位は `google.golang.org/grpc`(10.5 秒、2,259 回の再解析)、`grpc/internal/xds/httpfilter/extproc`(7.7 秒)、`cloud.google.com/go/spanner`(4.0 秒)。遅い関数は、`grpclb.lbBalancer.refreshSubConns`(1 回 300 から 380ms)と、gax の `invoke`(356ms、出力 44)。1 つの動的な呼び出しに全部の操作が通る形(memos の AWS SDK)かは、まだ調べていない。**別の課題として報告**。

**限界:** (1) `Row.Columns(&a)` の flows は、可変長の出力引数の中身に届かない(`ColumnByName` と `ToStruct` は届く)。`flows` が可変長の宛先の要素を書けないエンジン側の限界(`bench/gcp` の `ReadColumns`、`TestGCPRows` の `gcpKnownMisses` で記録)。(2) `spanner/apiv1` の `ExecuteSqlRequest.Sql` と、データベース管理の DDL は、まだない。(3) バージョンが上がってメソッドの名前や置き場所が変わると、行は一致しなくなる: `TestGCPRows` が失敗する(モジュールを上げるときの検査)。(4) 重いテストで、モジュールのキャッシュが必要(`RIVUS_REQUIRE_GCP=1` で、ないことを失敗にする)。

