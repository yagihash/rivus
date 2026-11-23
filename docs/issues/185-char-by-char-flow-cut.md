# #185 全: コードが文字を 1 つずつ読み書きすると、流れが切れる

- 状態: **open**
- 種類: サイレントな見逃し
- 言語: 全
- ROADMAP の節: 2
- 優先度: 高
- 守るもの: zoo `F/conv/char-*`(Go、Dart、Rust の 3 言語に同じ名前のセル)
- 題: コードが文字を 1 つずつ読み書きすると、流れが切れる

## 何が見逃されるか

文字列を 1 文字(1 バイト、1 コードユニット)ずつ読み、1 文字ずつ書いて作り直すコードを通ると、汚染が切れる。字句解析(lexer)、文字の置き換え、大文字・小文字の変換、自前の hex と base64 のエンコーダが、この形。ライブラリの内側(依存の中のパーサなど)にあっても同じで、その先の sink への経路が出ない。

見逃しは、出力に何も残らない(サイレント)。

## 原因

数値の型の値は汚染を持たない、という既定がある(`numeric_stop`)。1 文字を表す型(Go の `byte` と `rune`、Dart のコードユニット、Rust の `char` と `u8`)は、この既定では数値で、汚染を持てない。文字列の丸ごとの変換と切り出し(`[]rune(s)`、`string(rs[1:])`)は汚染を保つが、1 文字を取り出した値は保てない。

`numeric_stop` の記録は、関数にパラメータ経由でだけデータが来る場合(サマリの経路)を含まない。そのため、この見逃しには注記が付かない。

## zoo で固定した今日の結果

セルは `F/conv/char-<形>`(lexer、replace、case、reverse、hex、base64、index、append、range-write、rune-slice)。各セルは 脆弱 / 修正済み / デコイ。脆弱とデコイの流れを、エンジンを変えずにそのまま記録した。

- Go: 10 形のうち、見つかるのは 2(`index` の `string(s[0]) + s[1:]`、`rune-slice` の `[]rune(s)` の丸ごと変換と切り出し)。見つからないのは 8(lexer、replace、case、reverse、hex、base64、append、range-write)で、`@miss(silent)`。
- Dart: 10 形のうち、見つかるのは 3(`codeUnits` を使う `reverse`、`s[i]` を使う `index`、`runes` の丸ごと変換の `rune-slice`)。見つからないのは 7(`codeUnitAt` と `runes` の要素を読む形、`codeUnits` から始まる hex と base64 も、復号の側で切れる)で、`@miss(silent)`。
- Rust: 10 形のうち、見つかるのは 4(replace、reverse、index、rune-slice)。見つからないのは 6(lexer、case、hex、base64、append、range-write)。Rust は注記 `unmodeled_library_call` が付くので、`@miss(unmodeled_library_call)`。

Rust は、`char` と整数を数値と扱う(extractor の型の分類)ので、Go と同じ原因だと読んで見積もった。セルは、実際の結果を記録した。Rust で付く注記が、この原因の説明になっているかは、確かめていない。

## 直すときの方針

- 文字を表す型を、数値の既定の外に置くか、文字列から取り出した要素に由来を示すマークを持たせる。どちらにするかは、設計で決める。
- 文字から作った数値(`n*10 + int(r-'0')` のような「数に解釈した値」)は、今までどおり止める。
- 直したら、セルの `@miss` を外す(エンジンが直ったときだけ外せる)。
- 直す前でも、止まった場所を出力に残す: `summary.numeric_stops` を足した(関数ごとに数える。サマリの経路の分も、止まった関数に数える)。

## 出力での見つけ方

