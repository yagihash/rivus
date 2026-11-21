# 言語間の差の表(段階 1)

`go run ./bench/parity report` が作る。手で直さない。手順は [README.md](README.md)。**割合だけでなく実数を並べる**(分母が言語ごとに違う)。
「モデルがない」は見逃しではない(CORE-K のあと、モデルのない関数は「入ったものは出る」+ `unmodeled_library_call` で運ばれる)。
**既定の扱い**(`internal/taint/calls.go` の `unmodeled`): 受け手と引数のすべての中身が、結果のすべてに流れる(bool は除く)。**受け手や引数への書き込みは含まない**。コールバックは呼ばれない(CB-LIST)。
よって効くのは (ii) と (iii)、(iv) と pure(流れを切る)。(i) は、モデルがなくても流れは運ばれる(モデルは不確実性を減らすだけ)。


## 要約: 4 つの数を言語ごとに(実数と割合)

|  | Go | Dart | Rust |
| --- | --- | --- | --- |
| 組み込みの行(全部) | 585 | 476 | 553 |
| **(A) 量**: 標準ライブラリの公開 API(型から列挙)のうち、組み込みの行がある | 383 / 2678 (14%) | 未 | 未 |
|   うち型では「書く」(ii)関数に、書き込みを運ぶ行(ii)がある | 27 / 131 (21%)(行がある関数の中で。型の判定は上界) | 未 | 未 |
| **(B) 需要**: 実コードの、本体のないライブラリ関数への呼び出し(箇所)のうち、組み込みの行がある | 5936 / 6397 (93%) | 2503 / 4421 (57%); `--include-tests`(参考) 3660 / 14428 (25%) | 37653 / 69355 (54%) |
|   異なる呼び先(関数の種類。コードごとの重複あり)のうち、組み込みの行がある | 672 / 882 (76%) | 201 / 522 (39%) | 289 / 1522 (19%) |
|   うち標準ライブラリへの呼び出し(箇所) | 5936 / 6397 (93%) | 2487 / 2959 (84%); `--include-tests`(参考) 3641 / 4481 (81%) | 35636 / 37079 (96%) |
|   形 (ii) と (ii?)(書く)の箇所のうち、組み込みの行がある | 421 / 509 (83%)(名前・型の推定を含む) | 401 / 402 (100%)(名前・型の推定を含む) | 666 / 842 (79%)(名前・型の推定を含む) |
|   形 (iii)(コールバック)の箇所のうち、組み込みの行がある | 0 / 21 (0%)(名前・型の推定。行があっても中は運ばない) | 276 / 324 (85%)(名前・型の推定。行があっても中は運ばない) | 1301 / 1436 (91%)(名前・型の推定。行があっても中は運ばない) |
|   汚れた値が**届いた**呼び出しのうち、組み込みの行がある | 未(コアの診断が足りない。下の 2) | 未 | 未 |
| **(C) 正しさ**: `pure` の行のうち、実行で確かめた | 確かめた部分あり(299 行が pure。下の 3) | 未(94 行が pure) | 未(86 行が pure) |
| **(D) 当たっているか**: 鍵が呼び出しの行のうち、どの IR にも当たらない | 215 / 581 (37%) | 255 / 473 (54%) | 229 / 530 (43%) |
|   うち 綴りの疑い / API 不在 | 0 / 0 | 0 / - | 17 / - |
|   うち 別の型の同名メンバ(継承・Deref・trait の疑い) | 0 | 43 | 59 |
|   うち 使われていない(似た名前も IR にない) | 215 | 212 | 153 |

分母は言語ごとに違う(読んだコードが違う)ので、割合どうしを比べない。入力は 6 を見る。

## 1. (A) 量: 標準ライブラリの公開 API(型から列挙。分母を手で作らない)

**Go のみ(段階 1)**。`go/types` で、標準ライブラリ(173 パッケージ)の公開の関数とメソッドのうち、受け手か引数に、文字列・バイト列・コレクション・Reader / Writer・空のインターフェース・型パラメータ・それらを持つ構造体の類がある関数を数える(2678 個)。形は型から: (iii) = 関数型の引数がある、(ii) = ポインタの受け手・ポインタの引数・io.Writer の類・空のインターフェースがある(書ける。**上界**: `Len` のように読むだけのポインタ受け手のメソッドも入る)、(ii?) = スライスかマップの引数だけ(書き先かもしれない。上界)、(i) = 結果があり、書き先がない。モデルの有無は、組み込みの行に同じ名前があるか(pure・source・sink を含む)。**ジェネリック型のメソッドは数えない**。公開 API でも、使う人が稀な関数が大半なので、割合は「需要」(下)と合わせて読む。

