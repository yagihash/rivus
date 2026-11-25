# 信頼しない Rust を、隔離して読む

`rivus analyze -lang rust` は、Rust のプロジェクトを**ビルドします**(`cargo check`)。ビルドは、対象の `build.rs`、proc macro、`[build-dependencies]` を実行します。採用前のライブラリや、怪しいリポジトリを読むときは、それが、あなたのマシンで、あなたの権限(ホームの鍵、トークン、ネットワーク)で動くことになります。そこで、**macOS では既定で、OS のサンドボックス(Seatbelt、`sandbox-exec`)の下で、隔離して動かします**([ADR-093](adr/093-rust-isolation-macos-sandbox.md)。以前は Docker のコンテナ: ADR-064)。**Linux でも、カーネルが Landlock を持っていれば、Landlock と seccomp の下で、同じ形で隔離して動かします**(試作。[DESIGN-linux-sandbox.md](DESIGN-linux-sandbox.md)、下の「要件(Linux)」)。**サンドボックスが作れないとき(Landlock のない Linux、macOS と Linux 以外)は、既定では何も実行せずに止まります**。信頼できるコードは、`-unsafe-host-exec` で、このマシンの上で、隔離なしに読みます。

どちらで読んだかは、出力の `summary.isolation`(`mode`: `sandbox` / `host`。サンドボックスで `DEVELOPER_DIR` を渡したときは `developer_dir`: `{value, source}`、`source` は `default` か `user`)に出ます(Rust の抽出をしたときだけ)。

## `-lang` なし(auto)の動き

`Cargo.toml` のある対象を `-lang` なしで読むと、次のとおりになります(ADR-093 の追記)。`-lang rust` と書いたときは、聞かずに読みます。

| 状況 | 動き |
|---|---|
| macOS か Linux で、サンドボックスが使える、extractor とツールチェインがある、標準入力が端末 | 標準エラーに警告を出して `[Y/n]`(Enter は Yes) |
| 同じで `-yes` あり | 聞かずに読む |
| 標準入力が端末でない(`claude -p`、CI、パイプ)、`-yes` なし | 聞けない。何も報告せずに Yes にすることはしない。混在なら Rust を読まずにその旨を出し、Rust だけの対象はエラー(`-lang rust か -yes を`。終了コード 1) |
| extractor かツールチェインがない | 聞かずに、Rust は読まなかったと報告する(`rivus setup rust` か Homebrew で入る)。Rust だけの対象はエラー |
| サンドボックスがない(Landlock のない Linux、macOS と Linux 以外) | 聞かない、読まない。`-unsafe-host-exec` があっても auto では Yes にしない(隔離なしは、`-lang rust` と `-unsafe-host-exec` の両方を書いたときだけ)。サンドボックスがあっても、`-unsafe-host-exec` があれば、聞く文(サンドボックスの下で動く)と合わないので、auto では聞かず、読まない(`no-sandbox`) |

聞く文は、読むにはビルドが要り、対象の build script と proc macro が動くこと、サンドボックス(macOS は Seatbelt、Linux は Landlock と seccomp)の下で動くこと(ビルドの間はネットワークなし、対象・依存・作業ディレクトリの外は書けない)、依存を crates.io から取得すること、聞かれたくなければ `-lang rust`(か `-yes`)、読みたくなければ `-lang go,dart` のように指定すること、答えによって別の index になること、を示します。端末かどうかは、`unix.IoctlGetTermios`(`/dev/null` を端末と誤らない)で見ます。答えは 1 回の実行で 1 度だけ聞きます。index に載った言語は、あとの `graph -C` が引き継ぎます(また聞かない)。

読まなかった Rust は、`summary.unread.skipped_languages` と index の `meta.unread.skipped_languages` に、`{"lang":"rust","why":...}` で残ります。`why`: `declined`(聞いて No)、`no-terminal`(聞けず、`-yes` もない)、`not-installed`(extractor かツールチェインがない)、`no-sandbox`。失敗したパッケージ(`failed_packages`、`load_errors`)には数えません。この index では、`graph path` の「経路なし」は `exhaustive: false` で、`not_exhaustive_because.skipped_languages` が理由を示します。

