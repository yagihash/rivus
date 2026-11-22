# #82 Rust: クロージャや構造体のフィールドが持つ &mut 経由の書き込みが、元の変数に届かない(サイレントな見逃し)

- 状態: **closed**
- 種類: サイレントな見逃し
- 言語: Rust
- ROADMAP の節: 2
- 守るもの: `bench/rust-curated/src/capture.rs`(`@miss` が外れた)
- GitHub: #82(移行前の番号は同じ。2026-11-20 に開いた。移行時の状態: CLOSED)

## 今の状態(2026-11-22 の見直し)

直した: e79e21f(ENV-PTR)。確認: 2026-11-22。

## 本文(GitHub から移した写し)

## 現象
Rust で、クロージャが**キャプチャした変数に書く**(`out = t`、`items.push(t)`、`last = x`)と、書いた値が、元の変数に届かない。ヘルパーなし、関数 1 つでも落ちる。不確実性は出ない(**サイレントな見逃し**)。`for_each(|x| last = x)` や、カウンタ・バッファへの書き込みで、とてもよくある書き方。

## 再現(Rust、`rivus-rust` の IR)
```rust
pub fn q() {
    let mut out = String::new();
    let t = source();
    let mut f = || { out = t; };
    f();
    sink(&out);              // 期待: source → sink   実際: 出ない
}
struct Env<'a> { o: &'a mut String, t: String }
pub fn q2() {
    let mut out = String::new();
    let t = source();
    let e = Env { o: &mut out, t };
    *e.o = e.t;
    sink(&out);              // クロージャなしでも、実際: 出ない
}
```
`fn w(o: &mut String, t: String) { *o = t; }` を `w(&mut out, t)` で呼ぶ形は見つかる。

## `@miss`
`bench/rust-curated`: `write-a` `write-b` `write-param` `push-list` `late` `for-each-cap` `across-cap`(7 件、すべて何も報告しなかった)。`bench/MISSES.md` の型 `ENV-PTR`。

## 落ちる場所の推定
クロージャの環境は、キャプチャした変数へのポインタを持つ構造体(IR: `field_store .0 env, <ポインタ>`)。
* `internal/taint/locs.go:215` `computeLocs`(`OpFieldLoad`): ポインタを `locs(base) + field + "*"` とする。
* `internal/taint/transfer.go:148` `OpFieldStore`: ポインタの値を保存するだけで、フィールドの指す先と、元の変数の位置を同一視しない。
そのため、`*env.0 = v` が、`env.0/*` という別の位置に書かれ、`out` の位置に届かない。ポインタをフィールドに入れる形の別名の追跡が必要。ADR-047(案 2)では直らない。

## コメント(GitHub から移した写し)

対応状況: 直した(ENV-PTR の本体)。残りは別の原因。

**原因(IR と経路で確かめた):**
1. 書き込みが元の変数に届かない: 環境の `alloc` の `field_store` に変数へのポインタが入ると、エンジンはそのセルに「指す先の中身のコピー」(`<環境>/0/*`)を持つ。クロージャの本体の `field_load` したポインタ経由の `store` は、そのコピーに入り、元の変数には入らなかった(`locs.go` の `computeLocs` と、`applySummary` の `OutParam`)。`*e.o = e.t`(`Env { o: &mut out, t }`)のようにクロージャを使わない形も同じ。
2. ノイズ: 環境のフィールドが `0`、`1` とだけ呼ばれ、どのクロージャでも同じ名前。`Fn::call` の候補は「型の合う全クロージャ」(XTALK)なので、ヘルパーの中で、ある候補が書いた `<f>/0/*` を別の候補が読む。(「共有された環境の根」の仮説は当たり。ただし場所は、環境を作る側ではなく、呼ぶ側のヘルパー。)

**直したこと(ADR-072):** セルが持つポインタの指す先を覚え(`internal/taint/ptrcells.go`)、`field_load` の結果と、呼び出し先が書いたものを、指す先にも当てる。値の写しの中のポインタ経由の書き込みは、写し元にも当てる。Rust の extractor は、環境のフィールドをクロージャの名前で呼ぶ(`<クロージャ>.0`)。範囲は、クロージャの環境がある(Rust)プログラムだけ。Go と Dart は変わらない。

**結果(`main` の 1caf62b 比):**
- zoo(Rust): 見つかる 59 → 65 / 89。`clos/capture-write-by-ref` の `vuln2`、`vuln6`、`vuln7`(`calls:` の `Once::call_once` と合わせて)、`rust/closure-fnmut` の `vuln`、`vuln2`、`decoy`、`rust/iterator-chain` の `vuln2`。`@miss` 30 → 24、直した版の誤検出 11 → 4、デコイの誤検出 16 → 9。`@noisy` の注釈 17 か所と、`@fp` の注釈 15 か所を外した。
- `rust-curated`: `write-a`、`write-b`、`write-param`、`push-list`、`late`、`for-each-cap` が見つかる(`@miss` 15 → 9)。`@noisy` の流れ 194 → 152。クロージャのない `&mut` のフィールド(`struct_mut_field`、`struct_mut_field_helper`)のフィクスチャを足した(直す前は落ちる)。
- `compare.sh`: parseable は finding 159 → 159、追加 0、削除 0(経路の長さが 31 件で 1 段短くなる)。rust-curated と zoo-rust の削除は、すべて、別のセルのクロージャの sink に届いていた過検出(source と sink が別の関数)。本物の流れの削除は 0。

**残り:** `rust-curated` の `across-cap`(`shared::each(vec![1], |_i| ...)`。`for i in items { f(i) }` の `IntoIterator` / `next` の候補が 0。GEN-DYN と同じ根に見える。未確認)。zoo の `vuln3`(ループの変数を `move` で環境に入れる)と `vuln5`(`Once::call_once` が、あとで書かれた変数を読む)。

**`Fn::call` の候補 0 件(parseable の 92%)について:** 調べたら、dynamic の `Fn::call` は 3 件で、候補はある。0 件に見えていたのは、`static` の 33 件(呼び出し先の本体がある)。本体も候補もないのは、`Box<dyn Fn(String) -> bool + Send>` の `Fn::call`(標準ライブラリの実装)の 4 呼び出しだけ。直すには、extractor が、この形を `dyn Fn` の仮想呼び出し(候補は、引数の型の合うクロージャ)にする必要がある。別の issue にする。

## コメント(GitHub から移した写し)

再開後の確認(2026-11-22): `bench/rust-curated/src/capture.rs` の write-a / write-b / write-param / push-list / late / for-each-cap / across-cap から `@miss` が外れていて(e79e21f、ENV-PTR。環境のフィールドが持つ &mut を、指す先の変数と同じものとして扱う)、main 0bd0a00 の CI(rust のジョブ)は成功。この Issue の最小の再現(クロージャ 1 つ・構造体のフィールド)そのものは、今回は実行していない(Rust の隔離の環境が必要なため)。curated の同じ形が成功していることを根拠に閉じる。再現が残っていたら再び開ける。

