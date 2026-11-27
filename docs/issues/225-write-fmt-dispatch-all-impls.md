# #225 `write!` の `Write::write_fmt` が dynamic_dispatch で、プロジェクトの全 `impl Write` に飛び、zoo のセルが互いに干渉する

- 状態: **closed**
- 優先度: 中
- 種類: 解析(Rust のディスパッチの精度)
- 言語: Rust
- ROADMAP の節: -
- 守るもの: ある `impl Write` に書いた値が、別の `impl Write` の sink に届かないこと
- 進める完了条件: C(1)

## 何が起きるか

セル `dyn/iface-write-passed`(`@todo(PARITY)` のまま)を足して全体で走らせると、そのセルの source 3 つが、`stdlib_writer_own` 系のセルの sink に届く(`unexpected flow` 12 件、`must stay quiet` 2 件)。`ZOO_ALONE=1` では単独で PASS する。

`write!` の `Write::write_fmt` が dynamic_dispatch として扱われ、プロジェクトの全 `impl Write` に飛ぶため。実コードでは、誤検出の側の問題になる(ある `impl Write` に書いた値が、別の `impl Write` の sink に届く)。

## 今の状態(2026-11-27)

直した。レシーバが引数のメソッドの呼び出し(`applyMethodRow`)にも、interface 越しの呼び出しと同じ dispatch guard を付け、呼び出し側が渡した値の型で除く。`&mut b as &mut dyn Write` のような Rust の trait object への変換は、元の型を読む。型が std の型なら、プロジェクトの実装は除く。`F/dyn/iface-write-passed` のセルを足し、`@todo(PARITY)` を外した。ラッパー(`Box<ShellWriter>`)は #242。

## 直す方向の案(当時)

1. コアで、引数の実型でディスパッチを絞る(規模は中)。
2. またはセルを自前の trait にして形を変える(`write!` の形ではなくなるので、`@todo` を消す前に、PARITY として十分かを決める)。
