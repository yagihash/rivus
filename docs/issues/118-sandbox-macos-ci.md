# #118 macOS の CI のワークフロー(ツールチェインを入れて抽出まで)と、macOS の TestDogfood

- 状態: **partial**(後回しのハードニング。ADR-093。抽出までの確認は `sandbox-e2e` ジョブで入れた。macOS の `TestDogfood` は、まだ)
- 種類: CI
- 言語: Rust
- ROADMAP の節: 0
- 守るもの: -
- 補足(ROADMAP の節): 0(Homebrew のインストールの作業の続き)
- 補足(守るもの): まだない(直すときにテストを足す)

## 今の状態

今の `ci.yml` の `sandbox` ジョブは、`internal/sandbox` のテストだけ(ツールチェイン不要)。Rust の抽出と取得は、開発のマシンで回す。`TestDogfood` の darwin の見え方は、静的に読むので Linux の CI でも読めるが、macOS で実行するテスト(`RIVUS_SBX_*`)は CI にない。

## 終わりの条件

ツールチェインを入れ、抽出まで、`RIVUS_SBX_RED=1` / `RIVUS_SBX_NET=1` で実行する macOS のワークフロー。隔離に関わるパスが変わったとき、週 1、手動、タグ。

## 経緯

- 2026-11-25: `ci.yml` の `sandbox-e2e` ジョブ(`scripts/sandbox-e2e.sh`。macOS と Linux)で、固定の nightly と extractor を用意し、悪意のある crate を、サンドボックスの下で、取得から抽出まで読むようにした(Linux の隔離の追加: ADR-128 と一緒に)。`RIVUS_SBX_NET=1` の取得のテストと、macOS の `TestDogfood` は、まだ。

- 2026-11-22: macOS のサンドボックスへの切り替え(ADR-093)で、ユーザーが「切り替えを先に、ハードニングは後」と決めた分として、課題に上げた。
