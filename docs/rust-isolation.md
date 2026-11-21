# 信頼しない Rust を、隔離して読む

`rivus analyze -lang rust` は、Rust のプロジェクトを**ビルドします**（`cargo check`）。ビルドは、対象の `build.rs`、proc macro、`[build-dependencies]` を実行します。採用前のライブラリや、怪しいリポジトリを読むときは、それが、あなたのマシンで、あなたの権限（ホームの鍵、トークン、ネットワーク）で動くことになります。そこで、**既定では、Docker のコンテナの中で、隔離して動かします**（ADR-064）。

## 要件

- Docker（Docker Desktop、または Linux の Docker Engine）。`docker info` が通ること。Docker Desktop は、VM のメモリが、`-container-memory`（既定 6g）以上であること。
- イメージ `rivus-rust-extractor:nightly-2026-11-15`。**このリポジトリ（信頼できるソース）から**作ります。

  ```sh
  docker build -f docker/rust-extractor/Dockerfile -t rivus-rust-extractor:nightly-2026-11-15 extractor/rust
  ```

  ベースの Debian はダイジェストで、Rust は日付つきの nightly で、固定してあります。extractor のバイナリは、同じ Dockerfile の中で、`--locked` でビルドされます。対象のコードは、イメージに入りません。
- 対象に **`Cargo.lock`** があること（なければ、`cargo generate-lockfile`。これはコードを実行しません）。`--locked` と、ネットワークなしの抽出のためです。
- 対象のソースが、そのディレクトリの中で閉じていること（ディレクトリの外の path 依存は、見えません）。

Docker が使えないときは、**何も実行せずに、止まります**: 「Rust の extractor は、対象のビルドスクリプトとマクロを実行する。隔離のために Docker を使う。`-unsafe-host-exec` を付けると、この機械の上で直接実行する」。

## 2 段階

| 段階 | ネットワーク | 動くもの | 書けるところ |
|---|---|---|---|
| 1. 取得 | あり | `cargo fetch --locked` だけ。**ビルドしない**ので、`build.rs` もマクロも動かない。**対象の cargo の設定（`.cargo/config.toml`）は、この段階では使わない**（`.cargo` を空の tmpfs で覆う。ボリュームに前の実行の設定があれば、捨てる）。設定が要るプロジェクトは、理由を言って止まる（取得済みなら `-no-fetch`。vendor 済みは、この段階を飛ばす） | 依存のソース用のボリューム（`rivus-cargo-<Cargo.lock と .cargo/config の hash>`）だけ |
| 2. 抽出 | **なし**（`--network none`） | extractor が `cargo check --offline --locked` を走らせる。`build.rs` と proc macro は、**ここで、コンテナの中で**動く | 作業用の一時ボリューム（`target/` と IR の断片。コンテナと一緒に消える）と、`/tmp`（tmpfs）だけ |

IR は、コンテナの標準出力から、rivus に返ります。ボリュームは、ロックファイルの内容ごとに別なので、あるプロジェクトが取得したものを、別のプロジェクトが読むことはありません。取得したボリュームは、残ります（次の実行で、速くなる）。消すには:

```sh
docker volume ls --filter name=rivus-cargo-
docker volume rm $(docker volume ls -q --filter name=rivus-cargo-)
```

## コンテナの制限

| 制限 | 値 |
|---|---|
| ユーザー | `--user 10001:10001`（root ではない） |
| 権限 | `--cap-drop ALL`、`--security-opt no-new-privileges` |
| ルートのファイルシステム | `--read-only` |
| ネットワーク（抽出） | `--network none` |
| プロセス・スレッド | `--pids-limit`（`-container-pids`、既定 4096） |
| メモリ | `--memory` と `--memory-swap` が同じ（`-container-memory`、既定 6g） |
| CPU | `--cpus`（`-container-cpus`、既定は、この機械の CPU 数、最大 8） |
| 時間 | 各段階で、`-container-timeout`（既定 30 分）。超えたら、コンテナを `docker rm -f` する |
| マウント | 対象のディレクトリを、**同じパスで、読み取り専用**。取得した依存を（抽出では）読み取り専用。それだけ。ホームも、`docker.sock` も、マウントしない。ホームそのもの、またはそれを含むディレクトリを対象にすると、拒否する |
| 環境変数 | ホストのものは、渡さない（`docker run` に `-e` を付けない。イメージの `CARGO_HOME`、`RUSTUP_*`、`HOME`、`TMPDIR`、`PATH` だけ） |
| ツールチェイン | イメージの nightly だけ。対象の `rust-toolchain(.toml)` が別のものを求めても、取得しない（`RUSTUP_TOOLCHAIN` で固定。警告を出す。コンパイルできなければ、失敗する） |

| 出力 | IR は、コンテナの標準出力でだけ返ります（出力用のディレクトリを、ホストに置きません。ホストは、コンテナが書いたファイルを読みません）。大きさは `-container-max-output`（既定 4 GiB）で切ります |

