# #222 maturity-ledger を CI の週次ジョブにする

- 状態: **open**
- 種類: 道具・ベンチ
- 言語: 全
- ROADMAP の節: 1
- 守るもの: -

## 何が起きるか

`make maturity-ledger`(言語ごとの 4 つの指標を `bench/maturity-ledger.tsv` に 1 行ずつ足す。完了条件 G)は、いまは手元で週 1 回、人が動かしてコミットする。動かし忘れると、その週の行がない。

## 対処の案

CI の週次ジョブにする。そのまえに、Dart と Rust の extractor が CI で使えることを確かめる(Rust は、作業ツリーでビルドして `RIVUS_RUST` で指す。共有の場所には書かない)。行をコミットする方法(PR を自動で立てるか、artifact に出して人が写すか)も決める。`.github/workflows/` の変更なので、マージはユーザー。

Go の `no_edge`(`make observed-edges-go`)は、いまは `go vet` で止まって空になる。その直しは、この課題か、別の課題で扱う。
