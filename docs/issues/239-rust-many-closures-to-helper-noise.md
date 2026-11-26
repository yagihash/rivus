# #239 Rust: 34 個のクロージャを 1 つのヘルパーに渡すと、候補が広がり、セルの別のケースの sink に source が混ざる(`unexpected flow` 約 1100 件)

- 状態: **closed**(直った。原因は、Rust の絞り込み `decide` が出所の数の上限 32 を超えると絞らないことだった。上限を `ir.NarrowOriginMost`(1024)に分けた。ADR-050 の追記)
- 優先度: 中
- 種類: 過検出(誤った流れ)
- 言語: Rust(Go は 34 件とも正しい。Dart は 34 件とも見逃す: #77)
- ROADMAP の節: B / C
- 守るもの: zoo `F/clos/helper-overflow-many`(Rust のセル。`@todo` は外した)
- 進める完了条件: -

## 何が起きるか(事実)

同じヘルパー(`fn transform(s: String, f: impl Fn(String) -> String) -> String { f(s) }`)に、別々のクロージャを 34 個渡す。各クロージャは `m::sink_cmd(&x)` を呼ぶ。34 個の呼び出し元(`vuln` 〜 `vuln34`)は、それぞれ別の source を持つ。fixed と decoy も 1 つずつ(Go と Dart の同名セルと同じ形)。

- Rust: `f(s)` の候補が、型(`Fn(String) -> String`)の同じクロージャ全部に広がる。あるケースの source が、**別のケースのクロージャの中の sink** に届く。
- Go: 34 件とも、正しい source と sink の組だけが出る。
- Dart: 34 件とも見逃す(#77。`@miss(depth_limit)`)。

つまり、Rust は Dart のような見逃しではなく、逆の側(過大近似)に外れる。見逃しより安全な側だが、finding の数が増える。

## 数えた結果(2026-11-27。抽出器は `traitpath-out/rivus-rust-new`。セルを足した状態の `TestZooRust`。セルは、コミットしていない)

セルを zoo に足すと、`unexpected flow` は **1530 件**(source 行 × sink 行の組)。

| 範囲 | source 行 | sink 行 | 組 |
| --- | --- | --- | --- |
| セルの中(このセルだけを解析したとき。`ZOO_ALONE=1`) | 34 | 34(34 個のクロージャの `sink_cmd` の行) | 1122(= 34 × 33。自分以外の 33 個の sink 全部) |
| zoo 全体のとき、ほかのセルの sink に届く分 | 同じ 34 | 12(`helper_overflow.rs` の 10 行、`to_lib_call.rs` の 1 行、`to_lib_ref.rs` の 1 行) | 408(= 34 × 12) |
| 合計 | 34 | 46 | 1530 |

- どの source も、45 行の sink に届く(33 + 12)。
- fixed と decoy の `@sink(none)` の行にも届く(`must stay quiet` の行が、セルの中で 2 行。zoo 全体では、`helper_overflow.rs` に 2 行、`to_lib_call.rs` に 2 行も出る)。上の数には入っていない。
- ほかのセルの sink に届くのは、`transform` が別の関数でも、クロージャの型が同じ(`Fn(String) -> String`)ためと見える。**原因は未確認**。

## `@fp` に落とせない理由

別のケースの source が、`@sink(タグ)` の行に届く形は、`@fp` で書けない(`@fp` は `@sink(none)` の行と `@nosink(タグ)` の対のため)。1 行ごとに `@nosink(別のタグ) @fp` を 33〜45 個並べる形になる。さらに、ほかの 3 つのセルの行(既存のセル)にも付ける必要がある。`@noisy` は禁止。そのため、Rust のセルは書かず、`lib.rs` の `@todo(PARITY)` のまま、理由にこの課題を書く。

## 関連の見当(事実の 1 行)

候補の増え方が、#236(dispatch の候補の数の上限)と、#235(generic call の candidates の過大近似)と同じ向き。同じ原因かは未確認。

## 再現

34 個の関数(`vuln`、`vuln2`、…、`vuln34`)を持つセルを書く。各関数は、`let a = m::src("clos-helper-overflow-many-vulnN");` のあと `transform(a, |x| { m::sink_cmd(&x); x + "N" });` を呼ぶ。`bench/zoo-rust/cells/src/clos/helper_overflow.rs`(10 個の版)を 34 個に増やした形。fixed と decoy は、`helper_overflow.rs` と同じ。`cells/src/lib.rs` に `pub mod helper_overflow_many;`、`exec/src/cases.rs` に `register()` を足して `TestZooRust` を走らせる。

## 関連

- #77(Dart: 同じヘルパーに渡すクロージャが 33 以上のとき見逃す)
- #235、#236
- ADR-050、ADR-118
