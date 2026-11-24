# ADR-072: Rust の可変のキャプチャ(クロージャの環境のフィールドが持つ `&mut`)を、指す先の変数と同じものとして扱う。環境のフィールドは、クロージャの名前で分ける(ENV-PTR)

- 状態: 有効
- 関連: -

**背景:** Rust のクロージャは、キャプチャした変数を環境の構造体のフィールドに入れる(`alloc` + `field_store`)。`&mut` のキャプチャは、変数へのポインタ(変数の `alloc`)がフィールドに入る。クロージャの本体は、そのフィールドを `field_load` し、ポインタ経由で `store` する。エンジンは、ポインタを入れたセルを、**指す先の中身のコピー**(`<セル>/*`)として持つ。`field_load` したポインタの指す先も、その合成のセル(`<環境>/0/*`)で、元の変数ではない。だから、クロージャの中の書き込みは、環境のコピーに入り、元の変数に届かなかった(`rust-curated` の `write-a` など 6 件、zoo の `clos/capture-write-by-ref` の `vuln2`、`vuln6`、`rust/closure-fnmut`、`rust/iterator-chain`。`*e.o = e.t`(`Env { o: &mut out, t }`)のように、クロージャを使わない形も同じ)。

同じ抜けが、ノイズも作っていた。クロージャの環境のフィールドは `0`、`1` と番号で呼ばれ、どのクロージャでも同じ名前になる。`Fn::call` の候補は「型の合う全クロージャ」(XTALK)なので、ヘルパー(`fn run(f: impl FnOnce())`)の中で、ある候補が書いた `<f>/0/*` を、別の候補が(別の変数のつもりで)読み、sink に届いた。zoo の `@noisy(4)` と、それに伴う `@fp`(別のセルのクロージャの sink、`fixed` の sink)は、これだった。ENV-PTR の `@ref` は、**書き込みが届かないこと**と**混ざること**の両方を指していた。

**決めたこと:**
1. **セルが持つポインタの指す先を覚える**(`internal/taint/ptrcells.go`): `store` / `field_store` がポインタを入れたセル(スロット)ごとに、指す先の位置(`locs(値)`)を `ptrAlias` に持つ。環境をまるごと別のセル(クロージャを入れた変数)へ `store` するときは、スロットも、写した先に持つ。値(環境、構造体、ジェネリックな `impl FnMut`)をまるごと写したときは、写した先の中でポインタをたどる書き込み(`/*` を含むアクセスパス)は、写し元でもその書き込みとする(`copyOf`。`mut f: impl FnMut()` は、ヘルパーの中で、スロットに写されてから呼ばれる)。不動点は、`prepareLocs` の中で、位置と一緒に求める。
2. **使う場所は 2 つ:** (a) `field_load` / `load` が返すポインタの位置に、指す先を足す(同じ関数の中の `*e.o = x`)。(b) `applySummary` の `OutParam`(呼び出し先がパラメータの中に書いたもの)の書き込み先に、`mirror`(スロット経由の指す先、写し元)を足す。環境のコピーのセルは、残す(読む側は、これまでどおり、変数の中身をコピーから読む)。
3. **範囲:** 閉包の環境の `alloc` が 1 つでもあるプログラムだけ(`sg.Program.HasEnv`。Rust)。Go と Dart は、何も変わらない(`HasEnv` が偽で、`ptrAlias` も `copyOf` も空)。
4. **extractor(Rust):** 環境のフィールドの名前を、クロージャの名前つきにする(`<crate>::f::{closure#0}.0`。`field_name` と、`Aggregate(Closure)` の `field_store`)。環境を作る側と、クロージャの本体の `field_load` が、同じ名前になる。どのクロージャか分からない `Fn::call` の候補は、レシーバの環境の自分のフィールドだけを書き、読む。別のクロージャの書き込みは、混ざらない。レシーバが実際にそのクロージャのときの流れは、同じ名前で、そのまま通る。

