# docs の目次

読者ごとに、入口の文書を並べる。リポジトリ全体の入口は [../README.md](../README.md)。

- 状態は、2026-11-22、9c26a39 時点の確認。
- 1 つの話題に、正本の文書は 1 つ。ほかの文書は、要約とリンクだけにする。
- 「今どうなっているか」は ARCHITECTURE などの文書、「なぜそうなったか」は ADR。ADR は書き換えない。

## 読者ごとの入口

| 読者 | 知りたいこと | 読む文書 |
|---|---|---|
| 使い手 | 何が出るか、何を報告しないか、使い方、出力の読み方 | [../README.md](../README.md)、[rust-isolation.md](rust-isolation.md)(信頼できない Rust)、[GLOSSARY.md](GLOSSARY.md) |
| ルールを書く人 | ルールの書式、何が書けるか、確認方法 | [MODELING.md](MODELING.md)、[../examples/rules/README.md](../examples/rules/README.md)、[GCP-MODELS.md](GCP-MODELS.md) |
| rivus を使う AI エージェント | どのコマンドを、どのフラグで実行し、出力のどこを読むか。`rivus index` と `rivus graph`、答えの読み方 | [AGENT-GUIDE.md](AGENT-GUIDE.md)、[GRAPH.md](GRAPH.md)、[../README.md](../README.md) |
| 開発者 | 設計思想、今の構成、経緯、テストと測定 | [PRINCIPLES.md](PRINCIPLES.md)、[ROADMAP.md](ROADMAP.md)、[ISSUES.md](ISSUES.md)(課題の扱い。一覧は `make docs-gen`)、[ARCHITECTURE.md](ARCHITECTURE.md)、[adr/](adr/README.md)(ADR。一覧は `make docs-gen`)、`DESIGN-*.md`、[../bench/README.md](../bench/README.md) |
| レビューする人 | 不変条件、失敗の重さ、受け入れ済みの制約 | [PRINCIPLES.md](PRINCIPLES.md)、[../bench/MISSES.md](../bench/MISSES.md) |

## 文書の一覧

### 設計思想と、今の姿

| 文書 | 内容 |
|---|---|
| [PRINCIPLES.md](PRINCIPLES.md) | 設計思想。原則と、今のコードがどこまで届いているか |
| [ARCHITECTURE.md](ARCHITECTURE.md) | 今のアーキテクチャ(IR、構成、データ構造、解析の仕組み) |
| [GLOSSARY.md](GLOSSARY.md) | 用語集 |
| [ROADMAP.md](ROADMAP.md) | 優先順位と、次の作業 |

### 判断の経緯

| 文書 | 内容 |
|---|---|
| [adr/](adr/README.md) | 設計判断の記録(ADR)。1 件 1 ファイル。決めた時点の記録で、書き換えない。一覧は `make docs-gen WHAT=adr` |
| [DECISIONS.md](DECISIONS.md)、[ADR-INDEX.md](ADR-INDEX.md) | ADR の案内(書き方、状態の意味、注記)。ADR を足すときは触らない |

### 使い方と、書き方

| 文書 | 内容 |
|---|---|
| [AGENT-GUIDE.md](AGENT-GUIDE.md) | AI エージェント向けの利用ガイド(コマンド、フラグ、出力の読み方、ユースケース別の手順) |
| [GRAPH.md](GRAPH.md) | `rivus index` と `rivus graph` |
| [MODELING.md](MODELING.md) | ルールとモデルの書式(英語) |
| [GCP-MODELS.md](GCP-MODELS.md) | Google Cloud のモデルの、製品ごとの表 |
| [rust-isolation.md](rust-isolation.md) | 信頼しない Rust を隔離して読む |

### 個別の設計

冒頭に「状態」(案 / 一部実装 / 実装済み)がある。実装済みの部分の今の姿は ARCHITECTURE にあり、これらは経緯と、残りの計画を含む。

