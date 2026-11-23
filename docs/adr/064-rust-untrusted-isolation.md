# ADR-064: 信頼しない対象の Rust を、隔離して抽出する（採用: ユーザーが承認した「案 1: ネットワークなしのコンテナで、抽出器だけを動かす」。R-21、R-24、R-25）

- 状態: 一部覆った(→ADR-093)
- 関連: rust-isolation, DESIGN-fetch-network
- 状態の詳細: 同 ADR 内の訂正で「取得の段階は安全」を撤回; 訂正・追記あり(書き方を保つ)(ユーザー発見)。**コンテナの部分は ADR-093 が覆した**(macOS のサンドボックス)
- 要旨: Docker、2 段階、ネットワークなし抽出

**背景:** Rust の抽出器は、対象に `cargo check` を実行する（ADR-042）。対象の `build.rs`、proc macro、`[build-dependencies]` が実行される。rivus は、採用前のライブラリや怪しいリポジトリの確認に使うので、それは、利用者の権限（鍵、トークン、ネットワーク）で、他人のコードが動くことになる。README の「Go はコードを実行しない」も事実ではなかった（`go list` が cgo と依存の取得をする。R-25）。

**決めたこと:**
1. **既定は、Docker のコンテナの中で抽出する**（`rivus analyze -lang rust`）。Docker がなければ、**何も実行せずに**、明確なエラーで止まる（Docker を使う理由と、`-unsafe-host-exec` の説明を書く）。`-unsafe-host-exec`（または `RIVUS_UNSAFE_HOST_EXEC=1`）で、従来どおりホストで直接動かす。CI のジョブと、このリポジトリの自前のコード（`bench/zoo-rust`、`bench/rust-curated`、`extractor/rust/test`）は、信頼できるので、extractor のバイナリを直接呼ぶ（`RIVUS_RUST`）。
2. **2 段階:** (a) 取得（ネットワークあり。`cargo fetch --locked`。ビルドしないので、対象のコードは動かない）。依存のソースは、`Cargo.lock` と `.cargo/config` の内容の hash を名前にした volume に入る（プロジェクトごと。ほかのプロジェクトの読む依存と混ざらない）。(b) 抽出（`--network none`）。対象は、**ホストと同じパスに、読み取り専用で**マウント（IR のパスがホスト版と同じになる）。依存の volume は読み取り専用。書けるのは、使い捨ての volume（`/work`: `target/` と IR の断片）と tmpfs の `/tmp` だけ。extractor が `cargo check --offline --locked` を実行する（extractor に `--locked`、`--offline` を足した。R-25 の「`--locked` / `--offline` がない」も、ホストの実行で使える）。**IR は、コンテナの標準出力でだけ返る**（出力用のディレクトリをホストに置かない。ホストが、コンテナの書いたファイルを読むことがない。標準出力の大きさは `-container-max-output`、既定 4 GiB で切る）。
3. **コンテナの制限:** 非 root（10001）、`--cap-drop ALL`、`no-new-privileges`、`--read-only`、`--pids-limit`、`--memory`（swap も同じ）、`--cpus`、時間（各段階 `-container-timeout`、超えたら `docker rm -f`）、ホームも `docker.sock` もマウントしない、ホストの環境変数は渡さない。必要な設定は、`-container-env NAME=VALUE`（値をコマンドラインに書く）と `-container-ro-mount SRC:DST`（読み取り専用）で、**明示して**足す。
4. **イメージ:** `docker/rust-extractor/Dockerfile`。ベースは `debian:bookworm-slim` をダイジェストで固定、nightly は日付で固定（`rust-toolchain.toml` と一致することをビルドで確かめる）。extractor は、同じ Dockerfile の中で、このリポジトリのソースから `--locked` でビルドする。C のコンパイラ、make、cmake、pkg-config、perl（依存のビルドスクリプトが必要なもの）を入れる。それ以上は入れない。対象の `rust-toolchain` が別のツールチェインを求めても、取得しない（`RUSTUP_TOOLCHAIN` で固定、警告）。
5. **言語に依らない形:** `cmd/rivus/isolate.go` の、コンテナの制限（`limits`）と起動・時間切れ・出力の上限（`runContainer`、`limitWriter`）は、Rust を知らない。Rust 固有なのは、取得のコマンド（`fetchArgs`）と、抽出の呼び出し（`extractArgs`）だけ。Go（cgo、依存の取得。R-25）と Dart（`pub get`、`build_runner`）を、後で同じ作りに載せる（イメージと取得の段階を言語ごとに作る）。今回は、Rust だけ。
6. **README の表を事実に直した:** Go は `go list` が cgo の前処理と依存の取得をする（対象のソースは実行しない）、Dart は前提の手順が実行する、Rust は `build.rs` と proc macro を実行して、既定はコンテナ。

