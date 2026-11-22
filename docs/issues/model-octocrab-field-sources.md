# octocrab: Issue.body などの公開フィールドを source にできない(見逃し)

## 何が見逃されるか(または、何が誤って報告されるか)

issue、コメント、プルリクエストの本文や題(`Issue.body`、`Issue.title`、`Comment.body`、`PullRequest.body`)、コミットのメッセージ、ファイルの内容の base64 の文字列(`Content.content`)を、利用者のコードがフィールドとして直接読んで、`Command` などの sink に渡しても、見逃される。ファイルの内容は `Content::decoded_content` で読めば source になる(セル `decoded-content`)。ADR-108 の基準(その値を GitHub の利用者が書いた)では、これらも source にする値。

## 原因

`rust-octocrab.yaml` に、これらのフィールドを source にする行がない。Rust の組み込みモデルに、フィールドの source の前例がなく(`param_type` と `call_result` だけ)、Dart の `dart-github.yaml` と同じ形の穴(`docs/issues/model-github-field-source-in-dependency.md`)で、`kind: field` の行を足すと zoo の骨組みの中の行が source の場所に出る恐れがある。Rust で行を足せるかは、調べていない。

## 再現するセル

- `bench/zoo-rust-pkg/octocrab/cells/src/issue_body.rs` の `vuln` と `decoy` の `@miss(silent)`。
