# Dart の github: RepositorySlug を fromJson で作る、または owner と name に代入すると、サービスのメソッドへの流れが見逃される

## 何が見逃されるか

`package:github` の `RepositorySlug`(`/repos/<owner>/<name>/...` の owner と name を持つ型)は、利用者の値が入ると、別のリポジトリを指せる(カテゴリ `resource_name`)。今の `internal/model/builtin/dart-github.yaml` は、スラッグを**作る場所**にだけ sink の行がある(`RepositorySlug.new` の引数 0 と 1、`RepositorySlug.full` の引数 0)。そのため、次の 2 つの作り方は、サービスのメソッドに渡しても見つからない。

1. `RepositorySlug.fromJson({'owner': <利用者の値>, 'name': 'x'})` で作る。`fromJson` に行がない。
2. `RepositorySlug('o', 'x')` で作ったあとに、`slug.owner = <利用者の値>`(または `slug.name = ...`)と代入する。コンストラクタの引数は定数で、代入には行がない。

どちらも、スラッグを `RepositoriesService.getReadme(slug)` などに渡すと、利用者の値がリソースの指定に入る。`make model`(Dart)の実行では、サイレントな見逃しだった。zoo の骨組みでは、同じ流れが、不確かさの記録(trace)つきの見逃し(`unmodeled_library_call`)として出る。

## 原因

sink の位置が、値が**使われる場所**(サービスのメソッド)でなく、値が**作られる場所**(コンストラクタ)にある。作り方は、コンストラクタ、`full`、`fromJson`、フィールドへの代入と複数あり、行は 2 つの作り方だけを覆う。サービスのメソッドの行は、パス、ref、ブランチ、sha の引数だけにあり、スラッグを取るメソッドの引数には、ない(冒頭のコメントのとおり、「スラッグは、作る場所が sink」としたため)。

## 直す方向

スラッグを取るサービスのメソッドを、型で一括採用する(`docs/DESIGN-package-models.md` 6.12。これは、一括採用の最初の Dart の例になる)。

- sink の位置は、`RepositorySlug` 型の引数そのもの(引数の中の owner と name の値が、そのまま指定に入る)。
- そのうえで、今の `RepositorySlug.new` と `RepositorySlug.full` の sink の行は、流れの行(引数 → 戻り値)に変える。そうしないと、同じ流れが、作る場所と使う場所の 2 回で報告される。
- 6.12 の形で書けるか、何が足りないかは、同じ節の段落に書いた。

## 再現するセル

- `bench/zoo-dart/cells/lib/dart/github_slug_fromjson.dart`(`F/dart/github-slug-fromjson`): vuln に `@miss(unmodeled_library_call) @ref(model-dart-github-repositoryslug-miss)`。
- `bench/zoo-dart/cells/lib/dart/github_slug_assign.dart`(`F/dart/github-slug-assign`): vuln に `@miss(unmodeled_library_call) @ref(model-dart-github-repositoryslug-miss)`。

直ったら、2 つのセルの `@miss` を外す(同じコミットで)。比べるセルは、`F/dart/github-slug-owner`(`RepositorySlug(v, 'r')` で作る形。今も見つかる)。

## 調べた結果(2026-11-23)
### trace の有無の違い

`make model`(Dart)の実行では、サイレントな見逃し(trace なし)だった。zoo のセルが `@miss(unmodeled_library_call)` なのは、流れが途中で失われたマークではなく、同じ関数にある別の呼び出し(骨組みの `GitHub()`。本体を読めず、モデルもない)の不確かさが、trace の数え方(`missTraces`: source と sink の関数にある不確かさを数える)に入るため。実物のパッケージでは `GitHub()` は本体があり、その不確かさがないので、同じ流れが無マークになる。

### 流れは失われていない

エンジンは、この流れを失っていない。`RepositorySlug.fromJson`(生成された `repos.g.dart` の `_$RepositorySlugFromJson`)の本体も、`slug.owner = x` の代入(フィールドへの書き込み)も追われ、スラッグを取るメソッドの引数 0 に sink の行を足すと、実物の `github` 9.26.0 に対して、3 つの作り方(コンストラクタ、`fromJson`、代入)のどれも、`getReadme` で見つかる。見逃しの原因は 2 つだけ:

1. スラッグを取るメソッドの引数に、sink の行がない(上の「原因」のとおり)。
2. `getReadme` の本体の中の `GitHub.getJSON`(path の sink)は、依存の中なので、サイトとして探されない(ADR-099、課題 #144)。値は本体の中の sink に届くが、出力は `summary.dependency_sites.searched: false` と言うだけで、その流れのマークはない。`dart-github.yaml` の冒頭にある「本体を読むので `GitHub.request` に届く」は、プロジェクトのコードが `request` などを直接呼ぶときだけの話。

したがって、新しいエンジンの課題ではない(#144 の形)。修正の方向は上の「直す方向」のままで、スラッグを取るメソッドの引数に sink を置けば、3 つの作り方が全部見つかる。

### セルの注意

2 つのセルの `@miss(unmodeled_library_call)` の trace は、骨組みの `GitHub()`(`bench/zoo-dart/cells/packages/github/lib/src/common/github.dart`)の本体を読めないことによる。この骨組みを変えると trace が消え、`@miss(silent)` に直す必要が出る。直った(sink の行が入った)ときは、どちらにしても `@miss` を外す。
