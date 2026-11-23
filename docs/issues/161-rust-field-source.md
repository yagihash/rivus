# 161: Rust で `kind: field` の source が書けない(型名が `::` 区切りで、キーが空になる)

- 状態: **open**
- 種類: 道具(重さ: 中)
- 言語: Rust
- ROADMAP の節: 1
- 守るもの: zoo `F/pkg/octocrab/issue-body`(#224)
- 題: Rust: `kind: field` の source が書けない(`fieldKeyOf` が `.` を含む型名しか扱わず、`::` 区切りの Rust の型名でキーが空になる)。Issue や PR の本文などが見逃しになる
- 補足(種類): 道具(重さ: 中。サービスのクライアントの代表的な入力が、Rust ではすべて見逃しになる)

## 何が見逃されるか

GitHub のクライアントなど、サービスのクライアントの代表的な入力(Issue や PR の本文、コメント、コミットのメッセージ)は、公開フィールドで読まれる。Go と Dart では、`kind: field` の source で書ける(ADR-108 の基準で source にすべき値)。Rust では書けないので、これらは全部、サイレントな見逃しになる。例: `octocrab` の `Issue.body`。

## 原因

`fieldKeyOf`(`internal/sites/enumerate.go`)は、`<型>.<フィールド>` のキーを、`.` を含む型名(Go と Dart の `pkg.Type`)からしか作らない。Rust の型名は `octocrab::models::issues::Issue` のように `::` 区切りで `.` がないので、キーが空になり、行は一致しない。実際に `kind: field`、`field: octocrab::models::issues::Issue.body` の行を足して zoo を実行したが、セルの結果は変わらなかった(セル `issue-body`、`bench/zoo-rust-pkg/octocrab/`、課題 `model-octocrab-field-sources`)。Rust の組み込みモデルの source は、いまは `param_type` と `call_result` だけ。extractor は、フィールドの読み出しを `field_load`(フィールド名だけ。`extractor/rust/src/lower.rs`)にするが、読み出す値の型は、IR のローカルの型(`Locals[...].Type`)に入っているので、サイトの列挙は型名を持っている。

## 直す方向の候補(規模の見積もり)

- (a) `fieldKeyOf` が `::` の型名も扱う。キーは `crate::path::Type.field`(最後の `::` の後ろの名前と、フィールドの間を `.` でつなぐ)。型名の `<...>`(ジェネリクス)を落とす既存の処理と、`canon.Canon` が Rust の型名をどう直すかを確かめる。小さい変更(数十行と、Rust のセルの確認)。**第一候補**。
- (b) extractor が `field_load` に型名を足す。(a) で型名は取れているので、要らない見込み。
- (c) アクセサの `call_result` で代える。`octocrab` のフィールドは公開フィールドで、アクセサがないので、書けない。

## 実装と、Rust の骨組みの `Dep`

実装: `dcf3fb2`(ブランチ `feat/rust-field-source-key`)。Rust の骨組みは、既定では `Dep` ではない(cargo が、ワークスペースの根の下の path の依存をメンバーにするため)。`[workspace]` の `exclude = ["skel"]` で `Dep` になる。

## 目標

3 言語で、`kind: field` の source が同じ形で確認できる状態。いまの状態: Go は、骨組みが `Dep` として読まれるので、想定外の流れが出ない(確認済み)。Dart は、骨組みがプロジェクトの側として読まれるので、想定外の流れが出る(`model-github-field-source-in-dependency`)。Rust は、(a) のあと、骨組み(path の依存)が `Dep` として読まれるかを確認する。出たら Dart と同じ課題になる。

## 再現するセル

- `bench/zoo-rust-pkg/octocrab/cells/src/issue_body.rs` の `vuln` と `decoy` の `@miss`(#224 のブランチ)。直ったら、`rust-octocrab.yaml` に、`Issue`、`Comment`、`PullRequest` の本文と題、`Content.content` の `kind: field` の行を足し、`@miss` を外す。
