# #105 Rust: std::cmp::min / max が pure で、流れが落ちる

- 状態: **closed**
- 種類: 誤ったモデル(直した)
- 言語: Rust
- ROADMAP の節: 5
- 守るもの: 動物園 `F/conv/builtin-minmax`(Rust)

## 今の状態(2026-11-22 の見直し)

直した: ba0721c(Rust の `std::cmp::min` / `max` は、引数から結果を作る。`pure` ではない)。

## 経緯

2026-11-22 に、直してから記録した(「直ってから記録する」)。再現の詳細は書かない。