**確かめたこと:**
- zoo(Rust、`bench/zoo-rust`。base は、`calls:` の入った main): 見つかる 59 → 65 / 89、`@miss` 30 → 24、修正済みのケースの誤検出 11 → 4、デコイの誤検出 16 → 9、`@fp` 36 → 21。`@miss` を外した(見つかる側に変わった)のは、`clos/capture-write-by-ref` の `vuln2`、`vuln6`、`vuln7`(`vuln7` は、`calls:` の `Once::call_once` と、この変更の両方が必要)、`rust/closure-fnmut` の `vuln`、`vuln2`、`decoy`、`rust/iterator-chain` の `vuln2`。`@noisy` の注釈を外したのは 17 か所、`@fp` を外したのは 15 か所(`clos` のセル、`rust/closure-fnonce`)。decoy と fixed は検出されないまま(新しい `@fp` 0)。残る `@miss`: `vuln3`(ループの変数)、`vuln5`(`Once::call_once` が、あとで書かれた変数を読む形)。
- `rust-curated`(base 比): 見つかる側が 131 → 141(フィクスチャを足したので、足した分を含む。`@miss` は 15 → 9)、`@noisy` の流れ 194 → 152、`@fp` 22 → 18。`write-a`、`write-b`、`write-param`、`push-list`、`late`、`for-each-cap`、`spawned` の `@miss` を外した。足したフィクスチャ: `write_one_of_two`、`struct_mut_field`、`struct_mut_field_helper`、`struct_mut_field_const`(クロージャのない `&mut` のフィールド)。直す前の main では、足した 4 つのうち、見つかる側の 3 つが落ちる。
- `compare.sh origin/main HEAD`(`rust-curated`、`zoo-rust`、`parseable`。両側とも、自分の側の extractor で作った IR): **parseable は finding 159 → 159、追加 0、削除 0**、経路の長さが変わったのが 31 件(1 段短くなる。`catalog` のクロージャが、環境のフィールドを別々に持つので)。到達性・`other_sources` の変化は 0。時間は 3 → 3 秒、メモリは 1036 → 1119 MB。`rust-curated` は +10 / −46、`zoo-rust` は +7 / −122 と −1。
- 追加(17 件)は、すべて、source と同じ関数(クロージャを含む)の中の sink で、本物: 上の `@miss` を外した流れと、足したフィクスチャ。経路を読んで、書いたクロージャの `src` から、変数を通って、外の `sink` に届いていることを確かめた。追加のノイズは 0。
- 削除(`rust-curated` 46、`zoo-rust` 123)は、すべて、**別のセルのクロージャの sink に届く過検出**(source と sink が別の関数。1 件を除く)。例外の 1 件は、`capture-write-by-ref` の decoy の `quiet` のクロージャ(定数 `y`)の sink に、同じ decoy の `set` の `src` が届くもの(`@fp`。`@nosink` の注釈で、本物でないと決めてあるもの)。これらの sink は、`@noisy(3)` から `@noisy(8)` と uncertainty してあった sink で、数が合う(`spawned` の sink 8、`retained` の sink 3 ほか)。本物の流れの削除は 0。
- `go vet`、`go test ./...`(`RIVUS_DART`、`RIVUS_RUST`、`RIVUS_REQUIRE_*=1`): この変更で落ちるものは無い。`TestCuratedDart`、`TestDogfood`、`TestZooPureGo` が落ちるが、`calls:` の変更を入れた main(d277d63)でも同じに落ちる(別の変更の途中: Dart の `@miss` が見つかる側に変わる、`capture_unbound` が 37、Go の pure の行が呼ばれていない)。

**`Fn::call` の候補が 0 件、について(調べた):** parseable で `std::ops::Fn::call` の呼び出しは 36 件。**dynamic は 3 件で、候補は全部ある。** 残る 33 件は `static` で、29 種類の呼び出し先のうち 28 は本体がある(その `async` ブロックや、クロージャの関数)。本体も候補もないのは、`<Box<dyn Fn(String) -> bool + Send> as Fn<(String,)>>::call`(標準ライブラリの `Box<dyn Fn>` の実装。4 呼び出し)だけ。つまり、「dynamic で 0 件」ではなく、「`Box<dyn Fn>` の呼び出しが、`dyn` の仮想呼び出しにならず、標準ライブラリの本体のない static 呼び出しになる」。直すには、extractor が、この形を、`dyn Fn` の呼び出し(候補は、引数の型の合うクロージャ)にする必要がある。別の変更(別の issue)に回した。この ADR の範囲ではない。

**限界:** `across-cap`(`shared::each(vec![1], |_i| out = t.clone())`)は、まだ落ちる。呼ぶ側 `for i in items { f(i) }` で、`IntoIterator::into_iter` / `Iterator::next` の候補が 0(GEN-DYN と同じ根に見える。未確認)。`vuln5`(`Once::call_once` が、あとで書かれた変数を読む形)は、まだ落ちる(`RefCell` の書き込みの順と、`call_once` のモデルの組み合わせ。未調査)。ループの変数を `move` で環境に入れる形(`vuln3`)は、関数値のメモリ(`FUNC-VALUE-MEMORY`)。環境が `Box` に入って別の関数から呼ばれる形は、ポインタのコピーを `store` で追うところまでで、`Box<dyn Fn>` の内側の別名は追っていない。ポインタを入れた構造体を値で関数に渡し、その関数が書くのは追う(`struct_mut_field_helper`)が、返り値で返した構造体のポインタは追わない。`Fn::call` の候補の集合そのもの(XTALK の候補の絞り込み)は、案 A のまま(この変更は、候補が環境を混ぜるのを止めただけ)。


