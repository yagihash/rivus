# #242 Rust: レシーバが std の型(`Box<ShellWriter>`)のとき、`applyMethodRow` がプロジェクトの `impl Write` を呼ばず、流れが落ちる

- 状態: **open**
- 種類: サイレントな見逃し
- 言語: Rust
- ROADMAP の節: -
- 守るもの: zoo `F/dyn/stdlib-writer-boxed`
- 進める完了条件: C(1)

## 何が起きるか

プロジェクトの `impl Write`(`ShellWriter`)を `Box` に入れ、`&mut dyn Write` として渡して `write_all` で書くと、`ShellWriter::write` の sink に届く流れが見つからない。

`internal/taint/callrows.go` の `applyMethodRow` が、レシーバの型を `receiverType` で読み、`Box<ShellWriter>` のような std の型のとき、`isStdType` が真なので、何も呼ばずに戻る。std の型には std の実装があるという考えだが、`impl<W: Write + ?Sized> Write for Box<W>` は中の `W` の `write` を呼ぶので、プロジェクトの実装は包みの中にありうる。

設計メモ(候補の絞り)の I-3 で言う「唯一の落とす向き」である。ADR-118 (i)(絞りは、見逃しを増やさない)に反する既存の箇所。

## 守るもの

zoo のセル `F/dyn/stdlib-writer-boxed`(Rust だけのセル。vuln と decoy の source に `@miss(silent)`)。直したら、`@miss` を外す。

## 直す方向の案

std の型なら、その型の std の実装のモデルに任せる。プロジェクトの impl が包みの中にありうるなら(`Box`、`Rc`、`Arc`、`&mut`、`RefCell` などの型引数にプロジェクトの型があるとき)、全候補のまま(絞らない)にする。
