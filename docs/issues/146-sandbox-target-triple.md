# 146: Rust の出力に、読んだ構成(target triple)が出ない

- 状態: **open**(ADR-093 の弱さの 1 つ。DESIGN-macos-sandbox.md の SBX-7)
- 種類: 注記つきの限界
- 言語: Rust
- ROADMAP の節: 0
- 守るもの: -
- 題: Rust の出力に、読んだ構成(target triple)が出ない(macOS のサンドボックスは macOS 向けの分岐を読む)
- 補足(種類): 注記つきの限界(見逃しの向きの変化)

## 今の状態

Docker(Linux)から macOS のサンドボックスに替えたので、`cfg(target_os)` と `build.rs` の分岐は、macOS の側を読む(テスト用の crate `extractor/rust/test/cfg_os` で確かめた)。Linux の側にだけあるコードは、範囲の外になる。出力は、どの構成を読んだかを言わない。

## 終わりの条件

出力(`summary` のどこか)に、読んだ構成の target triple を出す。`--target` を指定して別の構成の型検査ができるかを調べ、できるなら、選べるようにする。守るものとして、`cfg_os` を使うテストを足す。
