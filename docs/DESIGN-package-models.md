# パッケージ単位の「本体を読まない」宣言と、モデルを書く標準の流れ(設計案)

状態: 案(2026-11-23。相談役の判断を反映済み)。段階 2(解析側。ADR-100)と段階 4(extractor に宣言を渡す。Go、Dart、Rust。ADR-105。7a 節)は実装済み。以下の 1〜6 節は、設計時の調査と試作の記録で、書き換えていない。測定は、Go の実際のプロジェクトでの利用から出た課題と、公開のベンチ(`bench/dart-gcp`、`bench/dart-grpc`)、AWS SDK for Go v2 の `service/s3` を数個呼ぶ小さい合成(リポジトリの外に作った。取得したコードは実行していない)で行った。

目的(ユーザーの方針): モデルの拡充が、解析だけでなく、読み込み(extractor)の時間とメモリにも効くようにする。`-no-deps` なしの実行が実用的に動くことが目標。前に止めてあった `unread_packages` の案([DESIGN-deps-cost.md](DESIGN-deps-cost.md) 7 節)を、この目的に合わせて作り直す。条件の土台は ADR-092(依存を流れのモデルで切る)と ADR-098(実装の多いインターフェースのメソッド)。


このドキュメントは、節ごとのファイルに分けてある(別々の PR が同じファイルを触って conflict しないようにするため)。節の番号は、他の文書、ADR、コードのコメントが引用しているので、変えない(「DESIGN-package-models 6.12」はこの一覧の 6.12 の行のファイル)。

**書き方**: このパッケージ単位のモデルの設計メモは、該当の節のファイルに足す。新しい節は、新しいファイル(`design-package-models/` の下。ファイル名は `節の番号-短い英語の名前.md`)と、下の一覧に 1 行を足す。

## 節の一覧

| 節 | ファイル |
|---|---|
| 1. 今の動作(コードの調査) | [design-package-models/01-current-behavior.md](design-package-models/01-current-behavior.md) |
| 2. 案 | [design-package-models/02-proposal.md](design-package-models/02-proposal.md) |
| 3. 測定(3.1 Go、3.2 Dart、3.3 見積もり) | [design-package-models/03-measurements.md](design-package-models/03-measurements.md) |
| 4. 「モデルを書く」標準の流れ(4 ステップ)と、型からの生成(4.1〜4.4) | [design-package-models/04-standard-flow.md](design-package-models/04-standard-flow.md) |
| 5. リスク | [design-package-models/05-risks.md](design-package-models/05-risks.md) |
| 6. モデル作成のセッションを並べて走らせる準備(概要) | [design-package-models/06-00-session-prep-overview.md](design-package-models/06-00-session-prep-overview.md) |
| 6.1 conflict しない置き方 | [design-package-models/06-01-no-conflict-layout.md](design-package-models/06-01-no-conflict-layout.md) |
| 6.2 zoo の `marks.lock` のゲート | [design-package-models/06-02-zoo-marks-lock-gate.md](design-package-models/06-02-zoo-marks-lock-gate.md) |
| 6.3 モデルの PR のゲート(CI) | [design-package-models/06-03-model-pr-gate.md](design-package-models/06-03-model-pr-gate.md) |
| 6.4 1 パッケージ分の作業の手順書 | [design-package-models/06-04-per-package-runbook.md](design-package-models/06-04-per-package-runbook.md) |
| 6.5 Rust の環境 | [design-package-models/06-05-rust-environment.md](design-package-models/06-05-rust-environment.md) |
| 6.6 最初の 3 本(Dart、Go、Rust)の確認の基準 | [design-package-models/06-06-first-three-criteria.md](design-package-models/06-06-first-three-criteria.md) |
| 6.7 モデルの鮮度(確認したバージョン、定期的な照合、実行のときのずれ) | [design-package-models/06-07-model-freshness.md](design-package-models/06-07-model-freshness.md) |
| 6.8 zoo のセルの依存(骨組みか、本物のパッケージか) | [design-package-models/06-08-zoo-cell-dependencies.md](design-package-models/06-08-zoo-cell-dependencies.md) |
| 6.9 module の path に major を持つ Go のパッケージ(`/v*`) | [design-package-models/06-09-go-module-major.md](design-package-models/06-09-go-module-major.md) |
| 6.10 確認していないバージョンへの適用の uncertainty | [design-package-models/06-10-unverified-versions-note.md](design-package-models/06-10-unverified-versions-note.md) |
| 6.11 モデル作成の手順のスクリプト化 | [design-package-models/06-11-model-authoring-script.md](design-package-models/06-11-model-authoring-script.md) |
| 6.12 一括採用(規則の単位の決定から、行を生成する) | [design-package-models/06-12-bulk-adoption.md](design-package-models/06-12-bulk-adoption.md) |
| 7. 実装の段階と規模 | [design-package-models/07-stages-and-scale.md](design-package-models/07-stages-and-scale.md) |
| 7a. 段階 4(extractor に宣言を渡す)の状態と測定(ADR-105) | [design-package-models/07a-stage4-extractor-declaration.md](design-package-models/07a-stage4-extractor-declaration.md) |
| 8. 決めたことと、残る判断 | [design-package-models/08-decisions.md](design-package-models/08-decisions.md) |
| 9. 再現 | [design-package-models/09-reproduction.md](design-package-models/09-reproduction.md) |
