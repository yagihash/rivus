# ADR-003: IR は φ なしの CFG ＋ 書き換え可能ローカル変数

- 状態: 有効
- 関連: ARCHITECTURE 2.1-2.3
- 状態の詳細: 命令数は 039/040 で具体化、IR 増分は ADR-043 追記
- 要旨: CFG の高さで切る。SSA 化はコア一本。Go は NaiveForm、lifted go/ssa は oracle

- **状況:** AST レベルで切ると構文差が大きすぎる。SSA レベルで切ると SSA 化を 3 言語ぶん書くことになる。
  さらに `go/ssa` は SSA 化済み（φ 入り）、Rust の MIR は SSA ではなくローカル変数の書き換え形、と形が揃っていない。
- **決定:** IR は**CFG のレベル**で切り、**φ を持たない**。値は書き換え可能なローカル変数で表す。SSA 化（dominator 計算と φ 挿入）はコアに一本化する。
  Go extractor は `ssa.NaiveForm` で `Alloc` / `Store` / `Load` が残る形を取る。
  lifted 版の `go/ssa`（NaiveForm なし）は捨てず、**コアの SSA 化の differential test の oracle** にする。
- **検討した代替:** IR に `phi` を任意で許し、コアの SSA 化を冪等にする案。
  `NaiveForm` により Go 側の追加コストがなくなったため、IR を複雑にするだけの利点がないとして不採用。
- **理由:** 制御構造は CFG まで下げると言語差がほぼ消える。3 言語が同じ抽象度に揃い、IR 設計が単純になる。
  SSA 化は言語非依存のグラフアルゴリズムなので一度だけ書けばよい。
  SSA 化は自作部分で最もバグを埋め込みやすく、症状が分かりにくいため、実績のある実装を正解として持つ価値が大きい。
- **帰結:** 命令セットは当初見積もりの 15 種類程度ではなく 26 種（ARCHITECTURE.md 2.3）。
  `slice` は独立命令にせず `copy` の `view` フラグに畳み（構文として存在するのは主に Go で、汚染の意味は `copy` と同じため）、
  `global_load` / `global_store` は独立させる（関数サマリの境界と `global_var` の書き込み箇所の列挙に必要なため）。
  `async` / `await` と例外は CFG のエッジで表し、`go` / `defer` / `await` は `call` のフラグで表す。
  イテレータなど言語固有の構文は extractor が脱糖する。分岐条件は式言語を作らず、値への参照 ＋ ソース範囲で持つ。