終了コード: 0 = 成功、1 = エラー、2 = 使い方の誤り、3 = Rust だけの対象で断った(何も読まなかった)、130 = 中断。

## 要件(macOS)

- `sandbox-exec`(macOS に入っています)。使えないとき(プロファイルが受け入れられないとき)は、**対象を動かす前に、rivus が確認し**、何も実行せずに止まります。
- 固定の nightly(`extractor/rust/rust-toolchain.toml` の `channel`、`rustc-dev`、`llvm-tools`、`rust-src`)と、extractor(`rivus-rust`)。**`rivus setup rust -source <リポジトリの extractor/rust>`**(明示のコマンド)が、サンドボックスの外で、信頼できるソースから用意します: ツールチェインがなければ、rustup でダウンロードする前に断ります(約 1 GB)。extractor は、`~/Library/Application Support/rivus/rust/extractor` に作られ、指定も `PATH` もなければ、そこの `rivus-rust` が使われます。置き場所は、`RIVUS_RUST_TOOLCHAIN_DIR`(固定のツールチェインのディレクトリ)と `RIVUS_RUST_EXTRACTOR`(`rivus-rust` のパス)でも指せます。`rivus-cargo-*` と extractor の置き場所(`~/Library/Application Support/rivus/rust`)は `RIVUS_RUST_DATA_DIR` で変えられます(rivus のテストは、利用者のディレクトリに書かないよう、一時ディレクトリを指します)。(extractor は、ビルドに使ったツールチェインの `lib` を、実行時に探します: 同じ場所を指すこと)。実行時に rustup は使いません。
- 対象に **`Cargo.lock`** があること(なければ、`cargo generate-lockfile`。これはコードを実行しません)。
- 対象のソースが、そのディレクトリの中で閉じていること。**対象の上の(祖先の)ディレクトリに `.cargo/config.toml` があると、何も実行せずに、そのファイルの名前を示して止まります**(cargo は祖先の設定も読み、サンドボックスはそれを読ませません。`~/.cargo/config.toml` を持つ人は、ホームの下の対象を、サンドボックスの下で読めません。回避: そのファイルを一時的に退ける。信頼できるコードなら `-unsafe-host-exec`)。

## 要件(Linux。試作)

- **Landlock が使えるカーネル**(Linux 5.13 以上で、LSM に `landlock` が入っていること: `cat /sys/kernel/security/lsm`)と、seccomp(x86-64 と arm64 だけ)。外部のプログラム(`bwrap` など)も、非特権のユーザー名前空間も要りません。作れないとき(Landlock がない、seccomp が通らない、コンテナの seccomp が Landlock の system call を塞いでいる)は、**対象を動かす前に、rivus が確認し**、何も実行せずに止まります。
- 抽出の段階の「ネットワークなし」は、seccomp(ソケットを作らせない)で成り立つので、Landlock のバージョンによりません。**他プロセスへのシグナルを塞ぐのは Linux 6.12 以上**(Landlock ABI 6)、**TCP のポートの制御は 6.7 以上**(ABI 4)です。古いカーネルでは、その分が弱くなります(詳細は [DESIGN-linux-sandbox.md](DESIGN-linux-sandbox.md) の 4 節)。
- 固定の nightly と extractor は、macOS と同じ手順(`rivus setup rust -source <リポジトリの extractor/rust>`)で用意します。ツールチェインは `~/.rustup/toolchains/<固定の nightly>-<x86_64|aarch64>-unknown-linux-gnu`、extractor と取得した依存の置き場所は `~/.config/rivus/rust`(`RIVUS_RUST_DATA_DIR` で変えられる)。
- ビルドスクリプトが C をコンパイルするので、C コンパイラとリンカ(`cc`、`ld`、`as`、`ar`)とヘッダ(`/usr/include`)がシステムにあること。実行を許すのは、名指しの一覧(`cc`、`gcc`、`ld`、`as`、`ar`、`/bin/sh` など)と、動的ローダーと、C コンパイラの内部のプログラムの置き場所(`/usr/lib/gcc`、`/usr/libexec`)だけで、`curl`、`ssh`、`python` などは、普通には実行できません。ただし、これは境界ではありません(Landlock は `execve` しか見ないので、動的ローダーを直接呼べば、読み取りを許した場所のプログラムは動く)。境界は、ファイルの読み書きとネットワークです。
- 対象の上の(祖先の)`.cargo/config.toml` と、`Cargo.lock` の要件は、macOS と同じです。

