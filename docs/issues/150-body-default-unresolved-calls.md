# 150: 差分テスト(標準ライブラリ): 本体を読む側が、解決できないインターフェース呼び出しを「引数が結果に流れる」と近似し、io.Copy / io.WriteString のエラーに見かけの流れが出る

- 状態: **open**
- 種類: 道具・ベンチ(差分テストの A 側の近似)
- 言語: Go
- 見つけたもの: `bench/model-diff`(`make model-diff PKG=...`)、PR #195

`bench/model-diff` の A(本体を読む側)は、`io.WriteString` と `io.Copy` の `arg1` からエラー(`result1`)への流れを見つける。本体を読むと、エラーは書き込む側(読み出す側)のもので、渡したデータは入らない。`w.(StringWriter)` / `dst.(ReaderFrom)` / `src.(WriterTo)` のように、型表明で得たインターフェースの呼び出しが未解決(実装を絞れない)で、未解決の呼び出しの既定(引数は結果へ流れる)が使われたための、見かけの流れ。

モデルに足すと誤検出が増えるので、差分テストの注釈 `# model-diff: miss-reviewed io.Copy@arg1 body-default ... #150` で残している(`internal/model/builtin/go-stdlib.yaml`)。

本体を読む側が、型表明で得た未解決の呼び出しを、既定で近似するのをやめる(または、結果のうちエラーには流さない)と、この種類の注釈は要らなくなる。要らなくなったら、注釈は「stale note」と出る。
