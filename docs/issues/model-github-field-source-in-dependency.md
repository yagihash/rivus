# github: GitHubFile.content と、Issue、IssueComment、PullRequest の body を source にできない

## 何が見逃されるか(または、何が誤って報告されるか)

リポジトリのファイルの中身(`GitHubFile.content`)や、issue、コメント、プルリクエストの本文(`Issue.body`、`IssueComment.body`、`PullRequest.body`)を、利用者のコードが直接読んで `sinkCmd` などに渡しても、見逃される。`GitHubFile.text`(`content` を base64 から戻した文字列)は source にしてあるので、見つかる(セル `dart/github-file-text`)。

## 原因

`kind: field` の source は、利用者のコードの読み出しだけでなく、依存の中の、そのクラスのコンストラクタ(`this.content` のような初期化の形式引数)と、依存の中のそのフィールドの読み出し(`text` の本体)にも、source の場所として出る。zoo で、`kind: field` の行を足すと、本物の経路とは別に、骨組みの中の行から出る流れが「想定外の流れ」として報告され、`TestZooDart` が失敗した。本物のパッケージでも、同じように、依存の中に 2 つめの source が出る。行を足さずに、見逃しとして残した。

## 再現するセル

- `bench/zoo-dart/cells/lib/dart/github_file_content.dart` の `vuln` と `decoy` の `@miss(silent)`。
- `bench/zoo-dart/cells/lib/dart/github_issue_body.dart` の `vuln` と `decoy` の `@miss(silent)`。
