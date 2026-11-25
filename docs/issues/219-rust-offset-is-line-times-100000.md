# #219 Rust: extractor の位置の offset が実バイト位置でなく `行 * 100000 + 桁` で、64 ビットの offset(`cpos`)を要する

- 状態: **open**
- 優先度: 低
- 種類: 道具(重さ: 低)
- 言語: Rust
- ROADMAP の節: -
- 守るもの: `TestRustStepsPastLine21475KeepTheirText`、`TestTheStepsOfAPathKeepALargeOffsetAndItsText`
- 題: Rust の extractor の位置の offset が実バイト位置でない(`cpos` の offset は 64 ビット)

## 何が起きるか

Rust の extractor は、位置の offset を `行 * 100000 + 桁` で作る(`extractor/rust/src/irjson.rs`)。約 21,475 行目から 32 ビットに収まらない。`internal/taint` の位置の詰め物(`cpos`)は、これを 32 ビットに切っていたので、経路の手順の offset が負になり、手順のスニペットが消えた(finding 本体の位置、id、fingerprint は影響を受けない)。`cpos` の offset を 64 ビットにして修正した(構造体は 20 から 24 バイト)。

## 影響の範囲

Rust の、約 21,475 行を超えるファイルの位置。見逃しはない。修正後は、`cpos` が 4 バイト大きいことだけが残る。

## 見直す条件

次のどちらかのとき、extractor が実バイト位置を出す案(offset が 32 ビットに収まり、`cpos` の offset を 32 ビットに戻せる)を見直す。

- Rust の extractor が実バイト位置を出せるようになったとき。
- `cpos` のメモリが、さらに問題になったとき。