対象のビルドが、設定を要るとき（たとえば parseable の `build.rs` は、UI をネットワークから取るので、隔離では失敗します）は、**明示して**足します。ホストの環境変数は、引き継ぎません。

```sh
mkdir /path/to/empty-assets
rivus analyze -lang rust -no-deps -C proj \
  -container-env LOCAL_ASSETS_PATH=/assets -container-ro-mount /path/to/empty-assets:/assets
```

`-no-fetch` は、取得の段階を飛ばします（前に取得したボリュームがあるとき）。`-rust-image`（`RIVUS_RUST_IMAGE`）は、イメージを変えます。

## 信頼できるコードを、速く読む（ホストで直接）

`-unsafe-host-exec`（または `RIVUS_UNSAFE_HOST_EXEC=1`）で、従来どおり、この機械の上で、`rivus-rust` を直接動かします（`-rust-extractor` / `RIVUS_RUST_EXTRACTOR` / `PATH`）。自分のコード、CI で自分のリポジトリを読むとき、このリポジトリのベンチ（`bench/rust-curated`、`bench/zoo-rust`、`extractor/rust/test`）向けです。**`.rivus-hostile` のあるプロジェクト（隔離の検証用の、悪意のある crate）は、このモードでは、動かしません。**

ベンチの道具（`bench/compare.sh`、`bench/parity/collect.sh`）は、この許可リストを、コードで持ちます。**この 3 つのパスの中だけ**をホストで読み、それ以外の Rust の対象（parseable など第三者のもの）は、`rivus extract -lang rust`（既定のコンテナ）で読みます。Docker がないときは、ホストに切り替えずに、**失敗します**（「Rust の第三者のコードは隔離の中でだけ読む。Docker を起動するか、このベンチを外す」）。`rivus extract -lang rust` は、抽出器の IR を、そのまま標準出力に書きます（`-unsafe-host-exec` がなければ、コンテナ）。

## できないこと（限界）

- **コンテナは、仮想マシンではありません。** カーネルは共有です。Docker（runc、カーネル）の脆弱性による脱出は、防げません。Docker とカーネルを、更新してください。最も硬くするなら、使い捨ての VM の中で、rivus を動かしてください。
- **macOS / Windows の Docker Desktop は、Linux の VM の中で、コンテナを動かします。** 境界は、コンテナと、VM と、ホストの共有（ファイル共有）の 3 つです。対象のディレクトリは、ファイル共有を通して、読み取り専用で見えます。VM が、ホストの共有ディレクトリ（ホームを丸ごと共有している設定）を持っていても、**コンテナには、対象のディレクトリしか、マウントしません**。VM の外は、Docker Desktop の脆弱性しだいです。
- **対象のビルドは、結果を書き換えられます。** コンテナの中の `build.rs` と proc macro は、IR の断片のディレクトリに書けるので、解析結果（finding を消す、偽の関数を足す）を、好きにできます。**隔離が守るのは、マシンと秘密で、結果の正しさではありません**（信頼できないコードの findings は、信頼できないものとして扱う）。
- **取得の段階は、ネットワークを使います。** 以前は、対象の `.cargo/config.toml` が `credential-provider`（同梱のスクリプト。`[registry] global-credential-providers`、`[credential-alias]` も同じ）を指し、`Cargo.lock` にそのレジストリの依存があって、レジストリが認証を要求すると、cargo がそのスクリプトを、ビルドの前に、ネットワークのあるコンテナで実行しました（**「取得の段階では、対象のコードは動かない」は誤りでした**。ADR-064 の訂正）。今は、取得の段階に、対象の設定を読ませません。確かめた範囲（`TestIsolationFetchRunsNoCodeOfTheProject`。ブリッジ上の偽のレジストリと、目印を書くだけのスクリプト）: `credential-provider`、`global-credential-providers` と `[source]` の置き換え、`[credential-alias]` は、修正前に、スクリプトが動き（赤）、修正後は動かない（緑）。`net.git-fetch-with-cli` は、イメージに git がなく、修正前も動かない（イメージに git を入れると赤になる試験がある）。`[http] proxy` は、修正前に、対象が選んだ先へ要求が出て、修正後は出ない。`Cargo.lock` の `source` が指す先へは、cargo 自身が要求を出す（コードは動かないが、内側のネットワークへの要求になる）ので、**取得の前に、`Cargo.lock` の取得元を検査します**: **既定で許すのは crates.io のレジストリだけ**です。git の依存は、ホストを問わず（github.com も）、ほかのレジストリも、**一覧（どのパッケージが、どのリポジトリの、どのコミットか）を出して止まり**、許し方（`-allow-fetch-host HOST`。繰り返せる）を 1 行で言います。見たうえで許してください。許したホストへの取得で、**そのリポジトリが指す先へも、要求が出ます**: git の依存を取るとき、cargo はサブモジュールも取り（URL は、依存のリポジトリの `.gitmodules` にあり、`Cargo.lock` には出ません）、公開のホストは、30x で別のアドレス（内側を含む）へ飛ばせます。確かめた範囲（`TestIsolationFetchFollowsWhatTheAllowedRepositorySays`。ブリッジ上の偽の git ホストと、内側のつもりの偽のレジストリ）: サブモジュールの URL と、30x の先の両方に、要求が出ました（**防げていません**。試験は、この限界を固定するもので、ネットワークの絞り込みが入ったら、要求が出ないことを確かめる試験に反転します）。**取得のネットワークの絞り込み（`docs/DESIGN-fetch-network.md`）が入るまでは、防げません**。許すのは、信用するリポジトリだけにしてください。**`Cargo.lock` だけでは足りません**: `cargo fetch --locked` は、`Cargo.toml` が名指す取得元（git の依存、`[patch]`、`registry-index`）へ、`Cargo.lock` が合わないと気づく**前に**、要求を出します（確かめた: `Cargo.lock` にない git の依存を `Cargo.toml` に書くと、偽のホストに要求が出た）。そこで、対象の中のすべての `Cargo.toml`（`target/` と `.git/` を除く）も、TOML として読み（コードは動かさない）、`git`、`registry-index`、`[patch."URL"]` を、同じ検査にかけます（読めない `Cargo.toml` も止めます）。TOML の読み方が cargo と食い違う入力は、残りうる穴です（絞り込みが塞ぐ）。**残っているもの**: 公開の名前が内側のアドレスに解決される場合（DNS）も、同じ絞り込みで塞ぐ予定です。取得したソースは、そのままボリュームに入ります（抽出では、読み取り専用）。
- **CPU とメモリを、使い切らせることは、できます**（上限までは）。サービス妨害は、時間と上限で、有限にするだけです。
- **カーネルへの、攻撃面は、残ります**（seccomp は、Docker の既定のプロファイル）。
- **R-21 は、直りません。** 対象の `.cargo/config.toml` の `[env]` で、IR の断片の場所を変えて、IR を空にできます（コンテナの中でも同じ）。安全の問題ではなく、結果の正しさの問題です（別に、コンパイルした crate と断片の突き合わせが要ります）。
- 対象が大きいと、コンテナの分、遅く、メモリも使います（`docs/DECISIONS.md` の ADR-064 に、測った値）。
- 同時に走る、ほかの `rivus` のコンテナとは、ボリュームを共有しません（抽出の作業用ボリュームは、コンテナごと）。取得のボリュームは、同じロックファイルなら、共有されます。

