# Dart の github: RepositorySlug を fromJson で作る、または owner と name に代入すると、サービスのメソッドへの流れが見逃される

## 何が見逃されるか

`package:github` の `RepositorySlug`(`/repos/<owner>/<name>/...` の owner と name を持つ型)は、利用者の値が入ると、別のリポジトリを指せる(カテゴリ `resource_name`)。今の `internal/model/builtin/dart-github.yaml` は、スラッグを**作る場所**にだけ sink の行がある(`RepositorySlug.new` の引数 0 と 1、`RepositorySlug.full` の引数 0)。そのため、次の 2 つの作り方は、サービスのメソッドに渡しても見つからない。

1. `RepositorySlug.fromJson({'owner': <利用者の値>, 'name': 'x'})` で作る。`fromJson` に行がない。
2. `RepositorySlug('o', 'x')` で作ったあとに、`slug.owner = <利用者の値>`(または `slug.name = ...`)と代入する。コンストラクタの引数は定数で、代入には行がない。

どちらも、スラッグを `RepositoriesService.getReadme(slug)` などに渡すと、利用者の値がリソースの指定に入る。`make model`(Dart)の実行では、サイレントな見逃しだった。zoo の骨組みでは、同じ流れが、不確かさの記録(trace)つきの見逃し(`unmodeled_library_call`)として出る。

## 原因

sink の位置が、値が**使われる場所**(サービスのメソッド)でなく、値が**作られる場所**(コンストラクタ)にある。作り方は、コンストラクタ、`full`、`fromJson`、フィールドへの代入と複数あり、行は 2 つの作り方だけを覆う。サービスのメソッドの行は、パス、ref、ブランチ、sha の引数だけにあり、スラッグを取るメソッドの引数には、ない(冒頭のコメントのとおり、「スラッグは、作る場所が sink」としたため)。

## 直す方向

スラッグを取るサービスのメソッドを、型で、まとめて採用する(`docs/DESIGN-package-models.md` 6.12。これは、まとめての採用の最初の Dart の例になる)。

- sink の位置は、`RepositorySlug` 型の引数そのもの(引数の中の owner と name の値が、そのまま指定に入る)。
- そのうえで、今の `RepositorySlug.new` と `RepositorySlug.full` の sink の行は、流れの行(引数 → 戻り値)に変える。そうしないと、同じ流れが、作る場所と使う場所の 2 回で報告される。
- 6.12 の形で書けるか、何が足りないかは、同じ節の段落に書いた。

## 再現するセル

- `bench/zoo-dart/cells/lib/dart/github_slug_fromjson.dart`(`F/dart/github-slug-fromjson`): vuln に `@miss(unmodeled_library_call) @ref(model-dart-github-repositoryslug-miss)`。
- `bench/zoo-dart/cells/lib/dart/github_slug_assign.dart`(`F/dart/github-slug-assign`): vuln に `@miss(unmodeled_library_call) @ref(model-dart-github-repositoryslug-miss)`。

直ったら、2 つのセルの `@miss` を外す(同じコミットで)。比べるセルは、`F/dart/github-slug-owner`(`RepositorySlug(v, 'r')` で作る形。今も見つかる)。
