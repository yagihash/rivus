# #200 Rust: `RIVUS_RUST` の値が空白で分けられるので、空白を含むパスを直接指すと失敗する

- 状態: **closed**
- 種類: 道具・ベンチ(環境変数の読み方。直した)
- 言語: Rust
- ROADMAP の節: -
- 守るもの: `cmd/rivus/extractor_command_test.go`(`TestSplitCommand`、`TestSplitCommandExistingPathWithSpaces`、`TestExtractorCommandWithSpaces`)

## 何が起きるか

extractor のコマンドを表す値は、`cmd/rivus/lang.go` で `strings.Fields` を使って語に分けられる(最初の語が実行ファイル、残りが引数)。そのため、`RIVUS_RUST` に、空白を含むパス(macOS の `Application Support` など)を直接入れると、パスが途中で切れ、実行ファイルが見つからずに失敗する。

## 回避

空白を含まない場所へのシンボリックリンクを作り、そのリンクのパスを指す。

## 修正の案

- 値に引用符を使えるようにして、引用符で囲んだ中の空白は分けない
- または、`RIVUS_RUST` は実行ファイルのパスだけにして(分けない)、引数は別の変数で渡す

## 補足

同じ分け方を使う、ほかの extractor のコマンドの値(`-rust-extractor`、`RIVUS_RUST_EXTRACTOR` など)も、同じ形で影響を受けるかどうかを、修正するときに確かめる。

## 今の状態(2026-11-24)

直した: 9ee80657(`splitCommand`)。`-rust-extractor`、`-dart-extractor`、`RIVUS_RUST_EXTRACTOR`、`RIVUS_DART_EXTRACTOR` は、どれも `extractorCommand` を通るので、同じ規則で読まれる。

- 値が、空白を含み、そのまま存在するファイルを指すなら、1 語(引用符なしで直接指せる)
- そうでなければ、空白で分け、`'...'` と `"..."`(中の `\"` と `\\` は文字)で囲んだ中は分けない。空白、引用符、バックスラッシュの前のバックスラッシュは、その文字を語の一部にする
- 引用符が閉じていなければ、エラーにする
- 引用符もバックスラッシュも、空白を含む存在するパスもない値は、今までと同じに分かれる(後方互換)

`RIVUS_RUST` と `RIVUS_DART` は、テストとベンチだけが読む値で、extractor の判定(`extractorCommand`)は読まない。`RIVUS_RUST` は、テストが 1 つのパスとして直接実行するので、空白を含んでいても分けられない。`RIVUS_DART` を `strings.Fields` で分けるテストは、そのままにした(`bench/go-synth/measure.py` は `shlex.split` を使う)。
