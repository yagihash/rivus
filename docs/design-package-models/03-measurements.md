## 3. 測定

公開の対象だけ。同時の重い処理は 2 つまで。1 回 10 分で打ち切り(`-timeout 540s`)。測定の間、この機械は他の作業で負荷が高かった(`uptime` の load average は 10〜25。実行ごとに記録したが、wall は 1〜2 割ずれる)。比べるときは、関数の数、IR のバイト数、`summary_elems`、`duration_ms` を見る。最大 RSS は、`/usr/bin/time -l` の値(Dart は extractor の子プロセスを含み、JIT の起動で、軽い実行でも 1 GB 前後になるので、メモリの比較には向かない)。rivus は `origin/main` の `00ff84f` のビルド。

### 3.1 Go: AWS SDK for Go v2 の `service/s3` を使う小さいサーバ(合成)

`GetObject`、`PutObject`、`ListObjectsV2` を呼ぶ HTTP のサーバ(`config.LoadDefaultConfig` と `s3.NewFromConfig`)。直接・間接のパッケージは 318(標準ライブラリを含む)。リポジトリの外(`~/tmp/rivus-pkgmodels/s3demo/`)に作った。リポジトリに入れるなら、`bench/go-s3/` に 1 つの `main.go` と `go.mod` / `go.sum`(約 60 行)。ベンチとして入れる(定期の層だけ。7 節)。

| 実行 | 読み込み + IR | 解析 | 合計 wall | 最大 RSS | 関数 | IR | 収束 | finding |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 既定(依存込み) | 約 3.0 秒 | 541 秒で打ち切り(`timed_out`、最大の SCC 613、`summary_elems` 148 万。時間の 99% 以上が依存。`dep_spent_ms` 5,135,512(ワーカーの合計)に対し `own_spent_ms` 3) | 546 秒 | 12.1 GB | 6,440 | 73.1 MB | しない | 1(確信 `requires_review`、不確実性 9) |
| `-no-deps` | 約 3.0 秒 | 0 秒 | 3.0 秒 | 0.69 GB | 5 | - | する | 同じ 1 件(不確実性 2) |
| 試作: `service/s3` を宣言 | 約 1.7 秒 | 11.6 秒 | 13.6 秒 | 1.6 GB | 3,184 | 27.6 MB(-62%) | する | 同じ 1 件 |
| 試作: `service/s3` と `smithy-go` を宣言(推移的な 11 パッケージを落とす) | 約 1.5 秒 | 0.1 秒 | 1.7 秒 | 0.67 GB | 789 | 9.1 MB(-88%) | する | 同じ 1 件 |
| 試作: `s3`、`sts`、`sso`、`ssooidc`、`signin` を宣言(推移的な 52 パッケージを落とす) | 約 1.0 秒 | 1.9 秒 | 3.0 秒 | 0.58 GB | 886 | 6.3 MB(-91%) | する | 同じ 1 件 |

- 試作は、リポジトリの外のコピーで、`Load` の `packages.Load` のあと、宣言のあるパッケージ(と、宣言のあるパッケージを通らないと届かないパッケージ)の `Syntax` と `TypesInfo` を捨てたもの(環境変数で指定。リポジトリには入っていない。試作は接頭辞で一致させたので、`s3` の宣言は `s3/types` なども含む。本実装では、配下を指すには明示の書き方が要る)。`go/ssa` は、本体のない関数として扱う。モデルの行は足していないので、未モデルの呼び出し(`unmodeled_library_call`)が 4 → 8 に増えた。finding の ID は、既定(未収束)、`-no-deps`、試作の 3 つで同じ(`f1:b073e7791ce4d699a83b`)。
- **Go の extractor の時間は、もともと 2.5〜3 秒で、メモリは約 1.1 GB(`rivus debug extract` で依存込みの IR を書いたとき)。Go では、宣言の効果の大部分は、解析の時間とメモリ(12 GB → 0.6〜1.6 GB)で、読み込みの時間(3 秒)は、ほとんど変わらない。** 段階 (a)(解析側だけ)で、この合成の主な効果は出る。
- パッケージごとのコスト(既定。`-pass-stats` の `packages_top` と、IR のバイト数。IR は全体 6,443 関数、73.1 MB):

| パッケージ | 関数 | IR のバイト(全体比) | 解析の時間(ワーカーの合計) | `summary_elems` |
| --- | --- | --- | --- | --- |
| `aws-sdk-go-v2/service/s3` | 2,880 | 41.6 MB(56.9%) | 4,197 秒 | 1,057,498 |
| `service/signin` | 346 | 2.9 MB(4.0%) | 126 秒 | 53,141 |
| `service/sts` | 293 | 2.6 MB(3.6%) | 147 秒 | 49,209 |
| `aws-sdk-go-v2/config` | 281 | 2.5 MB(3.4%) | 0.1 秒 | 2,573 |
| `service/ssooidc` | 205 | 1.7 MB(2.4%) | 162 秒 | 30,052 |
| `service/sso` | 193 | 1.5 MB(2.1%) | 114 秒 | 29,353 |
| `smithy-go/transport/http` | 152 | 0.9 MB(1.3%) | 67 秒 | 54,122 |
| `aws-sdk-go-v2/aws/retry` | 116 | 0.7 MB(1.0%) | 21 秒 | 1,353 |

