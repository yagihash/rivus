# ADR-026: 自作 SSA 化と differential test は Phase 1 終盤に導入する

- 状態: 有効
- 関連: ARCHITECTURE 2.7
- 状態の詳細: 完了: 036
- 要旨: 案 R を採用

- **状況:** コアの SSA 化（`ssabuild`）と、lifted `go/ssa` を oracle にした differential test（ADR-003）を、いつ導入するか。
- **検討した案:**
  - 案 P: Phase 2 で導入。Phase 1 は `go/ssa` だけで済み、設計に集中できる。ただし Phase 2 で SSA 化のバグと IR 切り出しのバグが同時に出て、切り分けが難しい
  - 案 Q: Phase 1 の早い段階で導入。バグを早く潰せるが、Phase 1 の目的（理論と設計を固める）から外れ、ブリーフの順序ともずれる
  - 案 R: Phase 1 の終盤で導入
- **決定:** **案 R。** Phase 1 のステップ 1.11b で導入する。
- **理由:** 解析の設計がほぼ固まり、`testdata` が揃った後なら、SSA 化の問題は「検出結果が変わった」という形で検知できる。
  Phase 2 の前に SSA 化だけを単独で検証できるので、Phase 2 の不確実性は「IR を挟むこと」だけに絞れる。
  ブリーフの順序への抵触も最小限。
- **帰結:** Phase 2 は `gossa` を extractor に置き換えて IR を挟むことに集中できる。

