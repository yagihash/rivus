# ADR-069: グローバルを通る finding に、証拠 `shared_state`(書き込み側の数、読み出し側の数)を付ける(出力の追加だけ)

- 状態: 有効
- 関連: DESIGN-shared-state
- 状態の詳細: 087 が拡張
- 要旨: 出力の追加のみ

**背景:** グローバルを通る流れは、「誰が書いたものを、誰が読むか」の組を失い、書き込み側全員の和を読み出し側に返す(`docs/DESIGN-shared-state.md`)。finding は、グローバルを通ったこと(経路の `global_var` の注記)は示すが、そのグローバルに何か所が書いたかは示さなかった。読む人は、「1 か所が書いている」のか「100 か所の和」なのかを見分けられなかった(gogs の +47 のような過検出を見分けるための材料がなかった)。

**決めたこと(設計の段階 1):**
1. finding に `shared_state`(配列。グローバルを通った finding だけ。フィールドは `omitempty`)を足す。1 要素 = `place`(`kind: global`、`name`)、`path`(読んだフィールドのアクセスパス。なければ省く)、`writers`(`flow`: **その finding の source の要素を、そのグローバルのそのパスに書いた関数の数**、`all`: そのグローバルに直接書く関数の数(`writersOf`)、`test`: `flow` のうち、ファイル名がテストのもの)、`readers`(そのグローバルのセルを読む・書く関数の数。グローバルが育つと再解析のマークを付けられる関数)。
2. `flow` は、`globalStore` に、要素ごとの書いた関数の集合(`writerFns`)を持たせて数える(今まである「最初に書いた関数の段」(`writers`)は、そのまま。経路の始まりに使う)。`commitGlobals` が、整列した順に書き込みを処理するので、決定的(集合の大きさだけを出す)。`global_var` の注記は、グローバルとパスを持つ(出力には出ない)。
3. `test` は、ファイル名だけの証拠(`_test.go`、`_test.dart`、`test/`、`tests/`)。**何も除かない**(範囲を絞る段階は別。IR のマークと、シグネチャの証拠は、まだ)。`excluded_test` と `first_writer` は、出さない(後者は、どれが最初かに意味がない。ADR-049)。
4. **出力の追加だけ**: finding の集合、経路、指紋は変わらない。`rivus.result/1` のバージョンは上げない(ADR-034 の、足すだけの変更の扱い)。

**確かめたこと(`compare.sh 7faf20e 6a5e478`、軽い層):** finding の追加 0、削除 0、経路・到達性・`other_sources` の変化 0(全 16 件の対象)。`shared_state` が付いた finding: gogs 2 つ 10 件ずつ(すべて `conf.Database` のフィールド。`flow` 1、`all` 2、`readers` 8)、gin-vue-admin 2 件(`injectionPaths`、1/1)、go-curated 4、go-scale 5(`flow` 1、`all` 1〜16)、zoo-go 14、zoo-dart 6。gophish、casdoor、dart-pub、parseable、rust-curated には、グローバルを通る finding がない(0)。`flow` > `all` のものは、なかった。zoo の 3 つ(`global-chain` で収束しない)は、実行のたびに `uncertainties` が 87〜91 で揺れる(base でも同じ。この変更とは関係がない)。

**限界:** `readers` は、読む関数と書く関数の和。複数のグローバルを通る finding は、配列で全部を出す。粒度の改善(持ち上げ、マップのキー)と、範囲の絞り込み(テストのマーク)は、別の段階。


