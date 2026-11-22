# octocrab: Issue.body などの公開フィールドを source にできない(見逃し)

## 何が見逃されるか(または、何が誤って報告されるか)

issue、コメント、プルリクエストの本文や題(`Issue.body`、`Issue.title`、`Comment.body`、`PullRequest.body`)、コミットのメッセージ、ファイルの内容の base64 の文字列(`Content.content`)を、利用者のコードがフィールドとして直接読んで、`Command` などの sink に渡しても、見逃される。ファイルの内容は `Content::decoded_content` で読めば source になる(セル `decoded-content`)。ADR-108 の基準(その値を GitHub の利用者が書いた)では、これらも source にする値。

## 原因

仕組みの制限。フィールドの source は `kind: field` の行で書くが、サイトの列挙(`internal/sites/enumerate.go` の `fieldKeyOf`)は、`<型>.<フィールド>` のキーを、`.` を含む型名(`pkg.Type`。Go と Dart の形)からしか作らない。Rust の型は `octocrab::models::issues::Issue` のように `::` で書かれ、`.` がないので、キーが空になり、行は一致しない。実際に `kind: field`、`field: octocrab::models::issues::Issue.body` の行を足して zoo を実行したが、セル `issue-body` の結果は変わらなかった(行は戻した)。Rust の組み込みモデルの source は `param_type` と `call_result` だけ。extractor は、フィールドの読み出しを `field_load`(フィールド名だけ)にする。

直す方向: `fieldKeyOf` が Rust の型名(`::` 区切り)を扱うようにする。そのとき、Dart の `dart-github` と同じ、骨組みの中の行(コンストラクタなど)が source の場所に出る問題が、Rust でも起きるかを確認する。

## 再現するセル

- `bench/zoo-rust-pkg/octocrab/cells/src/issue_body.rs` の `vuln` と `decoy` の `@miss(silent)`。
