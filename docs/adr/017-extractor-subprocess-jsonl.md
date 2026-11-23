# ADR-017: extractor との通信はサブプロセス ＋ JSON Lines

- 状態: 一部覆った(→ADR-039, ADR-040)
- 関連: ARCHITECTURE 2.6
- 状態の詳細: Go は同プロセスで IR を渡す: 039/040
- 要旨: バージョン付き JSON Lines、IR ダンプは一級

- **決定:** extractor はサブプロセスとして起動し、**バージョン付き JSON Lines** で IR を受け取る。先頭行は必ず `schema_version`。
  IR ダンプ（`rivus extract --lang=... --output=ir.jsonl`）を一級の成果物にする。
  重くなったら protobuf に移行する。extractor の出力の内容アドレスキャッシュ（キー = extractor のバージョン + 入力ファイル群のハッシュ + 設定）は、Phase 1 では不要だがインタフェースだけ想定しておく。
- **理由:** Phase 4 で Dart の CFG ビルダーをデバッグするとき、IR を人間が読んで検証できることが生命線になる。JSON Lines を選ぶ最大の理由はこれ。