## 2 段階

| 段階 | ネットワーク | 動くもの | 書けるところ |
|---|---|---|---|
| 1. 取得 | あり(`network-outbound`)。mach のサービスは、1 つも許さない | `cargo fetch --locked` だけ。**ビルドしない**ので、`build.rs` もマクロも動かない。**対象の cargo の設定は、この段階では使わない**(設定のない作業ディレクトリから実行する。cargo は設定を、カレントから探す)。設定が必要なプロジェクトは、理由を示して止まる(取得済みなら `-no-fetch`。vendor 済みは、この段階を飛ばす)。取得の前に、`Cargo.lock` と `Cargo.toml` の取得元を検査する(crates.io 以外は `-allow-fetch-host`) | rivus 専用の `CARGO_HOME`(`~/Library/Application Support/rivus/rust/rivus-cargo-<ロックファイルと設定のハッシュ>/`)と、作業ディレクトリだけ |
| 2. 抽出 | **なし** | extractor が `cargo check --offline --locked` を実行する。`build.rs` と proc macro は、**ここで、サンドボックスの中で**動く | 作業ディレクトリ(`target/` と IR の断片。終わると消える)だけ。取得した `CARGO_HOME` と対象は読み取り専用 |

利用者の `~/.cargo` は、どちらの段階でも、読みも書きもしません。IR は、extractor の標準出力から、rivus に返ります。

## サンドボックスの制限

以下は macOS(プロファイル: `internal/sandbox/seatbelt_darwin.go`)の表です。Linux(`internal/sandbox/landlock_linux.go`、`seccomp_linux.go`)は、同じ許可(読み取りは対象・依存・ツールチェイン・作業ディレクトリ・システムのライブラリ、書き込みは作業ディレクトリだけ、環境変数・時間・出力の上限は同じ)を Landlock のルールで作り、ネットワークは seccomp で `socket()` を全拒否にし(TCP も UDP も UNIX ソケットも)、`ptrace`・`process_vm_*`・`io_uring` などを塞ぎ、`/proc` を許可に入れません。Linux との違いは [DESIGN-linux-sandbox.md](DESIGN-linux-sandbox.md) の 4 節です。

macOS のプロファイルは、**既定ですべて拒否し、必要なものだけを、1 行ずつ理由つきで許します**。

| 制限 | 値 |
|---|---|
| ファイルの読み取り | 対象、取得した依存、固定のツールチェイン、extractor、作業ディレクトリ、システムのライブラリ(Apple の基本のプロファイルの範囲)、`/private/etc/ssl`。**ホームは、対象以外、名前も中身も見せない** |
| 書き込み | 作業ディレクトリだけ(取得では、`CARGO_HOME` も)。ビルドスクリプトと proc macro は、ビルドが作業ディレクトリに作るもの(= 対象のコードの実行そのもの。絞れない) |
| ネットワーク(抽出) | すべて拒否(UDP、IPv6、UNIX ソケット、名前解決を含む) |
| ほかのプログラム・サービス | mach のサービスの lookup は全拒否(Apple Events、`open`、`launchctl`、ペーストボード、キーチェーン、`cfprefsd`、名前解決のデーモン)。ほかのプロセスへのシグナルは拒否 |
| 実行してよいプログラム | 名指しの一覧(`/bin/sh`、`/bin/bash`、`cc`、`clang`、`ld`、`ar`、`xcrun`、`xcode-select`)、固定のツールチェイン、CommandLineTools、extractor、作業ディレクトリの中のもの |
| 環境変数 | ホストのものは渡さない(決めたものだけ) |
| 時間 | 各段階で `-isolation-timeout`(既定 30 分)。超えたら、プロセスグループごと kill |
| 出力の大きさ | `-isolation-max-output`(既定 4 GiB)。超えたら kill |
| ツールチェイン | 固定の nightly だけ。対象の `rust-toolchain(.toml)` が別のものを求めても、取得しない(警告を出す)。rustup は使わない |

