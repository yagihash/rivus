# #243 Go の標準ライブラリのコールバックの引数の数の表(`knownCallbackParams`、12 件)を、YAML の `params` に移す

- 状態: **open**
- 種類: 道具(重さ: 低)
- 言語: Go
- ROADMAP の節: -
- 守るもの: `TestCallbackParamsKnown`(`internal/model/callwarn_test.go`)
- 進める完了条件: D(1)

## 何が起きるか

`internal/model/callwarn.go` の `knownCallbackParams` が、`sort.Slice`、`slices.SortFunc`、`strings.Map` など 12 件の、コールバックの引数の数を、コードの表で持つ。`CallWarnings` が、`calls:` のエントリの `with` の数と比べて、助言を出す。解析の結果は変わらない。

## 直す方向の案

同じ関数の `calls:` は `internal/model/builtin/go-callbacks.yaml` にあり、エントリの `params`(`CallRow.Params`)で引数の数を書ける。12 件に `params` を書き、表を消す。`TestCallbackParamsKnown` は、YAML の値を go/types の署名と比べる形にする。一覧は [HARDCODED-APIS.md](../HARDCODED-APIS.md) の H1。
