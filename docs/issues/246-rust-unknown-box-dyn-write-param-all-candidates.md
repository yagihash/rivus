# #246 Rust: 引数で受けた、中身の分からない `Box<dyn Write>` を全候補で通す

- 状態: **open**
- 種類: 過検出
- 言語: Rust
- ROADMAP の節: -
- 守るもの: -(試したセルは、既存セルの `@fp` が要るので、入れていない。下の「試したセル」)

## 何が起きるか

`Box<dyn Write>` を `&mut dyn Write` にして関数に渡し、受けた側が `w.write_all(p)` を呼ぶとき、呼び出し側でその Box の中身の型が分からないと、プログラムの `impl Write` 全部が候補になる。中身は、呼び出し側が `heldType`(`internal/taint/heldtype.go`)で読めるときだけ決まる(#242 の修正)。決まらないのは、次のとき。

- Box が関数の外から来る(引数、フィールド、戻り値)。
- `Box<dyn Write>` に入れるときの `convert` を通る(`Box::new(Old)` の結果が `convert` を経て `store` される。`newType` は `convert` の値を見ない)。
- `mem::replace`、`mem::swap`、`*b = ...` する関数への `&mut b` など、読むだけでない使い方がある(`onlyRead` が false)。

その結果、source から、中身ではない別の `impl Write` の `write` の sink にも流れが届く。zoo の既存セルの間で、期待外の流れ(誤検知)が 20 件超出る。

## 試したセル

`mem::replace`、`mem::swap`、`*b = Box::new(Other)` する関数で `Box<dyn Write>` の中身を替えてから、`put(&mut b, ..)` で渡すセル `F/dyn/iface-box-replaced`(vuln、vuln2、vuln3、fixed、decoy)。置き場(リポジトリの外): `~/tmp/rivus-developer/issue242-out/iface_box_replaced.rs.txt`。

- 3 つの vuln は found(`@miss` にならない)。
- fixed と decoy を含む 5 つのケースの source から、既存セルの `@sink(none)` に期待外の流れが出る。例: `iface_box_replaced.rs:139`(decoy の source)から `iface_box_passed.rs:21`、`:67`、`stdlib_writer_boxed.rs:16`、`:66`。ほかに `stdlib_writer_own.rs`、`stdlib_writer_own_static.rs` の sink。

### 入れなかった理由

- `Box::new(Old)` を `Box<dyn Write>` に入れると IR に `convert` が入るので、`heldType` は "" を返す。#242 の「決めてしまって、本当の候補を落とす」ことは起きない。
- `Box<T>` の中身は、型の上で T のまま変えられない。`Box<ShellWriter>` を別の型に替える形は書けない(別の型に替えられるのは `Box<dyn Write>` だけ)。
- このセルを通すには、既存の 4 セル(`iface_box_passed`、`stdlib_writer_boxed`、`stdlib_writer_own`、`stdlib_writer_own_static`)の `@sink(none)` に `@fp` を足す必要がある。既存セルのマーク変更は advisor の判断が要る。

## 直す方向の案

- 受けた側(`put`)の引数が、呼び出し側で作られた Box のコピーなら、その中身(`convert` の前の `Box::<T>::new` の T)を、`convert` を通って読む。`mem::replace` などで書き換えられうるときは、決めない(`onlyRead`)。
- 呼び出し側のガード(`dispatchSpec.excludes`)が、`Box<dyn Write>` の中身を読めるようにする。
- #242 の PR(#657)で、決まるときは決まる。この課題は、決まらないときの過検出。
