# dio のモデルの、あとで対話的に見る 3 点

## 何が見逃されるか

(a) `BaseOptions(baseUrl: ...)` と、`RequestOptions` / `Options` の名前つき引数(`baseUrl`、`path`、`data` など)は sink にしていない。`named:` で書けるので、モデルに足す候補。
(b) `Interceptor`(`onRequest` などのオーバーライド)と `HttpClientAdapter`(`fetch`)は、dio が呼ぶ合流点で、モデルがない。dio が利用者のインターセプタへ渡す値は、追えるか確認していない。本体を読まない宣言の候補。
(c) dio のメソッドはジェネリクス(`get<T>`)で、差分テスト(`make model-diff PKG=package:dio`)が 39 行すべてを未検証にする(probes 0)。

## 原因

(a) 設定の値は、利用者の入力であることが少ないと判断して、最初のモデルでは外した。名前つき引数の sink を当てる骨組みが増えるのも理由。
(b) `calls:` の行は `TestZooCallbacksDart` の道具(標準ライブラリだけ)を通る必要があり、パッケージの行は書けない。`Interceptor.onRequest` は利用者がオーバーライドするメソッドで、`calls:` の `fn: argN` では指せない。
(c) 差分テストの正規表現が、ジェネリクスのメソッドのシグネチャを読めない。

## 再現するセル

なし(対話的な改善の候補)。
