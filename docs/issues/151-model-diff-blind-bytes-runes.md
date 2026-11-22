# 151: 差分テスト(標準ライブラリ): 本体を読む側は、バイトとルーンを 1 つずつ扱う関数で、入力が結果に届く流れを落とす(数として止める)

- 状態: **open**
- 種類: 道具・ベンチ(本体の読みの盲点)
- 言語: Go
- 見つけたもの: `bench/model-diff`(`make model-diff PKG=...`)、PR #195

`bench/model-diff` の A(本体を読む側)は、`encoding/hex`(`Encode`、`AppendEncode`、`DecodeString`、`EncodeToString`)、`encoding/base64`(`DecodeString`、`EncodeToString`)、`strconv.Quote`、`encoding/json.MarshalIndent`(arg0)、`encoding/xml` の `Decoder.Token` で、入力から結果への流れを見つけない。本体がバイトやルーンを 1 つずつ読んで書くので、1 つの値が数として止まる(`numeric_stop`)。**9 プローブ**(hex 4、base64 2、Quote 1、MarshalIndent 1、xml Token 1)。モデルの行(入力 → 結果)は正しい。

モデルの行は、`# model-diff: extra <symbol> ...` の注釈で残している。

設計の材料: 本体を読んで流れを落とす関数は、「中を読まない」(モデルの行を使う)と宣言したほうが検出が増える。ここで見つかったのは、標準ライブラリの 9 関数。本体を読む側のコストと検出のどちらを取るかの判断に使う。この種類の注釈は、これが直れば要らなくなる。
