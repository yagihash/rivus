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

`summary.numeric_stops.char_reads` は、文字の読み(`s[i]`、`range s`、rune、`codeUnitAt`)で汚染が止まった回数で、`top[]` に回数の多い 10 関数(`function`、`scope`、`count`)が出る。その関数は、文字を 1 つずつ扱っている。期待した sink が出ない(`-no-deps` では出る)ときは、まずその関数の周りを見る。`other`(長さ、カウンタ、変換、比較)は仕様どおりなので、数だけを見る。プロジェクトの関数に文字の読みがあれば、標準エラーに `N functions stopped a flow at per-character reads: summary.numeric_stops` が出る。

## 経緯

- 2026-11-23: 依存の中のパーサが、ユーザーの文字列を 1 文字ずつ読んで、その先の流れを失っていた件から見つかった。zoo に、この形のセルが無かった。
