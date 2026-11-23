# ADR-039: Phase 2 の IR を、Phase 1 の実測から具体化する（案）

- 状態: 一部覆った(→ADR-040)
- 関連: ARCHITECTURE 2.7
- 状態の詳細: 「コアがテキストを切る」を 040 が修正; 覆した先・注意: 040(状態が「案」のまま)
- 要旨: 命令数 26 のまま、型は TypeClass のみ

**状況:** 2.3 の命令セット（26 種）は Phase 1 の前に書いた。Phase 1 で、エンジンが `go/ssa` から使っているものが分かった（ARCHITECTURE 2.7.1）。

**案:**
1. 命令数は 26 のまま。`call` の戻り値をリストにし、`addr_of` に経路を持たせ、クロージャのキャプチャ値を `param` で受け、引数ごとのソース範囲を持たせる（フィールドの追加だけ）
2. 型は `TypeID` に加えて `TypeClass`（`pointer` / `interface` / `func` / `container` / `struct` / `scalar` / `other`）だけを持つ。構造は持たない
3. テキスト（スニペット、式、本体）は IR に運ばない。IR は `Pos` を持ち、テキストは言語ごとの `ir.Texts`（extractor と同じプロセス、同じ構文木）が位置から切り出す。**当初は「コアが切り出す」としたが、ステップ 2.3 で修正した:** 文の範囲や式の正規化は言語の構文の問題で、`expr@1` / `body@1` のハッシュの元になるため、コアが言語を知らずに切り出すことはできない
4. コールグラフと実装列挙は extractor の責務。結果は `call` の `dynamic(candidates)` と `implementations` レコードで IR に入る
5. Go は同じプロセスの中で IR の型を直接渡す。JSON Lines は `rivus extract` と、他言語の extractor のためだけに使う。両経路の一致をテストする
6. `const` は文字列の中身を省略せず持ち（上限を超えるときは `truncated` を明示）、文字列補間は extractor が `binop` か補間専用の `call` に展開する。テキストの切り出しは表示と指紋のためだけで、判定には使わない（ARCHITECTURE 2.7.2b）
7. 移行中は旧経路（`go/ssa` 直）と新経路を並走させ、`Report` を照合する。一致するまで旧経路を消さない

**未確定（レビューで決めたい）:**
- `TypeClass` の分類が足りるか（たとえば Rust の `Box` / `Rc` / 参照、Dart の `Future` / `Stream` をどの分類にするか）。Phase 3 の入口で Rust を見てから変えうる
- 「テキストは IR に運ばない」ことで、extractor を別プロセスにしたとき、コアが同じソースを読める前提が必要。コンテナ越しなどでは崩れる
- `Select` / `Send` / `Recv` を、命令にせずシンボル付きの `call` にしてよいか（`concurrency` の不確実性の出し方が、シンボルの名前に依存する）

