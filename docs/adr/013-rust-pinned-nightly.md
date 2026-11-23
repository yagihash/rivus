# ADR-013: Rust は固定 nightly が前提

- 状態: 有効
- 関連: rust-isolation
- 状態の詳細: 固定値: 042 の `nightly-2026-11-15`
- 要旨: 対象も固定 nightly でビルドできる必要。通る範囲を先に確認

- **状況:** `rustc_public`（旧 `stable_mir`）は nightly 専用。`RUSTC_BOOTSTRAP` で nightly 機能を stable に通す手段は正式サポートではなく、依存すると脆い。
- **決定:** 制約を先に宣言する。extractor は特定の nightly に固定される。**解析対象のコードも、その nightly でコンパイルできる必要がある。**
  Phase 3 の成果物は「全 Rust プロジェクトを解析できるツール」ではなく「固定した nightly でビルドが通る Rust プロジェクトを解析できるツール」。
  Phase 3 の最初のタスクは、`stable-mir-json` 等の既存ツールで実在のクレートがどれくらい通るかの確認。
- **理由:** 対象が新しい言語機能を使っていたり、edition や MSRV が噛み合わないと失敗する。リスクを受容した上で前提を明記する方が誠実。
- **帰結:** 想定より通らなければ、Rust の優先度を下げる判断もあり得る。

