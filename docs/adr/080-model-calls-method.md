# ADR-080: モデルの `calls:` に `method:`(既定のメソッドが、レシーバの同じ trait のメソッドを呼ぶ)

- 状態: 有効
- 関連: MODELING §3

**背景:** ADR-075 の限界 1: `out.write_all(buf)`(`out: &mut dyn Write`、自前の `impl Write`)は、標準の既定のメソッド `write_all` の呼び出しで、候補は実装の `write` でなく(`write_all` は上書きされていない)、`write_all` の本体はプログラムにない。`write` の中の sink が見つからなかった(zoo `dyn/stdlib-writer-own`)。`Iterator` の既定のメソッドも同じ形。

**決めたこと:** YAML の `calls:` の行に `method:` を足す(`fn` と、どちらか 1 つ)。「ライブラリの既定のメソッドが、レシーバに対して、同じ trait の、このメソッドを呼ぶ」。エンジン(`internal/taint/callrows.go` の `applyMethodRow`)は、プログラムの自前の実装(`<T as std::io::Write>::write`。依存は読まない: ADR-075 と同じ)を、レシーバと `with` で呼ぶ(呼び出しと同じ。結果、throw、書き込みは、そのメソッドのもの)。レシーバがその場で作られた 1 つの型と分かるとき(`&mut w` の `w`、trait object への変換を通して)は、その型の実装だけ。複数のときは `dynamic_dispatch` の不確実性。**位置は、レシーバを先頭に、すべてのオペランドを数える**(`arg0` がレシーバ。Rust のモデルの規約)。`ret`、`throw`、`writes`、`mode` は付けない。呼び出しグラフのエッジ(`callbackFuncs`)に、実装を入れた。標準の最初の行: `std::io::Write::write_all` → `write`(`with: [arg1]`)。harness(`bench/zoo-rust/callbacks`)が、`write` を記録する型で、本物の呼び出しと、行を照合する。

**確かめたこと:** zoo(Rust)で、`dyn/stdlib-writer-own` の vuln と decoy の本物の流れが見つかる(`@miss` を外した。decoy の別の writer の sink には届かない: レシーバの型で 1 つの実装に絞る)。rust-curated は変化なし。

**限界:** (1) 行は `write_all` だけ。`Read::read_to_string` → `read`、`Iterator` の既定のメソッド(`count`、`last`、`fold` が `next` を呼ぶ)などは、行を足す前に、harness の呼び出しと、`with` の位置(バッファの型の違い)を確かめる。(2) レシーバの型が分からないときは、実装の全部(`dynamic_dispatch` の不確実性が付く)。


