# ADR-020: Go 1.27 と `golang.org/x/tools` 最新

- 状態: 有効
- 関連: -
- 要旨: go/ssa は隔離(`internal/frontend/gossa`)

- **決定:** コアは Go 1.27、依存は標準ライブラリ ＋ `golang.org/x/tools`（最新）＋ YAML パーサ。CI は GitHub Actions。rivus 自身を rivus で解析できるようにする。
- **理由:** SSA / 型解決は新しい言語機能への対応がリリースごとに追加されるため、古いと Go 1.27 のコードで失敗することがある。
- **帰結:** Phase 1 では `go/ssa` に触れるコードを `internal/frontend/gossa` に隔離する（Phase 2 の IR 切り出しを楽にするため。必須）。

