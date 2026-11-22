# 131: `rivus setup rust -h`、`rivus models -h`、`rivus setup -h` の終了コードが 1(usage のエラー)

- 状態: **open**
- 種類: 道具(重さ: 低)
- 言語: 全
- 見つけた経緯: 利用ガイド(docs/AGENT-GUIDE.md)を書くときに、help とコードを読んで見つかった食い違い。v0.0.4 には入れない。

ほかのコマンドの `-h` は終了コード 0 だが、`rivus setup rust -h` は `flag: help requested` を出して 1、`rivus models -h` と `rivus setup -h` も usage のエラー(1)で終わる。`-h` は 0 で、usage を標準出力か標準エラーに出す形にそろえる。
