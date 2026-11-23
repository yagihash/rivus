# ADR-101: `examples/rules` のフレームワークのルールは、`docs/MODEL-QUEUE.md` に入ったものから、組み込みに昇格する(ADR-063 を ADR-096 / 097 の範囲で上書きする。2026-11-23)

- 状態: 有効
- 関連: MODEL-RUNBOOK、MODEL-QUEUE、examples/rules/README
- 状態の詳細: ADR-096 / 097 の範囲。個々の昇格の PR は ADR を書かない。手順書の「例からの昇格」
- 要旨: 一覧にないフレームワークは、今までどおり利用者が足す

**背景:** ADR-063 は「特定のフレームワークは、組み込みでなく `examples/rules` に置く(利用者が足す)」と決めた。ADR-096 / 097 は、組み込みの範囲を、外部のサービスのクライアントと、pub / Go / Rust の人気のパッケージ(公開の指標の順)まで広げた。`examples/rules/` の `go-gin.yaml`、`rust-actix-web.yaml`(ADR-063 より前に組み込みだったもの)は、`docs/MODEL-QUEUE.md` の「例から昇格」の行になった。手順書(`docs/MODEL-RUNBOOK.md`)は、最初の昇格の PR が新しい ADR を足す、としていたが、モデルの PR は `docs/DECISIONS.md` をさわらない(4 節)、ADR を書かない(6 節)ので、昇格の PR が、手順書のとおりには作れなかった(検証用の 2 本目)。

**決めたこと:**
1. `examples/rules/` のフレームワークのルールは、`docs/MODEL-QUEUE.md` の「例から昇格」の行に入ったものから、組み込みに昇格する。ADR-063 の「特定のフレームワークは利用者が足す」は、ADR-096 / 097 の範囲(`docs/MODEL-QUEUE.md` に入っているもの)で、この ADR が置き換える。範囲の外(一覧にないフレームワーク、利用者自身のラッパー)は、今までどおり、利用者が `.rivus/rules/` に足す。
2. **個々の昇格の PR は、ADR を書かない**(この ADR を参照するだけ)。昇格の PR の作業は、手順書の「例からの昇格」と 4 節のとおり。
3. 昇格した例のファイルは、`examples/rules/` から**消す**。`examples/rules/README.md` の、そのファイルの行は、「組み込みになった(`internal/model/builtin/<ファイル>`)」の 1 行にする(ファイル名で探す利用者のため)。`bench/*/rules.yaml` の例も同じ。
4. 既存の ADR の本文は書き換えない(ADR-063 の 2026-11-22 の追記と、ADR-077、ADR-096、ADR-097 は、この ADR の範囲で読む)。

**しないこと(この ADR では)**: 個々のモデルの内容、昇格の順番(`docs/MODEL-QUEUE.md`)。

