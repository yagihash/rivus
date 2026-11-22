# 設計: インストールに何が要るか(Homebrew の formula の形を決める材料)

> **状態: 案**(2026-11-22。棚卸し。実装はない)。ユーザーの指示(2026-11-22): **Homebrew でのインストールを、当面の最優先にする**。相談役の順: 棚卸し(この文書)→ 隔離の移行([DESIGN-macos-sandbox.md](DESIGN-macos-sandbox.md))→ formula と tap。**tap のリポジトリを作る、リリースのタグを打つ、は外に出る・戻せない操作なので、そのつどユーザーの確認が要る。**

## 1. 実行時に rivus が要るもの(言語ごと)

| 言語 | 実行時に要るもの | 見つけ方(今) | 確かめたこと(2026-11-22、手元、macOS arm64) |
| --- | --- | --- | --- |
| Go | `go` コマンド(`go list`、`go env GOROOT` を呼ぶ。`internal/frontend/gossa/load.go`)と、対象の依存(モジュールキャッシュ)。`go.mod` は `go 1.27` | `PATH` の `go` | Go のバイナリだけでは足りない: **実行時に Go のツールチェインが要る** |
| Dart | Dart SDK(extractor は `package:analyzer` で、SDK の `dart:*` のソースを読む)。extractor は Dart のパッケージ(`extractor/dart`、`executables: rivus-dart`) | `-dart-extractor` > `RIVUS_DART_EXTRACTOR` > `PATH` の `rivus-dart`。入れ方の案内は「`dart pub global activate --source path extractor/dart`」 | **`dart compile exe` で、単体の実行ファイル(19 MB、約 10 秒)にできる。ただし、実行時に Dart SDK が要る**: extractor は、SDK の場所を `Platform.resolvedExecutable` の 2 つ上のディレクトリから導く(`extractor/dart/lib/extract.dart:203`)ので、コンパイルした実行ファイルは、SDK の中にないと動かない(`version` ファイルが開けず落ちた)。SDK の場所を、環境変数か引数で渡せるようにする直しが要る |
| Rust | 固定の nightly(`nightly-2026-11-15`、`rustc-dev`、`llvm-tools`、`rust-src`)、extractor(`rivus-rust`。`cargo +nightly-2026-11-15 install rivus`)、C コンパイラとリンカ(対象のビルド)。今は Docker のコンテナの中で動かす(既定)。`-unsafe-host-exec` でホストの直接 | `-rust-extractor` > `RIVUS_RUST_EXTRACTOR` > `PATH` の `rivus-rust`。コンテナのイメージは `rivus-rust-extractor:nightly-2026-11-15`(ローカルで作る) | 隔離の移行(DESIGN-macos-sandbox.md)で、固定のツールチェインと extractor を、サンドボックスの外で用意する(`rivus setup rust`) |

## 2. rivus が、それらを、どこから探すか

- **環境変数と `PATH` だけ**(`cmd/rivus/lang.go`)。**インストール先からの相対(`$(brew --prefix)/…`)で見つける仕組みはない**。formula は、`bin` に `rivus-dart` と `rivus-rust` を置けば `PATH` で見つかる(`brew` の `bin` は `PATH` にある)。
- テストとベンチは、`RIVUS_DART` / `RIVUS_RUST`(リポジトリの中の extractor)を使う(リポジトリの外では使わない)。
- 今のなっていない所: (a) Dart の extractor が、SDK の場所を実行ファイルの位置から導く(上の表)。(b) Rust の extractor の入れ方の案内が、Docker 前提(イメージをリポジトリから作る)。(c) どの言語の入れ方の案内も、`brew` の言葉でない(`dart pub global activate --source path extractor/dart` は、リポジトリのチェックアウトが要る)。

## 3. バイナリに埋め込まれているもの

| もの | 埋め込み |
| --- | --- |
| 組み込みのモデル(`internal/model/builtin/*.yaml`) | **埋め込み済み**(`//go:embed *.yaml`)。インストール先に別のファイルは要らない |
| 見本のルール(`examples/rules/`) | **埋め込まれていない**(リポジトリの中だけ)。README と `analyze` の 1 行の知らせは、`-rules examples/rules/...` を指す。インストールした利用者は、見本のファイルを持っていない(formula が `share` に入れるか、`rivus` が書き出すコマンドを持つか、README が URL を指すかを決める) |
| 文書(`docs/`) | 埋め込まれていない |

