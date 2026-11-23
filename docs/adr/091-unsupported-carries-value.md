# ADR-091: 下げられなかった式が値を返すとき、extractor は結果と元の値を `unsupported` に持たせ、コアは元の値から結果を作る(相談役の判断、2026-11-22)

- 状態: 有効
- 関連: ARCHITECTURE の IR の命令
- 状態の詳細: 相談役の判断(2026-11-22)。Dart の解決できないコンストラクタ呼び出しから
- 要旨: Go と Rust の extractor は未確認

**背景:** Dart の extractor は、解決できないコンストラクタ呼び出し(`new C.nope(a)`)で、引数を下げたあと、結果を汚染のない定数(`nilConst`)にして済ませていた。`unsupported` は note だけを作り、値を作らないので、引数の汚染は結果へ流れず、流れは何も報告せずに消えた(note は全体の一覧にだけ残り、その流れの finding には付かなかった)。原則(PRINCIPLES 1 節、2 節)の「値を返す所で、汚染のない定数を返して済ませない」に反していた。

**決めたこと:** IR の `unsupported` は、引数(元になる値)と結果を持てる(検証 `validate.go` の個数を 0 固定から可変にした。既存の流れは 0 / 0 のままで変わらない)。値を作る構文が下げられなかったとき、extractor は、元の値を `args`、結果を `dsts` にして `unsupported` を出す。コアは、`args` の位置から `dsts` を作り(ほんとうより多く、少なくはない側)、note を流れの途中の `why` に付ける(通る finding は clear でなくなる)。元の値がない所(コンストラクタの tear-off、変数が見つからない、パターンの変数: 束縛する先の要素がない)は、これまでどおり `nilConst` と note で、全体の一覧にだけ残る。

**確かめたこと:** `cmd/rivus/dart_syntax_test.go` の `TestDartUnresolvedConstructorKeepsTheFlow`(直す前は finding 0 件、後は 1 件で clear でない)。dart-curated(114 件)と zoo-dart(163 件)の finding は、直す前後で同一。

**限界:** Go と Rust の extractor に、同じ形(下げられない式が値を返す所で、定数や nil を返している所)があるかは、まだ見ていない(Rust の表のとき)。

