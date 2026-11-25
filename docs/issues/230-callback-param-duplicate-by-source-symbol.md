# #230 同じ流れが、helper の経由で、source の `symbol` 違いで 2 件出る

- 状態: **open**
- 優先度: 低
- 種類: 出力の重複(精度ではなく、出力の形)
- 言語: Dart
- ROADMAP の節: -
- 守るもの: -
- 進める完了条件: -

## 何が起きるか

`callback_param` の行がある関数 A が、同じ行のある関数 B を、受けた関数のまま呼ぶとき(Dart の shelf_router では、`Router.get` が `Router.add` を呼ぶ)、ハンドラの引数に、A の行と B の行の、2 つの source のサイトが付く。ハンドラの引数から sink への流れは 1 つだが、source の `symbol` だけが違う finding が 2 件出る。zoo の `F/dart/shelf-router-extra-param`(vuln と decoy)で、source の位置と sink の位置が同じ finding が 1 件ずつ増えた(helper の呼び出し元をたどる変更の前は、1 件だった)。

流れは本物で、誤検出ではない。出力が重複して見えるだけ。

## 直す方向(案)

重複を畳む条件を、source の位置と sink の位置が同じで、経路の末尾が同じものに限る、など。`callback_param` の変更の PR では畳まない。
