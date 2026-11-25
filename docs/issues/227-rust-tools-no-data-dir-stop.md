# #227 Rust の実行の道具が、`RIVUS_RUST_DATA_DIR` を指していないとき、共有の場所を使わずに止まる

- 状態: **open**
- 優先度: 中
- 種類: 道具・ベンチ
- 言語: Rust
- ROADMAP の節: -
- 守るもの: 作業者の実行が、ユーザーの共有の場所(`~/Library/Application Support/rivus/`)に何も書かないこと
- 進める完了条件: -

## 何が起きるか

rivus の Rust の解析は、`cargo fetch` の置き場(`rivus-cargo-<ハッシュ>`)などを、`~/Library/Application Support/rivus/rust` に作る。`RIVUS_RUST_DATA_DIR` で別の場所を指せる。作業者が `RIVUS_RUST_DATA_DIR` を指さずに解析を実行すると、ユーザーの共有の場所にディレクトリが作られる(共有の場所には書かない規則に反する)。検証の実行で 1 回起きた。

## 現状(コードで確かめたこと)

- 読む場所: `cmd/rivus/isolate_sandbox_darwin.go:28-36` の `rustDataDir()`(darwin)と、`cmd/rivus/datadir_other.go:13-22`(それ以外)。`RIVUS_RUST_DATA_DIR` があればそれ、なければ `os.UserConfigDir()` の下の `rivus/rust`(darwin は `~/Library/Application Support/rivus/rust`)。
- 使う場所: `isolate_sandbox_darwin.go:134`(取得の置き場)、`lang.go:293`(`rivus setup rust` が作った extractor の探索)、`cache.go:229`、`cache.go:248`、`setup.go:45`。
- 指している: `cmd/rivus` のテスト全体(`cli_test.go:29` の `TestMain` が一時ディレクトリを指す。`zoo_rust_test.go` は同じパッケージなので、これに含まれる)、`extractor_command_test.go:84`、`cache_test.go:229`。
- 指していない: `RIVUS_RUST_DATA_DIR` を設定する道具は、ベンチ側には無い。一部が `HOME` を空のディレクトリにして避けている(`bench/graph_check.py:138-142` の `rust_env`、`bench/deps_bench.py:690-692` の `passline_run` の `home` 引数。後者は引数が無ければ避けない)。`bench/model-diff/rustspelling.go:690` は `rivus debug extract -lang rust` を環境のまま実行する。`Makefile` の `make test` / `make maturity-ledger` は、`go test` の `TestMain` を通る部分だけが守られる。

## 直す方向の案

ベンチや zoo の道具(`bench/`、`Makefile`)が、`RIVUS_RUST_DATA_DIR` を指していない(かつ `HOME` も空にしていない)とき、共有の場所を使わずに止める(エラーで、指す方法を表示)。利用者の通常の `rivus analyze` は、今までどおり共有の場所を使う(変えない)。テストでは、`RIVUS_RUST_DATA_DIR` をテストの一時ディレクトリに向ける補助にする案もある。

## 小さな PR で直せる見込み

見込みあり。`bench/` の Python の道具に、環境を整える共通の関数を 1 つ足して、`deps_bench.py`、`graph_check.py`、`model-diff` から使うだけで、`cmd/rivus` の本体は変えない。直す PR は別。
