# #83 Rust: ジェネリックな T の Vec を for で回すと、trait 呼び出しが dynamic になり、モデルが receiver を見ず、流れが落ちる(黙った見逃し)

- 状態: **open**
- 種類: 黙った見逃し
- 言語: Rust
- ROADMAP の節: 2
- 守るもの: -
- GitHub: #83(移行前の番号は同じ。2026-11-20 に開いた。移行時の状態: OPEN)

## 本文(GitHub から移した写し)

## 現象
Rust で、**型パラメータ `T` を使う関数**の中の `for i in items`(`items: Vec<T>`)で、要素の流れが落ちる。`Vec<String>` のような具体的な型では見つかる。不確実性は出ない(**黙った見逃し**)。ジェネリックなユーティリティ関数でよくある形。

## 再現(Rust)
```rust
fn g<T>(items: Vec<T>) -> Vec<T> { let mut o = Vec::new(); for i in items { o.push(i); } o }
pub fn e() {
    let items = vec![source()];
    let o = g(items);
    sink(&o[0]);             // 期待: source → sink   実際: 出ない
}
// fn g(items: Vec<String>) -> Vec<String> に変えると見つかる
```
`fn each<T>(items: Vec<T>, mut f: impl FnMut(T)) { for i in items { f(i); } }` に `|x| sink(&x)` を渡す形も落ちる(`across-each`)。`Fn` の `each3(items: Vec<String>, f)` は見つかる。

## `@miss`
`bench/rust-curated`: `across-each`(弱い痕跡: 無関係な `numeric_stop`)。`bench/MISSES.md` の型 `GEN-DYN`。

## 落ちる場所の推定(確かめた)
`T` に対する `IntoIterator::into_iter` / `Iterator::next` は、IR で `call dynamic`(具体的な型では `call static`)。dynamic では receiver が `Args[0]` で、`CallArgs()`(`internal/sg/sg.go:139`)は `Args[1:]`。モデル(`internal/model/builtin/rust-std.yaml`: `from: args`)は `selectVals`(`internal/taint/calls.go`)で `cc.args`(receiver を含まない)になるので、イテレータ(receiver)の中身が結果に流れない。
確かめた: `-rules` で `std::iter::IntoIterator::into_iter` と `std::iter::Iterator::next` に `flows: [{from: recv, to: result}]` を足すと、上の再現で source → sink が出る(足さないと出ない)。修正方法の候補: モデルの `from: args` が、dynamic のときも receiver を含むようにする(`args` の意味を、static と dynamic で揃える)。ADR-047(案 2)では直らない。

