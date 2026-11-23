# 設計メモ: Rust の extractor が、1 つの関数でパニックしても、ほかを読み続ける

> 実装済み。正はコードと `docs/ARCHITECTURE.md`。以後は更新しない。

状態: 実装済み(2026-11-23。[ADR-109](adr/109-rust-extractor-skip-fn.md)。課題 [#160](issues/160-rust-async-closure-extractor.md))。メモのとおりに実装した。

## 0. 問題と方針

`rustc_public` は、async closure の型(`ty::CoroutineClosure`)を、stable MIR に変換できず、`unimplemented!("FIXME(async_closures): Lower these to SMIR")` でパニックする(`compiler/rustc_public/src/unstable/convert/stable/ty.rs:484`。固定の nightly `5f4effd5` と、`rust-lang/rust` の master(2026-11-23)で同じ。nightly を上げても直らない)。パニックは、extractor 自身が呼ぶ変換(`item.body()`)の中で起き、今は 1 つの関数で、そのクレートの抽出が止まり、cargo が `could not compile` で止まり、解析全体が止まる。

方針: **関数ごとに `catch_unwind` して、その関数だけ飛ばし、ほかを読み続ける。飛ばしたことは、必ず利用者に見える形で残す。** 安全と言えない場合の逃げ道は、クレート単位で飛ばす(1 節)。

## 1. パニックのあとの rustc の状態は安全か

結論: **変換の中のパニックは、安全に飛ばせる。実験でも、同じセッションで次の関数を続けられた。** 根拠は 3 つ。

1. **`rustc_public` のコンテキスト。** 呼び出しは `CompilerInterface::with_cx` を通る(`compiler_interface.rs`)。そこで、`tables.borrow_mut()`(`RefCell<Tables>`)と `cx.borrow()` を持ったまま変換を呼ぶ。パニックは、その中で起きるが、どちらも RAII(`RefMut`、`Ref`)なので、巻き戻しで借用は解ける(借用が残って、次の呼び出しが `already borrowed` になることはない)。`TLV`(`scoped_thread_local`)は、`run` の外側の `set` が持つガードで、`catch_unwind` は、その内側(コールバックの中)にあるので、触れない。`Tables`(`DefId` と stable な ID の対応表)は、途中までの挿入が残るだけで、中身は壊れない(追加するだけの表)。
2. **rustc のクエリ。** 「パニックのあと、クエリが実行中のまま残り、最後に abort する」(`lower.rs` の `resolve` の注意書き)のは、**クエリの中**のパニックの話。今回のパニックは、クエリが全部終わったあとの、純粋な型の変換(`match ty.kind()`)の中で起きる。この時点で、実行中のクエリはない。ただし、**どのパニックでも安全、ではない**: `Instance::resolve` のように、クエリの中でパニックするものは、catch しても、あとで abort しうる。だから、catch する場所は、変換と lowering(`item.body()` と `lower::Lower::lower`)の呼び出しに限り、「catch したあとの rustc が abort したら、クレート単位で止まる(今と同じ)」を、最悪のときの動きとする。
3. **`UnwindSafe`。** `catch_unwind` は `UnwindSafe` を要求するが、`lower::Ctx` は `&mut` で渡すので、`AssertUnwindSafe` が要る。壊れうるのは、`Ctx` が、その関数の途中まで積んだもの(`cx.texts`、`cx.closures`、`cx.promoted` への挿入)。どれも、キーつきの追加だけで、途中までの分は、あとで使われない余分なものになる(実装のとき、`texts` に、その関数の位置の余分が残っても、出力に影響しないことをテストで見る)。

**実験**(リポジトリの外の、自作の合成。サンドボックス外、第三者のコードなし): `edition = "2024"` の 3 関数の crate(`before`、`async fn run`(中で `async |s: &str| s.len()` を使う)、`after`)。今の extractor は、パニックして終了コード 1、出力は空。`extract()` の 2 つのループ(クロージャの走査と、関数の lowering)を `catch_unwind(AssertUnwindSafe(..))` で囲んだ試作では、**終了コード 0。`before`、`after`、`run` は出力され、飛ばされたのは、async の本体のコルーチンと、その中の async closure の 2 関数。rustc は、最後まで abort しなかった**(24 行の IR)。

**だめな場合の逃げ道(クレート単位)**: catch 後に rustc が abort する別のパニックが見つかったら、そのクレートの `rustc` は、ドライバなしの普通の `rustc` で、同じ引数で再実行する(cargo は成功し、IR のフラグメントはなし)。クレートまるごとが「読まなかった」になる。今のメッセージ(`fix/rust-dep-compile-message`)は、この逃げ道もないときのためのもの。今回は実装しない(必要になるまで)。

## 2. 飛ばした関数は、呼び出しの所で必ず注記が付く

**IR**: 新しいレコード `fn_skipped`(関数ごと):

```json
{"kind":"fn_skipped","fn_skipped":{"unit":"google-cloud-auth","func":"google_cloud_auth::...::{closure#0}","dependency":true,
  "why":"rustc_public: not implemented: FIXME(async_closures): Lower these to SMIR"}}
```

- 既存の `body_not_read`(`internal/ir/ir.go`。ユニット単位の数だけ)は、「宣言したので読まなかった」の数で、理由と関数の名前がないので、使わない。`load_error`(`unread: true`)も、1 エントリ = 1 パッケージで数える(`unreadOf`)ので、関数ごとに出すと、パッケージ数が合わない。
- **呼び出しの所(2026-11-23 の判断で修正)**: 飛ばした関数は、本体が「ない」のではなく「読めなかった」。だから、`unmodeled_library_call`(本体もモデルもない = ライブラリの呼び出しとして扱う)にはしない。**注記の種類は、飛ばした関数の側で決める**:
  - **依存の関数**: 既存の `dependency_body_unread`(「依存の本体を読んでいない」)。detail に、`fn_skipped` の `why`(「the extractor could not read the body: <理由>」)を入れる。
  - **プロジェクト自身の関数**: 別の種類 `project_body_unread`(「プロジェクトの関数の本体を、extractor が読めなかった」。ライブラリの呼び出しと読める名前は避けた)。プロジェクトの関数は、モデルの行で代わりにならないので、見逃しの原因として目立たせる(`summary` の注意と合わせる)。
  - **モデルの行がある関数が飛ばされたとき**: モデルの行が使われ(本体の代わり)、注記は付かない(今の「本体のない関数にモデルがあれば、行を使う」と同じ)。
  - 判定は、呼び出しのサイトで、callee が `fn_skipped` にあるか(Go 側で、`fn_skipped` の `func` の集合を持つ)。**抽出側の確認事項**: 飛ばした関数の名前が、`cx.bodies`(本体のある関数の集合。`extract()` の先頭で作る)に入ったまま、呼び出しが「本体のある関数」として出ないこと。合成のテスト(7 節)で、「飛ばした関数を呼ぶ所に、上の注記が付く」を確認する。

**`summary`**: 新しい項目 `functions_skipped`(`body_replaced_packages` と同じ、トップに数、その下に、クレートごとの配列):

```json
"functions_skipped": {
  "functions": 12, "project": 0, "dependencies": 12,
  "crates": [
    {"crate": "google-cloud-auth", "dependency": true, "functions": 12,
     "why": "rustc_public: not implemented: FIXME(async_closures): Lower these to SMIR",
     "examples": ["google_cloud_auth::...::{closure#0}", "..."]}
  ]
}
```

- `crates` は、関数の数の多い順、最大 10。`examples` は、クレートごとに最初の 3 件。理由は、クレートの中で最も多いもの 1 つ(違うものがあれば、`why_other` に数)。`unread`(`summary.unread`)とは別: あちらは「パッケージが読めなかった」、こちらは「読んだが、いくつかの関数を飛ばした」。
- 標準エラーにも、実行の最後に 1 行(「N functions were skipped (M in the project): see summary.functions_skipped」)を出す。**`project` が 0 でないときは、stderr の最後の行に、警告として目立たせる**。

## 3. プロジェクト自身の関数で起きたときも、同じ扱いにするか

**同じ扱いにする。ただし、分けて数え、目立たせる**(相談役の考え。この実験の `ac` crate のパニックは、プロジェクト自身の関数だった)。

- 同じレコード(`dependency: false`)。`summary.functions_skipped.project` に数、`crates` に、プロジェクトのクレートが、依存より先に並ぶ。
- プロジェクトの関数を飛ばすと、見逃しが出る(その関数を通る流れが、見えない)。依存の関数は、モデルの行が代わりになりうる。**プロジェクトの関数が 1 つでも飛ばされたら、finding の `summary` の注意に出し(`project > 0`)、標準エラーの行も警告の形にする。** 呼び出しの所は、2 節のとおり(プロジェクトの関数は `project_body_unread`)。
- 既定で失敗にはしない(1 関数のために、解析全体を止めるより、見える形で続ける方がよい)。止めたい人のために、`-strict-extract`(仮の名前)のようなフラグは、実装の PR で、要るかを決める(本メモの範囲外)。

## 4. rustc のパニックのメッセージが、大量に出ないようにする

今のパニックは、1 回で `thread 'rustc' panicked at ...` の 3〜4 行(`RUST_BACKTRACE` があれば、バックトレース全部)。async closure の多い依存(`google-cloud-*` は、数十回)では、stderr に、大量に出る。

- ドライバ(`rustc` として呼ばれるプロセス)の起動時に、`std::panic::set_hook` を 1 度入れる。**catch の範囲の中(スレッドローカルのフラグが立っている間)だけ静かにし**、メッセージの最初の行と場所(`info.location()`)を、スレッドローカルに溜める。範囲の外のパニック(未知のバグ)は、元のフック(`take_hook` で取った既定)に任せて、今までどおり全部出す。
- catch したら、1 行だけ出す: `rivus-rust: skipped <関数> (<crate>): <メッセージの 1 行目> at <場所>`。1 クレートで 5 件を超えたら、出さずに数だけ数える(最後に `... and K more in <crate>`)。`RIVUS_RUST_PANIC_VERBOSE=1` で、全部出す(診断用)。
- cargo の出力は、extractor の `run_tee_stderr`(`fix/rust-dep-compile-message` のブランチ)で通るので、行の数が減れば、そのまま短くなる。

## 5. 実装の規模

- `extractor/rust/src/main.rs`: `extract()` の 2 つのループ(約 20 行ずつ)を、`catch_unwind` つきの小さな関数にまとめる。フック(約 30 行)、`fn_skipped` レコードの出力(約 15 行)、`cx.bodies` からの除外の確認(数行)。**合計は、約 100 行。** `lower.rs` は変えない(`resolve` の注意書きは、「クエリの外のパニックだけ catch する」に直す)。
- Go 側: `internal/ir`(`FnSkipped` の型、ストリームの読み書き。約 30 行)、`cmd/rivus`(読み込みから `Program` への渡し。約 20 行)、`internal/report`(`functions_skipped` の構造と集計。約 60 行。`body_replaced_packages` の書き方に合わせる)、`internal/taint`(呼び出しのサイトで、callee が `fn_skipped` なら、`dependency_body_unread` / `project_body_unread` の注記を付ける。約 30 行)。**合計は、約 130 行 + テスト。**
- 文書: `docs/ARCHITECTURE.md`(`summary` の項目)、`docs/AGENT-GUIDE.md`(出力の読み方)、`docs/MODELING.md` は変えない。ADR(実装の PR と一緒)。

## 6. `rivus` 側の変更と、`-no-deps` との関係

- `summary.functions_skipped`(2 節)と、標準エラーの 1 行。`-no-deps` では、依存を読まないので、依存の関数は飛ばされない(数えない)。プロジェクト自身の関数は、`-no-deps` でも同じに数える。
- **`-no-deps` は、今回の方針のあとも、残る**(依存を読まない選択として)。落ちたときの案内(`fix/rust-dep-compile-message` のメッセージ)は、catch できないパニックと、普通のコンパイルエラー用に残す。この方針が入ったあと、async closure で止まることは、なくなる。
- モデルとの関係: 飛ばした依存の関数に、モデルの行があれば、行が使われる(依存の本体の代わり)。`body_replaced_packages` で宣言したクレートは、段階 4(extractor に宣言を渡す)が Rust にできれば、そもそも読まない(別の課題)。

## 7. テスト

- **合成の crate**(リポジトリの中、`bench/` の Rust のテスト用。第三者のコードなし): 上の `ac` と同じ形(`before`、async closure を使う `async fn`、`after`。それを呼ぶ関数と、source から sink への流れ)。`RIVUS_RUST` を設定したテストで、(a) 終了コード 0、(b) `functions_skipped` に、その crate と 2 関数、(c) `before` と `after` を通る流れは、今までどおり見つかる、(d) 飛ばした関数を呼ぶ所に `dependency_body_unread` / `project_body_unread`(モデルの行がある関数は、注記なし)、(e) stderr に `thread 'rustc' panicked` がなく、`skipped` の行が 1 つ、を確認する(`skipOrFail(t, "RIVUS_RUST")`。CI の `rust` ジョブで必ず動く)。
- Go の単体テスト: `fn_skipped` の読み書き、`functions_skipped` の集計(プロジェクトと依存の分け、上位 10、`examples` 3)。
- Rust の単体テスト: フックの範囲の中と外(範囲の外のパニックは、既定のフックに行く)。
- 公開の依存での確認(サンドボックスの下だけ): `google-cloud-storage` 1.22.0 を使う最小の crate が、`rivus analyze -lang rust` で完走し、`functions_skipped.crates` に `google-cloud-auth` が出る。

## 8. 決めたこと(相談役。2026-11-23)

- `fn_skipped` は新しいレコード。
- プロジェクトの関数が飛ばされたとき、失敗にするフラグは、今は作らない(警告つきで続行。`project` が 0 でないときは、stderr の最後の行で目立たせる)。フラグは、求めが出てから。
- クレート単位の逃げ道(1 節)は、見つかってから。
- 呼び出しの所の注記は、`unmodeled_library_call` にしない(2 節)。
