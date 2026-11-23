# 設計判断記録（ADR）

rivus の設計判断を、後から経緯を追えるように記録する。形式は「状況 → 決定 → 理由 → 帰結」。

**ADR は `docs/adr/` に 1 件 1 ファイル**(`NNN-<内容が分かる名前>.md`)。一覧は `make docs-gen`(索引は作って見る。コミットしない)。足し方と見方は [adr/README.md](adr/README.md)。この文書と [ADR-INDEX.md](ADR-INDEX.md) は案内だけで、ADR を足すときに触らない。

## 出典の種類

- **ブリーフ**: 初期ブリーフで決まっていたこと
- **レビュー**: ブリーフへの質問（`history/QUESTIONS.md`）と回答を経て決まった、または訂正されたこと
- **実装**: 実装の途中で決めたこと

## 状態

ADR の本文は、決めた時点の記録で、書き換えない。覆すときは、新しい ADR を足し、古い ADR のヘッダの `状態` だけを直す(`一部覆った(→ADR-MMM)` / `覆った(→ADR-MMM)`)。

今の設計の説明は [ARCHITECTURE.md](ARCHITECTURE.md)、設計思想は [PRINCIPLES.md](PRINCIPLES.md)。
