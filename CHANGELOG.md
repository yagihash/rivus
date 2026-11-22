# Changelog

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
