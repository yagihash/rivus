# 152: 差分テスト(標準ライブラリ): 本体を読む側は、reflect を通る書き込みを追えず、入力が書き込み先に届く流れを落とす

- 状態: **open**
- 種類: 道具・ベンチ(本体の読みの盲点)
- 言語: Go
- 見つけたもの: `bench/model-diff`(`make model-diff PKG=...`)、PR #195

`bench/model-diff` の A は、`reflect` で値に書き込む関数(`encoding/json.Decoder.Decode` が `Token` の中で文字列を書く、`text/template` の `Execute` / `ExecuteTemplate` がデータを書き手に書く)の流れを見つけない。**3 プローブ**(json Token の `result0` 1、`text/template` の `Execute` と `ExecuteTemplate` の `arg1 → arg0` 2)。モデルの行は正しい。

`# model-diff: extra ...` の注釈で残している。

設計の材料: reflect で書く関数は、本体を読んでも流れが見えない。本体を読む側の盲点として、「中を読まない」宣言(モデルの行を使う)のほうが検出が増える。
