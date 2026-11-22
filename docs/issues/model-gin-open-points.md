# gin のモデル: zoo のセルで確認できない行がある(sink 7 行と、レシーバ位置の flows 51 行)

## 何が見逃されるか(確認できない行)

`internal/model/builtin/go-gin.yaml`(gin v1.12.0)の次の行は、いまの zoo のセルでは、流れを確認できない。モデルが誤っていても、セルでは気づけない。

1. **sink の 7 行**(`go-gin.dead-allow` に理由つきで載せた): `(*gin.Context).Redirect`、`File`、`FileFromFS`、`FileAttachment`、`SaveUploadedFile`、`String`、`Data`。骨組みの本体が何も記録しないので、`@sink(<tag>)` を実行の記録と照合できない。宣言の確認は `bench/model-diff` だけ。
2. **レシーバ位置の flows(`*gin.Context` の getter と binder、`Params.Get`、`Params.ByName`)**: 差分テストで「未検証」の 51 行。`*gin.Context` は `New...` の関数も変換もない型で、source を入れられない。確認は zoo のセル(`query`、`bind`)だけで、そのセルが当たるのは `Query`、`Param`、`GetHeader`、`PostForm`、`Cookie`、`ShouldBindJSON` の 6 行。残りの 45 行は `.dead-allow` にある。

## 原因

1. 骨組みの sink が `mark.Sink*` を呼ばない(呼ぶと、記録の場所が骨組みの中になり、セルの行と合わない)。
2. 差分テストに、Go の型への値の作り方の注釈(Dart の `model-diff: value` に当たるもの)がない。

## 再現するセル

なし(骨組みが `zoo/mark` を呼ぶ形にするまで。別の作業が進行中)。`@miss` / `@fp` のセルは作らない。