| 形 | 標準ライブラリの API | 組み込みにある | 割合 |
| --- | --- | --- | --- |
| (i) 値を返すだけ | 1037 | 198 | 19% |
| (ii) 引数・受け手に書く | 1133 | 131 | 12% |
| (ii?) 書き先になりうる型(スライス・マップ)だけ。上界 | 280 | 28 | 10% |
| (iii) コールバックを呼ぶ | 152 | 7 | 5% |
| 運ばない型(結果も書き先もない) | 76 | 19 | 25% |
| **計** | 2678 | 383 | 14% |

**型では書く・コールバックを呼ぶ関数に、組み込みの行があるとき、その行は何と言っているか**(行の形。(ii) の行だけが書き込みを運ぶ。(i) は、書き込みを落とす。pure は、書き込みも切る。型の判定は上界なので、(i) と pure の中には、読むだけの関数が入る):

| 型から見た形 | モデルがある | 行の形 (ii) | (i) | pure | (iii) | (iv) |
| --- | --- | --- | --- | --- | --- | --- |
| ii | 131 | 27 | 42 | 51 | 0 | 11 |
| ii? | 28 | 5 | 3 | 18 | 0 | 2 |
| iii | 7 | 0 | 1 | 6 | 0 | 0 |

Go の呼び出しの行 581 のうち、標準ライブラリの名前 574、それ以外(grpc・protobuf・x/* など)7。

**Dart と Rust は未**(形が決まってから。Dart の `dart:*` は型のライブラリ URI から、Rust の `std` は rustdoc の JSON から、同じ基準で列挙できる見込み)。

## 2. (B) 需要: 読んだコードから、本体のないライブラリ関数への呼び出し

**Go の注意**: 解析は、標準ライブラリ以外の依存のソースを読んで、その関数の本体を解析する(`Deps`)。モデルのない依存の関数への呼び出しは「本体のない呼び出し」ではないので数えない(表の「本体を読む依存」の列)。Dart と Rust は `--no-deps` で抽出しているので、依存と SDK への呼び出しは、本体がない側に数える(bench の設定と同じ)。

**呼び出しの箇所**(IR の `call` 命令)のうち、呼び先が IR に本体のないもの(静的な呼び出しで、呼び先の関数が IR にない。動的な呼び出しで、候補が IR にない)を「ライブラリへの呼び出し」とし、呼び先の名前に**組み込みの行があるか**を数える。IR だけから数える(解析は要らない)ので、**汚れた値が届いたかは見ていない**(下の「届いた」を参照)。形は、行があれば行の形、なければ型(Go の標準ライブラリ)か名前(コールバック・書き込みの類)から。形の記号: `-` = 型では運ぶものがない(`reflect.Value` のように、型に出ない書き込みを含む)、`?` = 分からない、`ii?` = 書き先になりうる(型がスライス・マップで、データを返さない関数。または名前が push / add / write の類)。

### Go

| 入力 | ライブラリへの呼び出し(箇所) | モデルがある箇所 | うち標準ライブラリへ(箇所 / モデルあり) | 異なる呼び先 | モデルがある呼び先 | (Go)本体を読む依存への呼び出し(数えない) | 解析の `unmodeled_library_call` | `unmodeled_top`: 汚れが届いた(上位 20 の合計) |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| casdoor | 1668 | 1551 / 1668 (93%) | 1551 / 1668 (93%) | 182 | 130 / 182 (71%) | 1253 | 78 | 3874 (20 個) |
| gin-vue-admin | 1247 | 1138 / 1247 (91%) | 1138 / 1247 (91%) | 207 | 165 / 207 (80%) | 1547 | 53 | 6268 (20 個) |
| gogs | 2918 | 2766 / 2918 (95%) | 2766 / 2918 (95%) | 347 | 269 / 347 (78%) | 4766 | 172 | 4306 (20 個) |
| gophish | 564 | 481 / 564 (85%) | 481 / 564 (85%) | 146 | 108 / 146 (74%) | 543 | 76 | 1087 (20 個) |

形ごと(実コードの合計。参考の入力は入れない):

形は、モデルがある箇所は行の形、ない箇所は型(Go の標準ライブラリ)か名前(コールバック・書き込みの類)からの推定。**Dart / Rust と Go の標準ライブラリ以外では、モデルのない箇所の形の大半が「分からない」**ので、(ii)(iii) の「割合」は、分かった分の中の割合で、実際より高い。

| 形 | ライブラリへの呼び出し(箇所) | モデルがある箇所 | 割合 |
| --- | --- | --- | --- |
| (i) 値を返すだけ | 3594 | 3536 | 98% |
| (ii) 引数・受け手に書く | 496 | 421 | 85% |
| (ii?) 書き先になりうる型(スライス・マップ)だけ。上界 | 13 | 0 | 0% |
| (iii) コールバックを呼ぶ | 21 | 0 | 0% |
| (iv) source / sink / sanitizer / boundary | 343 | 343 | 100% |
| pure(流れを切る) | 1636 | 1636 | 100% |
| 運ばない型(結果も書き先もない) | 150 | 0 | 0% |
| 形が分からない(モデルがなく、型も引けない) | 144 | 0 | 0% |

**casdoor**: 汚れた値が届いた、モデルのない呼び先の上位(`reached` = 届いた回数、`on_path` = finding の経路に載った数):

| 呼び先 | reached | on_path | 形 |
| --- | --- | --- | --- |
| `(reflect.Value).Set` | 631 | 0 | - |
| `(reflect.Value).Interface` | 548 | 0 | - |
| `(reflect.Value).Elem` | 427 | 0 | - |
| `reflect.ValueOf` | 417 | 0 | - |
| `(google.golang.org/protobuf/reflect/protoreflect.List).Append` | 265 | 0 | ii? |
| `(reflect.StructTag).Get` | 190 | 0 | i |
| `(reflect.Value).Index` | 190 | 0 | - |
| `reflect.Indirect` | 152 | 0 | - |

**gin-vue-admin**: 汚れた値が届いた、モデルのない呼び先の上位(`reached` = 届いた回数、`on_path` = finding の経路に載った数):

| 呼び先 | reached | on_path | 形 |
| --- | --- | --- | --- |
| `(reflect.Value).Set` | 944 | 0 | - |
| `(reflect.Value).Interface` | 904 | 0 | - |
| `reflect.ValueOf` | 655 | 0 | - |
| `(reflect.Value).Elem` | 617 | 0 | - |
| `(time.Time).Month` | 441 | 0 | - |
| `(reflect.Value).Index` | 435 | 0 | - |
| `(reflect.Value).String` | 245 | 0 | - |
| `reflect.Indirect` | 236 | 0 | - |

**gogs**: 汚れた値が届いた、モデルのない呼び先の上位(`reached` = 届いた回数、`on_path` = finding の経路に載った数):

| 呼び先 | reached | on_path | 形 |
| --- | --- | --- | --- |
| `(reflect.Value).Set` | 811 | 0 | - |
| `(reflect.Value).Interface` | 580 | 0 | - |
| `reflect.ValueOf` | 482 | 0 | - |
| `(reflect.Value).Elem` | 464 | 0 | - |
| `(google.golang.org/protobuf/reflect/protoreflect.List).Append` | 233 | 0 | ii? |
| `reflect.Indirect` | 193 | 0 | - |
| `(reflect.Value).Index` | 191 | 0 | - |
| `(reflect.Value).Int` | 145 | 0 | - |

**gophish**: 汚れた値が届いた、モデルのない呼び先の上位(`reached` = 届いた回数、`on_path` = finding の経路に載った数):

| 呼び先 | reached | on_path | 形 |
| --- | --- | --- | --- |
| `(reflect.Value).Interface` | 282 | 0 | - |
| `reflect.ValueOf` | 126 | 0 | - |
| `(reflect.Value).Index` | 88 | 0 | - |
| `(reflect.Value).Addr` | 87 | 0 | - |
| `(reflect.Value).Set` | 82 | 0 | - |
| `(golang.org/x/net/html.writer).WriteString` | 51 | 0 | ii? |
| `(reflect.Value).FieldByName` | 51 | 0 | i |
| `reflect.New` | 40 | 0 | i |

### Dart

| 入力 | ライブラリへの呼び出し(箇所) | モデルがある箇所 | うち標準ライブラリへ(箇所 / モデルあり) | 異なる呼び先 | モデルがある呼び先 | (Go)本体を読む依存への呼び出し(数えない) | 解析の `unmodeled_library_call` | `unmodeled_top`: 汚れが届いた(上位 20 の合計) |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| dart-pub | 4421 | 2503 / 4421 (57%) | 2487 / 2959 (84%) | 522 | 201 / 522 (39%) | - | 909 | 1273 (20 個) |
| dart-pub-tests(参考。合計に入れない) | 14428 | 3660 / 14428 (25%) | 3641 / 4481 (81%) | 679 | 235 / 679 (35%) | - | - | - |

形ごと(実コードの合計。参考の入力は入れない):

形は、モデルがある箇所は行の形、ない箇所は型(Go の標準ライブラリ)か名前(コールバック・書き込みの類)からの推定。**Dart / Rust と Go の標準ライブラリ以外では、モデルのない箇所の形の大半が「分からない」**ので、(ii)(iii) の「割合」は、分かった分の中の割合で、実際より高い。

| 形 | ライブラリへの呼び出し(箇所) | モデルがある箇所 | 割合 |
| --- | --- | --- | --- |
| (i) 値を返すだけ | 1094 | 1094 | 100% |
| (ii) 引数・受け手に書く | 401 | 401 | 100% |
| (ii?) 書き先になりうる型(スライス・マップ)だけ。上界 | 1 | 0 | 0% |
| (iii) コールバックを呼ぶ | 324 | 276 | 85% |
| (iv) source / sink / sanitizer / boundary | 107 | 107 | 100% |
| pure(流れを切る) | 625 | 625 | 100% |
| 形が分からない(モデルがなく、型も引けない) | 1869 | 0 | 0% |

**dart-pub**: 汚れた値が届いた、モデルのない呼び先の上位(`reached` = 届いた回数、`on_path` = finding の経路に載った数):

| 呼び先 | reached | on_path | 形 |
| --- | --- | --- | --- |
| `package:path/path.dart#join` | 272 | 1 | ? |
| `package:args/command_runner.dart#Command.usageException` | 158 | 0 | ? |
| `package:path/path.dart#normalize` | 116 | 1 | ? |
| `package:path/path.dart#relative` | 75 | 0 | ? |
| `package:args/command_runner.dart#Command.addSubcommand` | 74 | 0 | ? |
| `dart:core#Set.contains` | 69 | 0 | ? |
| `package:path/path.dart#absolute` | 66 | 1 | ? |
| `package:yaml/src/yaml_node.dart#YamlNode.span` | 59 | 0 | ? |

### Rust

| 入力 | ライブラリへの呼び出し(箇所) | モデルがある箇所 | うち標準ライブラリへ(箇所 / モデルあり) | 異なる呼び先 | モデルがある呼び先 | (Go)本体を読む依存への呼び出し(数えない) | 解析の `unmodeled_library_call` | `unmodeled_top`: 汚れが届いた(上位 20 の合計) |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| parseable | 69355 | 37653 / 69355 (54%) | 35636 / 37079 (96%) | 1522 | 289 / 1522 (19%) | - | 20528 | 7751 (20 個) |

形ごと(実コードの合計。参考の入力は入れない):

形は、モデルがある箇所は行の形、ない箇所は型(Go の標準ライブラリ)か名前(コールバック・書き込みの類)からの推定。**Dart / Rust と Go の標準ライブラリ以外では、モデルのない箇所の形の大半が「分からない」**ので、(ii)(iii) の「割合」は、分かった分の中の割合で、実際より高い。

| 形 | ライブラリへの呼び出し(箇所) | モデルがある箇所 | 割合 |
| --- | --- | --- | --- |
| (i) 値を返すだけ | 28883 | 28883 | 100% |
| (ii) 引数・受け手に書く | 666 | 666 | 100% |
| (ii?) 書き先になりうる型(スライス・マップ)だけ。上界 | 176 | 0 | 0% |
| (iii) コールバックを呼ぶ | 1436 | 1301 | 91% |
| (iv) source / sink / sanitizer / boundary | 141 | 141 | 100% |
| pure(流れを切る) | 6662 | 6662 | 100% |
| 形が分からない(モデルがなく、型も引けない) | 31391 | 0 | 0% |

**parseable**: 汚れた値が届いた、モデルのない呼び先の上位(`reached` = 届いた回数、`on_path` = finding の経路に載った数):

| 呼び先 | reached | on_path | 形 |
| --- | --- | --- | --- |
| `tracing::field::FieldSet::value_set_all` | 1274 | 0 | ? |
| `tracing::__macro_support::__tracing_log` | 948 | 0 | ? |
| `tracing::field::debug` | 836 | 0 | ? |
| `tracing::field::display` | 830 | 0 | ? |
| `serde::de::MapAccess::next_value` | 577 | 0 | ? |
| `serde::ser::SerializeStruct::serialize_field` | 477 | 0 | ? |
| `tracing::Event::dispatch` | 426 | 0 | ? |
| `serde::de::SeqAccess::next_element` | 382 | 0 | ? |

**届いた割合の限界**: `-pass-stats` の `unmodeled_top` は**モデルのない関数だけの上位 20**で、モデルがある関数に汚れが届いた回数と、`reached` の総数は出ない。このため「汚れが届いた呼び出しのうちモデルが当たった割合」は、まだ出せない(上の「`unmodeled_top`」の列は、モデルのない側の上位 20 の合計で、分母ではない)。出すには、コアの診断に、モデルがある呼び出しの `reached` の合計と、モデルのない呼び出しの `reached` の合計(20 件で切らない)を足す必要がある(コアは触っていない。助言者への依頼事項)。

## 3. (C) 正しさの現状

|  | Go | Dart | Rust |
| --- | --- | --- | --- |
| `pure` の行(実数。行 = 呼び出しの名前ごと) | 299 | 94 | 86 |
| `pure` の実行の検査(`TestZooPure*`): 呼んだ結果の部分 / うち誤り | 50 / 18 | 未(検査がない) | 未(検査がない) |
| 表 S(組み込みの全行を、行ごとに実行で確かめる) | `pure` の一部だけ(上)。propagator / sanitizer / source / sink の行は未 | 未 | 未 |
| zoo(共通の形のマス): マス / ケース / 出る / `@miss` | 30 / 92 / 26 of 32 / 6 | 22 / 76 / 25 of 32 / 7 | 49 / 167 / 49 of 69 / 20 |

zoo のテストの出力(`go test ./cmd/rivus -run 'TestZooPureGo|TestZooGo|TestZooDart' -v` の記録の行):

```
6 findings have an assumed_call on their path
zoo-dart: cells 22 / cases 76 | found 25 / 32 | @miss 7 (trace 4, silent 3, no rule 0) | fixed reported wrongly 0 | decoys reported wrongly 5 / 22 (with the trace container_element_imprecision 3, without 2) | carry map[] | manual 0 (0.0%) | run and annotation disagree 0 | statements not run 0
zoo-dart: rivus: 45 flows found, 57 sinks quiet, 9 flows not found (@miss), 10 reported wrongly (@fp)
zoo-dart: common cells 14 | found in zoo-go and not in Dart 0 (@ref(PARITY)) | only in Dart: 8 with @na(go: reason), 0 common and not yet in zoo-go | cells of zoo-go that Dart has no counterpart of (na.txt) 3, not written yet (stage 3) 13
zoo-dart silent misses: 4 / 9 (44%)
--- PASS: TestZooDart (15.09s)
zoo-go pure: 50 parts called, 50 of them of pure rows (the model has 479 pure rows), 18 wrong (R-03)
--- PASS: TestZooPureGo (0.90s)
0 findings have an assumed_call on their path
zoo-rust: cells 49 / cases 167 | found 49 / 69 | @miss 20 (trace 7, silent 13, no rule 0) | fixed reported wrongly 4 | decoys reported wrongly 5 / 49 (with the trace container_element_imprecision 1, without 7) | carry map[] | manual 0 (0.0%) | run and annotation disagree 0 | statements not run 0
zoo-rust: rivus: 88 flows found, 137 sinks quiet, 31 flows not found (@miss), 13 reported wrongly (@fp)
zoo-rust silent misses: 19 / 31 (61%)
zoo-rust: parity with zoo-go: common cells 30 | common vulnerable flows 32 (found in both 26, missed in both 4) | found in Go and not in Rust 2 [dyn-generics-ptr-vuln state-receiver-field-across-entries-vuln] | decoys reported in Rust and not in Go 3 [dyn-http-middleware-chain-decoy dyn-sink-in-method-iface-decoy state-save-restore-decoy]
zoo-rust: vulnerable flows found 49 / 69
--- PASS: TestZooRust (2.73s)
4 findings have an assumed_call on their path
zoo-go: cells 30 / cases 92 | found 26 / 32 | @miss 6 (trace 2, silent 4, no rule 0) | fixed reported wrongly 0 | decoys reported wrongly 2 / 30 (with the trace container_element_imprecision 2, without 0) | carry map[] | manual 0 (0.0%) | run and annotation disagree 0 | statements not run 0
zoo-go: rivus: 53 flows found, 88 sinks quiet, 10 flows not found (@miss), 4 reported wrongly (@fp)
--- PASS: TestZooGo (1.75s)
ok  	github.com/yagihash/rivus/cmd/rivus	21.633s
```

## 4. (D) 当たっているか: どの IR にも当たらなかった行

行の鍵(`symbol`)が、読んだ IR の呼び先の名前(`callee.symbol`)か、関数の名前(`kind: param` の行)と**文字列で一致**するか。**IR に出ないだけ(使われていないだけ)の行と、綴りの誤りで当たらない行は区別できない**ので、次に分ける: **API 不在**(Go のみ: `go/types` にその名前がない = 綴りの誤りか、消えた API)、**綴りの疑い**(同じ API の別の綴りが IR にある: 末尾が同じで経路・レシーバの `*` などが違う)、**別の型の同名メンバ**(Dart と Rust のみ: 同じライブラリ(crate)の別の型に、同じメンバ名の呼び出しが IR にある。`List.join` に対する `Iterable.join` のように、宣言した型が違う: 継承・Deref・trait の疑い。`new`・`get` などの一般的な名前は除く)、**使われていない**(IR に似た名前もない)。一致は `match: implementations` の展開と、`ValueCands`(コアが作る関数値の候補)を見ない。

|  | Go | Dart | Rust |
| --- | --- | --- | --- |
| 組み込みの行 | 585 | 476 | 553 |
| 鍵が呼び出しの行(分母) | 581 | 473 | 530 |
| **当たった行**(どれかの IR) | 366 / 581 (63%) | 218 / 473 (46%) | 301 / 530 (57%) |
|   うち実コードの IR に当たった | 361 / 581 (62%) | 214 / 473 (45%) | 298 / 530 (56%) |
| **当たらなかった行** | 215 / 581 (37%) | 255 / 473 (54%) | 229 / 530 (43%) |
|   API 不在(go/types にない。Go のみ) | 0 | 0 | 0 |
|   **綴りの疑い**(別の綴りが IR にある) | 0 | 0 | 17 |
|   **別の型の同名メンバ**(同じライブラリの、継承・Deref・trait の疑い) | 0 | 43 | 59 |
|   使われていない(似た名前も IR にない) | 215 | 212 | 153 |

形ごと(行 / 当たった / 当たらず・綴りの疑い・API 不在・別の型の同名メンバ / 当たらず・使われていない):

| 形 | Go | Dart | Rust |
| --- | --- | --- | --- |
| i | 171 / 123 / 0 / 48 | 250 / 113 / 28 / 109 | 337 / 186 / 48 / 103 |
| ii | 38 / 25 / 0 / 13 | 42 / 22 / 6 / 14 | 25 / 11 / 7 / 7 |
| iii | 0 / 0 / 0 / 0 | 27 / 15 / 0 / 12 | 32 / 25 / 4 / 3 |
| pure | 299 / 181 / 0 / 118 | 94 / 48 / 8 / 38 | 86 / 52 / 10 / 24 |
| iv | 73 / 37 / 0 / 36 | 60 / 20 / 1 / 39 | 50 / 27 / 7 / 16 |

綴りの疑いと別の型の同名メンバの例(モデルの鍵 -> IR にある別の綴り。全部は [DEAD.md](DEAD.md)):

- Go
  - (なし)
- Dart
  - `dart:async#Completer.sync` -> `dart:async#Future.sync`
  - `dart:async#Stream.error` -> `dart:async#Future.error`
  - `dart:async#StreamIterator.current` -> `dart:async#Zone.current`
  - `dart:async#Stream.timeout` -> `dart:async#Future.timeout`
  - `dart:async#StreamSink.addError` -> `dart:async#StreamController.addError`
  - `dart:async#StreamIterator.cancel` -> `dart:async#StreamSubscription.cancel`
  - `dart:async#StreamSink.close` -> `dart:async#EventSink.close`
  - `dart:convert#HtmlEscape.convert` -> `dart:convert#JsonEncoder.convert`
  - `dart:core#num.parse` -> `dart:core#DateTime.parse`
  - `dart:core#Iterable.reversed` -> `dart:core#List.reversed`
- Rust
  - `tokio::fs::read_to_string` -> `std::fs::read_to_string`
  - `tokio::fs::File::open` -> `std::fs::File::open`
  - `tonic::request::Request::into_inner` -> `tonic::Request::into_inner`
  - `tonic::request::Request::get_ref` -> `tonic::Request::get_ref`
  - `tonic::request::Request::metadata` -> `tonic::Request::metadata`
  - `tonic::metadata::map::MetadataMap::get` -> `tonic::metadata::MetadataMap::get`
  - `tonic::metadata::map::MetadataMap::get_all` -> `tonic::metadata::MetadataMap::get_all`
  - `tonic::metadata::MetadataMap::iter` -> `tonic::metadata::GetAll::iter`
  - `tonic::metadata::map::MetadataMap::iter` -> `tonic::metadata::GetAll::iter`
  - `tonic::metadata::value::MetadataValue::to_str` -> `tonic::metadata::MetadataValue::to_str`

## 5. 組み込みの行数を、形で分けた数(実数)

形は行の flows から決める。(ii) = いずれかの flow の行き先が結果でない(`recv` / `argN`)。(iii) = **名前による推定**(モデルの書式はコールバックを表せない。CB-LIST)。(iv) = source / sink / sanitizer / boundary / config_read。pure = `pure: true`。「呼び出しでない鍵」は、型・フィールド・変数・marker(IR の呼び先では当たらない)。

| 行の種類 | Go | Dart | Rust |
| --- | --- | --- | --- |
| **組み込みの行(全部)** | 585 | 476 | 553 |
| (i) propagator: 値を返すだけ | 171 | 250 | 337 |
| (ii) propagator: 引数・受け手に書く | 38 | 42 | 25 |
| (iii) propagator: コールバック(名前の推定) | 0 | 27 | 32 |
| pure | 299 | 94 | 86 |
| (iv) source / sink / sanitizer / boundary / config_read | 73 | 60 | 50 |
| 呼び出しでない鍵(型・フィールド・変数・marker) | 4 | 3 | 23 |
|   うち source | 4 | 9 | 4 |
|   うち sink | 53 | 38 | 41 |
|   うち sanitizer | 3 | 4 | 0 |
|   うち propagator | 508 | 413 | 480 |
|   うち config_read | 13 | 9 | 5 |

## 6. 読んだもの

| 言語 | 入力(IR) | 実コード | 関数 | 呼び先の名前(種類) | 注記 |
| --- | --- | --- | --- | --- | --- |
| Go | casdoor | はい | 1107 | 1154 | 7b0b426a |
| Go | gin-vue-admin | はい | 1148 | 944 | 7bc7c3051067 |
| Go | go-curated | いいえ(fixture) | 280 | 194 |  |
| Go | gogs | はい | 2953 | 2258 | 8a3b8198a |
| Go | gophish | はい | 395 | 524 | 27d13a05 |
| Go | zoo-go | いいえ(fixture) | 326 | 179 |  |
| Dart | dart-curated | いいえ(fixture) | 340 | 109 |  |
| Dart | dart-pub-tests | はい | 4847 | 2011 | 5e24336e; --include-tests |
| Dart | dart-pub | はい | 2196 | 1628 | 5e24336e; the extractor leaves test/ out (its default, ADR-062) |
| Dart | zoo-dart | いいえ(fixture) | 238 | 103 |  |
| Rust | parseable | はい | 8557 | 4840 | 5c20b8e4; an IR that was made before (not extracted again) |
| Rust | rust-curated | いいえ(fixture) | 373 | 176 |  |
| Rust | zoo-rust | いいえ(fixture) | 557 | 253 |  |

## 7. 読み方の注意

- 「当たった」は、**読んだ IR にその名前の呼び出しがある**こと。当たらなかったことは、行が誤りであることではない(使われていないだけかもしれない)。実コードの入力を増やすと、「使われていない」は減る。
- (iii) の形は名前による推定。モデルの書式がコールバックを表せない(CB-LIST)ので、(iii) の行は、モデルがあっても、コールバックの中を運ばない。
- Rust は、自前のコード(rust-curated)と、すでに IR のある parseable だけを読んだ(C-19)。
