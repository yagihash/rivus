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
- 直す前でも、`numeric_stop` を、サマリの経路でも記録して、出力に残す。

## 経緯

- 2026-11-23: 依存の中のパーサが、ユーザーの文字列を 1 文字ずつ読んで、その先の流れを失っていた件から見つかった。zoo に、この形のセルが無かった。

## 追記(2026-11-23): numeric_stops の確認から

### 実例(memos、公開のプロジェクト、00fcd170)

`summary.numeric_stops` の `char_reads` は 4967 件、760 関数。上位は、`gopkg.in/yaml.v3` の字句解析(`yaml_parser_fetch_next_token` の 407 など)、`yuin/goldmark` の構文解析(`parseBlock` の 150 など)、`tidwall/pretty`。YAML や Markdown の解析を通る流れが、依存の本体の中で、1 文字ずつ読む処理で切れている、という「どこで切れるか」の実例になる。ここにはパッケージ名と件数だけを書く。

数は、場所の数ではなく止めた回数で、関数の中の反復で同じ場所が複数回入る。大小の比較に使う目安で、407 は 407 か所ではない。

### 方針(相談役の判断)

文字の汚染の本体(由来の bit)を実装するときは、`numeric_stops` の `isCharRead` を、流れの判断には使わない。`isCharRead` は、言語に固有の名前を決め打ちで持つ(`std::iter::Iterator::next`、`std::str::Bytes`、`String.codeUnits`、`::as_bytes`、`iter.next`、型の名前 `byte` / `rune` / `char`)。「文字列の要素の取り出し」は、extractor が IR に付ける(命令のマークか、値の種類)。そのとき、`isCharRead` も IR のマークを見る形に置き換える。コアは IR だけを見る、という分担に戻す。

### CEL / antlr が上位 10 に出ない理由(調査中)

CEL と antlr が、`char_reads` の上位 10 に出ない理由は、調査中。推測: antlr の字句解析は、入力を `IntStream.LA()` のようなインターフェースのメソッドで 1 文字ずつ `int` として受け取り、表を引いて進む。そのため `isCharRead` に当たらず、`other` に入るか、数えられていない。確認の方法: 関数ごとの `char_reads` と `other` の件数を、パッケージ名に `antlr` と `cel` を含むものについて出す。結果が出たら、この節に足す。
