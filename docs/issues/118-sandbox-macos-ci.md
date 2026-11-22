# #118 macOS の CI のワークフロー(ツールチェインを入れて抽出まで)と、macOS の TestDogfood

- 状態: **open**(後回しのハードニング。ADR-093)
- 種類: CI
- 言語: Rust
- ROADMAP の節: 0(Homebrew のインストールの作業の続き)
- 守るもの: まだない(直すときに試験を足す)

## 今の状態

今の `isolation.yml` は、`internal/sandbox` の試験だけ(ツールチェイン不要)。Rust の抽出と取得は、開発のマシンで回す。`TestDogfood` の darwin の見え方は、静的に読むので Linux の CI でも読めるが、macOS で走らせる試験(`RIVUS_SBX_*`)は CI にない。

## 終わりの条件

ツールチェインを入れ、抽出まで、`RIVUS_SBX_RED=1` / `RIVUS_SBX_NET=1` で走らせる macOS のワークフロー。隔離に関わるパスが変わったとき、週 1、手動、タグ。

## 経緯

- 2026-11-22: macOS のサンドボックスへの切り替え(ADR-093)で、ユーザーが「切り替えを先に、ハードニングは後」と決めた分として、課題に上げた。
