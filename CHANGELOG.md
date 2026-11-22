# Changelog

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
