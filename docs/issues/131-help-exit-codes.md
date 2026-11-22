# 131: `rivus setup rust -h`、`rivus models -h`、`rivus setup -h` の終了コードが 1(usage のエラー)

- 状態: **closed**
- 種類: 道具(直した)
- 言語: 全
- 見つけた経緯: 利用ガイド(docs/AGENT-GUIDE.md)を書くときに、help とコードを読んで見つかった食い違い。v0.0.4 には入れない。

ほかのコマンドの `-h` は終了コード 0 だが、`rivus setup rust -h` は `flag: help requested` を出して 1、`rivus models -h` と `rivus setup -h` も usage のエラー(1)で終わる。`-h` は 0 で、usage を標準出力か標準エラーに出す形にそろえる。

## 直した内容

- `-h` は、どのコマンドでも終了コード 0 で、エラーの文を出さず、使い方だけを出す(`-help-all` つきも同じ)。`rivus setup rust -h` は、flag の `ErrHelp` を `main` がエラーとして出していたので、`main` で `ErrHelp` を静かに 0 で終わらせる。
- `rivus models` と `rivus setup` に、使い方(`modelsUsage`、`setupUsage`)を足した。`-h` は標準出力に出して 0、引数なしと知らないサブコマンドは標準エラーに出して 2(`rivus graph`、`rivus debug` と同じ)。
- `rivus extract`(旧名)は、案内を出して 2 のまま。
- 守るもの: `cmd/rivus/flagsides_test.go` の `TestHelpExitsZeroEverywhere`。
