# shelf_router: ハンドラの 2 つめ以降の引数(経路で取った文字列)が source にならない

## 何が見逃されるか(または、何が誤って報告されるか)

`Router.get('/item/<id>', (Request request, String id) => ...)` のように、ハンドラが `Request` のあとに、経路の `<id>` で取った文字列を `String` の引数で受けるとき、その `id` は利用者の入力だが、source として扱われない。`id` を `sinkCmd` などに渡しても、見逃される。同じ値を `request.params['id']` で読む形は、`Request` が source なので見つかる(セル `dart/shelf-router-params`)。

## 原因

`Request` は `param_type` の source(`dart-packages.yaml`)で、ハンドラの引数の型で決まる。`String` の引数には型で決められる source がなく、`calls:` の行は、呼び返す関数の引数に source を足せない(`with:` は `recv` と `argN` だけ)。`Router.get` の `handler` の 2 つめ以降の引数を source にする行が、いまの形式にない。

## 再現するセル

`bench/zoo-dart/cells/lib/dart/shelf_router_extra_param.dart` の `vuln` と `decoy` の `@miss(silent)`。