## 同じ作りを、ほかの言語に

コンテナの制限と、起動・時間切れ・出力の上限は、言語を知りません（`cmd/rivus/isolate.go`）。Rust 固有なのは、イメージと、取得（`cargo fetch`）と、抽出の呼び出しだけです。Go（cgo、依存の取得）と Dart（`pub get`、`build_runner`）は、後で、同じ作りに載せます。

**載せるときに、最初に確かめること**: 取得の道具が、**対象の設定で、コードを動かすか、内側へ要求を出すか**。Go は `go mod download` の `GOPROXY` / `GOFLAGS` / `GONOSUMDB` / `.netrc`（`go.work`、`go.mod` の `replace`、`toolchain`）、Dart は `pub get` の hosted の URL（`pubspec.yaml` の `hosted:`、`PUB_HOSTED_URL`、`pub_cache` の設定）と、フック（`pubspec` の `executables`、`build_runner`）。Rust で、取得が、対象の設定で `credential-provider` のスクリプトを動かした（直した）のと同じ種類のものが、あるはずだと考えて調べる。取得の段階のネットワークの絞り込みは、言語を知らない形にする（`docs/DESIGN-fetch-network.md`）。

## 検証

`extractor/rust/test/hostile/` は、悪意のある `build.rs` と proc macro を持つ、自作の crate です。見えるもの（マウントの一覧、環境変数の名前、読めるファイルの中の、秘密の置き場の名前）と、止まらない `build.rs`、フォークの無限ループ、メモリを使い切るもの（コンテナの中だけで止まり、コンテナが残らない）、IR の断片のディレクトリに置く、リンクと大きなファイルも試します。ホストの鍵（のつもりのファイル）、プロジェクトの隣のファイル、ネットワーク（1.1.1.1、8.8.8.8、名前の解決）、`/` と `/etc` と対象と依存への書き込み、環境変数（トークンの名前）、`docker.sock`、root と capability を試し、**コンテナの中で、すべて失敗する**ことを、`TestIsolationHostileBuild` が確かめます（`go test ./cmd/rivus -run Isolation`。Docker とイメージが要る。CI では `RIVUS_REQUIRE_DOCKER=1`）。この crate は、ホストでは、動かしません（`.rivus-hostile` を rivus が拒否し、`build.rs` は、イメージの外では、何もせずに panic する。`RIVUS_UNSAFE_HOST_EXEC=1` のときは、テストが skip する）。