`service/s3` の 1 つの宣言で、IR が 62% 減り、解析が打ち切りの 541 秒から 11.6 秒になった。次の `smithy-go`(ミドルウェアのスタック: 動的な呼び出しの合流の元)を足すと、解析が 0.1 秒になる。

### 3.2 Dart(`bench/dart-gcp`、`bench/dart-grpc`。`dart pub get --offline`、コピーで実行)

内訳: `load` は extractor と IR の読み込みの合計(コアの `-v` の段階。extractor の中を分ける計時は、今はない)。

| 対象 | 実行 | load(extractor + IR) | 解析(`duration_ms`) | wall | 関数(コアが読む) | finding | 備考 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `dart-grpc` | 既定 | 18.8 秒 | 0.31 秒 | 19.3 秒 | 2,409 | 3 | `dep_spent_ms` 907、`own_spent_ms` 43。uncertainty の合計 293 |
| `dart-grpc` | `-no-deps` | 6.7 秒 | 0.01 秒 | 6.7 秒 | 307 | 3 | uncertainty 379 |
| `dart-grpc` | 試作: `grpc` を宣言 | 11.6 秒 | 0.07 秒 | 11.7 秒 | 1,034 | 3 | uncertainty 300。`grpc` が import する `http2`、`protobuf` なども、ほかから届かなければ読まない |
| `dart-gcp` | 既定 | 50.6 秒 | 1.37 秒 | 52.3 秒 | 4,593 | 19 | `dep_spent_ms` 5,207、`own_spent_ms` 142。uncertainty 738 |
| `dart-gcp` | `-no-deps` | 9.4 秒 | 0.01 秒 | 9.5 秒 | 570 | 19 | uncertainty 974 |
| `dart-gcp` | 試作: `googleapis` を宣言 | 22.3 秒 | 0.75 秒 | 23.1 秒 | 3,433 | 19 | uncertainty 787 |
| `dart-gcp` | 試作: `googleapis`、`googleapis_auth`、`gcloud`、`google_cloud_storage`、`google_cloud_protobuf`、`google_cloud_rpc` を宣言 | 25.0 秒 | 0.30 秒 | 25.4 秒 | 2,669 | 19 | uncertainty 833 |

- 試作は、extractor のコピーで、`visit`(import をたどる処理)が、宣言のあるパッケージのライブラリを見たら、そのファイルを追加せず、import もたどらないようにしたもの(環境変数で指定。リポジトリには入っていない)。finding は全部同じ件数。
- **Dart は、extractor が時間の大半(既定で 9 割以上)。** 宣言を extractor に渡す段階 (b) で、`dart-gcp` の load が 50.6 秒 → 22〜25 秒に減った(`-no-deps` の 9.4 秒が下限)。**これは試作の数字で、実装(7a 節)の実測は 14.6 → 10.7 秒、最大 RSS 1.59 → 1.23 GB(`dart-grpc` はほぼ同じ)。ライブラリの要素の構築が残り、試作のように `import` をたどらない版は、結果が変わるので入れなかった。**2 番目の宣言(6 パッケージ)で load が増えたのは、負荷のぶれの範囲(2 つの実行は別の時刻)。解析側だけ(段階 (a))では、解析 1.4 秒しか減らず、extractor の時間は変わらない。
- パッケージごとのコスト(`packages_top` の関数の数。コアが読む関数で、extractor が読むファイルの数ではない):

| パッケージ | `dart-gcp` の関数 | 解析の時間 | `dart-grpc` の関数 |
| --- | --- | --- | --- |
| `googleapis` | 1,105 | 385 ms | - |
| `protobuf` | 521 | 250 ms | 521 |
| `http2` | 407 | 365 ms | 407 |
| `collection` | 289 | 65 ms | 277 |
| `grpc` | 265 | 539 ms | 259 |
| `async` | 138 | 639 ms | 138 |
| `googleapis_auth` | 137 | 151 ms | - |
| `gcloud` | 121 | 62 ms | - |
| `google_cloud_storage` | 107 | 249 ms | - |

extractor が読む量(ファイル数)は、`docs/DESIGN-deps-cost.md` の 10 節(`googleapis` の使われない API のライブラリで 39 秒 / 3.7 GB)にある。この表は、コアが読む関数の数だけ。

### 3.3 「宣言すると、読み込みがどれだけ減るか」の見積もり

| 対象と宣言 | 時間(load) | メモリ | 関数 | IR |
| --- | --- | --- | --- | --- |
| Go の S3 合成、`service/s3` | ほぼ変わらない(3.0 → 約 1.7 秒) | 解析 12.1 GB → 1.6 GB | -51% | -62% |
| Go の S3 合成、`s3` と `smithy-go` | 変わらない | 12.1 GB → 0.67 GB | -88% | -88% |
| Go の S3 合成、5 つのサービス | 変わらない | 12.1 GB → 0.58 GB | -86% | -91% |
| Dart の `dart-gcp`、`googleapis` | 50.6 → 22.3 秒(-56%) | 測れていない(上の注) | -25% | - |
| Dart の `dart-gcp`、6 パッケージ | 50.6 → 25.0 秒(-51%) | 測れていない | -42% | - |
| Dart の `dart-grpc`、`grpc` | 18.8 → 11.6 秒(-38%) | 測れていない | -57% | - |

