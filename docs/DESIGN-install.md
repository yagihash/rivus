# 設計: インストールに何が必要か(Homebrew の formula の形を決める材料)

> **ユーザーの指示による例外(2026-11-22)。方向性(導入の手軽さには投資しない)は変えていない。** この作業の範囲は、「ユーザーが `brew install` で使える所まで」。それを超える仕上げ(いろいろな環境への対応、ビルド済みの配布の整備、インストールの文書の作り込み)は、ユーザーが頼まない限り、しない。
> **状態: Homebrew での配布は、v0.0.2 で動いている(2026-11-22)。** formula は tap(`yagihash/homebrew-tap`)にあり、リリースのタグが打たれると、`bump-tap` のジョブが `url` と `sha256` を書き換える。**6 節の「nightly を keg の `libexec` に置く」は、その後に変更した**: nightly は keg の外(`$(brew --prefix)/var/rivus/rustup`)に置く。keg の中に置くと、Homebrew がインストール後に行うライブラリの参照先の書き換えが、rustup の配布するライブラリで失敗し(ヘッダの余白が足りない)、`brew install` がエラーで終わるため。keg の外に置くと、rivus の更新のたびに約 1.1 GB を取得し直すこともなくなる。`rivus-rust` は keg の `libexec` に置き、参照先は `var` の側の nightly を指す。利用者向けの説明は README の「インストール」。以下は、決定までの経緯として残す。
>
> (経緯)Rust のインストール方法(6 節)は、ユーザーが決めた(2026-11-22)。ほかは案(棚卸し)。ユーザーの指示(2026-11-22): **Homebrew でのインストールを、当面の最優先にする**。相談役の順: 棚卸し(この文書)→ 隔離の移行([DESIGN-macos-sandbox.md](DESIGN-macos-sandbox.md))→ formula と tap。**tap のリポジトリを作る、リリースのタグを打つ、は外に出る・戻せない操作なので、そのつどユーザーの確認が必要。**

## 1. 実行時に rivus が必要なもの(言語ごと)

| 言語 | 実行時に必要なもの | 見つけ方(今) | 確かめたこと(2026-11-22、手元、macOS arm64) |
| --- | --- | --- | --- |
| Go | `go` コマンド(`go list`、`go env GOROOT` を呼ぶ。`internal/frontend/gossa/load.go`)と、対象の依存(モジュールキャッシュ)。`go.mod` は `go 1.27` | `PATH` の `go` | Go のバイナリだけでは足りない: **実行時に Go のツールチェインが必要** |
| Dart | Dart SDK(extractor は `package:analyzer` で、SDK の `dart:*` のソースを読む)。extractor は Dart のパッケージ(`extractor/dart`、`executables: rivus-dart`) | `-dart-extractor` > `RIVUS_DART_EXTRACTOR` > `PATH` の `rivus-dart`。入れ方の案内は「`dart pub global activate --source path extractor/dart`」 | **`dart compile exe` で、単体の実行ファイル(19 MB、約 10 秒)にできる。ただし、実行時に Dart SDK が必要**: extractor は、SDK の場所を `Platform.resolvedExecutable` の 2 つ上のディレクトリから導く(`extractor/dart/lib/extract.dart:203`)ので、コンパイルした実行ファイルは、SDK の中にないと動かない(`version` ファイルが開けず異常終了した)。SDK の場所を、環境変数か引数で渡せるようにする修正が必要 |
| Rust | 固定の nightly(`nightly-2026-11-15`、`rustc-dev`、`llvm-tools`、`rust-src`)、extractor(`rivus-rust`。`cargo +nightly-2026-11-15 install rivus`)、C コンパイラとリンカ(対象のビルド)。macOS は、サンドボックスの下で動かす(既定。ADR-093)。`-unsafe-host-exec` でホストの直接 | `-rust-extractor` > `RIVUS_RUST_EXTRACTOR` > `PATH` の `rivus-rust`。`RIVUS_RUST_TOOLCHAIN_DIR`(固定のツールチェインのディレクトリ。なければ `~/.rustup/toolchains/...`) | 決まった(6 節): formula が、インストールの中で、固定のツールチェインと extractor を keg の `libexec` に作る。ソースから作る人は `rivus setup rust -source` |

## 2. rivus が、それらを、どこから探すか

