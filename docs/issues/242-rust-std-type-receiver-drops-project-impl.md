# #242 Rust: レシーバが std の型(`Box<ShellWriter>`)のとき、`applyMethodRow` がプロジェクトの `impl Write` を呼ばず、流れが落ちる

- 状態: closed
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

## 試したこと(2026-11-27、#225 の修正のとき)

`Box`、`Rc`、`Arc`、`RefCell`、`Mutex`、`BufWriter` などのラッパーのときは、全候補のままにする変更を試した。抽出器の型は型引数を落とす(`std::boxed::Box`。`Box<ShellWriter>` と `Box<FixedWriter>` が区別できない)ので、`stdlib-writer-boxed` の `@miss` 2 件は見つかるようになるが、同じプロジェクトの `impl Write` 全部に流れ、期待外の流れが約 21 件増える(boxed、own、own_static のセルの sink)。既存セルのマークの変更が要るので入れていない。絞るには、型引数を抽出器が残すか(`Box::<ShellWriter>::new` の呼び出し先の ID には型が出ている)、Box の中身を値の流れでたどる必要がある。

## 直した(2026-11-27)

- 抽出器と IR は変えていない(`rivus.ir/1` のまま)。型引数は IR の型には無いが、`Box::new` などの呼び出し先の ID(`std::boxed::Box::<T>::new`)にあるので、Box、Rc、Arc、RefCell、Mutex、RwLock、BufWriter、LineWriter をその場で作って `store` した値の型を、エンジンが読む(`internal/taint/heldtype.go` の `heldType`)。
- `receiverType` が、ラッパーのときは中身の型を返す。`applyMethodRow` の impl の絞りと、呼び出し側のガード(`dispatchSpec.excludes`)の両方が、それで絞る。
- 中身が分からないラッパー(関数の外から来た `Box` など)は、全候補のまま通す(`stdLeafType` の型だけ落とす。ADR-118 (i))。
- zoo: `F/dyn/stdlib-writer-boxed` の `@miss` を外した。セル `F/dyn/iface-box-passed`(`Box` を `&mut dyn Write` で引数に渡し、`w.write_all(p)` を直接呼ぶ)を足した。
