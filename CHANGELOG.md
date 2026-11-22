# Changelog

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
