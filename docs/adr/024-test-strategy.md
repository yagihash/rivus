# ADR-024: テスト戦略

- 状態: 有効
- 関連: DESIGN-zoo, bench/README
- 状態の詳細: zoo 等で拡張: DESIGN-zoo、ADR-043/046
- 要旨: 検出するケースと「検出されない」ケースを両方 assert

- **決定:**
  - 言語ごとのマイクロベンチマーク集（`testdata/`）。検出されるべきケースと、意図的に検出できないケース（「検出されないこと」を assert）の両方を置く。期待される terminal kind も assert する
  - コアの SSA 化と lifted `go/ssa` の differential test（ADR-003）
  - Phase が進むごとに、同じ意味のコードを複数言語で書いたクロス言語の等価性テストを足す（IR の中立性の検証）
  - rivus 自身を rivus で解析し、`attributable_to: rivus` の比率を監視する
- **理由:** これがないと、精度を上げたつもりで別のものを壊したことに気づけない。後から足せないので最初に決める。

