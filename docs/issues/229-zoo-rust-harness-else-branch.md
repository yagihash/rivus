# #229 zoo-rust のハーネスが、`if let` の else 側が実行されないセルを扱えない

- 状態: **open**
- 優先度: 中
- 種類: 道具・ベンチ
- 言語: Rust
- ROADMAP の節: -
- 守るもの: -
- 進める完了条件: C(1)(領域 C の測定の限界)

## 何が起きるか

zoo-rust のセルは、実行して、`llvm-cov` の region がすべて実行されることを確かめる(`cmd/rivus/zoo_rust_test.go:148` から `:199`。実行回数が 0 の region があると、`obs.Uncovered` に入り、`:220` から `:222` で「a region of the cells did not run」のエラーになる。`TestZooTruthRust` も同じ判定で落ちる)。

`if let Some(p) = ... { ... }` は、else 側(暗黙の else)の region が、セルの実行では実行されない。そのため、最も一般的な書き方である `if let` と `match` で一方の枝が実行されないセルは、zoo に置けない。今のセルは、`.unwrap()` など、全部の region が実行される形で書いている。

## 直す方向の案

- そのセルだけ、判定を緩めるマークを足す(マークのあるセルは、実行されない region を許す)。
- セルを、全部の region が実行される 2 つの関数に分ける。
