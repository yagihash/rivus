# 課題の一覧

課題は、リポジトリの中のファイルで持つ(GitHub の Issue は使わない)。一覧は、この `ISSUES.md`。1 件は `docs/issues/<番号>-<短い名前>.md`。

- **番号**: GitHub の番号を引き継ぐ(#74〜#99)。zoo の `@ref(#N)` と、コミットの `Zoo-Loosened:` の行が指すのは、この番号。**新しい課題の番号は、`docs/issues/` の最大の番号 + 1**(この文書に「次の番号」の行は持たない。PR のたびに衝突するため)。番号は相談役が付ける。100〜107 は、直してから記録したもの。#1〜#73 は、日次レビューの指摘(GitHub 側で閉じ済み。ここには写していない)。
- **番号を持たない課題**: モデルを作る PR が見つけた見逃しと誤検出は、`docs/issues/model-<パッケージ>-<名前>.md`(様式は [`issues/README.md`](issues/README.md))に書き、zoo の `@ref(model-<パッケージ>-<名前>)` から指す。**この一覧の表には、PR では載せない**(載せると、PR のたびに表の末尾で衝突する)。相談役が、あとでまとめて番号にして表に載せる。`TestIssuesFiles` が、番号の一意性、表と番号つきのファイルの一致、名前の形のファイルの節を確かめる。
- **状態**: `open` / `partial`(一部直った。残りを題に書く)/ `closed`(直した: コミットとテスト)/ `deferred`(後回し。理由と見直す条件)/ `wontfix`(やらない)。
- **誰が何をするか**: 中身と状態は相談役が決め、開発者が書いてコミットする。開発者が作業中に見つけたものは、相談役を待たずに `open` で足してよい(次の報告に番号を書く)。閉じるのは、相談役がレビューで確かめてから(開発者は「閉じられる」と報告する)。`deferred` と `wontfix` は相談役が決める。
- **公開のリポジトリの決まり**: 直していない rivus 自身の弱点(対象が意図して起こせるサイレントな見逃し、隔離の漏れ)の再現の詳細は、ここに書かない。直るまでは、相談役のレビューのファイル(`.claude/reviews/`。追跡されない)にだけ置き、直ってから `closed` で記録する。
- ROADMAP の各項目に、対応する課題の番号を並べ、終わりの条件に「これらを閉じる」を書く。

## 開いているもの

| 番号 | 題 | 種類 | 言語 | ROADMAP の節 | 状態 | 守るもの |
| --- | --- | --- | --- | --- | --- | --- |
| [#74](issues/74-path-widening.md) | 位置集合の不動点: 上限超過時のパスの widening と、locs() のソート回避(#18 の残り) | 道具・ベンチ(性能) | 全 | 後回し | open | - |
| [#75](issues/75-when-arg-type-via-variable.md) | when_arg_type の型の条件が、変数を経由した値で成り立たず、sink が見逃される | サイレントな見逃し | Go | 3 | open | - |
| [#76](issues/76-captured-vars-in-helper.md) | Dart / Go: ヘルパーに渡したクロージャがキャプチャした変数の読み書きが、ヘルパーの中の呼び出しで落ちる(サイレントな見逃し) | サイレントな見逃し | Dart / Go | 2 | partial | zoo `F/clos/*`、`bench/rust-curated` |
| [#77](issues/77-many-closures-to-helper.md) | Dart: 同じヘルパーに渡すクロージャが 9 種類以上だと、呼び出しが未解決になり、コールバックの中の sink・throw・書き込み… | 注記つきの見逃し(push 後) | Dart | 1 / 2 | partial | zoo `F/clos/helper-overflow`(32 まで)、`F/clos/helper-overflow-many`(33 以上。`@miss(depth_limit)`)、`cmd/rivus/dart_limit_test.go` |
| [#78](issues/78-returned-closure-captures.md) | Dart / Go: 関数が返したクロージャを呼ぶと、キャプチャした値と中の source が落ちる(カリー化、ファクトリ、functiona… | サイレントな見逃し | Dart / Go | 2 | open | zoo `F/clos/returned` |
| [#79](issues/79-callbacks-held-in-fields.md) | Dart / Rust: List やフィールドに持たれたコールバック(エミッタ、リスナー、遅延評価)の中の sink に、値が届かない | サイレントな見逃し | Dart / Rust | 2 | open | zoo `F/clos/stored`、`F/dyn/func-field` |
| [#80](issues/80-stdlib-callbacks-dart.md) | Dart: 標準ライブラリのコールバック(catchError、whenComplete、putIfAbsent、Map.forEach… | サイレントな見逃し | Dart | 2 / 3 | open | - |
| [#81](issues/81-callback-of-callback.md) | Dart: クロージャが、パラメータで受けた関数を呼ぶ形(g(a)。コールバックのコールバック)で、流れが失われる | サイレントな見逃し(一部) | Dart | 2 | partial | zoo `F/clos/callback-of-callback`(`@miss(depth_limit)`) |
| [#83](issues/83-rust-generic-vec-trait-call.md) | Rust: ジェネリックな T の Vec を for で回すと、trait 呼び出しが dynamic になり、モデルが receiv… | サイレントな見逃し | Rust | 2 | open | - |
| [#84](issues/84-gossa-load-instability.md) | go/ssa のロードの揺れ: 同じ入力で、関数の数が実行ごとに数件違う(stats.functions) | 道具・ベンチ | Go | 2(経路の欠け・揺れ) | open | - |
| [#85](issues/85-rust-fn-call-ref-self.md) | Rust: Fn / FnMut を &self で呼ぶと、レシーバがセルでクロージャ候補にガードが付かず、呼び出し元ごとに絞れない(誤検出) | 過検出 | Rust | 後回し | open | zoo(Rust)`F/clos/two-callers` |
| [#86](issues/86-rust-model-dynamic-read-impls.md) | Rust: モデルを適用した動的呼び出しは read_impls でないと実装の本体を読まず、本体の source の見逃しと、捨てる値の… | サイレントな見逃し | Rust | 2 | open | - |
| [#87](issues/87-dart-dynamic-invoke-model.md) | Dart: dynamic なメソッド呼び出しが dart:core#dynamic.invoke のモデルで覆われ、本体の sourc… | 注記なしの見逃し・誤検出(一部) | Dart | 2 | partial | `graph` の `exhaustive`(GRAPH-3 まで) |
| [#90](issues/90-global-path-missing-writer-step.md) | グローバルを通る流れの経路から書いた側の段が消える(経路が読んだ側から始まる)。同じ長さの経路は関数 ID の順で選ばれる | 誤った経路 | 全 | 2(経路の欠け・揺れ) | open | - |
| [#91](issues/91-graph-avoid-call-and-paths.md) | graph: --avoid-call の位置の寄せと複数の呼び出し点、位置なしのステップ、culprits が空のヒント、依存の絶対パ… | 道具・ベンチ | 全(graph) | 4 | open | - |
| [#92](issues/92-bench-scoring-silence.md) | ベンチの採点が、見逃しが何も報告していないかと、経路に source の関数が出ることを見ない | 道具・ベンチ | 全 | 1 | open | - |
| [#93](issues/93-bench-noisy-swap.md) | ベンチの @noisy(N) が入れ替わりに気づけない。curated が unsupported を見ない | 道具・ベンチ | 全 | 1 | open | - |
| [#94](issues/94-stats-unstable-misnamed.md) | stats.unstable が、1 回で収束した解析でも 10 件並ぶ(名前と中身が合わない) | 道具・ベンチ | 全 | 後回し | open | - |
| [#95](issues/95-graph-index-key-gaps.md) | graph の索引の鍵に、Dart の extractor のソース、Rust のワークスペースの Cargo.lock、-C の外の … | 道具・ベンチ | 全(graph) | 4 | open | - |
| [#96](issues/96-workers-output-difference.md) | -workers 1 と 2 以上で、8 関数以上の SCC を含む入力の出力(代表の経路、stats)が違う(R-43。直した) | 道具・ベンチ(直した) | 全 | 2(経路の欠け・揺れ) | closed | `cmd/rivus/determinism_test.go`(`TestDeterministicLargeSCC`) |
| [#98](issues/98-dart-tostring-edges.md) | Dart: 文字列補間と sinkAny が呼ぶ toString が、呼び出しのエッジ(グラフ)にならない | 道具・ベンチ(グラフのエッジ) | Dart | 1 | open | - |
| [#99](issues/99-dart-value-calls-zero-candidates.md) | Dart: 値として持たれた関数・クロージャ(フィールド、map、返したクロージャ、変数)の呼び出しが、候補 0 件のグラフのエッジになる | 道具・ベンチ(グラフのエッジ) | Dart | 1 | open | zoo `F/clos/*`、`TestZeroCandidateCalls`、`TestFuncValuesDart` |
| [#108](issues/108-model-row-loses-flows-silently.md) | モデルの行が、既定の扱いより流れを減らしても、何も報告しない | サイレントな見逃し(モデルの書き方の抜け) | 全 | 3 / 5 | open | `cmd/rivus/coverage_stub_test.go`(テンプレートのみ) |
| [#109](issues/109-stale-extractor-used-silently.md) | extractor(やイメージ)が古くても、何も報告せずに、その結果を出す | 誤った結果を何も報告せずに出す | Rust / Dart | 0 / 1 | open | -(直すときにテスト) |
| [#110](issues/110-sandbox-fetch-proxy.md) | 取得の段階のネットワークを絞る(宛先の許可リストと、解決したアドレスの検査) | サイレントな見逃しではない(隔離の漏れ) | Rust | 0 | open | - |
| [#111](issues/111-sandbox-bsd-inventory.md) | bsd.sb の許可の棚卸しと、変わったら失敗するテスト | 隔離の漏れの見落とし | Rust | 0 | open | - |
| [#112](issues/112-sandbox-metadata-and-userdata.md) | メタデータ(存在)が見える範囲と、ホームの外の利用者のデータのテスト | 隔離の漏れの見落とし | Rust | 0 | open | - |
| [#113](issues/113-sandbox-memory-process-watch.md) | メモリとプロセス数の監視 | 隔離の漏れ(DoS) | Rust | 0 | open | - |
| [#114](issues/114-sandbox-leftover-report.md) | 残ったかもしれないプロセスの報告(列挙だけ。kill しない) | 注記つきの限界 | Rust | 0 | open | - |
| [#115](issues/115-sandbox-profile-line-tests.md) | プロファイルの各行に、「拒否が実際に効いている」テストを対応させる | 隔離の漏れの見落とし | Rust | 0 | open | - |
| [#116](issues/116-sandbox-signal-same-sandbox.md) | シグナルを same-sandbox にする(cargo が子にシグナルを送れるように) | 隔離の仕上げ | Rust | 0 | open | - |
| [#117](issues/117-sandbox-terminal.md) | 端末(制御文字、/dev/tty) | 隔離の仕上げ | Rust | 0 | open | - |
| [#118](issues/118-sandbox-macos-ci.md) | macOS の CI のワークフロー(ツールチェインを入れて抽出まで)と、macOS の TestDogfood | CI | Rust | 0 | open | - |
| [#119](issues/119-setup-rust-polish.md) | rivus setup rust の作り込み(進み具合、更新、掃除) | 道具 | Rust | 0 | open | - |
| [#120](issues/120-sandbox-ancestor-cargo-config.md) | 祖先の .cargo/config.toml があると、抽出が止まる(読み取りだけ許す案) | 利用者を止める(閉じる向き) | Rust | 0 | open | - |
| [#121](issues/121-sandbox-review-sw6-sw7.md) | サンドボックスのレビューの小さい点(SW-6、SW-7) | 隔離の仕上げ | Rust | 0 | open | - |
| [#122](issues/122-ir-file-has-toolchain-path.md) | IR の `file` に、std のソースの絶対パス(ツールチェインの場所)が入る | 再現性(重さ: 低) | Rust | 0 | open | - |
| [#123](issues/123-graph-callers-skipped-languages.md) | graph callers / callees / dispatchers の答えが、読まなかった言語を示さない | サイレントな見逃し | 全(graph) | 0 | open | - |
| [#124](issues/124-resolve-func-values-sites-and-external-candidates.md) | `-resolve-func-values`: 補った候補がサイトの列挙に戻らない。本体のない候補では補った注記が出ない | 道具・ベンチ(重さ: 低) | 全 | 0 | open | - |
| [#126](issues/126-timeout-inside-large-scc.md) | `-timeout` の超過: 巨大な SCC の中で期限を見ない(並列ラウンドの中は直した) | 利用者を止める(直した) | 全 | 0 | closed | `cmd/rivus/passstats_top_test.go`(`TestTimeoutInsideALargeSCC`) |
| [#127](issues/127-timeout-overrun-remaining.md) | `-timeout` の超過の残り: 1 つの関数の中と、extractor の段階の時間が `stats` に出ない | 利用者を止める | 全 | 0 | open | - |
| [#144](issues/144-dependency-sink-reach-list.md) | 汚染された値が、依存の中の sink のルールに一致する呼び出しに届いたことを、finding とは別の一覧で出す(案) | 検出の見逃し(サイレント。案) | 全 | 0 | open | - |
| [#137](issues/137-dart-implementations-of-interface-sinks.md) | Dart で、インターフェースのメソッドの sink が、パッケージの外の実装の型の変数からの直接の呼び出しに届かない | 検出の見逃し(重さ: 低) | Dart | 0 | open | - |
| [#138](issues/138-suppressions-pure-flows.md) | `origin: llm` の `pure` と、流れを切る `flows` のモデルが流れを消しても、`suppressions` と `llm_influenced` に残らない | 正確さ(サイレントな見逃し) | Go | 0 | closed | `cmd/rivus/resolve_test.go`(`TestLLMPureModelSuppressionIsRecorded`) |
| [#136](issues/136-zsh-completion-equals-form.md) | zsh の補完で、`-lang=` のように `=` でつなぐ形のとき、値の一覧が出ない | 道具(重さ: 低) | 全 | 0 | open | - |
| [#139](issues/139-named-arg-sink.md) | Dart: 名前つき引数を指す sink の書式がない(`Query(kind: ...)` などの、位置でない引数が sink にできない) | 道具(重さ: 中) | Dart | 1 | open | zoo `F/dart/gcloud-datastore-query-kind` |
| [#140](issues/140-one-category-per-symbol.md) | モデル: 1 つのシンボルに複数のカテゴリの行を持てない(同じシンボルの行は足し合わされ、先頭のカテゴリになる) | 道具(重さ: 低) | 全 | 0 | open | zoo `F/dart/googleapis-pubsub-subscription-name` |
| [#141](issues/141-gql-named-bindings-fp.md) | Dart: 定数の GQL の `namedBindings` に入力を入れる形が、`gqlQuery.queryString` の sink で報告される(`field_imprecision`) | 精度(誤検出。重さ: 低) | Dart | 0 | open | zoo `F/dart/googleapis-datastore-run-query` |
| [#142](issues/142-go-gcp-resource-name.md) | Go の Google Cloud のモデルに、名前の sink(`resource_name`)を足す(Storage のバケットとオブジェクト、Pub/Sub のトピックと購読) | モデル(重さ: 中) | Go | 1 | open | - |
| [#146](issues/146-sandbox-target-triple.md) | Rust の出力に、読んだ構成(target triple)が出ない(macOS のサンドボックスは macOS 向けの分岐を読む) | 注記つきの限界 | Rust | 0 | open | - |
| [#147](issues/147-sandbox-workspace-crate.md) | ワークスペースの中の crate が、サンドボックスの下で抽出できない | 利用者を止める(閉じる向き) | Rust | 0 | open | - |
| [#148](issues/148-linux-rust-extractor-without-isolation.md) | Linux(CI、クラウドのモデル作成)での Rust の extractor の扱い(隔離なしでは、取得したクレートを実行しない) | 道具(重さ: 中) | Rust | 0 / 3 | open | - |
| [#156](issues/156-override-method-params-source.md) | 利用者がオーバーライドするメソッド(`Interceptor.onRequest` など)を、ライブラリが呼ぶ形のモデルの書式がない | サイレントな見逃し(書式の不足) | 全 | 2 / 3 | open | - |
| [#157](issues/157-dart-generic-method-model-diff.md) | Dart のジェネリクスのメソッド(`get<T>`)が、差分テストで検証できない | 道具(重さ: 中) | Dart | 3 | open | - |
| [#158](issues/158-service-client-sources-review.md) | 既存のサービスのクライアントのモデル(`dart-gcp` など)を、「その値を誰が書いたか」の基準(ADR-108)で見直す(候補: Pub/Sub のメッセージの本文) | モデル(重さ: 中) | 全 | 1 | open | - |
| [#159](issues/159-source-label-external-service.md) | source に、外部のサービスが運んだ、ほかの利用者の入力を区別するラベルがない | 書式(重さ: 低) | 共通 | 3 | open | - |
| [#160](issues/160-rust-async-closure-extractor.md) | Rust の extractor が、`google-cloud-auth` 1.19.0 を読むときに rustc の未実装(async closure)で落ちる(`-no-deps` では動く)。落ちた理由も利用者に伝わらない | 道具(Rust の extractor) | Rust | 3 | open | - |
| [#161](issues/161-rust-field-source.md) | Rust: `kind: field` の source が書けない(`fieldKeyOf` が `.` を含む型名しか扱わず、`::` 区切りの Rust の型名でキーが空になる)。Issue や PR の本文などが見逃しになる | 道具(重さ: 中) | Rust | 1 | open | zoo `F/pkg/octocrab/issue-body`(#224) |

## 閉じたもの

| 番号 | 題 | 種類 | 言語 | ROADMAP の節 | 状態 | 守るもの |
| --- | --- | --- | --- | --- | --- | --- |
| [#133](issues/133-graph-doc-certainty-threshold.md) | docs/GRAPH.md の「答えの読み方(要約)」の certainty の閾値が、コードと help と違う(3 と 20) | 文書(重さ: 低) | 全 | 0 | closed | `docs/GRAPH.md`(閾値は 20) |
| [#132](issues/132-setup-usage-source.md) | `rivus setup` の usage が `[-source …]` を任意に見せている(実際は必須) | 文書(重さ: 低) | Rust | 0 | closed | `cmd/rivus/setup.go`(usage) |
| [#130](issues/130-container-leftovers.md) | `-allow-fetch-host`、`-no-fetch` の help と `lang.go` のコメントに、廃止済みのコンテナの記述が残っている | 文書(重さ: 低) | Rust | 0 | closed | `cmd/rivus/analyze.go`、`lang.go`(help とコメント) |
| [#129](issues/129-tests-flag-help.md) | `-tests` の help が「`_test.go` を含める」だけ(Dart と Rust にも効く) | 文書(重さ: 低) | 全 | 0 | closed | `cmd/rivus/common.go`(`-tests` の help) |
| [#128](issues/128-sites-go-only.md) | `rivus sites` が Go だけを読む: Dart と Rust のディレクトリで、エラーなしの空の結果になる | サイレントな見逃し(重さ: 高) | 全 | 0 | closed | `cmd/rivus/dart_tests_default_test.go`(`TestSitesReadsTheLanguagesOfTheDirectory`) |
| [#82](issues/82-rust-mut-ref-writes.md) | Rust: クロージャや構造体のフィールドが持つ &mut 経由の書き込みが、元の変数に届かない(サイレントな見逃し) | サイレントな見逃し | Rust | 2 | closed | `bench/rust-curated/src/capture.rs`(`@miss` が外れた) |
| [#88](issues/88-dart-passed-closures.md) | Dart: 渡しただけのクロージャ(Timer.run、scheduleMicrotask、Future.delayed、Button(… | サイレントな見逃し | Dart | 1 | closed | zoo `F/dart/closure-to-sdk`(`Timer.run`、`scheduleMicrotask`、`Future.microtask`、`Future.delayed`、`Button(onPressed:)`) |
| [#89](issues/89-param-source-leaks-to-callers.md) | param の source の汚染が、呼び出し元へ書き戻り、source と関係のない関数の sink に届く(構造的な誤検出) | 過検出 | Go | 2 | closed | Go の最小の再現(`kind: param` の source) |
| [#97](issues/97-call-method-instance.md) | Dart: call メソッドを持つクラスのインスタンスを関数のように呼ぶと(r(x))、呼び出し先が未解決になる | サイレントな見逃し | Dart | 1 | closed | zoo `F/dart/callable-class`(`@miss` が外れた)、`cmd/rivus/dart_callable_test.go` |
| [#100](issues/100-dart-syntax-error-files.md) | Dart: 構文エラーのファイル | サイレントな見逃し(直した) | Dart | 1 | closed | `cmd/rivus/dart_syntax_test.go` |
| [#101](issues/101-dart-unresolved-constructor.md) | Dart: 解決できないコンストラクタ | サイレントな見逃し(直した) | Dart | 1 | closed | `cmd/rivus/dart_syntax_test.go`(`TestDartUnresolvedConstructorKeepsTheFlow`) |
| [#102](issues/102-dart-compile-error-functions.md) | Dart: コンパイルエラーの関数 | 注記なしの finding(直した) | Dart | 1 | closed | `cmd/rivus/dart_syntax_test.go` |
| [#103](issues/103-dart-missing-package-config.md) | Dart: package_config.json がない | サイレントな見逃し(直した) | Dart | 1 | closed | `cmd/rivus/dart_packageconfig_test.go`(`TestDartPackageWithoutPackageConfigIsSaid`) |
| [#104](issues/104-go-builtins-min-max-unsafe.md) | Go: 組み込み関数 min / max / unsafe | サイレントな見逃し(直した) | Go | 1 | closed | zoo `F/conv/builtin-minmax`、`F/conv/unsafe-string`、`cmd/rivus/go_builtin_test.go` |
| [#105](issues/105-rust-cmp-min-max-pure.md) | Rust: std::cmp::min / max の pure | 誤ったモデル(直した) | Rust | 5 | closed | zoo `F/conv/builtin-minmax`(Rust) |
| [#106](issues/106-path-exhaustive-honesty.md) | graph: path の exhaustive | 誤った答え(直した) | 全(graph) | 1 | closed | `internal/graphq/exhaustive_test.go` |
| [#107](issues/107-zoo-recursion-approximation-verdict.md) | zoo: recursion_approximation の判定 | 道具・ベンチ(直した) | 全 | 1 | closed | zoo のテスト(`cmd/rivus/zoo_*_test.go`) |
| [#125](issues/125-dart-parent-of-packages-dependency-ids.md) | Dart: 複数のパッケージの親を -C に指すと、依存が file: ID になり、解析されない(サイレント) | サイレントな見逃し(直した) | Dart | 1 | closed | `cmd/rivus/dart_depcontext_test.go` |
| [#131](issues/131-help-exit-codes.md) | `rivus setup rust -h`、`rivus models -h`、`rivus setup -h` の終了コードが 1(usage のエラー) | 道具(直した) | 全 | 0 | closed | `cmd/rivus/flagsides_test.go`(`TestHelpExitsZeroEverywhere`) |
| [#135](issues/135-index-taint-default-output.md) | `rivus index -taint` が `-o` を省くと、対象のディレクトリの `.rivus/index.json` に書く | 安全・文書(重さ: 中) | 全 | 0 | closed | `TestIndexTaintDefaultsToTheUserCache` |
| [#134](issues/134-graph-doc-rules-not-recorded.md) | docs/GRAPH.md の「index に渡したフラグは、与えなければ DB に記録されたものを使う」が `-rules` に当てはまらない | 文書または道具(重さ: 中) | 全 | 0 | closed | `TestGraphUsesTheRecordedRulesFile` |
| [#143](issues/143-model-template-result-first-only.md) | モデルの `to: result` は最初の戻り値だけ。テンプレートと組み込みの行が、error への流れを落とす | サイレントな見逃し(直した) | Go | 1 | closed | `TestGoRowsThatSayResultAreForOneResult`、`TestModelsCoverageStubsLoseNoFinding` |
| [#145](issues/145-named-in-flows-and-sources.md) | モデルの `flows` と `sources` に、名前つき引数を指す書式がない(`named:` は sink だけ) | 道具(重さ: 低) | Dart | 後回し | open | - |
| [#149](issues/149-callback-params-source.md) | ハンドラの引数が source になる書式がない(callback params): shelf_router の `router.get('/item/<id>', (Request r, String id) => ...)` の `id` | サイレントな見逃し(書式の不足) | Dart / Go / Rust | 2 / 3 | open | - |
| [#150](issues/150-body-default-unresolved-calls.md) | 差分テスト(標準ライブラリ): 本体を読む側が、解決できないインターフェース呼び出しを「引数が結果に流れる」と近似し、io.Copy / io.WriteString のエラーに見かけの流れが出る | 道具・ベンチ(差分テストの A 側の近似) | Go | 後回し | open | `bench/model-diff` の注釈 |
| [#151](issues/151-model-diff-blind-bytes-runes.md) | 差分テスト(標準ライブラリ): 本体を読む側は、バイトとルーンを 1 つずつ扱う関数で、入力が結果に届く流れを落とす(数として止める) | 道具・ベンチ(本体の分析の盲点) | Go | 後回し | open | `bench/model-diff` の注釈 |
| [#152](issues/152-model-diff-blind-reflect.md) | 差分テスト(標準ライブラリ): 本体を読む側は、reflect を通る書き込みを追えず、入力が書き込み先に届く流れを落とす | 道具・ベンチ(本体の分析の盲点) | Go | 後回し | open | `bench/model-diff` の注釈 |
| [#153](issues/153-model-diff-blind-any-switch.md) | 差分テスト(標準ライブラリ): 本体を読む側は、`any` の型 switch を通る入力を追えず、結果に届く流れを落とす | 道具・ベンチ(本体の分析の盲点) | Go | 後回し | open | `bench/model-diff` の注釈 |
| [#154](issues/154-model-diff-blind-map-conversion.md) | 差分テスト(標準ライブラリ): 本体を読む側は、マップ型の変換(`textproto.MIMEHeader(h)`)を通る書き込みを、元のマップに結ばない | 道具・ベンチ(本体の分析の盲点) | Go | 後回し | open | `bench/model-diff` の注釈 |
| [#155](issues/155-body-default-map-lookup-key.md) | 差分テスト(標準ライブラリ): 本体を読む側は、マップの検索のキーが、見つけた値に流れると近似する(Header.Get / Values) | 道具・ベンチ(差分テストの A 側の近似) | Go | 後回し | open | `bench/model-diff` の注釈 |
| [#163](issues/163-library-as-project-candidates.md) | ライブラリそのものを解析の対象にして、sink と source のモデルの候補を、根拠つきで出す(案) | 道具(モデル作成の候補の元) | Go が先 | 後回し | open | `docs/DESIGN-package-models.md` 6.11 |
