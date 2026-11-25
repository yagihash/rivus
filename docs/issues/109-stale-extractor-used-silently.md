# #109 extractor(や、入れてあるイメージ)が古くても、rivus は何も報告せずに、その結果を出す

- 状態: **closed**
- 種類: 誤った結果を何も報告せずに出す(直した)
- 言語: Rust / Dart
- ROADMAP の節: 0 / 1
- 守るもの: 補足を参照
- 題: extractor(やイメージ)が古くても、何も報告せずに、その結果を出す
- 補足(種類): 誤った結果を何も報告せずに出す(uncertainty なし)
- 補足(言語): Rust / Dart(extractor を別に持つ言語)
- 補足(ROADMAP の節): 0 / 1(Homebrew のインストールの前に必要)
- 補足(守るもの): `TestIRSchemaVersionMatchesShape`、`TestExtractorsWriteTheSchemaVersion`、`internal/extractorhash` のテスト、`bench/test_deps_bench.py`、`bench/test_graph_check.py`

## 何が起きるか

rivus は、extractor(`rivus-rust`、`rivus-dart`、Docker のイメージの中の extractor)の**バージョンが、rivus 本体と合っているかを確かめずに**、IR を読む。2026-11-22 に、手元の Docker のイメージが古く(リポジトリの extractor でなく、前に作ったイメージの中の extractor)、`bench/zoo-rust` の finding が 298 件と出た(今の extractor では 174 件)。何も報告せずに、古い extractor の結果が出た。

## あるべき出力

extractor が自分のバージョン(rivus と同じ出所のバージョンの文字列、または IR の書式のバージョン)を答えるインターフェースを持ち、rivus は、使う前に、バージョンの一致を確かめる。合わなければ、**止まって、作り直し方を示す**(何も報告せずに使うことはしない)。IR の先頭のレコードに extractor のバージョンを入れ、`summary` にも出す(どの extractor で読んだ結果かが、出力に残る)。Dart と Rust の両方。Docker の経路は消すが、同じ抜けは、サンドボックスでも、Homebrew(`rivus setup rust` で作った extractor が、rivus 本体より古い / 新しい)でも残る。

## 直した内容

終わりの条件は、ADR-125 と ADR-126 の線引きで読み替えた。スキーマの違いは止める。ソースの違いは、止めずに記録して表示する。

- ADR-125(スキーマの違いは止める。PR #527): IR のスキーマのバージョン `rivus.ir/1`、読む側の `SchemaError`(作り直し方を示して止まる。Rust、Dart、`-ir`)、形のハッシュのテスト(列挙の値も)。
- ADR-126(ソースの違いは、止めずに記録して表示する):
  - PR #529: ソースのハッシュの計算(`internal/extractorhash`、`scripts/extractor-hash`)と埋め込み(本体は ldflags の `-X`、extractor 側の埋め込み)、Rust の `--rivus-version`、Dart の `--version`。
  - PR #537: `summary.extractor`、`rivus version`、`rivus version -expected-hash rust|dart`、隔離の下で `--rivus-version` を呼ぶ `rustExtractorVersionSandboxed`。
- ベンチの失敗(PR #539): `deps_bench.py` と `graph_check.py` が、`summary.extractor` の `match` を照合し、`yes` 以外を失敗にする。
- ソースの tarball(PR #536): Release のアセット `rivus_X.Y.Z_source.tar.gz`(attestation つき)と `bump-tap`。`yagihash/rivus-action` の対応は、rivus-action の PR #14(v0.0.19 以降が対象)で、準備済み。
- 確認(2026-11-25): v0.0.19 の Homebrew(`brew upgrade rivus`)で、`rivus version` が、Rust の extractor `source c61c23a4a906 (match)`、Dart の extractor `source 13b238ce08d7 (match)` を出した。`rivus version -expected-hash rust|dart` の値は、extractor の `--rivus-version` と `--version` の値と完全に一致した。

この課題とは別の話で、ユーザーの判断を待っているもの: tap の attestation の検証(`bump-tap` で検証する案がある)。

## 経緯

- 2026-11-22: Docker の基準を取り直すときに見つかった。相談役の判断で、課題に上げた。
- 2026-11-22(追記 ii): Homebrew では、formula の wrapper が `RIVUS_RUST_TOOLCHAIN_DIR` と `RIVUS_RUST_EXTRACTOR` を keg の中に固定するので、バージョンがずれる余地は小さい(DESIGN-install.md の 6a)。この課題が主に効くのは、`rivus setup rust` で作る人。
- 2026-11-25: IR のスキーマのバージョン(`rivus.ir/N`)を、形が変わるたびに上げるルールにし、形のハッシュのテストで強制した。読む側は、版が違う extractor のストリームを、作り替え方を示して止める(Rust、Dart、`-ir`)。止める条件はバージョンの不一致で、extractor のコミットは見ない。ロジックだけの変更は、バージョンでは捕まらない: extractor のソースのハッシュ(ADR-126)の計算と埋め込み(本体は `-ldflags -X`、extractor は `--version` / `--rivus-version` に出す)は入れた(2026-11-25)。`summary` に extractor のバージョンを出す(どの extractor で読んだ結果かが、出力に残る)ことと、バージョンを上げ忘れた extractor 側(Rust、Dart)の形の検査。 2026-11-25(段階 2): rivus が読む前に extractor に 1 回だけ聞き(Rust は、対象を読む起動と同じ隔離の下で `--rivus-version`。Dart は `--version`)、本体のハッシュと照合して、止めずに `summary.extractor`(`yes`、`no`、`unknown_binary`、`unknown_extractor` の 4 つを分ける。`-ir` は `n/a(-ir)`)と標準エラーの警告に記録し、`rivus version` に並べ、スキーマ違いで止まるメッセージに extractor のパスを足した。残り: Homebrew の formula の変更。バージョンを上げ忘れた extractor 側の形の検査は、`TestExtractorsWriteTheSchemaVersion` が行う。
- 2026-11-25(段階 3): ベンチと合格基準の道具が、`summary.extractor` の `match` が `yes` でない実行を失敗にするようにした(`deps_bench.py`: `extractor_mismatch(<match>)`、終了コード 1、台帳に行を足さない。`compare.sh --graph-check`: 各側の本体と extractor を、その側の木のハッシュで作り、合わない側を NOT RUN にする。左右でハッシュが違うのは失敗にしない)。更新時刻の照合(#508)は置き換えた。テストは `bench/test_deps_bench.py`、`bench/test_graph_check.py`(4 つの状態で失敗、`yes` で通る)。素の `go build` の本体は動く。判断: **closed にしない**。終わりの条件の「バージョンが合わない extractor を使おうとすると、止まって作り替え方を示す」は、`rivus` 本体では、ADR-126 の決定どおり止めずに警告と記録にしている(止めるのはスキーマの違いだけ)。止めるのはベンチと合格基準の道具だけ。Homebrew の formula の変更(keg の本体にハッシュを入れる)が残っているので、`partial` のまま。
- 2026-11-25(closed): 段階 1 から 3 と、Homebrew での確認が済み、closed にした。直した内容は上の節。
