# Changelog

## [v0.0.10](https://github.com/yagihash/rivus/compare/v0.0.9...v0.0.10) - 2026-11-23

### Features
- feat(model): まとめての採用(adopt_rules)の書式とローダー(段階 1) by @yagihash in https://github.com/yagihash/rivus/pull/267
- feat(model): まとめた行の decision に、重なった決定の id を全部書く by @yagihash in https://github.com/yagihash/rivus/pull/270
- ci: test のジョブを、arm64 の Linux のランナーで動かす by @yagihash in https://github.com/yagihash/rivus/pull/274
- perf(test): 重いテストの、同じ解析や exe のコンパイルの繰り返しを減らす by @yagihash in https://github.com/yagihash/rivus/pull/275

## [v0.0.9](https://github.com/yagihash/rivus/compare/v0.0.8...v0.0.9) - 2026-11-23

### Features
- feat: Rust でも summary.unverified_versions を出す(Cargo.lock から。#162 を閉じる) by @yagihash in https://github.com/yagihash/rivus/pull/237
- feat(model): make model に -gen-skeleton を足す(Go の骨組みとセルのテンプレートを生成する。段階 2) by @yagihash in https://github.com/yagihash/rivus/pull/238
- feat: Dart でも summary.unverified_versions を出す(pubspec.lock から。#162 の Dart の分) by @yagihash in https://github.com/yagihash/rivus/pull/236
- feat(model): 骨組みの生成を、モデルの行が指すものだけに絞る by @yagihash in https://github.com/yagihash/rivus/pull/241
- feat(model): モデルを作る手順のコマンド(段階 1。Go だけ)と rejected: の節 by @yagihash in https://github.com/yagihash/rivus/pull/233
- feat(model): dio の MultipartFile.fromFile と fromFileSync を path_traversal の sink にする by @yagihash in https://github.com/yagihash/rivus/pull/245
- ci: rust のジョブのテストを -v で実行する(テストごとの時間を測るため) by @yagihash in https://github.com/yagihash/rivus/pull/250
- perf: Dart の extractor を、コンパイルした実行ファイルで動かす(テストと利用者の起動の時間を減らす) by @yagihash in https://github.com/yagihash/rivus/pull/252
- feat(rust): 宣言のあるパッケージの本体を extractor が読まない(Rust の段階 4) by @yagihash in https://github.com/yagihash/rivus/pull/254
- chore: make check に TestDogfood を足す(capture_unbound の上限を、push の前に知る) by @yagihash in https://github.com/yagihash/rivus/pull/253
- feat(model): make model を Dart にも広げる(段階 1) by @yagihash in https://github.com/yagihash/rivus/pull/243
- feat(model): dart の googleapis の BigQuery で、テーブルのビュー、マテリアライズドビュー、ルーチンの SQL を sql_injection の sink にする by @yagihash in https://github.com/yagihash/rivus/pull/257
- feat: analyze にカテゴリで finding を含める・外すフラグを足す(外した件数は summary に出す) by @yagihash in https://github.com/yagihash/rivus/pull/259
- test(model): dart github の RepositorySlug の見逃しを @miss のセルにする by @yagihash in https://github.com/yagihash/rivus/pull/258
- ci: test、dart、rust のジョブを 1 種類のシャードにまとめ、どのテストも 1 回だけ実行する by @yagihash in https://github.com/yagihash/rivus/pull/260
### Bug Fixes
- fix: 確認していないバージョンの照合を、言語ごとの記録とだけ行う by @yagihash in https://github.com/yagihash/rivus/pull/240
- fix: サンドボックスの下で Rust のビルドスクリプトが動くように、DEVELOPER_DIR をコマンドラインツールに向ける by @yagihash in https://github.com/yagihash/rivus/pull/256

## [v0.0.8](https://github.com/yagihash/rivus/compare/v0.0.7...v0.0.8) - 2026-11-23

### Features
- feat: 差分テスト(段階 1)。bench/model-diff と make model-diff(Go と Dart) by @yagihash in https://github.com/yagihash/rivus/pull/186
- feat: 宣言のあるパッケージの本体を、extractor が読まない(Go と Dart。段階 4、ADR-105) by @yagihash in https://github.com/yagihash/rivus/pull/191
- fix: body_replaced_packages の functions の意味を段階で変えない(functions_declared を分ける)。ADR-105 と設計の実測を直す by @yagihash in https://github.com/yagihash/rivus/pull/193
- feat: モデルのファイルの verified_versions を、ローダーのフィールドにする(rivus models versions) by @yagihash in https://github.com/yagihash/rivus/pull/192
- feat: source の新しい種類 callback_param(渡された関数の引数が source。段階 1、ADR-106) by @yagihash in https://github.com/yagihash/rivus/pull/197
- feat(zoo): 外部のパッケージのセルを、ディレクトリから列挙する(Go と Rust)。Dart の pubspec を生成物にする by @yagihash in https://github.com/yagihash/rivus/pull/198
- shelf_router: ハンドラの追加の引数を callback_param の source にする(#181 の @miss を解消) by @yagihash in https://github.com/yagihash/rivus/pull/199
- feat: callback_param が引けなかった登録を summary に出す(callback_param_unresolved) by @yagihash in https://github.com/yagihash/rivus/pull/200
- feat(model): dart dio の組み込みモデル by @yagihash in https://github.com/yagihash/rivus/pull/202
- feat(model): go gin の組み込みモデル(例から昇格) by @yagihash in https://github.com/yagihash/rivus/pull/208
- feat(zoo): 外部のパッケージの骨組みが zoo-mark で記録する(Go と Rust) by @yagihash in https://github.com/yagihash/rivus/pull/209
- feat(model): dart shelf のモデルを広げる by @yagihash in https://github.com/yagihash/rivus/pull/207
- feat: パッケージの calls: の行を、実行でなく zoo のセルで確認する(checked_by: cell) by @yagihash in https://github.com/yagihash/rivus/pull/206
- feat(model): dart github の組み込みモデル by @yagihash in https://github.com/yagihash/rivus/pull/212
- feat(model): rust actix-web の組み込みモデル(例から昇格) by @yagihash in https://github.com/yagihash/rivus/pull/204
- feat(model): rust axum のモデルを広げる by @yagihash in https://github.com/yagihash/rivus/pull/213
- feat(model): go cloud.google.com/go/storage の組み込みモデル by @yagihash in https://github.com/yagihash/rivus/pull/214
- feat(model): go jmoiron/sqlx のモデルを広げる by @yagihash in https://github.com/yagihash/rivus/pull/216
- feat(model): rust google-cloud-storage の組み込みモデル by @yagihash in https://github.com/yagihash/rivus/pull/217
- feat(model): dart googleapis_auth の組み込みモデル by @yagihash in https://github.com/yagihash/rivus/pull/222
- feat(model): rust octocrab の組み込みモデル by @yagihash in https://github.com/yagihash/rivus/pull/224
- fix(sites): Rust の型名(`::` 区切り)で `kind: field` の source のキーを作る(#161) by @yagihash in https://github.com/yagihash/rivus/pull/230
- feat(zoo): Dart の骨組みを依存として読む(--dep-dir)。dart-github の kind: field の source を検出にする by @yagihash in https://github.com/yagihash/rivus/pull/231
- feat(rust): extractor が変換で落ちた関数を飛ばして続ける(fn_skipped、summary.functions_skipped、ADR-109。課題 #160 の (b)) by @yagihash in https://github.com/yagihash/rivus/pull/226
- feat(model): Go のモデルの行に、module の path の major を `/v*` と書けるようにする by @yagihash in https://github.com/yagihash/rivus/pull/228
- feat: 確認していない major / バージョンに当てたとき、summary.unverified_versions に出す(Go。#228 のあと) by @yagihash in https://github.com/yagihash/rivus/pull/229
- feat(model): go google/go-github の組み込みモデル by @yagihash in https://github.com/yagihash/rivus/pull/225
- # chore: rust-web.yaml をパッケージごとのファイルに分ける by @yagihash in https://github.com/yagihash/rivus/pull/235
### Bug Fixes
- test: dead-allow の「一致した」指摘を、通常は記録だけにする(モデルの PR が共有の .dead-allow を直さなくてよい) by @yagihash in https://github.com/yagihash/rivus/pull/187
- fix(model): 差分テストで見つかった標準ライブラリのモデルの MISS を直す(json、url、flag、template ほか)。miss-reviewed の注釈 by @yagihash in https://github.com/yagihash/rivus/pull/195
- fix(model): 差分テストの続き。miss-reviewed を種類つきに絞る、写しのビルド(wasm)の修正、標準ライブラリのモデルの追加の修正(課題 #150〜#155) by @yagihash in https://github.com/yagihash/rivus/pull/201
- fix(model): database/sql の Conn と Tx.Prepare の SQL 文字列を sink にする by @yagihash in https://github.com/yagihash/rivus/pull/219
- fix(rust): 依存のクレートのコンパイルで落ちたとき、クレート名とバージョン、依存の側で落ちたこと、-no-deps を出す(課題 #160 の (a)) by @yagihash in https://github.com/yagihash/rivus/pull/220
- feat(model): 標準ライブラリ os の、行がない関数に行を足す by @yagihash in https://github.com/yagihash/rivus/pull/223

## [v0.0.7](https://github.com/yagihash/rivus/compare/v0.0.6...v0.0.7) - 2026-11-22

### Features
- feat: -pass-stats で、finding の注記が集中するシンボルの上位と、動的な呼び出しの候補の数の分布を出す by @yagihash in https://github.com/yagihash/rivus/pull/156
- feat(model): package:http の Client の組み込みモデル(dart-http.yaml)と、委譲のクリーク全体をモデルで切る基準(ADR-098) by @yagihash in https://github.com/yagihash/rivus/pull/160
- feat(model): Google Cloud の Dart のクライアントの組み込みモデル(dart-gcp.yaml: Storage、Pub/Sub、Datastore、BigQuery の sink) by @yagihash in https://github.com/yagihash/rivus/pull/162
- feat(model): 名前つき引数を指す sink(named:)と、1 つのシンボルの複数カテゴリの sink(課題 #139、#140) by @yagihash in https://github.com/yagihash/rivus/pull/169
- feat: summary.dependency_sites で「依存の中の source と sink は探していない」ことを言う(ADR-099)。言い回しの規則の追加 by @yagihash in https://github.com/yagihash/rivus/pull/165
- feat: finding に categories(同じ位置を読む行の全部のカテゴリ)を足す。flows / sources の named を課題 #145 にする by @yagihash in https://github.com/yagihash/rivus/pull/176
- chore: モデルの PR が並んで出ても conflict しない置き方(marks.d、dead-allow の分割、cases.dart の生成、課題の model-*)と、ゲートの @ref の確認 by @yagihash in https://github.com/yagihash/rivus/pull/178
- feat: パッケージ単位の宣言(body_replaced_packages)。モデルが代わりをするパッケージは、解析の前に本体を落とす(段階 2、ADR-100) by @yagihash in https://github.com/yagihash/rivus/pull/179
### Bug Fixes
- fix: origin が builtin でないモデルの pure と、結果に届かない flows が流れを消した場所を、suppressions に記録する by @yagihash in https://github.com/yagihash/rivus/pull/161
- fix: models coverage のテンプレートと組み込みの行で、複数の戻り値の関数に to: results を使う(to: result は最初の戻り値だけ) by @yagihash in https://github.com/yagihash/rivus/pull/163
- fix: models coverage のテンプレートで、インターフェース、slice、map のレシーバへの書き込みの行も書く by @yagihash in https://github.com/yagihash/rivus/pull/164
- fix(ci): zoo-loosened を merge-base との差分で見て、新しいセルの @sink(none) / @miss / @fp の行の追加は承認の行を求めない by @yagihash in https://github.com/yagihash/rivus/pull/167

## [v0.0.6](https://github.com/yagihash/rivus/compare/v0.0.5...v0.0.6) - 2026-11-22

### Features
- feat: -pass-stats の summary_sizes に、source の箇所の広がりと、グローバルが持つ要素の数を足す by @yagihash in https://github.com/yagihash/rivus/pull/153
- feat: rivus completion zsh(コマンドの表から生成する zsh の補完。サブコマンドとフラグに自動で追従する) by @yagihash in https://github.com/yagihash/rivus/pull/155
### Bug Fixes
- fix: -h はどのコマンドでも終了コード 0 にする(課題 #131) by @yagihash in https://github.com/yagihash/rivus/pull/152

## [v0.0.5](https://github.com/yagihash/rivus/compare/v0.0.4...v0.0.5) - 2026-11-22

### Features
- feat: index -taint の既定の書き先を利用者のキャッシュにする(互換性を壊す)と、graph が記録された -rules を使う by @yagihash in https://github.com/yagihash/rivus/pull/149
- feat: -pass-stats に、要約の大きさの分布と、要素の内訳(stats.summary_sizes)を足す by @yagihash in https://github.com/yagihash/rivus/pull/150
### Bug Fixes
- fix: rivus sites が analyze と同じ言語の選択を通す(Dart と Rust のディレクトリが空の結果にならない) by @yagihash in https://github.com/yagihash/rivus/pull/145
- fix(dart): dart pub get をしていない(または古い)パッケージを、標準エラーの専用の 1 行で言う by @yagihash in https://github.com/yagihash/rivus/pull/151

## [v0.0.4](https://github.com/yagihash/rivus/compare/v0.0.3...v0.0.4) - 2026-11-22

### Features
- feat: -resolve-func-values(候補 0 件の値の呼び出しに、関数値のパスで候補を補う。既定 off) by @yagihash in https://github.com/yagihash/rivus/pull/120
- feat: -resolve-func-values で補った辺を索引に入れ、-timeout を全部の回で共有する by @yagihash in https://github.com/yagihash/rivus/pull/126
- feat: -pass-stats-top と -pass-stats-sort、最大の SCC のパッケージ別の内訳、巨大な SCC の中でも -timeout を効かせる by @yagihash in https://github.com/yagihash/rivus/pull/135
- feat: rivus extract を rivus debug extract に移し、-h は普段使うフラグだけを出す by @yagihash in https://github.com/yagihash/rivus/pull/136
### Bug Fixes
- fix(dart): 複数のパッケージの親を -C に指しても、依存が package: の ID で解析される(サイレントな読み落とし) by @yagihash in https://github.com/yagihash/rivus/pull/133
- fix(dart): 許可の外の依存を読まなかったことを、summary.unread と標準エラーに出す by @yagihash in https://github.com/yagihash/rivus/pull/139
- fix(dart): 同じ依存の 2 つのバージョンの報告に、どちらが読まれて解析されたかを書く by @yagihash in https://github.com/yagihash/rivus/pull/140

## [v0.0.3](https://github.com/yagihash/rivus/compare/v0.0.2...v0.0.3) - 2026-11-22

## [v0.0.2](https://github.com/yagihash/rivus/compare/v0.0.1...v0.0.2) - 2026-11-22

### Features
- feat: -lang なし(auto)でも、聞いたうえで Rust を読む。読まなかった言語は結果に残す by @yagihash in https://github.com/yagihash/rivus/pull/107
### Bug Fixes
- fix(taint): -workers 1 も SCC を回ごとに解析し、出力が -workers に依らないようにする(#96) by @yagihash in https://github.com/yagihash/rivus/pull/101
### Dependencies
- ci: Renovate を設定する(Go・Rust・Dart の依存と Actions) by @yagihash in https://github.com/yagihash/rivus/pull/106

## [v0.0.1](https://github.com/yagihash/rivus/commits/v0.0.1) - 2026-11-22
