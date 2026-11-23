# ADR-040: Phase 2 の結果: 解析エンジンを中立 IR の上に移した（ステップ 2.0〜2.6）

- 状態: 有効
- 関連: ARCHITECTURE 2.7
- 要旨: 旧経路を除く。コアは go/ssa を知らない

**結果:** Go のコードは、`go/ssa`（NaiveForm）→ extractor（`goir`）→ 中立 IR → SSA グラフ（`irssa`、`sg`）→ エンジン（`taint`）の順に流れる。コアは `go/ssa` を知らない（`go list -deps` で確認）。旧経路は除いた。

**照合（移行中に両経路を動かして JSON を比べた）:**
- サイト: ID、指紋 4 種、位置、スニペット、ラベル、`literal_only`、並び順が、testdata（212 サイト）、rivus 自身、ghmint、x/tools（243 サイト）で完全に一致
- 解析結果: testdata の全ケース、rivus 自身（36 検出 / 614 不確実性）、ghmint（15 / 176）で完全に一致。x/tools（4 / 10,628）は、不確実性が 1 件だけ違う（generic な関数の中の range-over-func で、位置のない合成された呼び出しの扱い）。許容した

**決めたこと（ADR-039 への追加・修正）:**
1. **テキストは言語ごとの `ir.Texts` が切り出す**（ADR-039 の 3 を修正）。文の範囲と式の正規化は言語の構文の問題で、指紋の元になるため
2. **TypeClass は 9 つ**（`pointer` / `interface` / `func` / `container` / `struct` / `bool` / `string` / `number` / `other`）。`container` は「別の場所の要素を指す値」（スライス・map・チャネル）に絞り、配列は `struct`。`bool` はモデルのない呼び出しの結果を捨てるのに、`string` / `number` は定数のゼロ値の綴りに必要
3. **言語の操作は、シンボル付きの `call` にする**（チャネル、範囲、`select` の送受信）。専用の命令は増やさない（命令は 26 のまま）。これらのシンボルの意味は、エンジンが知っている（将来、モデルに出せるかもしれない）
4. **コールグラフは extractor が作り、`call` の `candidates` に入れる。** Go では、VTA が NaiveForm の上だと粗いので、lifted 版から取って対応づける
5. **`rules.Expand`（実装列挙）は、まだ Go のパッケージを直接引く**（`gossa.Program` が `model.Resolver` を実装）。IR の `implementations` レコードを使う形への移行は、2 つ目の言語を足すときに行う

**性能:** x/tools（15.6 万行）で、107 秒・最大 4.9 GB → 97 秒・最大 2.4 GB。IR に移した直後は最大 6.2 GB に増えたが、`-v` の各段階の生きているヒープ（読み込み後 306 MB、IR 後 453 MB、`sg` 後 761 MB、解析後 461 MB）から、増えた分は解析中の状態だと分かった。関数ごとの解析の状態（セルとキャッシュ）を、結果に必要な分（検出・抑制・不確実性）だけ残して手放したら、解析結果は変わらずに 2.4 GB になった。

**残る課題:**
- `taint` の `go/token` と `go/constant`（ガードの定数評価）。言語に依らないので、後回し
- 別プロセスの extractor（Rust、Dart）は、`Texts` と `rules.Expand` の相当を、extractor が提供する形が必要

