# ADR-126: extractor のソースのハッシュを、ビルドのときに埋め、extractor の `--version` で答える(2026-11-25)

- 状態: 有効
- 関連: ADR-125、ARCHITECTURE 2.7、DESIGN-install、#109
- 要旨: IR の形が同じでロジックだけが違う extractor を見分けるため、extractor のソースのハッシュを、本体と extractor の両方のビルドに埋める

**背景:** ADR-125 のスキーマのバージョンは、IR の形が変わったときだけ上がる。extractor のロジックだけを直した変更(形は同じ)では、古い extractor が検査を通り、結果が違うのに何も言われない。

**決定:**
1. extractor のソースのハッシュを、`internal/extractorhash` の 1 か所で計算する(`go run ./scripts/extractor-hash rust|dart`)。言語ごとのファイルの表(Rust: `build.rs`、`Cargo.toml`、`Cargo.lock`、`rust-toolchain.toml`、`src/`。Dart: `pubspec.yaml`、`pubspec.lock`、`lib/`、`bin/`)を、パスのバイト順に、パス・長さ・中身(CRLF は LF)の組で SHA-256 に畳み、先頭に表のバージョン(`v1`)を入れる。`test/`、`target/`、`.dart_tool/`、README などは入れない。表にもない除外にもないファイルが extractor のディレクトリに増えると、テストが落ちる。
2. 本体は、ビルドのときに `-ldflags "-X main.rustSourceHash=… -X main.dartSourceHash=…"` で持つ(`make build`、`scripts/build-release-archives.sh`)。素の `go build` は `unknown`。生成した定数ファイルは置かない(extractor を触る PR 同士の衝突と、更新し忘れを避ける)。
3. extractor は、自分のビルドのときのハッシュを、`--version` で答える。Dart は `rivus_dart rivus.ir/N source=<hash>`(`dart compile exe -DRIVUS_SRC_HASH=<hash>`)、Rust は `--rivus-version` で `rivus-rust rivus.ir/N source=<hash>`(`--rivus-version` は、ドライバーの判定や引数の解釈、cargo より前に、固定の文字列で答える。環境に `RIVUS_RUST_DRIVER` があっても答えは変わらない。`--version` はドライバーが rustc の探りに使うので触らない。`build.rs` が環境変数 `RIVUS_EXTRACTOR_SRC_HASH` を埋める)。`rivus setup rust` / `rivus setup dart` が、`-source` の中身から同じ計算をして渡す。
4. ハッシュは、IR の先頭行には書かない。形が変わらないので、`schema_version` は `rivus.ir/1` のまま。

**止めるか、記録するかの線(ADR-125 のとおり):** スキーマが違えば止める。ソースのハッシュが違うときは、止めずに記録する(段階 2 以降)。

**段階:** 段階 1(この ADR の変更)は、計算と埋め込みだけ。rivus は、extractor の `--version` をまだ呼ばず、照合も警告も `summary` への記録もしない。Rust の解析での取り方(解析のたびに extractor が書くファイルにするか、`rivus version` が隔離の外で `--rivus-version` を呼ぶか)は、段階 2 で決める(extractor の作業用ディレクトリは終了で消えるので、そのままでは rivus が読めない)。対象のビルドが書き換えうる経路は、IR そのものと同じ範囲に限る。段階 2 は、読む前に extractor の `--version` を 1 回呼んで本体のハッシュと照合し、警告と `summary.extractor` に出す。`--version` の呼び出しは、隔離の外で extractor を動かすことになるので、例外として、ユーザーの承認を待つ。

**帰結:** 結果は変わらない(古い extractor は、段階 1 では止まらない)。`rivus setup` で作った extractor と、`make build` で作った本体は、同じ計算のハッシュを持つ。
