# ADR-081: Rust のモデルの位置は、trait 越し(dynamic)の呼び出しでも、レシーバが `arg0` になる(`self_first`)。`GEN-DYN` を直す。`Box<dyn Fn>` の呼び出しは、設計文書にして入れない

- 状態: 有効
- 関連: DESIGN-generics-dispatch
- 要旨: `Box<dyn Fn>` は入れない

**背景:** `GEN-DYN`(bench/MISSES.md): ジェネリックな `T` に対する `items.into_iter()`、`it.next()` は、抽出器が呼び出し先を決められず、dynamic な呼び出しになる。コアは dynamic な呼び出しのレシーバを引数と別に持ち(`args` はレシーバを除く。Go のインターフェースのメソッドの規約)、Rust のモデルは `self` を先頭に引数を数える(`from: args`、`arg1`)ので、`from: args` がレシーバ(`Vec` そのもの)を含まず、結果に何も流れなかった。同じ関数が、型が分かる呼び出しでは見つかり、`T` では落ちる(docs/DESIGN-generics-dispatch.md)。

**決めたこと:**
1. モデルの集合に `self_first: true`(`rust-std.yaml`、`rust-misc.yaml`)。その集合のシンボルの dynamic な呼び出しでは、レシーバを引数の先頭に置いて、モデルを適用する(`internal/taint/calls.go` の `applyModel`)。型が分かる呼び出しと同じ位置になる。レシーバを別に持つ規約は、ガード、候補の絞り込みのために、そのまま。
2. **`Box<dyn Fn>` の呼び出しは、入れない**(docs/DESIGN-generics-dispatch.md の (2))。抽出器で `Box` の `Fn` の実装の呼び出しを dynamic にし、モデルの行(今までの既定と同じ)を足す試作は、zoo で本物の流れが 7 件増える一方、検出されないはずの sink に届く所が 16 か所増えた(レシーバの出所が、`Option` / `HashMap` / `Vec` の中で、ADR-079 の絞り込みがたどれない)。精度を犠牲にする判断なので、入れない。コンテナの要素までたどる設計が済んでから、同じ測定で入れる。

**確かめたこと:** zoo(Rust)の `dyn/generics-ptr` の vuln と decoy の本物の流れが見つかる(`@miss` を外した)。zoo Rust: 本物の流れ 114 → 116、`@miss` 36 → 34、検出されないはずの sink に届く所の増減 0。rust-curated: 変化なし(147、`@miss` 8)。

**限界:** rust-curated の `across-each`(別の crate の `each<T>(items: Vec<T>, f)`)は、まだ見つからない(別の原因が残る。未確認)。`Iterator` の既定のメソッドが `next` を呼ぶ形は、ADR-080 の `method:` の行を足す作業。

