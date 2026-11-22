# 147: ワークスペースの中の crate が、サンドボックスの下で抽出できない

- 状態: **open**(ADR-093 の限界。DESIGN-macos-sandbox.md の 3.7c)
- 種類: 利用者を止める(閉じる向き)
- 言語: Rust
- ROADMAP の節: 0

## 今の状態

祖先のディレクトリに `Cargo.toml` があり、cargo がワークスペースを探す crate は、サンドボックスが祖先を読ませないので、`failed searching for potential workspace` で落ちる。祖先の `Cargo.toml` を名指しで許すだけでは足りなかった(`.cargo/config.toml` の探索などが、許可の外の読み取りで失敗する)。関連: #120(祖先の `.cargo/config.toml`)。

## 終わりの条件

ワークスペースの中の crate を、(a) 祖先の必要なファイルだけを、名指しで読み取り専用に許して抽出する、(b) 対象をワークスペースのルートにするよう案内して止まる、のどちらかにする。判断は相談役。#120 と同じ仕組みで解く。
