# ADR の一覧

ADR は `docs/adr/` に 1 件 1 ファイル(`NNN-<内容が分かる名前>.md`)。**一覧の表は、手で持たない**。番号、題、状態、関連する文書の表は、各ファイルの先頭のヘッダから `make docs-gen` が作る(コミットしない)。この文書を足す PR は要らない。

- 一覧: `make docs-gen WHAT=adr`(標準出力)。ファイルに書くなら `make docs-gen WHAT=adr OUT=docs/ADR-TABLE.md`(`.gitignore` の対象)。
- ディレクトリの一覧(`ls docs/adr`)も、そのまま索引になる。ファイル名が内容を表す。
- ADR は決めた時点の記録で、書き換えない。**今どうなっているか**は、各 ADR のヘッダの `状態` と、[ARCHITECTURE.md](ARCHITECTURE.md)、[PRINCIPLES.md](PRINCIPLES.md) で見る。
- 書き方、足し方、ヘッダの形は [adr/README.md](adr/README.md)。

## 出典の種類

- **ブリーフ**: 初期ブリーフで決まっていたこと
- **レビュー**: ブリーフへの質問(`history/QUESTIONS.md`)と回答を経て決まった、または訂正されたこと
- **実装**: 実装の途中で決めたこと

## 状態

覆すときは、新しい ADR を足し、古い ADR のヘッダの `状態` だけを直す(`一部覆った(→ADR-MMM)` / `覆った(→ADR-MMM)`)。

今の設計の説明は [ARCHITECTURE.md](ARCHITECTURE.md)、設計思想は [PRINCIPLES.md](PRINCIPLES.md)。
