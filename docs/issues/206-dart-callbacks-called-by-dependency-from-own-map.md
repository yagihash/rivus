# 206: Dart: 依存・骨組みのフレームワークが、渡されたコールバックを、自分の map / リストから呼ぶ形が、グラフのエッジにならない

- 状態: **open**
- 種類: 道具・ベンチ(グラフのエッジ)
- 言語: Dart
- ROADMAP の節: 1
- 守るもの: -(エッジだけの欠落は、zoo の注釈(流れの正解)では表せない。観測した呼び出しとの照合の再実行で見る)
- 題: Dart: 依存が、渡されたコールバックを自分の map / リストから呼ぶ形のエッジがない(根 E)

## 現象

`shelf_router` の `Router.call` や、Flutter の `Navigator.pushNamed` のように、依存や骨組みのフレームワークが、登録されたコールバックを、レシーバが持つ map やリストから取り出して呼ぶ形は、グラフ(`rivus index`)で、呼び出し先のエッジがない。観測した呼び出しとの照合(方法 B。`bench/zoo-dart/observed/README.md`)で、`no_edge` として見え続ける。流れは、`callback_param` の行などで足りているセルが多い(#196 の「3」)。

## 根と件数

- 根 E。zoo-dart の `no_edge` 6 件。
- 例のセル: `dart/shelf-router-params`、`dart/flutter-route-args`。
- `-resolve-func-values`(消した。[DESIGN-callouts.md](../DESIGN-callouts.md) 5.6)は、この形の候補を補っていた。フラグで補ったときの過検出の形: `Navigator.pushNamed` の `routes[name]!(...)` に、3 つのルートの関数がすべて付く(候補が呼び出し点ごとに 1 つの集合で、呼び出し元ごとに分かれない)。

## 直す所

- 根 D(#99)と同じ道具(フィールド・map・リストの書き込みから、それを読む側をたどる。`internal/valuecalls/narrow.go`)を、依存の中のフィールドや map まで、パッケージを跨いでたどる。根 D のあと。
- モデルの `calls:` は、あとで別の関数が、レシーバの持つ関数を呼ぶ形を書けない(#196 の「3」。書式の拡張が要る)。

## 大きさ

大(根 D のあと)。

## 偽の edge

足すと過大になる: 同じ map のどの書き込みも候補になるので、キーを区別しないと、ルートの関数がすべて付く(`flutter-route-args` の decoy)。キーを区別すれば足さない。

## uncertaintyのない見逃しにはならない

候補 0 件の値の呼び出しには、通常のパスの `unresolved_callee` が付く。