- **環境変数と `PATH` だけ**(`cmd/rivus/lang.go`)。**インストール先からの相対(`$(brew --prefix)/…`)で見つける仕組みはない**。formula は、`bin` に `rivus-dart` と `rivus-rust` を置けば `PATH` で見つかる(`brew` の `bin` は `PATH` にある)。
- テストとベンチは、`RIVUS_DART` / `RIVUS_RUST`(リポジトリの中の extractor)を使う(リポジトリの外では使わない)。
- 今のなっていない所: (a) Dart の extractor が、SDK の場所を実行ファイルの位置から導く(上の表)。(b) Rust の extractor の入れ方の案内が、Docker 前提(イメージをリポジトリから作る)。(c) どの言語の入れ方の案内も、`brew` の言葉でない(`dart pub global activate --source path extractor/dart` は、リポジトリのチェックアウトが必要)。

## 3. バイナリに埋め込まれているもの

| もの | 埋め込み |
| --- | --- |
| 組み込みのモデル(`internal/model/builtin/*.yaml`) | **埋め込み済み**(`//go:embed *.yaml`)。インストール先に別のファイルは不要 |
| 見本のルール(`examples/rules/`) | **埋め込まれていない**(リポジトリの中だけ)。README と `analyze` の 1 行の知らせは、`-rules examples/rules/...` を指す。インストールした利用者は、見本のファイルを持っていない(formula が `share` に入れるか、`rivus` が書き出すコマンドを持つか、README が URL を指すかを決める) |
| 文書(`docs/`) | 埋め込まれていない |

## 4. 配り方の選択肢

| | (i) formula が、ソースからビルドする | (ii) ビルド済みの実行ファイルを、リリースに置き、formula は取ってくる |
| --- | --- | --- |
| 必要な依存 | ビルド時: `go`(`go 1.27`)。Dart の extractor は、ビルド時に `dart compile exe`、または、実行時に `dart` を `depends_on` | formula は、ダウンロードだけ。実行時の依存(`go`、Dart SDK)は同じ |
| 初回の時間 | ビルドの分(Go: 数十秒〜、Dart: 約 10 秒)。依存の取得(`go mod download`)。formula のビルドは、サンドボックスの中で行われ、ネットワークが制限される(Go の依存の取得は、`brew` が許す形にする必要がある) | ダウンロードだけ(速い) |
| 署名と公証(Gatekeeper) | **出ない**(ビルドは、利用者の機械で行われる) | **出る**: ダウンロードした実行ファイルは、隔離属性が付き、署名と公証がないと、macOS が起動を止めることがある(`brew` は、ダウンロードの属性を外すが、確かめる) |
| リリースの仕組み | 不要(タグだけ。formula は `url` と `sha256`) | 必要(arm64 / x86_64、macOS / Linux のビルドを、CI で作り、リリースに置く。外へ出る操作) |
| 更新の追従 | タグごとに formula を更新 | 同じ + ビルド済みの作成 |

(選択はユーザーに聞く事項。**この表ができたので、相談役からユーザーに聞く**。)

## 5. `brew` の Linux

Go と Dart は動く(`go` と Dart SDK が、それぞれ `brew` で入る)。**Rust の信頼できないコードは、隔離の手段がない(macOS のサンドボックスだけにする)ので、何も実行せずに止まる**(DESIGN-macos-sandbox.md の 3.7)。既知のコードは `-unsafe-host-exec`。formula の `caveats` に、そう書く。

## 6. Rust: formula が、インストールの中で、ツールチェインと extractor を作る(**決まった**。ユーザー、2026-11-22。相談役が手元で確かめた)

