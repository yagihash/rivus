# 153: 差分テスト(標準ライブラリ): 本体を読む側は、`any` の型 switch を通る入力を追えず、結果に届く流れを落とす

- 状態: **open**
- 種類: 道具・ベンチ(本体の読みの盲点)
- 言語: Go
- 見つけたもの: `bench/model-diff`(`make model-diff PKG=...`)、PR #195

`bench/model-diff` の A は、引数が `any` で、本体が型 switch で取り出す関数の流れを見つけない。`fmt` の `Sprintf`(arg1)、`Sprint`、`Sprintln`(arg0)、`Fprintf`、`Fprint`、`Fprintln`(書き手へ)と、`go/parser.ParseFile` の `src`。`mime/multipart.Writer.CreateFormFile` は、ヘッダの行を `fmt.Sprintf` で作る(**2 プローブ**)。**10 プローブ**(fmt 6、`ParseFile` 2、`CreateFormFile` 2)。`fmt` の出力は引数でできているので、モデルの行は正しい。

`# model-diff: extra ...` の注釈で残している。

設計の材料: これは、本体を読むと検出が**減る**例(`fmt.Sprintf` を本体で読むと、引数が結果に届かない)。`any` を型 switch で開く関数は、「中を読まない」と宣言したほうが、見逃しが減る。実際の解析で、`fmt` が本体を読まれる経路があるかを、別に確かめる。
