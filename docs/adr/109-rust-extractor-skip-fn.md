# ADR-109: Rust の extractor は、変換できない関数を飛ばして、ほかを読み続ける(`fn_skipped`。2026-11-23)

- 状態: 有効
- 関連: DESIGN-rust-extractor-panic、課題 #160

**背景:** `rustc_public` は、async closure の型を stable MIR に変換できず、`unimplemented!`(`FIXME(async_closures)`)でパニックする(固定の nightly でも、`rust-lang/rust` の master でも同じ)。パニックは、extractor 自身が呼ぶ変換の中で起き、1 つの関数で、そのクレートの抽出が止まり、cargo が止まり、解析全体が止まった。`google-cloud-auth` を通る依存(Google Cloud の Rust のクライアント全部)は、依存込みでは解析できなかった([課題 #160](../issues/160-rust-async-closure-extractor.md)、[DESIGN-rust-extractor-panic.md](../DESIGN-rust-extractor-panic.md))。

**決めたこと:**
1. **関数ごとに、変換と lowering(`item.body()` と `lower::Lower::lower`)を `catch_unwind` で囲み、パニックした関数だけ飛ばす。** 飛ばす根拠: 変換のパニックは、クエリが終わったあとの純粋な変換の中で起き、`rustc_public` のコンテキストの借用(`RefMut`)は巻き戻しで解ける。クエリの中のパニックは、catch しても abort しうるので、catch は変換と lowering に限る(abort したら、そのクレートが失敗する。今と同じ)。
2. **飛ばしたことは、必ず見える形で残す。** IR に `fn_skipped`(`unit`、`func`、`dependency`、`why`)。`summary.functions_skipped`(関数の数、プロジェクトと依存の数、クレートごと(最大 10。プロジェクトが先)の数・理由・例 3 件)。呼び出しの所の uncertainty: 依存の関数なら `dependency_body_unread`、プロジェクトの関数なら `project_body_unread`(どちらも、本体がないのでなく「読めなかった」。`unmodeled_library_call` にはしない)。モデルの行がある関数は、行が使われて、uncertainty は付かない。
3. **プロジェクト自身の関数が飛んだときも、同じ扱い**で、分けて数える。標準エラーの最後の行に警告を出す。**失敗にするフラグは作らない**(求めが出てから)。
4. **パニックの出力は、1 行にまとめる。** ドライバの `panic::set_hook` で、catch の範囲の中だけ静かにし、`rivus-rust: skipped <関数> (<クレート>): <理由の 1 行目> (at <場所>)` を出す(1 クレートで 5 件まで。残りは数だけ)。範囲の外のパニックは、元のフックのまま。`RIVUS_RUST_PANIC_VERBOSE=1` で全部出す。
5. **クレート単位の逃げ道(catch のあとに rustc が abort する場合に、ドライバなしの rustc で再実行する)は、見つかってから。** `-no-deps` は、依存を読まない選択として残る。

**しないこと(この ADR では)**: async closure の本体を読む(上流の `rustc_public` の実装を待つ)。飛ばした関数のモデルを自動で作ること。