**`DEVELOPER_DIR`**: ビルドスクリプトのリンクは `xcrun` を動かし、サンドボックスは Xcode.app を読ませません。そこで、`-isolation-env` に `DEVELOPER_DIR` がなく、`/Library/Developer/CommandLineTools` があるときは、`DEVELOPER_DIR` をそこに向けます(`-v` で出し、`summary.isolation.developer_dir` に残る。サンドボックスの読めるパスは広げない)。`-isolation-env DEVELOPER_DIR=...` を渡したときは、そのまま使います。コマンドラインツールがなく、`xcode-select -p` が Xcode.app のときは、実行の前に、原因と修正の方法(`xcode-select --install`、または読めるパスの `DEVELOPER_DIR`)を言って止まります。

**ビルドに設定を足す**(parseable の `build.rs` は、UI をネットワークから取るので、隔離では失敗します): `-isolation-env NAME=VALUE`(ビルドに渡す環境変数。ホストのものは引き継がない)と、`-isolation-read PATH`(ビルドが読める追加のパス)。

## 信頼できるコードを、速く読む(ホストで直接)

`-unsafe-host-exec`(または `RIVUS_UNSAFE_HOST_EXEC=1`)で、このマシンの上で、`rivus-rust` を直接動かします(`-rust-extractor` / `RIVUS_RUST_EXTRACTOR` / `PATH`)。自分のコード、CI で自分のリポジトリを読むとき、このリポジトリのベンチ(`bench/rust-curated`、`bench/zoo-rust`、`extractor/rust/test`)向けです。Landlock のある Linux では、信頼できないコードは既定で隔離の下で読めるので、これは自分のコードを速く読むための道です。Landlock のない Linux(古いカーネル、LSM の無効、コンテナの seccomp)では、これだけが通ります。CI の `rust` ジョブは、このリポジトリのコードだけを、これで読みます。**`.rivus-hostile` のあるプロジェクト(隔離の検証用の、悪意のある crate)は、このモードでは、動かしません。**

ベンチの道具(`bench/compare.sh`、`bench/parity/collect.sh`)は、この許可リストを、コードで持ちます。**この 3 つのパスの中だけ**をホストで読み、それ以外の Rust の対象(第三者のもの)は、`rivus debug extract -lang rust`(サンドボックス)で読みます。サンドボックスがないときは、ホストに切り替えずに、**失敗します**。

## 承知している弱さ(ADR-093。ユーザーが受け入れた)

