# 160: Rust の extractor が、`google-cloud-auth` 1.19.0 を読むときに rustc の未実装(async closure)で落ちる(`-no-deps` では動く)

- 状態: **open**
- 種類: 道具(Rust の extractor)
- 言語: Rust
- ROADMAP の節: 3
- 守るもの: -
- 題: Rust の extractor が、`google-cloud-auth` 1.19.0 を読むときに rustc の未実装(async closure)で落ちる(`-no-deps` では動く)。落ちた理由も利用者に伝わらない
- 補足(種類): 道具(Rust の extractor。依存込みの解析が全部止まる。重さ: 中〜大。優先度: 高め)
- 見つけた経緯: `google-cloud-storage` 1.22.0 の組み込みモデル(`internal/model/builtin/rust-google-cloud-storage.yaml`)を作るとき

## 再現(公開のクレートの名前とバージョンだけ)

`google-cloud-storage = "=1.22.0"` に依存する最小のクレート(`src/lib.rs` で `google_cloud_storage::client::Storage` を使う)を作り、`cargo generate-lockfile` して、macOS のサンドボックスの下で実行する。この機械では、`xcode-select` が Xcode を指すので `-isolation-env DEVELOPER_DIR=/Library/Developer/CommandLineTools` も足した(別の話。下の「別件」)。

```
rivus debug extract -lang rust -isolation-env DEVELOPER_DIR=/Library/Developer/CommandLineTools -C <クレート>
rivus analyze -lang rust -isolation-env DEVELOPER_DIR=/Library/Developer/CommandLineTools -C <クレート>
```

## 落ち方(2026-11-23 に確認)

- 終了コード 1(`debug extract` と `analyze` のどちらも)。標準出力は空(IR も、報告も、`summary` も出ない)。
- 標準エラーに、依存の取得の経過のあと、次が出る。
  - `thread 'rustc' panicked at .../compiler/rustc_public/src/unstable/convert/stable/ty.rs:484:17: not implemented: FIXME(async_closures): Lower these to SMIR`
  - `error: could not compile `google-cloud-auth` (lib)`(cargo の行)、続けて、extractor の rustc の長い起動行(数 KB)
  - `rivus-rust: cargo check failed (exit status: 101): the code has to compile`
  - `rivus: the extract stage under the sandbox failed (exit 1)`
- `-no-deps` を付けると、終了コード 0 で、プロジェクトの関数だけが出る(依存を読まないので、`google-cloud-auth` の rustc を通らない)。
- 固定している nightly: `extractor/rust/rust-toolchain.toml` の `channel = "nightly-2026-11-15"`(rustc の commit は、パニックの出力の `5f4effd5...`)。`google-cloud-auth` は edition 2024 で、async closure(`async || ...`)を使っている。stable MIR(`rustc_public`)の変換が、async closure の型を未実装(`FIXME(async_closures)`)のままパニックする。

## 利用者に伝わっているか

**伝わっていない(部分的)。** 「どのクレートで」は、cargo の `could not compile `google-cloud-auth``の 1 行だけで、数 KB の rustc の起動行に挟まれて埋もれる。「なぜ」は、rustc のパニックの 2 行(`not implemented: FIXME(async_closures)`)だけで、rivus の言葉の説明がない。rivus 自身の最後の行は `the code has to compile`(コードがコンパイルできる必要がある)で、**利用者のコードが悪いと読める**(実際は、extractor の未実装)。`-no-deps` で回避できることも、案内されない。落ちたあとに、`summary` も、部分の結果も残らない。

## 影響

Google Cloud の Rust のクライアント(`google-cloud-*` 系)は、認証に `google-cloud-auth` を通るので、どれを使っても、依存込みの解析が止まる(`google-cloud-storage` 1.22.0 で確認。ほかのクレートは未確認)。ほかの async closure を使う依存でも、同じ形で止まるはず。モデル(組み込みの sink の行)は、依存を読まない `-no-deps` でだけ使える形になる。クレートの本体を読む照合(差分テスト、`rivus models coverage`)は、Rust では対象外のまま、このクレートでは依存込みでも動かせない。

## 別件(同じ確認で分かった)

macOS で `xcode-select -p` が `/Applications/Xcode.app/...` を指す機械では、サンドボックスの下で build script のリンクが失敗する(`cc` が `xcrun` 経由で Xcode の `libxcrun.dylib` を読もうとして、Seatbelt が拒否する。プロファイルは `/Library/Developer/CommandLineTools` だけを許す: `cmd/rivus/isolate_sandbox_darwin.go`)。`-isolation-env DEVELOPER_DIR=/Library/Developer/CommandLineTools` で回避できる。プロファイルが Xcode も許すか、`DEVELOPER_DIR` を既定で渡すかは、別に決める。

## 直す方向の候補

1. **async closure を扱える nightly に上げる**(`rustc_public` が `FIXME(async_closures)` を実装した日付)。固定の nightly の更新は、extractor の IR が変わりうるので、Rust の zoo と公開のベンチの前後を測る。いちばん素直。
2. **その型の変換だけ、パニックを避ける**(extractor の側で、変換できない関数を「本体を読まなかった」として飛ばす。IR に `body_not_read` の形がある)。依存の 1 関数で全体が止まらなくなる。ただし、飛ばした関数を通る流れは、見逃しになるので、必ず `summary` に出す。
3. **`body_replaced_packages` で `google-cloud-auth` を宣言する案**(ADR-105)。段階 4 の「extractor に宣言を渡す」は、Rust ではまだ(`docs/DESIGN-package-models.md` 7a 節: Rust は「まだ」)。宣言のパッケージを extractor が読まない形(`cargo check` の rustc の wrapper が、そのクレートを `--emit=metadata` だけで通す)が要る。そのクレートも rustc を通る(型検査)ので、パニックが MIR の変換の前に起きるなら効く。起きる場所の確認が先。
4. **利用者への伝え方を直す**(どの案でも別に要る): 失敗したクレートの名前と「extractor が未対応(rustc の未実装)」を、rivus の言葉で 1〜2 行にまとめて出し、`-no-deps` を案内する。`the code has to compile` は、extractor が落ちた場合には出さない。
