# 154: 差分テスト(標準ライブラリ): 本体を読む側は、マップ型の変換(`textproto.MIMEHeader(h)`)を通る書き込みを、元のマップに結ばない

- 状態: **open**
- 種類: 道具・ベンチ(本体の分析の盲点)
- 言語: Go
- ROADMAP の節: 後回し
- 守るもの: `bench/model-diff` の注釈
- 題: 差分テスト(標準ライブラリ): 本体を読む側は、マップ型の変換(`textproto.MIMEHeader(h)`)を通る書き込みを、元のマップに結ばない
- 見つけたもの: `bench/model-diff`(`make model-diff PKG=...`)、PR #195

`bench/model-diff` の A は、`net/http.Header.Set` / `Add` の `arg1`(値)がレシーバ(ヘッダ)に入る流れを見つけない(**2 プローブ**)。本体は `textproto.MIMEHeader(h).Set(key, value)` で、マップ型を変換した値への書き込みが、元のマップへの書き込みとして追われない。ヘッダに値が入るのは確か(モデルの行は正しい)。

`# model-diff: extra (net/http.Header).Set #154 ...` の注釈で残している(`internal/model/builtin/go-net-http.yaml`)。

設計の材料: 本体を読む側が、マップ(スライス)型の変換を別名として扱えば、この種類の注釈は要らなくなる。