## 4. 配り方の選択肢

| | (i) formula が、ソースからビルドする | (ii) ビルド済みの実行ファイルを、リリースに置き、formula は取ってくる |
| --- | --- | --- |
| 要る依存 | ビルド時: `go`(`go 1.27`)。Dart の extractor は、ビルド時に `dart compile exe`、または、実行時に `dart` を `depends_on` | formula は、ダウンロードだけ。実行時の依存(`go`、Dart SDK)は同じ |
| 初回の時間 | ビルドの分(Go: 数十秒〜、Dart: 約 10 秒)。依存の取得(`go mod download`)。formula のビルドは、サンドボックスの中で行われ、ネットワークが制限される(Go の依存の取得は、`brew` が許す形にする必要がある) | ダウンロードだけ(速い) |
| 署名と公証(Gatekeeper) | **出ない**(ビルドは、利用者の機械で行われる) | **出る**: ダウンロードした実行ファイルは、隔離属性が付き、署名と公証がないと、macOS が起動を止めることがある(`brew` は、ダウンロードの属性を外すが、確かめる) |
| リリースの仕組み | 要らない(タグだけ。formula は `url` と `sha256`) | 要る(arm64 / x86_64、macOS / Linux のビルドを、CI で作り、リリースに置く。外へ出る操作) |
| 更新の追従 | タグごとに formula を更新 | 同じ + ビルド済みの作成 |

(選択はユーザーに聞く事項。**この表ができたので、相談役からユーザーに聞く**。)

## 5. `brew` の Linux

Go と Dart は動く(`go` と Dart SDK が、それぞれ `brew` で入る)。**Rust の信頼できないコードは、隔離の手段がない(macOS のサンドボックスだけにする)ので、何も実行せずに止まる**(DESIGN-macos-sandbox.md の 3.7)。既知のコードは `-unsafe-host-exec`。formula の `caveats` に、そう書く。

## 6. `rivus setup rust` と formula

`rivus setup rust`(固定の nightly と extractor を、サンドボックスの外で、信頼できるソース(rivus 自身)から用意する、**明示のコマンド**)。formula は、**`post_install` で勝手にダウンロードしない**。`caveats` で、「Rust を読むときは、`rivus setup rust` を 1 回実行する」と案内する。置き場所は、利用者のデータのディレクトリ(`~/Library/Application Support/rivus/` を案)。

## 7. formula の下書き(このリポジトリの中に置ける。tap を作る前)

formula の中身(`depends_on`、`caveats`、`test do`)は、先にこのリポジトリの中に下書きとして置ける(外に出ない)。`depends_on`: ビルド時 `go`、実行時 `go`(`go list` のため)と、Dart の扱い(上の表)。`test do`: `rivus version`(足したあと)と、小さい Go のプロジェクトの解析。

## 8. なっていない所の一覧(直す仕事)

1. Dart の extractor の SDK の場所を、実行ファイルの位置からでなく、環境変数または `dart` の位置から導く(単体の実行ファイルにするため)。
2. 見本のルールを、インストールした利用者が使える形にする(`share` に入れる、または書き出すコマンド)。
3. 入れ方の案内を、`brew` の言葉にする(README、エラーメッセージ)。
4. **`rivus version` / `--version` が、ない**(確かめた: `unknown command`)。formula の `test do` に要るので、足す。版の埋め込み(`-ldflags`)と、リリースのタグ付け。
5. Rust: `rivus setup rust` と、隔離の移行。
6. `go` の版(`go 1.27`)を、formula の `depends_on` で満たせるか(`brew` の `go` の版が、`go.mod` の要求より古いと、ビルドできない)。

## 9. ユーザーに聞くこと(相談役が聞く)

配り方(i) か (ii)、tap の名前、Dart の extractor を、ソースからビルド・単体の実行ファイル・実行時に `dart` のどれにするか。
