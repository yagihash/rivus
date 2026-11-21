# Rust: ジェネリクスと `Box<dyn Fn>` の dynamic な呼び出し(調べたこと、直したこと、直していないこと)

ADR-081 の背景。Rust の動物園(`bench/zoo-rust`)と rust-curated で、2 つの形が見つからなかった: (1) ジェネリックな `T` に対する trait の呼び出し(`for i in items { f(i) }` の `IntoIterator::into_iter` / `next`。`bench/MISSES.md` の `GEN-DYN`)、(2) `Box<dyn Fn(A) -> B>` の呼び出し(`(self.f)(x)`、`map.get(k).unwrap()(x)`)。

## (1) ジェネリックな trait の呼び出し(`GEN-DYN`): 直した

**原因(確かめた):** Rust の標準ライブラリのモデルは、引数の位置を `self` を先頭にして数える(`flows: [{from: arg1, to: arg0}]`、`Iterator::next` は `from: args`)。型が分かっていて、関数そのものを静的に呼ぶときは、`call` 命令の引数がそのまま `arg0`、`arg1` になる。型パラメータ `T` に対する呼び出し(`items.into_iter()`、`it.next()`)は、抽出器が呼び先を決められず、**dynamic な呼び出し**になる。コアは、dynamic な呼び出しの受け手を、引数とは別に持つ(`recv`、`args` は受け手を除く。Go のインターフェースのメソッドの規約)。そのため、`from: args` は受け手(`Vec` そのもの)を含まず、`into_iter` の結果に何も流れなかった。同じ関数が、型が分かる呼び出しでは見つかり、`T` では落ちる。

**直し方(ADR-081):** モデルの集合に `self_first: true` を足し(`rust-std.yaml`、`rust-misc.yaml`)、その集合のシンボルの dynamic な呼び出しでは、受け手を引数の先頭に置いて、モデルを当てる(`internal/taint/calls.go` の `applyModel`)。型が分かる呼び出しと同じ位置になる。受け手を別に持つ規約は、ほかのこと(ガード、候補の絞り)のために、そのまま。

**確かめたこと:** 動物園(Rust)の `dyn/generics-ptr`(vuln と decoy の本物の流れ)が見つかる。ほかの流れの増減は、rust-curated、動物園で 0(`@miss` が外れたのは 2 件だけ)。parseable は `bench/compare.sh` で確かめた(ADR-081)。

**まだ見つからないもの:** rust-curated の `across-each`(別の crate `shared::each<T>(items: Vec<T>, f)`)は、直らなかった(`@miss(silent)` のまま)。`for i in items { f(i) }` の `into_iter` / `next` の位置は直ったが、ほかの原因が残っている(依存の crate の中の関数で、ヘルパー側の呼び出しの先がクロージャの束縛を持たない、など)。原因の特定は、別の機会。

## (2) `Box<dyn Fn>` の呼び出し: 試して、入れなかった(設計の判断が要る)

**何が起きるか(確かめた):** `f(x)` で `f: Box<dyn Fn(&Td)>`(多くは、`Option` や `HashMap` の中、構造体のフィールドにある)は、MIR では `<Box<dyn Fn(&Td)> as Fn<(&Td,)>>::call(&f, (x,))`。解決された先が `Box` の `Fn` の実装(本体なし)で、抽出器は、本体のない静的な呼び出しにする。モデルもないので、`unmodeled_library_call` の既定(入るものは出る)だけが当たり、クロージャの中の sink は読まれない(動物園の `dyn/func-field`、`dyn/func-map`、`clos/stored` の vuln2 / vuln3、`clos/capture-write-by-ref` の vuln3、の 4〜6 件)。

**試した直し:** (a) 抽出器で、`Box` の `Fn` / `FnMut` / `FnOnce` の実装の呼び出しを、dynamic な呼び出し(候補は、引数の型の合うクロージャ。`&dyn Fn` の仮想呼び出しと同じ。シンボルは `std::boxed::Box::call`)にする。(b) そのシンボルのモデルの行(`flows: [{from: args, to: result}]`。`self_first` なので、受け手と引数が結果に入る = 今までの既定)を足し、ADR-075 の「モデルと本体の和」で、候補のクロージャの本体も読む。(c) 受け手の出どころの絞り(ADR-079)に、`Box::new(クロージャ)` の結果を足す。

**測った結果(動物園 Rust と rust-curated。(a)〜(c) を入れた版と、入れない版):**

| | 入れない | 入れた |
| --- | --- | --- |
| 動物園の本物の流れ(`@source` と `@sink` の組)が見つかる | 116(`@miss` 34) | 123(`@miss` 27)。増えた 7: `clos/stored` の vuln2 と vuln3、`dyn/func-field` と `dyn/func-map` の vuln と、decoy の中の本物の流れ(各 1)、`clos/capture-write-by-ref` の vuln3 |
| 動物園の、静かなはずの sink(`@sink(none)` と `@nosink`)に流れが届く | 164 か所が静か | 148 か所が静か(16 か所に届く) |
| rust-curated の本物の流れ | 147 | 147(`holder_boxed` の 2 件は、(b) の行を足すまで落ちた: 今までの既定を残すための行) |

**入れなかった理由:** 本物の流れが増える一方、**静かなはずの sink に、別のケースの source が届く**(16 か所)。原因は XTALK と同じ型: `Box` の中のクロージャの出どころ(`Option<Box<dyn Fn>>` の `as_ref().unwrap()`、`HashMap` の `get`、`Vec` の要素)が、**メモリを通る**ので、受け手の出どころを辿れず(ADR-079 は、関数の結果、フィールド、コンテナの要素を辿らない)、候補が「引数の型の合う全クロージャ」になる。引数のないクロージャ(`Box<dyn Fn()>`)は、型が合うものが、動物園全体のほかのケースのものまで全部。検出が +7、誤検出(静かなはずの sink に届く所)が +16 で、精度を犠牲にする判断なので、**入れない**(ユーザーの方針: 「精度を犠牲にするものは確認する」)。

**直すなら(次の設計):** 受け手の出どころを、コンテナの要素まで辿る(`Option` の `Some.0` のフィールド、`HashMap` / `Vec` の要素への store → load を、「その値を入れた store の全部」として数える。ADR-079 の「環境のフィールドの store を、名前で全部数える」の延長。キーの区別は要らない: 要素のどれか、で足りる)。それが辿れたとき(`Box::new(クロージャ)` を `Some(..)` に入れ、`h.cmd.as_ref().unwrap()` で取り出す形)に、絞って入れる。コンテナの内容を、関数をまたいで、もれなく数えるには、(1) 要素への書き込みが、その `alloc` のほかの別名から行われないこと(エスケープの検査。ADR-079 のセルと同じ)、(2) `Vec::push` / `HashMap::insert` が、本体のないモデルの呼び出しなので、「引数が受け手の要素に入る」というモデルの行を前提にすること、が要る。**抽出器の変更(a)と行(b)は小さい**ので、(c)を拡張する設計(コンテナの要素)が済んだあとに、同じ測定で入れる。

## Dart(ADR-075 の限界 3)

`docs/DESIGN-capture.md` の末尾の段落(「Dart は同じ型の問題」)に書いた。実装は別の機会。
