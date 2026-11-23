# #198 Go: 文字を読み出して byte に直して append する形で、流れが切れる(依存の中の文字バッファ)

- 状態: **open**
- 種類: サイレントな見逃し
- 言語: Go
- ROADMAP の節: 2
- 優先度: 高
- 守るもの: zoo `F/conv/char-decode-rune`、`F/conv/char-range-append`
- 題: `utf8.DecodeRuneInString` で読む形と、`range` + `append(byte(r))` で書く形で、流れが切れる

## 何が見逃されるか

#185 と同じ原因(1 文字を表す値は数値で、汚染を持てない)の、zoo に無かった 2 つの形。

- `utf8.DecodeRuneInString(s[i:])` で 1 文字ずつ読み、`append(buf, byte(r))` で `[]byte` に書き、あとで `string(buf[i:j])` で取り出す(バッファの型のフィールドに持たせても同じ)。
- `for _, r := range s` で読み、`append(buf, byte(r))` で書く。

`append(buf, r)`(`[]rune` に書く形)も、`DecodeRuneInString` で読んだあとでは切れる。出力に何も残らない。

## 起きた場所(依存込みと `-no-deps` で結果が違った)

公開の Go のプロジェクトの測定で、`-no-deps` では出て、依存込みでは出ない sql_injection が 12 件あった。経路は、利用者の文字列が式言語(フィルタ)の依存のパーサを通る形。依存の中のソース文字列のバッファが、この形(`DecodeRuneInString` のループで `[]byte`、`[]uint16`、`[]rune` に書く)で、そこで流れが切れる。

確かめたこと(小さいコードで、`-no-deps` と依存込みを比べた):

- その依存の、バッファを作る関数を呼ぶだけの小さいコードでも、依存込みでは 0 件、`-no-deps` では出る。
- そのバッファのパッケージだけ `-body-replaced` で本体を読まない宣言にすると、8 件のうち 7 件が戻る(戻らない 1 件は、別のパーサの呼び出しの形。切れる別の場所がある。未調査)。
- この依存を使わず、同じ形(`DecodeRuneInString` のループ + `append(byte(r))`、`range` + `append(byte(r))`、`append(rune)`)を自分のコードに書くと、`-no-deps` でも 0 件で、`summary.numeric_stops.char_reads` にその関数が出る。

## 12 件そのものは

コードを読んだ範囲では、12 件の経路の演算子の部分は、文字列から定数への写しで決まる値で、利用者の文字列は引数(`?` の置き場)にだけ入る。つまり `-no-deps` の 12 件は実在しない経路で、依存込みで消えるのは結果として正しい。ただし**理由が違う**: 流れが切れたからで、本体を読んで安全と分かったからではない。同じ依存のパーサを通る実在の経路(たとえば、パースした文字列がそのまま sink に届く形)があっても、出ない。

## Dart と Rust

同じ形(1 文字ずつ読んで、1 文字ずつ詰め直す)は、Dart と Rust にもありうる(`codeUnitAt` や `runes` で読んで `List<int>` に足す形、`chars()` で読んで `push(c as u8)` する形)。#185 の表で、Dart と Rust でも近い形(`append`、`range-write`、`lexer`)が切れているので、同じ原因と見るが、この 2 つの形そのものは確かめていない。Rust は `lib.rs` の `@todo(PARITY)` に 2 行、Dart はセルが無いので zoo-dart のテストが「まだ」と数える(`conv/char-decode-rune`、`conv/char-range-append`)。セルを足すときに、実際の結果を記録する。

## 直すとき

#185 の修正(文字を数値の既定の外に置く、または由来のマーク)に含める。直したら、2 つのセルの `@miss` を外す。
