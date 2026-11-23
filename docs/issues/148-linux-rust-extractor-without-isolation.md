# 148: Linux(CI、クラウドのモデル作成)での Rust の extractor の扱い

- 状態: **open**(案。DESIGN-macos-sandbox.md の 8 節、DESIGN-package-models.md の 6.5 節)
- 種類: 道具(重さ: 中)
- 言語: Rust
- ROADMAP の節: 0 / 3
- 守るもの: -
- 題: Linux(CI、クラウドのモデル作成)での Rust の extractor の扱い(隔離なしでは、取得したクレートを実行しない)

## 今の状態

Linux には隔離の手段がなく、`-lang rust` は何も実行せずに止まる(`-unsafe-host-exec` だけが通る)。Rust の extractor は、読むときに `build.rs` と proc macro を実行する。CI の `rust` ジョブは、このリポジトリのコードだけを `-unsafe-host-exec` で読む。クラウドのモデル作成では、隔離なしに、取得した第三者のクレートを読まない(線: **隔離なしでは、取得したクレートのビルドスクリプトとマクロを実行しない**。モデルは書き、extractor を使う確認は CI に任せる)。

## 終わりの条件

Linux の隔離の手段を決める(bubblewrap が最有力。クラウドのコンテナで非特権のユーザー名前空間が使えるかは未確認)。使えるなら、`internal/sandbox` に Linux の実装を足し、2 段階(取得はネットワークあり、抽出は `--unshare-net`)にする。使えないなら、Rust のモデルの確認は CI(と、人の手元)に限る、と文書に残す。
