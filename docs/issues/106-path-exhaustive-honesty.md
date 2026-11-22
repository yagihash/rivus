# #106 graph: path の「経路なし」が、省いたものがあるのに exhaustive になる

- 状態: **closed**
- 種類: 誤った答え(直した)
- 言語: 全(graph)
- ROADMAP の節: 1
- 守るもの: `internal/graphq/exhaustive_test.go`

## 今の状態(2026-11-22 の見直し)

直した: 28a6395、9c315cc、794ae58、d216671、ad29644、93adcd7(`path` の「経路なし」の `exhaustive` は、何も省いていないときだけ。`GRAPH.md`、`DESIGN-graph.md` G-15)。

## 経緯

2026-11-22 に、直してから記録した(「直ってから記録する」)。再現の詳細は書かない。

