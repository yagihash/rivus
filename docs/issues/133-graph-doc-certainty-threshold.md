# 133: docs/GRAPH.md の「答えの読み方(要約)」の certainty の閾値が、コードと help と違う(3 と 20)

- 状態: **open**
- 種類: 文書(重さ: 低)
- 言語: 全
- 見つけた経緯: 利用ガイド(docs/AGENT-GUIDE.md)を書くときに、help とコードを読んで見つかった食い違い。v0.0.4 には入れない。

docs/GRAPH.md は閾値を 3 と書いているが、コードと help は 20(3 は callees の展開の閾値)。文書を直す。
