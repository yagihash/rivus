# #201 `goir.Extract` の、lifted を使わない経路(`callgraph.BuildGo` の fallback)が、テストからだけ使われて残っている

- 状態: **open**
- 種類: 道具(テストと本番の経路の違い)
- 言語: Go
- ROADMAP の節: -
- 守るもの: -

## 何が起きるか

`goir.Extract`(`internal/frontend/goir/goir.go`)は、候補(呼び出しの行き先)を、次の順で作る。

1. 読み込みの間に作った表(`LiftedTable`。本番の解析の経路)
2. `p.Lifted` から作る表
3. どちらも無いとき、`callgraph.BuildGo` で、naive 形のプログラムの呼び出しグラフから作る(fallback)

本番(解析の経路)は 1 を使う。3 は、テストが `gossa.Load` を lifted なしで呼んだときだけ通る(`cmd/rivus/dead_rows_test.go`、`internal/irssa/irssa_test.go`、`internal/sites/sites_test.go`、`internal/taint/helper_test.go`、`internal/frontend/accept/accept.go` など)。

そのため、テストが、本番とは違う経路で作った IR を検査していることになる。#425 で見つかった `$bound` のクロージャの候補の揺れも、`debug extract` がこの別の経路を通っていたことが原因だった。同じ種類の食い違いが、ほかのテストにも残っている可能性がある。

## 修正の案

- 経路を 1 つにする: テストも lifted の経路(`WithLifted` と `OnLifted = LiftedTable`)で読み込む。テスト用の読み込みの関数を 1 つにまとめるとよい。
- または、fallback(`callgraph.BuildGo` の呼び出し)を消す。lifted が無ければ、`Extract` はエラーにする。

どちらにするかは、相談役が決める。

## 順番

今の作業の順(メモリの作業)のあとに修正する。
