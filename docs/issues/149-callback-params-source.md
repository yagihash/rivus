# 149: ハンドラの引数が source になる書式の残り(callback params): 登録のヘルパーの引数として渡す形、変数が 2 つの関数を持つ形、フィールド・要素が引けない(段階 1 の `callback_param` は入った)

- 状態: **partial**
- 種類: サイレントな見逃し(書式の不足)
- 言語: Dart / Go / Rust
- ROADMAP の節: 2 / 3
- 守るもの: -
- 題: ハンドラの引数が source になる書式の残り(callback params): 変数が 2 つの関数を持つ形、フィールド・要素が引けない(段階 1 の `callback_param` は入った。登録のヘルパーの引数として渡す形は、ヘルパーの呼び出し元をたどって引く)。例: shelf_router の `router.get('/item/<id>', (Request r, String id) => ...)` の `id`
- 補足(状態): **部分的に解決**(段階 1 = `callback_param` が入った。ADR-106。関数リテラル、名前のある関数、それを持つ変数は source になる。登録のヘルパーの引数として渡す形は、ヘルパーの呼び出し元の実引数をたどって引く(3 段まで。zoo の `F/dart/callback-param-helper` は Dart も Go も検出される)。**変数が 2 つの関数を持つ形、フィールド・要素は引けない**: uncertainty(`unresolved_callee`)は呼び出しを持つ関数に付くので、source / sink の関数から見ると `@miss(silent)`。shelf_router のモデルの行は、モデルの PR が足す)
- 補足(種類): サイレントな見逃し(書式の不足。モデルを足しても検出されない行が増える)
- 補足(言語): Dart(shelf_router)。同じ形が Dart の dart_frog、Rust の actix-web、Go の gin の登録の形にも出る
- 補足(確認 2026-11-23): 段階 1 は main に入っている(36514d7、9c5ff70、c75619e。zoo の `F/dart/callback-param-helper` の `@miss(silent) @ref(#149)` が残る)。残り: 変数が 2 つの関数を持つ形、フィールド・要素、Go / Rust の登録の形。直っていないので open
- 設計: [../DESIGN-callback-sources.md](../DESIGN-callback-sources.md)

`router.get('/item/<id>', (Request r, String id) => ...)` の `id` は利用者の入力だが、source にならない。`param_type` は型で決めるので、`String` のような一般の型の引数は指せない。`calls:` の行は、呼び返す関数の引数に source を足せない。「引数として渡された関数の、N 番目以降の引数は source」を書く書式がない。

zoo のセル `F/dart/shelf-router-extra-param`(PR #181 のブランチ。`docs/issues/model-shelf_router-extra-path-params.md`)の `@miss(silent)` が、この課題の再現。書式が入ったら、そのモデルの課題を閉じ、セルの `@miss` を外す(見逃しが減る方向なので、ゲートを緩める変更ではない)。

## 見えるか(uncertainty の場所)

引けなかった登録の呼び出しは、`unresolved_callee`(detail は `callback_param_unresolved:` で始まる)として、**呼び出しのある関数**に付く。source / sink の関数ではない(どの関数が source になるはずだったかは分からない)ので、zoo の `@miss` の分類は `silent` のまま(定義は広げない)。利用者には `summary.callback_param_unresolved`(`count`、`sites[]`)で、finding が 0 件でも見える。`rivus resolve` の `absent_now` で見えるかは、未確認(前後の組の finding が要る。試していない)。

Go でも同じ形(登録のヘルパーの引数として渡す形)を確認した。Go の組み込みのモデルには `callback_param` の行がない(0 件)ので、起きるのは、利用者が `callback_param` の行を自分で書いたときに限られる。
