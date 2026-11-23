# #174 Rust: 別名で得たポインタが、sink や呼び出しの引数へそのまま渡るとき、`pointer_alias` の注記が付かない

- 状態: **open**
- 種類: 過検出(注記の欠け)
- 言語: Rust
- ROADMAP の節: -
- 守るもの: -

## 何が誤って報告されるか

`pointer_alias`(ポインタ越しの書き込みを別名で近似した所を通った)の注記は、別名が作った書き込み・読み出しのステップ(`store`、`field_store`、`load`、`field_load`、要約の `mirror`)にだけ付く。別名で場所が増えたポインタを、読み出しも書き込みもせず、sink の引数や、呼び出しの引数へそのまま渡すと、ステップができないので、注記が付かない。別名のせいで流れがあると言っている finding なのに、`no_uncertainty_on_path` と出うる。

## 原因

注記はステップ(ホップ)に付ける。引数として場所を渡すだけの所は、ホップを作らない(`internal/taint/ptrcells.go` の `ptrAliasLocs` は、store と load の所でしか使っていない)。

## 修正の案

引数の場所を集める所で、`ptrAliasLocs` に入る場所があれば、その呼び出し(sink)のステップに注記を付ける。優先度: medium。
