# #223 `rivus.query`、`sites`、`resolve`、`coverage`、`graph` のスキーマに、`rivus.result` と同じ形の固定とバージョンの規則を広げる

- 状態: **open**
- 優先度: 低
- 種類: 道具・テスト(出力のスキーマ)
- 言語: -
- ROADMAP の節: -
- 守るもの: -
- 題: `rivus.result` 以外の出力のスキーマにも、形のハッシュの表とバージョンの規則を置く

## 何が足りないか

ADR-127 で、`rivus.result` は、形(キーと型、`kind` と `attributable_to` の値)が変わるたびに、バージョンを上げる規則になった。上げ忘れは `TestResultSchemaVersionMatchesShape` が落とす。ほかの出力のスキーマには、この規則とテストがない。

| スキーマ | 場所 | 今のバージョン |
|---|---|---|
| `rivus.query` | `internal/report/query.go` | `/1` |
| `rivus.sites` | `cmd/rivus/sites.go` | `/1` |
| `rivus.resolve` | `internal/report/resolve.go` | `/1` |
| `rivus.coverage` | `internal/report/coverage.go` | `/0` |
| `rivus.graph`(索引) | `internal/graphdb` | 名前 `rivus.graph/1`、整数のバージョン 2 |

形が変わっても、バージョンが動かないので、古い読む側が、何も言われずに読むことになる(#109、ADR-125、ADR-127 と同じ事例)。

## 対処の案

1. 各スキーマに、ADR-127 と同じ作りのテスト(reflect で型を歩き、キーと型をハッシュにして、表 `<name>Shapes` の `SchemaVersion` のエントリと比べる)を足す。失敗のメッセージは、新しいハッシュと、直す 2 か所を出す。
2. 今の形を最初のエントリにする(`rivus.coverage/0` を `/1` に上げるかは、決める)。
3. `rivus.query` は `taint.Finding` などを `rivus.result` と共有している。`rivus.result` の形が変わると、`rivus.query` の形も変わる。バージョンを揃えるか、別々に上げるかを決める。
4. `rivus.graph` は、名前の `/1` と整数のバージョンの 2 つがある。1 つにするかを決める。表のハッシュの対象に、保存した索引の表の列と、`meta` のキーを含めるかを決める。

A(1) の完了(`rivus.result` のスキーマのバージョンと形のテスト)には、入れない。
