# ADR-042: Rust の extractor の最初のバージョン（Phase 3）

- 状態: 一部覆った(→ADR-062, ADR-064)
- 関連: extractor/rust, rust-isolation
- 状態の詳細: ホスト実行は 064 で既定が隔離に、テストを読むことは 062; 訂正・追記あり(書き方を保つ)。追記あり。覆した先・注意: 064/062
- 要旨: `rustc_public` で MIR、固定 nightly、`cargo check`

**結果:** `extractor/rust`（Rust 製）が、`rustc_public` で MIR を読み、中立 IR を JSON Lines で出す。ワークスペースを `cargo check` で、`RUSTC_WORKSPACE_WRAPPER` に自分を指して実行し、rustc は普通にコンパイルし、ドライバが各クレートの IR を書く。`rivus analyze -ir` で読む。サンプル（`extractor/rust/test/sample`）の注釈つきの 12 本の流れ（直接、フィールド、クロージャ、分岐、関数、`&mut` を渡す、`Vec`、`enum`、`format!`、`String::push_str`、`std::env::args` から `Command::new`、`fs::read_to_string`）がすべて見つかり、検出されないはずの sink（定数、`len()` を通したもの）には届かない（`cmd/rivus/rust_test.go`）。

**決めたこと:**
1. **nightly を 1 つに固定する**（`nightly-2026-11-15`）。`rustc_public` は不安定で、コンパイラと一緒に動く。更新は意図して行う
2. **MIR は IR にほぼそのまま下りる**（ブロックの CFG、書き換え可能なローカル）。アドレスを取ったローカル（`&x`）だけセル（`alloc` / `load` / `store`）に置く。射影のある場所は `field_load` など。`(*p).f` は 1 回のアクセス
3. **シンボルは関数のパス**（`std::fs::read_to_string`）。トレイトのメソッドは、実装でなくトレイトの名前（`std::ops::Deref::deref`）で、どの実装にも効く。実装のジェネリクスは落とす。**レシーバは最初の引数**（`arg0`）
4. **呼び出しは `Instance::resolve` で解く:** 本体のあるもの → 静的。ないもの → 外部の静的（モデルが決める）。解けないもの（型パラメータ、トレイトオブジェクト）→ `dynamic`（ワークスペースの impl を候補に）。クロージャは `Fn::call` を型で解き、引数のタプルをほどく
5. **巻き戻し（`drop`、`assert`、クリーンアップ）は描かない。** パニックは関数の中の流れではない
6. **位置は行と列**（`rustc_public` はバイトのオフセットをくれない）。ストリームのオフセットは `行 * 100000 + 列`
7. **コアの修正（Rust で見つかった）:** ポインタの `copy` は同じポインタ（別名）として扱う。以前は、コピーを通した書き込みが元に届かなかった（Dart のオブジェクトの別名にも効く）
8. **Mac の SDK:** 最新の SDK（27.0）をリンカが読めず、`tapi error` で、stable の hello world も通らなかった。`SDKROOT` を古い SDK にすると通る（環境の問題。README に書いた）

**残る課題:** `extractor/rust/README.md` の「Not done」。`async`、ジェネリクスの単相化、フレームワーク（actix-web、axum）の入口、引数のテキスト。

**追記（Rust の仕上げ）:**
- **`core::` と `alloc::` は `std::` に正規化する**（`std` が再公開しているので、ルールは `std::` だけでよい）
- **トレイトオブジェクトの呼び出し**（`Instance` の種類が `Virtual`）は `dynamic` にして、ワークスペースの impl を候補にする。型パラメータ越しの呼び出しも同じ
- **`Box`・`Unique`・`NonNull`・`Rc`・`Arc` のフィールドは、ポインタそのもの**として読む（`vec![a, b]` は `Box::new_uninit` に生ポインタ越しに書き込む形に展開され、フィールドを本物のフィールドにすると流れが切れた）
- ジェネリック関数の呼び出しは、型引数つきのインスタンス名ではなく、定義の名前で本体を探す
- サンプルは、トレイトオブジェクト、ループ、イテレータとクロージャ、`?`、`HashMap`、メソッド、`while`、`match` を含み、`cmd/rivus/rust_test.go` が 20 本以上の流れを確かめる。ジェネリックな関数は多相のまま解析するので、その `T::method` の呼び出しは、すべての impl が候補になる（文脈を区別しない）

**追記（crates.io の `rivus`）:** `extractor/rust` を、クレート名 `rivus`（0.1.0）として公開できる形にした。`cargo package` で、パッケージにしたものがそのままビルドできることを確かめた（`rust-toolchain.toml`、`build.rs`、README、LICENSE を含む）。実行ファイルの名前は `rivus-rust` のまま（`rivus` は Go 本体のコマンドなので、衝突させない）。`cargo +nightly-2026-11-15 install rivus` で入る。**公開はしていない。** 名前を押さえるための `rust/`（0.0.0）は、同じ名前なので、公開の前に、どちらかにまとめる必要がある。

