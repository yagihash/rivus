# 149: ハンドラの引数が source になる書式がない(callback params)

- 状態: **部分的に解決**(段階 1 = `callback_param` が入った。ADR-106。関数リテラル、名前のある関数、それを持つ変数は source になる。**登録のヘルパーの引数として渡す形、変数が 2 つの関数を持つ形、フィールド・要素は引けない**: 注記(`unresolved_callee`)は呼び出しを持つ関数に付くので、source / sink の関数から見ると `@miss(silent)`。zoo の `F/dart/callback-param-helper`。shelf_router のモデルの行は、モデルの PR が足す)
- 種類: サイレントな見逃し(書式の不足。モデルを足しても検出されない行が増える)
- 言語: Dart(shelf_router)。同じ形が Dart の dart_frog、Rust の actix-web、Go の gin の登録の形にも出る
- 設計: [../DESIGN-callback-sources.md](../DESIGN-callback-sources.md)

`router.get('/item/<id>', (Request r, String id) => ...)` の `id` は利用者の入力だが、source にならない。`param_type` は型で決めるので、`String` のような一般の型の引数は指せない。`calls:` の行は、呼び返す関数の引数に source を足せない。「引数として渡された関数の、N 番目以降の引数は source」を書く書式がない。

zoo のセル `F/dart/shelf-router-extra-param`(PR #181 のブランチ。`docs/issues/model-shelf_router-extra-path-params.md`)の `@miss(silent)` が、この課題の再現。書式が入ったら、そのモデルの課題を閉じ、セルの `@miss` を外す(見逃しが減る方向なので、ゲートを緩める変更ではない)。
