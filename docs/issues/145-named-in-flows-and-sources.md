# 145: モデルの `flows` と `sources` に、名前つき引数を指す書式がない(`named:` は sink だけ)

- 状態: **open**
- 種類: 道具(重さ: 低)
- 言語: Dart

sink には `named: [kind]` がある(#139、`docs/MODELING.md` 4a-1 節)が、`flows` の `from` / `to` と `sources` の `params` は位置だけを指す。名前つき引数を通す関数(`f(a, {x})` の `x` が結果に流れる)は、宣言の順の位置で書くしかなく、オーバーライドが名前つき引数を足すと位置がずれる。
`internal/ir` の `Instr.ArgNames` を使う形で、`from: named:x` のような書式を足す。`flows` はサマリーの段階(呼び出しの位置ではなく、シンボルの宣言)で使われるので、sink のように呼び出しから位置を決める方法は使えない。callee の宣言の仮引数の名前を IR の `Func` が持つ必要がある。設計が先。
