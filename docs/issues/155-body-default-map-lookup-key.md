# 155: 差分テスト(標準ライブラリ): 本体を読む側は、マップの検索のキーが、見つけた値に流れると近似する(Header.Get / Values)

- 状態: **open**
- 種類: 道具・ベンチ(差分テストの A 側の近似)
- 言語: Go
- 見つけたもの: `bench/model-diff`(`make model-diff PKG=...`)、PR #195

`bench/model-diff` の A は、`net/http.Header.Get` / `Values` の `arg0`(キー)から結果への流れを見つける(**2 プローブ**)。値はキーでできていないので、データの流れではない。マップの要素の取得を、キーの値にも依存すると近似するための、見かけの流れ。

モデルに足すと誤検出が増える(キーは利用者の入力であることが多く、結果が汚染される)ので、`# model-diff: miss-reviewed (net/http.Header).Get@arg0 body-default #155 ...` で残している(`go-net-http.yaml`)。

本体を読む側が、マップの検索でキーを結果に流さないようにすれば、この種類の注釈は要らなくなる(#150 と同じ種類)。