- **`sandbox-exec` は、Apple が非推奨にしたまま動いている仕組みです**(macOS)。
- **メモリ、CPU、プロセス数の強い上限がありません**(Docker にはあった)。時間の上限と、プロセスグループごとの kill で代えます。
- **ユーザー境界がありません**(あなたと同じユーザーで動きます。防御は、プロファイルだけ)。**ホストのカーネルに直接触れます**(Docker Desktop では、Linux の VM のカーネルでした)。実行できるプログラム、システムのファイルが見える範囲も、Docker より広い。
- **同じユーザーのほかのプロセスの引数と環境(`kern.procargs2`)が、サンドボックスの下から読めます**(拒否する書き方を、5 通り試して、見つかりませんでした)。受け入れの根拠: **読んだものを外に出す経路がありません**(ネットワーク、名前解決、UNIX ソケット、ほかのアプリへの依頼、許可の外への書き込み、共有メモリ: `TestSandboxClosesEveryWayOut`)。**残る経路: rivus の出力(解析の結果とログ)に、対象が決める文字列が載ること。信頼できないコードの解析の結果とログは、そのコードが書いたものを含みえます。そのまま公開の場所に出さないでください。**
- **プロセスグループを抜けるプロセス**(`setsid`、二重フォーク)は、実行の終わりに止められません(サンドボックスの下のまま。ファイルとネットワークには触れません)。残ったかもしれないことは、報告するだけです(rivus は、自分が起動したグループの外のプロセスに、シグナルを送りません)。
- **取得の段階のネットワークは、絞っていません**(宛先の許可リスト、解決したアドレスの検査がない)。リダイレクトとサブモジュールで、許した取得元が別の宛先へ向けられます。Docker の既定の bridge と同じ(今より弱くはならない)。
- **解析する構成が、実行する OS 向けになります**: `cfg(target_os)` で分かれるコードは、macOS では macOS の側、Linux では Linux の側を読みます(Docker では Linux の側でした)。同じ対象でも、OS によって、読む関数が変わります(出力に target triple を出すこと: 課題 #146)。
- **Linux の隔離は試作です**(ADR-093 は macOS の決定で、Linux は [DESIGN-linux-sandbox.md](DESIGN-linux-sandbox.md))。取得の段階のネットワークは、Linux でも絞っていません。Landlock は、ポート番号までしか絞れず(アドレスは指定できない)、ファイルの存在と属性(`stat`)は見えます。ユーザー境界がないのは、macOS と同じです。**対象のコードは、同じユーザーのファイルのモードや時刻(`chmod`、`utimes`)を変えられます**(Landlock が止めない。中身は読めず、書けない)。止めずに、作業ディレクトリの外のものを、実行の終わりに stderr に出します(`rivus: warning: the build changed the mode ...`)。古いカーネルでは、Linux 6.2 未満は何も実行せずに止まり、6.12 未満は、同じユーザーのほかのプロセスにシグナルを送れるので、警告を出して動かします。メモリ・プロセス数・ファイルの大きさには、緩い `RLIMIT` があります(強い上限ではありません)。
- **対象のビルドは、結果を書き換えられます。** 作業ディレクトリの IR の断片は、対象のビルドから見えるので、解析結果(finding を消す、偽の関数を足す)を、好きにできます。**隔離が守るのは、マシンと秘密で、結果の正しさではありません**(信頼できないコードの findings は、信頼できないものとして扱う)。
- **R-21 は、直りません。** 対象の `.cargo/config.toml` の `[env]` で、IR の断片の場所を変えて、IR を空にできます(安全の問題ではなく、結果の正しさの問題)。
- **対象が大きいと、時間もメモリも、そのまま使います**(上限がない)。

**後回しにしたハードニング**は、課題の一覧にあります([ISSUES.md](ISSUES.md)の #110 以降): 取得のプロキシ、実行の許可の絞り込み、`bsd.sb` の許可の棚卸し、メタデータの見える範囲、メモリとプロセス数の監視、残ったプロセスの報告、プロファイルの各行のテスト、シグナルの `same-sandbox`、端末、macOS の CI(全体)。

## 同じ作りを、ほかの言語に

サンドボックス(`internal/sandbox`)は、言語を知りません(プログラムと、読み・書き・実行の許可と、時間・出力の上限だけ)。Rust 固有なのは、2 段階の呼び出しと、取得前の検査(`cmd/rivus/isolate_sandbox_darwin.go`、`isolate_sandbox_linux.go`、`isolate_fetch.go`、`isolate_sources.go`)だけです。Go(cgo、依存の取得)と Dart(`pub get`、`build_runner`)は、後で、同じ作りに載せます。

**載せるときに、最初に確かめること**: 取得の道具が、**対象の設定で、コードを動かすか、内側へ要求を出すか**。Go は `go mod download` の `GOPROXY` / `GOFLAGS` / `GONOSUMDB` / `.netrc`(`go.work`、`go.mod` の `replace`、`toolchain`)、Dart は `pub get` の hosted の URL(`pubspec.yaml` の `hosted:`、`PUB_HOSTED_URL`、`pub_cache` の設定)と、フック(`pubspec` の `executables`、`build_runner`)。Rust で、取得が、対象の設定で `credential-provider` のスクリプトを動かした(直した)のと同じ種類のものが、あるはずだと考えて調べる。

## 検証

`extractor/rust/test/hostile_sandbox/` は、悪意のあるビルドスクリプトを持つ、自作の crate です。ビルドスクリプトが、probe(`internal/sandbox/probe`)を動かし、サンドボックスから出る、知っている限りの方法を試します(ホームの鍵のつもりのファイル、ネットワーク、書き込み、`osascript` / `launchctl` / `open`、ペーストボード、キーチェーン、設定の書き込み、UNIX ソケット、名前解決)。**すべて失敗する**ことを、`TestSandboxedBuildScriptCannotEscape` が確かめます。サンドボックスなしでは、同じ方法が成功すること(抜け道が実在すること)を、`TestSandboxHolesAreRealWithoutTheSandbox` が確認し、サンドボックスの下で止まることを `TestSandboxStopsEveryHoleOfTheProbe` と、外に出る経路の組 `TestSandboxClosesEveryWayOut` が確かめます。取得の段階で対象の設定を読まないこと、ビルドスクリプトが動かないこと、時間・出力の上限・環境変数も、テストがあります。**これらのテストは、開発のマシンで回します**(`RIVUS_SBX_RED=1`、取得のテストは `RIVUS_SBX_NET=1`。害のないマーカーを、サンドボックスの外に作ることがあり、終わりに消します)。CI(`.github/workflows/ci.yml の sandbox ジョブ`)は、隔離に関わるパスが変わったとき、週に 1 回、手動で、`internal/sandbox` のテストだけを macOS で実行します。

**Linux** のテストは `internal/sandbox/landlock_linux_test.go` です(probe は `internal/sandbox/probelinux`)。サンドボックスなしでは出られること(`TestSandboxProbeEscapesOnTheHost`)、サンドボックスの下では全部止まること(`TestSandboxClosesEveryWayOut`)、実際の `cargo check` で、`build.rs` が C コンパイラは動かせて、秘密の読み取り・外への書き込み・ネットワーク・`curl` はできないこと(`TestSandboxBuildScriptIsContained`)、を確かめます。CI の `sandbox-linux` ジョブが、同じ条件で、`ubuntu-latest`、`ubuntu-26.04-arm`、`ubuntu-24.04`、`ubuntu-24.04-arm`(どれも Linux 7.0 系。古いカーネルは、テストのバイナリで ABI を下げて作る)で実行し、root でも、もう一度実行します(`RIVUS_SBX_RED=1`: サンドボックスが作れないランナーは、飛ばさずに失敗)。**`sandbox-e2e-linux` と `sandbox-e2e-darwin` ジョブ**(`scripts/sandbox-e2e.sh`)は、固定の nightly と extractor を用意し、悪意のある `build.rs` と、proc macro・`build.rs` を持つ依存(serde)の crate を、`rivus analyze -lang rust` で読み、出られないこと、`summary.isolation.mode` が `sandbox` であることを確かめます(対照として、同じ `build.rs` をホストで直接動かすと、出られること)。PR では、変更したファイルによって、macOS のジョブ(`sandbox`、`sandbox-e2e-darwin`)と Linux のジョブ(`sandbox-linux`、`sandbox-e2e-linux`)が、それぞれ必要なときだけ動きます(`ci.yml` の `changes` ジョブの `isolation_common`、`isolation_darwin`、`isolation_linux`。共通のファイル(`internal/sandbox/sandbox.go`、`cmd/rivus/isolate*.go`、`lang.go`、`setup.go`、`internal/report/**`、`extractor/rust/**`、`go.mod`、`ci.yml` など)は両方、`*_darwin*` は macOS だけ、`*_linux*` は Linux だけ)。main への push、週 1 回、手動では、両方が動きます。

`.rivus-hostile` のあるプロジェクトは、`-unsafe-host-exec` では動かしません(目印がホストに残らない)。