**追記（Rust の実コードでの検証）:**
- **`vercel/workflow`**（SWC プラグイン、1.1 万行、クレート 4 つ）: 抽出 39 秒、1,692 関数、解析 78 ミリ秒、`unsupported` 0。source と sink がないので、検出は 0 件で正しい
- **`vercel-labs/agent-browser/cli`**（ブラウザを操る CLI、14.8 万行）: 抽出 2 分 29 秒（release ビルドのドライバ）、IR 702 MB、34,097 関数。解析は 8〜10 秒、最大 6.7 GB（読み込みが大半）。**`std::io::stdin` から `Command::args` へ**、MCP サーバーが受け取った JSON の `restore` が、起動する子プロセスの引数に届く流れを含め、`std::env::args` / `stdin` から `fs` と `reqwest` の sink への流れ 28 件が出た。経路は 8 関数・約 55 歩で、読める
- **見つけたこと:** (1) 抽出器が、ファイルの行を、位置ごとに複製していて、2 乗の時間だった（`Rc` で共有して直した）。(2) **`file_content`（ファイルの中身）を、sink が受け付けると、「ファイルを読んで、その中身をパスにする」流れが 800 件近く出て、使い物にならなかった。** Rust と Dart の sink は `accepts: [user_input]` だけにした（読んだファイルの中身を信頼しない設定は、ルールで足せる）。(3) 実コードで多かったモデルのない呼び出し（`fmt::Formatter` の `debug_*`、serde、tokio、futures）を `rust-misc.yaml` に足して、モデルなしの呼び出しは 18,681 → 6,191 に減った
- **残る課題:** 動的ディスパッチの不確実性が 1.6 万件（`dyn` の多い非同期コード）。IR が大きい（702 MB）ので、JSON の詳細（位置の重複など）を絞る余地がある

**追記（IR を絞る: 位置のファイル名、1 回目）:**
- agent-browser の IR の中身を測ると、関数のレコードが 99.8%、そのうち命令が 73%、命令の位置（`pos`）だけで 44.8% だった。位置の `file`（パス）が、命令ごとに繰り返されていた
- **`Func.file`** を足した。関数が、その位置が入っているファイルを 1 回だけ書き、そのファイルにある位置は `file` を省く。読み込みが戻す（`ir.Func.expand`）ので、コアは変わらない。書く側は `Program` の書き出し（Go）と Rust の extractor が使う。Dart は、まだ省かない（読む側は両方を受け付ける）
- 読み込みは、型・関数・シンボルの文字列を共有する（`internal/ir/compact.go` の `interner`）
- 結果: agent-browser の IR が 702 MB → **523 MB（−25%）**。解析の結果は、件数が同じ（findings 28、uncertainties 22,529）。最大メモリは 7.1 GB → 7.0 GB で、ほとんど減らなかった。メモリの大半は、IR の文字列ではなく、SSA とグラフにある。**メモリを減らすには、IR の形ではなく、読んだあとの表現を絞る必要がある**
- 残る候補: `line`/`col`/`offset` の重複（Rust は `offset` が `line`,`col` から決まる）、直前の命令と同じ位置の省略、`locals` と基本ブロックの終端

**追記（メモリを減らす: 2 回目）:** agent-browser の最大 RSS が **7.0 GB → 2.9 GB**（x/tools は 2.4 GB → 1.2 GB）。時間は同じ（9 秒、13 秒）で、findings と uncertainties の件数も同じ（28 / 22,529、4 / 9,883）。
- **測ったこと:** 生きているデータは、IR が 1.3 GB、グラフが 1.6 GB で、IR とグラフが同時に残っていた。グラフは、IR の命令（`sg.Instr.IR`）と、SSA 形式（`sg.Func.Form`）を持ち続けていたので、IR は解放できなかった
- **やったこと:** (1) `sg.Func.Form` を持たない（誰も読んでいなかった）。(2) `sg.Instr.IR` を、読む 9 個のフィールドだけの `InstrInfo` にして、`sg.Func.IR` は、ブロックと変数を落とした写しにした。(3) `sg.BuildConsuming` が、関数を 1 つ作るごとに、IR の関数を手放す（`Build` は、テストのために、IR を残す）。そのため、サイトの列挙を、グラフの構築の前に動かした。(4) GC の `GOGC` を 50 にした（環境変数があれば、それが優先）。時間はほとんど変わらず、最大 RSS が約 1.7 GB 減った
- **残り:** 読み込み（`ir.Read`）は、全部を読んでからグラフにするので、そのときに IR 全体（1.3 GB、`reflect.growslice` が大半）を持つ。読みながらグラフにすれば、さらに約 1 GB 減らせる。解析中の一時割り当て（`applySummary`、`collectUncached`）は、まだ手を付けていない

**追記（Rust の extractor は、テストを既定で読まない）:** `cargo check` から `--all-targets` を外した。ライブラリと実行ファイルだけがコンパイルされるので、`#[cfg(test)]`、`tests/`、`examples/`、`benches/` は、**ビルドの対象にならない**（名前や場所で見分けるのではなく、確実に入らない）。`rivus-rust --include-tests [dir]` で、`--all-targets` に戻せる。agent-browser では、関数の約 16%（34,097 のうち約 5,600）がテストのモジュールだった。含めたときも、含めないときも、利用者が前提を取り違えないように、README に書いた。Dart（`lib/` だけを読む）と Go（パッケージの読み込みの設定）は、別の話で、今回は変えていない。

