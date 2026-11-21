# Rust: `async` の中の `Fn::call` の候補を絞る(調べたこと)

ADR-079(案 A)は、動物園と rust-curated で `@fp` / `@noisy` を減らしたが、parseable では、複数の候補を持つ `Fn::call` 3 か所のうち 1 か所しか絞れなかった。主因の呼び出し(15 候補)の受け手は、**`async` の状態機械(coroutine)のフィールド**で、出どころを辿れず、全候補のままだった。ここは、その原因を、独立した小さい例(`bench/rust-curated/src/narrow_async.rs`)で再現して調べた記録。

## IR での `async` の形(確かめた)

(`rivus-rust` の出力。`async` ブロックは `{async block@file:l:c}` という型、`async fn` は、本体の `{closure#0}` という名前の関数 + coroutine。)

* **`async move { g(&s) }`**: 呼び出し元の関数で、coroutine を `alloc`(型 `{async block@…}`)し、捕獲した値を `field_store`(フィールド名は `"0"`、`"1"`…: **番号だけ**)で入れる。`Poll` する側(executor)はコードにないので、抽出器は、coroutine を作った所で、本体を呼ぶ(`Pin { pointer: &mut coroutine }` を作って、`call 本体(pin, 0)`。lower.rs の `AggregateKind::Coroutine`)。
* **本体**(`関数名::{closure#N}`): パラメータは `pin`(と resume の引数)。coroutine は `pin.pointer`(`field_load`)から取り、捕獲した値は、その `addr_of`(path = フィールド `"0"`)で読む。中の `g(&s)` は `Fn::call`(受け手 = そのアドレス)。
* **`async fn`** は、同じ形: 関数自身は、パラメータを coroutine の `"0"`、`"1"` に入れて、本体を呼ぶ。`.await` と `Future` は、本体の `Poll` の分岐(`Ready.0` の `field_store`)になり、呼び出しの先ではない(`spawn` の中の `tokio::spawn` は、本体のない関数。引数の future は、すでに本体が走っている)。

## 辿れなかった原因(根拠つき。2 つ)

1. **coroutine のフィールドの名前**: `narrow.go` は、環境のフィールドを、**クロージャの名前を持つフィールド**(`{closure#…}.0`)で数える(同じ名前への store を全部集める。ADR-079)。クロージャの環境のフィールドは、抽出器が `{closure@…}.0` のように、そのクロージャの名前を付けるが、**coroutine のフィールドは `"0"` だけ**。`isClosureField` が偽で、`unknown_origin`。付け方を揃えると、(同じ番号の)別の coroutine のフィールドと混ざらない利点もある。
2. **ジェネリックなヘルパーの中の `f.clone()`**: `async` ブロックに入れる前に、ヘルパーの中で `let g = f.clone()` と複製する形(`for_each_live_node` の `api_fn.clone()`)。`F: Fn + Clone` の型パラメータに対する `clone` は、抽出器が実装を決められず、**dynamic な `Clone::clone`**(候補は意味のない 1 個)。`narrow.go` が辿る `clone` は、**クロージャの型の静的な `clone` だけ**で、dynamic のものは辿らず(`unknown_origin`)。さらに、`f` がパラメータなので、`&f` のセル(`alloc` + `store`)があり、その別名が `clone` に渡ると、「セルの別名が、受け手以外に渡った(= 外に出た)」として、セルも辿れなかった。これは `async` に限らない(同期のジェネリックなヘルパーでも、同じ)。

(`async` の本体が別の関数に出る点は、原因ではない。本体は、coroutine を作った関数から呼ばれ、フィールドの store(作った関数の中)も、IR にある。)

## 再現(`narrow_async.rs`)

型は形ごとに別(`Qa`〜`Qd`)。(1) `async move` ブロックがクロージャの clone を捕獲、(2) `async fn` がクロージャを引数に取る、(3) `for_each_live_node` の形(`async fn`、ループの中で clone して `async move`)、(4) 出どころが 2 つで片方が辿れない(捕獲のあるクロージャを返す関数)。各形に、呼び元が 2 つ(本物と別)と、渡されないが型の合うクロージャ(`*_extra`)がある。**直す前**: `*_extra`(渡されていないクロージャ)の sink に、本物の source が届く(候補が 3 個のまま)。
