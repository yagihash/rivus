# #109 extractor(や、入れてあるイメージ)が古くても、rivus は何も報告せずに、その結果を出す

- 状態: **open**
- 種類: 誤った結果を黙って出す(痕跡なし)
- 言語: Rust / Dart(extractor を別に持つ言語)
- ROADMAP の節: 0 / 1(Homebrew のインストールの前に要る)
- 守るもの: まだない(直すときに、版の不一致で止まる試験)

## 何が起きるか

rivus は、extractor(`rivus-rust`、`rivus-dart`、Docker のイメージの中の extractor)の**版が、rivus 本体と合っているかを確かめずに**、IR を読む。2026-11-22 に、手元の Docker のイメージが古く(リポジトリの extractor でなく、前に作ったイメージの中の extractor)、`bench/zoo-rust` の finding が 298 件と出た(今の extractor では 174 件)。何も報告せずに、古い extractor の結果が出た。

## あるべき出力

extractor が自分の版(rivus と同じ出どころの版の文字列、または IR の書式の版)を答えるインターフェースを持ち、rivus は、使う前に、版の一致を確かめる。合わなければ、**止まって、作り直し方を示す**(黙って使わない)。IR の先頭のレコードに extractor の版を入れ、`summary` にも出す(どの extractor で読んだ結果かが、出力に残る)。Dart と Rust の両方。Docker の経路は消すが、同じ穴は、サンドボックスでも、Homebrew(`rivus setup rust` で作った extractor が、rivus 本体より古い / 新しい)でも残る。

## 終わりの条件

版が合わない extractor を使おうとすると、止まって作り直し方を示す試験が、Rust と Dart にある。`summary` に extractor の版が出る。

## 経緯

- 2026-11-22: Docker の基準を取り直すときに見つかった。相談役の判断で、課題に上げた。
- 2026-11-22(追記 ii): Homebrew では、formula の wrapper が `RIVUS_RUST_TOOLCHAIN_DIR` と `RIVUS_RUST_EXTRACTOR` を keg の中に固定するので、版がずれる余地は小さい(DESIGN-install.md の 6a)。この課題が主に効くのは、`rivus setup rust` で作る人。
