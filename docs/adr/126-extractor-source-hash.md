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

**止めるか、記録するかの線(ADR-125 のとおり):** スキーマが違えば止める。ソースのハッシュが違うときは、止めずに記録する。

**読む側(段階 2): 解析の前に、extractor に 1 回だけ聞く。Rust は、対象を読む起動と同じ隔離の下で聞く。例外は作らない。**
5. Rust と Dart を読む解析の前に(`rivus analyze`、`index` など、`-lang` で選ばれた言語ごとに 1 回)、extractor に、自分のソースのハッシュを聞き、本体の値(`main.rustSourceHash`、`main.dartSourceHash`)と照合する。Go と、`-ir` で読む保存した IR は、聞く相手がない。
6. **Dart**: 実行ファイルの `--version` を、そのまま呼ぶ(Dart の extractor は、もともと隔離なしで動く。`graphkey.go` の `--version` の先例と同じ)。標準入力なし、20 秒で打ち切る(`dart run` で起動するコマンドは、起動が遅い)。
7. **Rust**: `--rivus-version` を、対象を読む起動と同じ仕組み(同じサンドボックスのプロファイル、環境を固定して組み立てる起動)で、1 回呼ぶ(`rustExtractorVersionSandboxed`)。ただし、対象のディレクトリ、cargo の置き場、取得、書き込み、ネットワークは、どれも要らないので、与えない。環境は `PATH`、`HOME`、`TMPDIR` だけ(`RIVUS_RUST_DRIVER` などは渡さない。`--rivus-version` は `main()` の先頭で固定の文字列を返すので、環境に影響されない)。読み取りはツールチェインの `lib`(`librustc_driver` にリンクされているため)と extractor だけ。作業ディレクトリは、空の一時ディレクトリ(`--rivus-version` という名前のファイルを、古い extractor がディレクトリ名として拾わないため)。標準入力なし、5 秒で打ち切る。`-unsafe-host-exec` のときだけ、利用者が隔離なしを選んでいるので、同じ引数と環境で直接起動する。**隔離のない環境(Linux で `-unsafe-host-exec` なし)では、呼ばない**。「未確認(この環境には隔離がない)」とする(その環境では、解析も、隔離なしには何も実行せずに止まる)。ADR-093 の「Rust の extractor は、隔離の下でしか動かさない」は、そのまま守られる(この呼び出しも隔離の下)ので、ADR-093 に例外を足さない。
8. **照合の結果は 4 つ(と、聞けなかった 2 つ)に分け、止めずに記録する**: `yes`(一致)、`no`(両方わかっていて違う)、`unknown_binary`(本体が `unknown`。素の `go build`)、`unknown_extractor`(extractor が答えない。古い extractor の `source=` なし、`--rivus-version` を知らず終了コード 2、`source=unknown`、呼び出しの失敗、打ち切り。理由を文で書く)、`unchecked`(隔離がなく、聞かなかった)、`n/a(-ir)`(保存した IR。1 件だけ、ハッシュなし)。対処が違うため、分ける: `no` と `unknown_extractor` は extractor を作り直す(`rivus setup rust|dart -source`)、`unknown_binary` は本体を `make build` で作り直す。本体と extractor の両方が不明のときは、本体の側を先に出す。
9. **記録**: `summary.extractor`(言語ごとの `{lang, path, schema, source_hash, expected_hash, match, detail}`。Go だけの解析にはない)と、`yes` 以外のとき標準エラーに警告 1 行(extractor のパスと作り直し方つき)。uncertainty の kind にはしない(ADR-008 の kind は、finding ごとの不確かさ用。これは、解析全体の前提の話)。`summary` の項目を足すだけなので、出力のスキーマのバージョン(`rivus.result/1`)は上げない(ADR-034 の、足すだけの変更の扱い。項目は `omitempty`)。
10. **`rivus version`**: 第 1 行は今までどおり(スクリプトが読む)。続けて、IR のスキーマのバージョンと、言語ごとの extractor の行(パス、スキーマ、ソースのハッシュ 12 桁、一致か不一致か)。Rust の行は、解析と同じ `rustExtractorVersionSandboxed`(隔離なしの環境では「未確認」。`-unsafe-host-exec` か `RIVUS_UNSAFE_HOST_EXEC=1` のときだけ直接起動)。スキーマが違って止まるときのメッセージ(ADR-125 の `SchemaError`)に、その extractor のパスを足す。

**段階(すべて済み。課題 #109 は closed):** 段階 1 は、計算と埋め込み(`--version` / `--rivus-version` の答え)。段階 2 は、上の 5 から 10(読む側の照合、記録、`rivus version`)。ベンチ(`deps_bench.py`、`compare.sh`)での利用は段階 3(下の 11)。

**ベンチと合格基準(段階 3、ユーザーの承認 2026-11-25): ここだけは、`yes` 以外を失敗にする。**
11. `bench/deps_bench.py`(`make bench-passline`)は、Rust と Dart の対象で、解析の報告の `summary.extractor` の `match` を読み、`yes` 以外(`no`、`unknown_binary`、`unknown_extractor`、`unchecked`、`n/a(-ir)`、項目がない場合)を、その実行の失敗にする(`extractor_mismatch(<match>)`、終了コード 1、台帳に行を足さない。状態ごとに作り替え方を出す)。ファイルの更新時刻で見ていた照合(#508)は、これに置き換えた(時刻に依らないので、`git` が使えなくても、rebase で時刻が動いても、誤判定しない)。`deps_bench.py` は、`-rivus` を指さないとき、`make build` と同じ値を付けて本体を作り、Dart の extractor は `dart run -DRIVUS_SRC_HASH=<hash>` で起動する。`compare.sh --graph-check` は、各側の本体を、その側の木の extractor のハッシュを付けて作り、extractor もその側のハッシュで作る。左右でハッシュが違う(extractor を変える PR)のは失敗にせず、各側が自分の extractor と合っていること(`match` が `yes`)を各側で確かめて、合わない側を NOT RUN(終了コード 2)にする。素の `go build` / `go install` の本体は、これまでどおり動く(警告が出るだけ)。失敗にするのはこの 2 つの道具だけで、「`make build` で作る」運用を前提にする。

**帰結:** 解析の結果(IR、finding、uncertainty)は変わらない。出力に `summary.extractor` が足される。古い extractor と違う extractor は、止まらずに、警告と記録が出る。`rivus setup` で作った extractor と、`make build` で作った本体は、同じ計算のハッシュを持つ。
