# #247 Rust: ループで再適用した呼び出しのあと、パラメータに依る return のガードで止まる流れが報告される

- 状態: **open**
- 種類: 過検出
- 言語: Rust
- ROADMAP の節: -
- 守るもの: zoo `F/guard/reapply-loop`(Rust の `decoy` の静かな項目に `@fp @ref(#247)`)

## 何が起きるか

ループで呼び出しを再適用し(`r = pass(true, format!("{}{}", r, s))`)、そのあとの `return` が、引数の `skip` に依る関数がある。`skip` が真のとき、この関数は空の文字列を返すので、呼び出し側で `skip` に `true` を渡した行の sink は、静かでなければならない。Go と Dart では静かだが、Rust では、その sink に流れが報告される。

zoo のセル `F/guard/reapply-loop`(`bench/zoo-rust/cells/src/guard/reapply_loop.rs`)の `decoy` で再現する。

```rust
fn looped(skip: bool, s: &str) -> String {
    let mut r = String::new();
    for _ in 0..3 {
        r = pass(true, format!("{}{}", r, s));
    }
    if skip {
        return String::new();
    }
    r
}

fn decoy() {
    let v = m::src("guard-reapply-loop-decoy");
    m::sink_cmd(&looped(true, &v));  // 静かでなければならない。Rust では報告される
    m::sink_sql(&looped(false, &v)); // 流れがある(報告される)
}
```

実行では、`skip` が真の側の sink は、マーカーを受け取らない(`TestZooExecRust` が確かめる)。

## 原因(分かっている範囲)

- 同じ形の Go のセルは、#733 の修正の前は同じ過検出だった。ループで再適用する呼び出しのホップが、2 つの経路から要素を受けて「ガードなし」になり、そのあとの `return` のガード(`!skip`)が連鎖ごと捨てられていた(`guardsOfChainFrom`)。#733 で直った。
- Rust は、#733 の修正のあとも静かにならない。Go と Dart でそろわない理由は、**未調査**。次のどれかと考えているが、確かめていない。
  - Rust の抽出器が作る IR では、`for _ in 0..3` のループの形、`format!` の呼び出し、`String::new()` の戻りが、Go や Dart と違い、ループの再適用のホップが別の形になる
  - `if skip { return String::new(); }` の分岐が、Rust の IR では、ガードとして使える形(パラメータに依る分岐)になっていない
  - 呼び出し側の `looped(true, &v)` の定数の引数を、ガードの判定が読めていない
- 試した確認: Go、Dart、Rust の 3 つのセルは、同じ形で書いてある(`pass` を `true` で呼ぶ、`skip` で戻りを決める)。

## 再現するセル

- `bench/zoo-rust/cells/src/guard/reapply_loop.rs` の `decoy`: `m::sink_cmd(&looped(true, &v)); // @sink(none) @fp @nosink(guard-reapply-loop-decoy) @fp @ref(#247)`
- 同じ形の Go と Dart のセルは、`@fp` なしで通る(`bench/zoo-go/cells/guard/reapply_loop/reapply_loop.go`、`bench/zoo-dart/cells/lib/guard/reapply_loop.dart`)

## 調べる手がかり

- `ZOO_ALONE=1` で、このセルだけを Rust で解析し、`decoy` の sink の流れの、ホップごとのガードを読む。ガードが付いているのに `feasible` で消えないのか、ガードが付いていないのかで、上の 3 つのどれかが分かる
- Rust の抽出器の IR(`rivus debug extract`)で、`looped` の `if skip` が `TermBranch` で、条件がパラメータ `skip` から作られているかを見る
