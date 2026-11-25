# #217 `graph-check` の zoo-go が毎回 exit 4 で終わり、完全な same と言えない

- 状態: **open**
- 優先度: 低
- 種類: 道具・ベンチ
- 言語: Go
- ROADMAP の節: -
- 守るもの: `python3 -m unittest bench.test_graph_check`
- 題: zoo-go の `--graph-check` は収束しないセルがあり、終了コード 4(収束だけが欠ける)で終わる

関連: #211(Dart の `--graph-check`。zoo-dart も同じ形)、docs/WORKFLOW.md(メモリとコストの変更は、グラフの表現が前後で同じことを示す)。

## 何が起きるか

zoo-go には、パス上限で止まるセル(`state/global_chain`)が意図して入っている。`rivus analyze` はそのセルで終了コード 4(解析が完全でない)になるので、`bench/compare.sh --graph-check zoo-go ...` は、7 行がすべて `same` でも `convergence: NOT CONVERGED` の行が付き、終了コード 4 で終わる(0 にならない)。

定期 CI の `graph-check` ジョブは、zoo-go の呼び出しだけ 4 を許す(`::notice::` を出す)。go-std10 と go-curated は 0 だけを合格にする。これは検査を緩める変更で、緩めた範囲は zoo-go の 4 だけ。

## 限界

zoo-go の結果は「止まった時点の状態が前後で同じ」ことまでしか示さない。止まらないはずのほかのセルが、将来止まるようになっても、4 のままなので、この検査では見つからない(zoo の期待値と `cmd/rivus` の `zoo_test.go` が見る)。完全な same とは言えない。

## 見直す条件

zoo-go の意図したセル(パス上限で止まるもの)を、別の仕組みで確かめられるようになったとき。そのセルを `--graph-check` の対象から分け、zoo-go 側で 0 だけを合格にできる。