- **formula は、`install` の中で、固定の nightly と `rivus-rust` を、keg の `libexec` に作る。`rustup` は formula の依存。** bottle(ビルド済みの配布)にしない。`post_install` や、利用者の `rivus setup rust` には頼らない。
- **作った場所 = 置く場所。** `rivus-rust` の `LC_RPATH` は、ビルドに使ったツールチェインの `lib` の絶対パス(`extractor/rust/build.rs`)。ビルドの場所から別の場所へ移すと、`dyld: Library not loaded: @rpath/librustc_driver-…` で動かない。Homebrew は keg に直接インストールするので、`RUSTUP_HOME=<libexec>/rustup` で作れば、そのまま動く。移す必要が出たら(bottle にするなど)、`install_name_tool -rpath <旧 lib> <新 lib>` と `codesign --force -s -`(arm64 は再署名が必要)で直す。
- **手順(コマンドの並び。`L` = keg の `libexec`、`TC` = `L/rustup/toolchains/nightly-2026-11-15-<arch>-apple-darwin`)**:
  1. ネットワーク(static.rust-lang.org、約 1 GB。`L/rustup` に 1.1 GB): `RUSTUP_HOME=L/rustup CARGO_HOME=L/cargo rustup toolchain install nightly-2026-11-15 --profile minimal -c rustc-dev -c llvm-tools -c rust-src --no-self-update`(既定のツールチェインは設定しない)。
  2. ネットワーク(crates.io。extractor の依存、`--locked`): `env -i SDKROOT=$(/usr/bin/xcrun --show-sdk-path) PATH=TC/bin:/usr/bin:/bin HOME=<ビルドの空き> RUSTUP_TOOLCHAIN=nightly-2026-11-15 CARGO_HOME=<ビルドの空き>/cargo CARGO_TARGET_DIR=<ビルドの空き>/target TC/bin/cargo install --path extractor/rust --locked --root L/rust-ext`(`rivus setup rust` と同じ環境)。
  3. wrapper(`write_env_script`)が、`RIVUS_RUST_TOOLCHAIN_DIR=TC` と `RIVUS_RUST_EXTRACTOR=L/rust-ext/bin/rivus-rust` を設定して、`rivus` を呼ぶ。実行時に rustup は使わない。利用者の `~/.rustup` と `~/.cargo` は、読みも書きもしない。
- **`SDKROOT` が必要。** なければ、この機械の `cc` が CLT の `MacOSX27.0.sdk` を選び、`libSystem.tbd` が読めず `tapi error: malformed file` でリンクに失敗した(Homebrew のビルド環境が `SDKROOT` を設定するはずだが、formula の中で確かめる)。`CARGO_TARGET_DIR` を指さないと、ソースの中の `extractor/rust/target` に出る。
- **確かめたこと(2026-11-22、macOS arm64、scratch の中、`HOME` を空にして)**: 上の形で、`bench/zoo-rust` の `rivus extract -lang rust`(サンドボックスの下)が、`~/.rustup` と `~/Library/Application Support/rivus` に頼らずに動き、IR は今の環境のものと同一だった(違うのは、IR の `file` に入る std のソースの絶対パスだけ。課題 #122)。ビルドの場所のツールチェインを消して、ツールチェインと extractor を移した場合は、rpath の書き換えと再署名で動き、IR は同一だった。
- **必要なもの**: Xcode CLT(`cc` とリンカ)。ディスク約 1.1 GB。インストールの間のネットワーク(static.rust-lang.org と crates.io)。
- **`rivus setup rust -source <extractor/rust>`** は、Homebrew を使わない人(リポジトリから作る人)の手段として残す。置き場所は、利用者のデータのディレクトリ(`~/Library/Application Support/rivus/`)。

## 6a. バージョンの一致

extractor が自分のバージョンを答え、rivus は合わなければ止まる(課題 #109)。formula では、wrapper が env を固定するので、ずれる余地は小さい。`setup rust` で作る人は、この確認が前提(#109、#119)。

## 7. formula の下書き(このリポジトリの中に置ける。tap を作る前)

formula の中身(`depends_on`、`caveats`、`test do`)は、先にこのリポジトリの中に下書きとして置ける(外に出ない)。`depends_on`: ビルド時 `go`、実行時 `go`(`go list` のため)、Dart の扱い(上の表。`dart-sdk` を依存に)、Rust は `rustup`(6 節)。`test do`: `rivus version`(足したあと)と、小さい Go のプロジェクトの解析。

## 8. なっていない所の一覧(直す作業)

1. Dart の extractor の SDK の場所を、実行ファイルの位置からでなく、環境変数または `dart` の位置から導く(単体の実行ファイルにするため)。
2. 見本のルールを、インストールした利用者が使える形にする(`share` に入れる、または書き出すコマンド)。
3. 入れ方の案内を、`brew` の言葉にする(README、エラーメッセージ)。
4. **`rivus version` / `--version` が、ない**(確かめた: `unknown command`)。formula の `test do` に必要なので、足す。バージョンの埋め込み(`-ldflags`)と、リリースのタグ付け。
5. Rust: `rivus setup rust` と、隔離の移行。
6. `go` のバージョン(`go 1.27`)を、formula の `depends_on` で満たせるか(`brew` の `go` のバージョンが、`go.mod` の要求より古いと、ビルドできない。**確かめた(2026-11-22): `brew info go` は stable 1.27.4 で、`go 1.27` を満たす**)。

## 9. ユーザーに聞くこと(相談役が聞く)

配り方(i) か (ii)、tap の名前、Dart の extractor を、ソースからビルド・単体の実行ファイル・実行時に `dart` のどれにするか。
