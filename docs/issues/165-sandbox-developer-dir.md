# Rust: macOS のサンドボックスの下で、Xcode.app が選ばれていると、ビルドスクリプトのリンクが失敗する

- 状態: closed
- 種類: 道具・ベンチ
- 言語: Rust
- ROADMAP の節: -
- 守るもの: `TestSandboxDeveloperDirDecision`(`cmd/rivus/isolate_devdir_test.go`)

## 何が起きるか

`xcode-select -p` が `/Applications/Xcode.app/Contents/Developer` のとき、ビルドスクリプトのある Rust の crate をサンドボックスの下で解析すると、ビルドスクリプトのリンクが `xcrun` で失敗する(サンドボックスが Xcode.app を読ませないため)。`-isolation-env DEVELOPER_DIR=/Library/Developer/CommandLineTools` を足すと通るので、利用者が一件ずつ手で回避していた。

## 修正の方法

`cmd/rivus/isolate_devdir.go` の `decideDeveloperDir`。サンドボックスの読めるパスは広げない。

1. macOS 以外: 何も足さない。
2. `-isolation-env` に `DEVELOPER_DIR` がある: そのまま使う(`source: user`)。
3. `/Library/Developer/CommandLineTools` がある: `DEVELOPER_DIR` をそこに向ける(`source: default`)。SDK(`SDKROOT`)も、その開発ディレクトリで引く。
4. コマンドラインツールがなく、`xcode-select -p` が `.app` の中: 実行の前に、原因と修正の方法(`xcode-select --install`、または `-isolation-env DEVELOPER_DIR=<読めるパス>`)を言って止まる。
5. それ以外: 何も足さない。

渡した値は、`-v` で標準エラーに出て、`summary.isolation.developer_dir`(`value`、`source`)に残る。渡さなかったときは、`summary` は変わらない。

## Homebrew で入れた rivus も同じか

コードの上では、同じ。サンドボックスのプロファイルと、ビルドに渡す環境(ホストの環境を引き継がない)は、`rivus` のバイナリの中にあり、Homebrew の配布物がそれを変える箇所は見つからなかった。Xcode.app が選ばれている機械で、Homebrew で入れた rivus を実際に動かして確かめることは、していない(確認できず)。この修正は、バイナリの中の挙動なので、Homebrew のバージョンも、これを含むバージョンに上がれば修正される。