`summary.numeric_stops.char_reads` は、文字の取り出し(`s[i]`、`range s`、rune、`codeUnitAt`)で汚染が止まった回数で、`top[]` に回数の多い 10 関数(`function`、`scope`、`count`)が出る。その関数は、文字を 1 つずつ扱っている。期待した sink が出ない(`-no-deps` では出る)ときは、まずその関数の周りを見る。`other`(長さ、カウンタ、変換、比較)は仕様どおりなので、数だけを見る。数える形は言語で違い、**数えない形は 0 のままで、止まっていないとは限らない**: Dart のコールバックに渡る要素(`runes.map((r) => ...)`、セル `char-replace`)と、Rust の `bytes()`・`chars()`・`as_bytes()[i]`・`char` 型以外の byte の取り出しは数えない(Rust の byte の取り出しは数えない形がある: #185)。セルでの確認は `TestZooCharReadsDart`、`TestZooCharReadsRust`。プロジェクトの関数に文字の取り出しがあれば、標準エラーに `N functions stopped a flow at per-character reads: summary.numeric_stops` が出る。

## 経緯

- 2026-11-23: 依存の中のパーサが、ユーザーの文字列を 1 文字ずつ読んで、その先の流れを失っていた件から見つかった。zoo に、この形のセルが無かった。

## 追記(2026-11-23): numeric_stops の確認から

### 実例(memos、公開のプロジェクト、00fcd170)

`summary.numeric_stops` の `char_reads` は 4967 件、760 関数。上位は、`gopkg.in/yaml.v3` の字句解析(`yaml_parser_fetch_next_token` の 407 など)、`yuin/goldmark` の構文解析(`parseBlock` の 150 など)、`tidwall/pretty`。YAML や Markdown の解析を通る流れが、依存の本体の中で、1 文字ずつ読む処理で切れている、という「どこで切れるか」の実例になる。ここにはパッケージ名と件数だけを書く。

数は、場所の数ではなく止めた回数で、関数の中の反復で同じ場所が複数回入る。大小の比較に使う目安で、407 は 407 か所ではない。

### 方針(相談役の判断)

文字の汚染の本体(由来の bit)を実装するときは、`numeric_stops` の `isCharRead` を、流れの判断には使わない。`isCharRead` は、言語に固有の名前を決め打ちで持つ(`std::iter::Iterator::next`、`std::str::Bytes`、`String.codeUnits`、`::as_bytes`、`iter.next`、型の名前 `byte` / `rune` / `char`)。「文字列の要素の取り出し」は、extractor が IR に付ける(命令のマークか、値の種類)。そのとき、`isCharRead` も IR のマークを見る形に置き換える。コアは IR だけを見る、という分担に戻す。

### IR の `CharRead` への置き換えの途中(結論。2026-11-24)

IR の命令に目印 `CharRead`(`char_read`)を足し、Go の extractor が付ける(`s[i]` の文字列の添字と、`range s` の `iter.next_string`)。コアの `isCharRead` は 2 ステップで、目印があれば真、なければ今までの判定(型の名前と、言語に固有のシンボルの名前)。2 ステップの間、フラグが off の出力(`numeric_stops` を含む)は、3 言語とも同一。

残りの仕事(必ずやる):
- Dart の PR と Rust の PR で、それぞれの extractor が `CharRead` を付け、コアの中の言語に固有の名前(`charReadSymbols`、`iter.next`、`std::iter::Iterator::next` とその型の名前、`isTextSequence` の名前)を消す。型の名前(`byte`、`rune`、`char`)で当たる値を、目印にするか、型の分類にするかは、その PR で決める。
- 流れの判断(`-char-taint` で運ぶかどうか)は、**目印だけ**を見る。今の判定は、数える用途(`numeric_stops`)にだけ残し、それも上の PR で消す。
- 目印を付けない言語では、フラグを付けても文字の汚染は運ばれない。そのことを出力に出す(例: `summary` に、文字の汚染が有効な言語の一覧)。これは、フラグを足す PR で。

### CEL / antlr が上位 10 に出ない理由(測定済み。memos 00fcd170、2 回、結果は同一)

- 関数ごとの `char_reads` と `other` の件数を、使い捨ての出力で取った(844 関数。2 回の表は完全に一致。#329 のあとの main に、この PR を合流させたもの)。
- **antlr のパッケージの関数は、`char_reads` にも `other` にも 0 関数**(1 件も数えられていない)。最初の推測(「`other` に入る」)は外れた。
- cel-go は 33 関数、`char_reads` 102 件、`other` 0 件。最上位は `common.Error.ToDisplayString` と `ext.parseFormatString` などの 9 件(`char_reads` の順位は 112 位前後)。
- まだ分かっていないこと: antlr の字句解析に、そもそも汚染が届いていないのか(source から遠い)、届いても、止める処理が `isCharRead` にも `other` にも当たらないのか。分けるには、antlr の関数の要約に入る source の有無を見る必要がある。
- 限界(確認済みの範囲): `numeric_stops` が示すのは、記録された止まりだけ。antlr のように、数えられていない字句解析は、この数では指せない。YAML(`yaml.v3` 36 関数、1065 件)と Markdown(`goldmark` 102 関数、948 件)は、数えられている。

### `-char-taint` の Go の実装で、通らない形(結論。2026-11-24)

Go の zoo の 8 形のうち、`-char-taint` を付けると 7 形(append、case、hex、lexer、range-write、replace、reverse)が通る。**base64 は通らない**。原因は 2 つで、どちらも設計どおりの止め: (1) `uint32(s[i+j])` は、別の数の型への変換なので、文字のマークを落とす(`byte`・`rune` への変換は保つ)。(2) 復号の `strings.IndexByte(alphabet, c)` は、モデルが文字列から作る数で、マークを落とす。符号化の `alphabet[n>>18&63]` は、`n` が `uint32` を通ったあとなので、マークがない。hex が通るのは、`c>>4` で `byte` のまま表を引くため(表を文字で引く形は、引いた値に文字のマークを付ける)。

base64 のセルは、`@miss(silent) @ref(#185)` のままにする(フラグを付けても外れない。`@flag` は付けない)。通すには、数の型への変換でマークを保つ規則が要る。それは、`int(c)` で数になった値(`int(c - '0')`、長さではなく数として使う値)まで運ぶことになり、設計の「数への変換では止める」と両立しない。Dart と Rust でも、同じ形(`codeUnitAt` を `int` で受けて表を引く、`as u32`)は、同じ理由で通らない見込み。