**検証（`TestIsolationHostileBuild`、`TestIsolationResourceLimits`。Docker とイメージが必要。CI では `RIVUS_REQUIRE_DOCKER=1`）:** 悪意のある `build.rs` と proc macro を持つ自作の crate（`extractor/rust/test/hostile/`）が、ホストのファイル、プロジェクトの隣のファイル、ネットワーク（IP、名前の解決）、`/` `/etc` 対象 依存への書き込み、トークンの環境変数、`docker.sock`、root と capability を試して、**すべて失敗する**（対照: ネットワークを開けると、ネットワークの 4 つが `ESCAPED` になる = 検知が働く）。見えるもの: マウントは、対象（ro）、`/cargo-home`（ro）、`/work`、`/tmp` と、システムのものだけ。環境変数の名前は、cargo、rustup、イメージのものだけ。秘密の置き場の名前のファイルは、読めるファイルの中にない（読めるのは約 2 万ファイル、イメージのツールチェイン）。IR の断片のディレクトリに置いた、リンク、20 MB のファイル、知らない名前は、コンテナの中で読まれるだけで、抽出は進み、`/etc/passwd` の中身は IR に出ない。止まらない `build.rs`、フォークの無限ループ（`--pids-limit 64`）、メモリを使い切る `build.rs`（384 MB で SIGKILL）は、コンテナの中で止まり、コンテナは残らない。この crate は、ホストでは動かない（`.rivus-hostile` を rivus が拒否、`build.rs` はイメージの外では panic、`RIVUS_UNSAFE_HOST_EXEC=1` ではテストが skip）。

**parseable（`5c20b8e4`、`-no-deps`）:** コンテナで 159 finding、ホスト版の IR からの 159 と、同じ（sink、規則の組が一致、`stats` は時間以外が同一）。取得 + 抽出 + 解析の全体で 92〜138 秒（ホスト版は抽出 70 秒前後 + 解析数秒）。コンテナのメモリは、約 3.7 GiB まで（上限 6 GiB。Docker Desktop の VM は 約 8 GB）。parseable の `build.rs` は、UI をネットワークから取るので、**隔離すると失敗する**（これが隔離の効果）。ホスト版と同じく、`-container-env LOCAL_ASSETS_PATH=/assets -container-ro-mount <空のディレクトリ>:/assets` で、空の assets を渡す。

**限界（`docs/rust-isolation.md`）:** コンテナは VM ではない（カーネルは共有。runc とカーネルの脆弱性は防げない）。macOS の Docker Desktop は、境界が VM とファイル共有になる。**対象のビルドは、コンテナの中で、IR を書き換えられる**（隔離が守るのはマシンと秘密で、結果の正しさではない）。取得は、対象の `.cargo/config.toml` が指す先に出る（コンテナの中。ホームも認証情報もない）。依存が `/work` を超える大きさのビルドは失敗する。`Cargo.lock` が必要。

**訂正(安全上の抜け。ユーザーが発見):** 上の「取得(ビルドしないので、対象のコードは動かない)」は、**事実ではなかった**。取得の段階も、対象の `.cargo/config.toml` を cargo が読む。`credential-provider` などに同梱のスクリプトを指し、`Cargo.lock` にそのレジストリの依存があって、レジストリが認証を要求すると、cargo はそのスクリプトを、**ビルドの前、ネットワークのあるコンテナで**実行する。**修正(済み)**: 取得の段階では、対象の cargo の設定を読ませない(`.cargo` を空の tmpfs で覆う。ボリュームの中の設定は取得の前に捨てる。設定が必要なプロジェクトは、理由を示して止める。vendor 済みは取得を飛ばす)。テストは `TestIsolationFetchRunsNoCodeOfTheProject`(修正前は失敗、修正後は成功)。**残り**: `Cargo.lock` の取得元(内側のホストなどへの要求)の検査と、取得のネットワークの絞り込み(`docs/DESIGN-fetch-network.md`)。

**R-21（残る。課題として記録）:** 対象の `.cargo/config.toml` の `[env]` で `RIVUS_IR_DIR` を空にすると、IR が空になり終了コード 0 になる。隔離では直らない（コンテナの中でも、同じことが起きる。安全の問題ではなく、結果の正しさの問題）。隔離のモードでも、「コンパイルした crate と IR の断片の照合（0 件なら失敗、欠けたら load_error）」が別に必要。今回は直さない（`extractor/rust` の別の変更）。

**探索:** 信頼しない対象の探索は、3 言語とも、隔離のモードで回す（Go と Dart は、5 のとおり、後で同じ作りに載せる）。

