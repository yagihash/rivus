# #193 解析する関数の集合が、実行ごとに変わる(本体を作らないはずの標準ライブラリの関数が、一部だけ入る)

- 状態: partial(関数の集合は閉じた。残りは「残り」の節)
- 種類: ADR-118 の (ii)(順序とマシンに依らない)の違反
- 言語: Go
- ROADMAP の節: -
- 守るもの: テスト `TestLoadFunctionSetIsStable`、`TestLoadFunctionSetIsClosed`
- 題: 解析する関数の集合が、実行ごとに変わる(本体を作らないはずの標準ライブラリの関数が、一部だけ入る)

## 何が起きるか

同じバイナリ・同じ入力でも、エンジンに渡す関数の数が実行ごとに変わる。memos(公開のプロジェクト)で 40781 から 40791 個。差は、標準ライブラリ(`skip` のパッケージ)の約 150 から 200 個の関数(sync、internal/sync、unique、crypto/internal/fips140/hmac、slices.xorshift など)が、入ったり入らなかったりすること。

## 原因

`internal/frontend/gossa/load.go` の関数のループは map の順で、中の `ownerPkg(fn)` が `fn.Origin()` を呼ぶ。ジェネリックの実体の中の無名関数(例 `sync.OnceValues[string, error]$1$1`)の `Origin()` は、元のジェネリック関数の本体を遅延して作り、その構築が、`sync.Pool` や `RWMutex` のメソッドの本体も連鎖して作る。`Origin()` より先に見た関数は「本体なし」として飛ばされ、あとに見た関数は入る。`ssa.BuildSerially` にしても同じ(並行の構築が原因ではない)。

## 説明と実際の動作のずれ

「標準ライブラリは本体を読まない」(`skip`)という説明に対して、実際には、go/ssa がジェネリックの実体や型のメソッドの本体を遅延して作るため、約 150 から 200 個の標準ライブラリの関数の本体が解析される。これを `summary` に出すべきかは、あとで判断する。

## 修正の概要

ループの前に、全関数に `Origin()` を呼んで関数の集合を取り直し、本体を持つ関数の数が変わらなくなるまで繰り返す(`allFunctions`。止める条件は、本体を持つ関数の数と、関数の総数の両方が変わらないこと)。memos で 4 回とも 40830 個、関数 ID の一覧が同一(差 0)。ロード時間とピークは変わらない。本体を持つ関数は +344、解析する関数は約 +45(標準ライブラリのメソッドが、必ず入る側に揃う)。

テスト: `internal/frontend/gossa/load_test.go` の `TestLoadFunctionSetIsStable`(同じ入力を 3 回読み込み、関数 ID の一覧が同一)、`TestLoadFunctionSetIsClosed`(`Origin()` を全関数に呼んでも本体が増えない)。

## 前提

ADR-118。

## 残り

- コールグラフ(`internal/callgraph/callgraph.go` の `AllFunctions(prog)`)は、`Load` のあとに呼ばれるので、閉じた集合を見るはず。ただし、コールグラフの構築(CHA / VTA)が、そのあとで新しい関数(メソッドのラッパー、bound、thunk)を作るか、作るなら、それが `p.Funcs` にない関数への辺になるか、は未確認。`p.Lifted` の側(`goir.go` の `AllFunctions(p.Lifted)`)が、`p.Funcs` と同じ関数 ID の一覧になっているかも、1 回比べる(一致しないなら別の課題)。
- memos の一致の確認は 2 回(`functions`、`summary_elems`、`reanalyses`、widening の数、finding の ID が一致)。ほかの測定のついでに、回数が増えたら、そのつど記録する。一致しない回が出たら、すぐ報告する。
- `TestDeterministic*`(main でだけ実行される)は、今回の非決定性を検出できなかった。推測: 対象が小さく、標準ライブラリのジェネリックの無名関数を通らないため。`TestLoadFunctionSetIsStable` が、その抜けを埋める(未確認の部分: 対象が小さいため、修正なしで失敗するのは、関数の数が変わる場合に限る。上の 3 回の読み込みで確認済み)。
- 「標準ライブラリは本体を読まない」という説明と実際の動作のずれを、`summary` に出すかは、あとで判断する。
