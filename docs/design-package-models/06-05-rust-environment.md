### 6.5 Rust の環境(調査と案。実装はしない)

状況: `make setup`(`scripts/dev-setup.sh`)は、Go と Dart だけを入れる(「Rust は対象外」)。Rust のモデルを作って確認するには、Rust の extractor が動く必要がある。調べたことと、見積もり(**この環境では入れて測っていない**。数字は、ドキュメントと CI の設定からの見積もり)。

1. **Linux でのビルドの手順と時間**: CI の `rust` ジョブと同じ。`rustup toolchain install`(`extractor/rust/rust-toolchain.toml` の `nightly-2026-11-15`、`minimal` + `rustc-dev`、`llvm-tools`、`rust-src`。取得は約 1 GB: `docs/rust-isolation.md`)、`cargo build --locked`(`rustc_private` にリンクする extractor のビルド)。時間は、回線とマシンしだいで、nightly の取得に 1〜数分、ビルドに数分(CI の `timeout-minutes` は 60)と見込む。実測は、段階の最初に取る。
2. **Linux の隔離**: rivus には、ない(`-lang rust` は、サンドボックスがないと何も実行せずに止まる。`-unsafe-host-exec` だけが通る。ADR-093、`docs/rust-isolation.md`。`docs/DESIGN-macos-sandbox.md` は、「あとで bubblewrap などを足せるよう、起動のインターフェースを 1 つにする」としている)。候補: bubblewrap(`bwrap`。名前空間でファイルとネットワークを絞る。取得の段階はネットワークあり、抽出の段階は `--unshare-net`。macOS のプロファイルと同じ 2 段階にできる)、`unshare`(同じ名前空間。ファイルの絞り込みを自分で組む)、gVisor(ランタイム。コンテナの外から使うもので、クラウドのセッションの中では使えない)。**クラウドのコンテナの中では、非特権のユーザー名前空間が使えないことが多く、`bwrap` が動くかは、環境で確かめるまで分からない(未確認)。** 動かないなら、Linux の隔離は、特権のあるホスト(VM)が要る。コンテナそのものを隔離として使うのは、セッションの資格情報(トークン、署名の鍵)が同じコンテナにあるので、勧めない。
3. **`make setup` に含めるか**: **含めない。別のターゲット(`make setup-rust`)にする。** 取得が大きく、Go と Dart のモデルには要らない。
4. **5 分の制約**: クラウドのセットアップは、5 分以内に終わらないとキャッシュされない。nightly の取得(約 1 GB)と extractor のビルドを足すと、超える見込みが高い(未測定)。超えるものを `make setup` に入れると、Go と Dart のキャッシュまで失う。キャッシュの置き場所は、`~/.rustup`、`~/.cargo`、`extractor/rust/target`。`make setup-rust` は、Rust のモデルを取るセッションの最初に走らせる(毎回のコスト)か、別のキャッシュ(別のセットアップ)に分ける。
5. **動かせない場合: 「作るが、Rust のテストは CI に任せる」は成り立つか**: **部分的に成り立つ。** モデルの YAML の検証は、Go のテスト(`internal/model/builtin` のテスト)で手元でも確認できる。Rust の zoo のセルは、第三者のコードを実行しない形(`bench/zoo-rust` は std だけ。依存のスタブは `path:` の自前のコード)で作れるので、CI の `rust` ジョブ(`-unsafe-host-exec`、自分のリポジトリのコードだけ)で確認できる。ただし、手元で試せないので、PR の CI が失敗する確率が高く、修正の往復が CI の時間になる。**差分テストの Rust 版は、第三者のクレートの本体を読む(= `cargo check` で、その `build.rs` と proc macro が動く)ので、取得したコードを実行しない規則と衝突する。** rivus の Rust extractor は、読むときに `build.rs` と proc macro を実行する設計で、隔離(2)がないと、この環境では動かせない。CI のランナーで、取得した第三者のクレートを読む差分テストを走らせるのも、同じ理由で勧めない(隔離のあるジョブに限る)。

**推奨: Rust は、いまは CI に任せる。つまり、定期のセッションでは Rust のパッケージを取らない(Go と Dart だけ)。** Rust のモデルは、Linux の隔離(bubblewrap など。段階を 1 つ足す。中規模。環境で `bwrap` が動くことを先に確かめる)と `make setup-rust` が入ってから、差分テストとあわせて始める。それまでに Rust のモデルが要るときは、人(開発者)が、手元(macOS のサンドボックス)で作る。

