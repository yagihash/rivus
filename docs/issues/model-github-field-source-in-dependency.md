# github: GitHubFile.content と、Issue、IssueComment、PullRequest の body を source にできない

優先度: 高め(`Issue.body` などは、GitHub のクライアントでいちばん代表的な入力。サイレントな見逃しのまま長く置かない)。

## 何が見逃されるか(または、何が誤って報告されるか)

リポジトリのファイルの中身(`GitHubFile.content`)や、issue、コメント、プルリクエストの本文(`Issue.body`、`IssueComment.body`、`PullRequest.body`)を、利用者のコードが直接読んで `sinkCmd` などに渡しても、見逃される(モデルに、その行を足していないため)。`GitHubFile.text`(`content` を base64 から戻した文字列)は source にしてあるので、見つかる(セル `dart/github-file-text`)。

## 原因

`kind: field` の source の行を足すと、zoo で、骨組みの中の行(コンストラクタ `GitHubFile({this.content})` と、`text` の本体の読み出し)が source の場所に出て、そこから出る流れが「想定外の流れ」になり、`TestZooDart` が失敗した。行を足さずに、見逃しとして残した。

事実の確認(`rivus analyze -sites`):

- zoo の骨組み(`bench/zoo-dart/cells/packages/github`。プロジェクトの下の path の依存)では、source の場所が、骨組みの `GitHubFile.new`(2 行目)と `GitHubFile.text`(8 行目)に出る。ここでは、骨組みの関数が、依存(`fn.Dep`)でなく、プロジェクトの関数として扱われている。
- 本物の `github` 9.26.0(pub のキャッシュの依存)を使う小さなプロジェクトで、同じ `kind: field` の行(`GitHubFile.content`、`Issue.body`)を試すと、source の場所は、プロジェクトの `main.dart` の中だけで、パッケージの中には出ない(`internal/sites/enumerate.go` の `if fn.Dep { continue }`、ADR-099 のとおり)。

つまり、**本物の依存では起きず、zoo の骨組みがプロジェクトの側として読まれるためだけの問題**。モデルの `kind: field` の行を足してよいが、zoo のテストが失敗する。

## 直す方向の候補

1. zoo の骨組み(`cells/packages/` の path の依存)を、依存(`Dep`)として扱う(テストの側の設定。サイトの列挙が骨組みを飛ばす。ほかのセルへの影響の確認が要る)。
2. 骨組みの中の source の場所から出る流れを、zoo の照合が許す(骨組みの行を除く)。
3. フィールドの source を、プロジェクトの関数の中の読み出しだけに絞る(本物の依存では、すでにそうなっている)。

直ったら、`dart-github.yaml` に、`GitHubFile.content` と、`Issue`、`IssueComment`、`PullRequest` の `body` の `kind: field` の行を足し、下のセルの `@miss` を外す。

## 再現するセル

- `bench/zoo-dart/cells/lib/dart/github_file_content.dart` の `vuln` と `decoy` の `@miss(silent)`。
- `bench/zoo-dart/cells/lib/dart/github_issue_body.dart` の `vuln` と `decoy` の `@miss(silent)`。
