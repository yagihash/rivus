# #244 標準ライブラリの型の性質(「ほかの書き込み先を持たない」「書き込みが中身だけに入る」)が、コードの表にあり、ルールで宣言できない

- 状態: **open**
- 種類: 道具(重さ: 低)
- 言語: Rust / Go
- ROADMAP の節: -
- 守るもの: -
- 進める完了条件: D(1)

## 何が起きるか

型の性質が、コードの表で決まっている。(1) `internal/taint/guards.go` の `stdLeafType`: Rust の `Vec`、`String`、`File` など 12 種は、ほかの書き込み先を持たないとして、`dyn Write` の呼び出しの候補から外す。(2) `internal/taint/calls.go` の `contentOnly`: Go の `bytes.Buffer` と `strings.Builder`、Rust の標準ライブラリの型は、書き込みが中身だけに入る(ADR-082)。

利用者は、自分のライブラリの型について、同じ宣言ができない。表にない型は、「持つかもしれない」として流れを残す(過検出の側)ので、見逃しにはならない。

## 直す方向の案(推測)

ルールのファイルに、型の宣言の節を足す(型の名前、「ほかの書き込み先を持たない」「中身だけに書き込む」)。書式の設計が先。一覧は [HARDCODED-APIS.md](../HARDCODED-APIS.md) の H2 と H3、[YAML-LIMITS.md](../YAML-LIMITS.md) の L9。