| 文書 | 状態 | 主題 |
|---|---|---|
| [DESIGN-graph.md](DESIGN-graph.md) | 実装済み | `rivus graph` と保存するファイル。使い方の正本は GRAPH.md |
| [DESIGN-zoo.md](DESIGN-zoo.md) | 一部実装 | zoo と wild のベンチ。使い方の正本は `bench/zoo/README.md` |
| [DESIGN-callouts.md](DESIGN-callouts.md) | 一部実装 | 関数値を通る流れ(関数値の正本) |
| [DESIGN-callback-sources.md](DESIGN-callback-sources.md) | 案 | 引数として渡された関数の、N 番目以降の引数を source にする書式(shelf_router のハンドラの追加の引数から。#149) |
| [DESIGN-shared-state.md](DESIGN-shared-state.md) | 一部実装 | グローバルと共有される場所(グローバルの正本) |
| [DESIGN-capture.md](DESIGN-capture.md) | 一部実装 | クロージャのキャプチャの値 |
| [DESIGN-package-models.md](DESIGN-package-models.md) | 案 | パッケージ単位の「本体を読まない」宣言(extractor の段階から)と、モデルを書く標準の流れ(生成、差分テスト、宣言)、モデル作成のセッションを並べる準備。本文は節ごとのファイル(`design-package-models/`)。設計メモは節のファイルに足し、新しい節は新しいファイルと一覧に 1 行 |
| [MODEL-RUNBOOK.md](MODEL-RUNBOOK.md) | 手順書 | 1 パッケージ分の組み込みモデルを作る手順(モデル作成のセッション向け) |
| [MODEL-QUEUE.md](MODEL-QUEUE.md) | 一覧(決定) | 組み込みモデルを作る順番(3 言語をまたぐ 1 本)と、公開の指標、カバー済みの表 |
| [DESIGN-field-sinks.md](DESIGN-field-sinks.md) | 一部実装 | 構造体のフィールドを sink にする(`parts`) |
| [DESIGN-rust-extractor-panic.md](DESIGN-rust-extractor-panic.md) | 実装済み | Rust の extractor が、1 つの関数のパニック(async closure の未実装など)で止まらず、その関数だけ飛ばして続ける。飛ばしたことの見せ方(#160) |
| [DESIGN-async-dispatch.md](DESIGN-async-dispatch.md) | 実装済み | Rust の `async` の中の `Fn::call` |
| [DESIGN-generics-dispatch.md](DESIGN-generics-dispatch.md) | 一部実装 | Rust のジェネリクスと `Box<dyn Fn>` |
| [DESIGN-memory.md](DESIGN-memory.md) | 一部実装 | メモリ(英語) |
| [DESIGN-fetch-network.md](DESIGN-fetch-network.md) | 案 | 取得のコンテナのネットワークを絞る |
| [DESIGN-observed-calls.md](DESIGN-observed-calls.md) | 案(最小の試行あり) | 実行して観測した呼び出しと、グラフの照合(Dart から) |
| [DART-MODEL-CANDIDATES.md](DART-MODEL-CANDIDATES.md) | 案(決めてもらう資料) | Dart の組み込みモデルにする pub パッケージの候補(公開の指標で選ぶ) |

### 歴史的な文書

[history/](history/) にある。今の設計とは異なる所がある。中身は書き換えていない。

| 文書 | 内容 |
|---|---|
| [history/BRIEF.md](history/BRIEF.md) | 最初の計画(ブリーフ) |
| [history/QUESTIONS.md](history/QUESTIONS.md) | ブリーフへの質問と指摘 |
| [history/ARCHITECTURE-plans.md](history/ARCHITECTURE-plans.md) | 以前の ARCHITECTURE.md から移した、Phase の計画、実施記録、実測(9〜11 節など) |

## ほかの場所

- テストと測定: [../bench/README.md](../bench/README.md)
- extractor: [../extractor/dart/README.md](../extractor/dart/README.md)、[../extractor/rust/README.md](../extractor/rust/README.md)
- ルールの見本: [../examples/rules/README.md](../examples/rules/README.md)
- AI セッションへの指示(相談役、開発者): [../.claude/prompts/](../.claude/prompts/)。相談役のレビューは `.claude/reviews/` に置くが、git は追跡しない(公開リポジトリなので、直していない脆弱性の詳細を入れない)
