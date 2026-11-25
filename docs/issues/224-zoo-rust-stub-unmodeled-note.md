# #224 zoo-rust のスタブが、全セルに `unmodeled_library_call` を付け、`@miss` の `trace` と `silent` の区別を壊している

- 状態: **open**
- 優先度: 中
- 種類: 道具・ベンチ(zoo の測り方)
- 言語: Rust
- ROADMAP の節: -
- 守るもの: 台帳の `miss` の種類(`silent` と `trace`)が、Rust でも意味を持つこと
- 進める完了条件: C(1)

## 何が起きるか

Rust の zoo では、`m::sink_cmd`、`m::sink_sql` を呼ぶ関数に、スタブの本体がないことによる `unmodeled_library_call` の注記が必ず付く(`analyze -rules bench/zoo-rust/rules.yaml` で確かめられる)。検出されないセルは、実際には注記なしの見逃し(`silent`)でも、注記が付いているので `trace` に見える。台帳の `miss` の種類は、Rust の zoo では情報を持たない(`bench/README.md` の「この数が見えないもの」に書いた)。

## 直す方向の案

1. スタブに本体を付ける(注記の原因をなくす)。
2. または、zoo-rust だけ、`unmodeled_library_call` を、セルの検出の判定(`trace` か `silent` か)から外す。

どちらも、セルの検出の結果は変えない。
