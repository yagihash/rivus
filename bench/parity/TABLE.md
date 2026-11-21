# 言語間の差の表(段階 1)

`go run ./bench/parity report` が作る。手で直さない。手順は [README.md](README.md)。**割合だけでなく実数を並べる**(分母が言語ごとに違う)。
「モデルがない」は見逃しではない(CORE-K のあと、モデルのない関数は「入ったものは出る」+ `unmodeled_library_call` で運ばれる)。
**既定の扱い**(`internal/taint/calls.go` の `unmodeled`): 受け手と引数のすべての中身が、結果のすべてに流れる(bool は除く)。**受け手や引数への書き込みは含まない**。コールバックは呼ばれない(CB-LIST)。
よって効くのは (ii) と (iii)、(iv) と pure(流れを切る)。(i) は、モデルがなくても流れは運ばれる(モデルは不確実性を減らすだけ)。


## 要約: 4 つの数を言語ごとに(実数と割合)

|  | Go | Dart | Rust |
| --- | --- | --- | --- |
| 組み込みの行(全部) | 599 | 477 | 540 |
| **(A) 量**: 標準ライブラリの公開 API(型から列挙)のうち、組み込みの行がある | 396 / 2678 (15%) | 未 | 未 |
|   うち型では「書く」(ii)関数に、書き込みを運ぶ行(ii)がある | 27 / 133 (20%)(行がある関数の中で。型の判定は上界) | 未 | 未 |
| **(B) 需要**: 実コードの、本体のないライブラリ関数への呼び出し(箇所)のうち、組み込みの行がある | 5948 / 6397 (93%) | 2867 / 4421 (65%); `--include-tests`(参考) 4774 / 14428 (33%) | 38151 / 69234 (55%) |
|   異なる呼び先(関数の種類。コードごとの重複あり)のうち、組み込みの行がある | 676 / 882 (77%) | 217 / 522 (42%) | 302 / 1516 (20%) |
|   うち標準ライブラリへの呼び出し(箇所) | 5948 / 6397 (93%) | 2517 / 2959 (85%); `--include-tests`(参考) 3701 / 4481 (83%) | 35652 / 37019 (96%) |
|   形 (ii) と (ii?)(書く)の箇所のうち、組み込みの行がある | 421 / 507 (83%)(名前・型の推定を含む) | 401 / 402 (100%)(名前・型の推定を含む) | 666 / 839 (79%)(名前・型の推定を含む) |
|   形 (iii)(コールバック)の箇所のうち、組み込みの行がある | 0 / 11 (0%)(名前・型の推定。行があっても中は運ばない) | 276 / 303 (91%)(名前・型の推定。行があっても中は運ばない) | 1305 / 1436 (91%)(名前・型の推定。行があっても中は運ばない) |
|   汚れた値が**届いた**呼び出しのうち、組み込みの行がある | 未(コアの診断が足りない。下の 2) | 未 | 未 |
| **(C) 正しさ**: `pure` の行のうち、実行で確かめた | 確かめた部分あり(272 行が pure。下の 3) | 未(93 行が pure) | 未(82 行が pure) |
| **(D) 当たっているか**: 鍵が呼び出しの行のうち、どの IR にも当たらない | 224 / 595 (38%) | 239 / 474 (50%) | 199 / 517 (38%) |
|   うち 綴りの疑い / API 不在 | 0 / 0 | 0 / - | 17 / - |
|   うち 別の型の同名メンバ(継承・Deref・trait の疑い) | 0 | 25 | 33 |
|   うち 使われていない(似た名前も IR にない) | 224 | 214 | 149 |

分母は言語ごとに違う(読んだコードが違う)ので、割合どうしを比べない。入力は 6 を見る。

## 1. (A) 量: 標準ライブラリの公開 API(型から列挙。分母を手で作らない)

**Go のみ(段階 1)**。`go/types` で、標準ライブラリ(173 パッケージ)の公開の関数とメソッドのうち、受け手か引数に、文字列・バイト列・コレクション・Reader / Writer・空のインターフェース・型パラメータ・それらを持つ構造体の類がある関数を数える(2678 個)。形は型から: (iii) = 関数型の引数がある、(ii) = ポインタの受け手・ポインタの引数・io.Writer の類・空のインターフェースがある(書ける。**上界**: `Len` のように読むだけのポインタ受け手のメソッドも入る)、(ii?) = スライスかマップの引数だけ(書き先かもしれない。上界)、(i) = 結果があり、書き先がない。モデルの有無は、組み込みの行に同じ名前があるか(pure・source・sink を含む)。**ジェネリック型のメソッドは数えない**。公開 API でも、使う人が稀な関数が大半なので、割合は「需要」(下)と合わせて読む。

| 形 | 標準ライブラリの API | 組み込みにある | 割合 |
| --- | --- | --- | --- |
| (i) 値を返すだけ | 1037 | 198 | 19% |
| (ii) 引数・受け手に書く | 1133 | 133 | 12% |
| (ii?) 書き先になりうる型(スライス・マップ)だけ。上界 | 280 | 28 | 10% |
| (iii) コールバックを呼ぶ | 152 | 18 | 12% |
| 運ばない型(結果も書き先もない) | 76 | 19 | 25% |
| **計** | 2678 | 396 | 15% |

**型では書く・コールバックを呼ぶ関数に、組み込みの行があるとき、その行は何と言っているか**(行の形。(ii) の行だけが書き込みを運ぶ。(i) は、書き込みを落とす。pure は、書き込みも切る。型の判定は上界なので、(i) と pure の中には、読むだけの関数が入る):

| 型から見た形 | モデルがある | 行の形 (ii) | (i) | pure | (iii) | (iv) |
| --- | --- | --- | --- | --- | --- | --- |
| ii | 133 | 27 | 56 | 39 | 0 | 11 |
| ii? | 28 | 6 | 5 | 15 | 0 | 2 |
| iii | 18 | 0 | 1 | 14 | 3 | 0 |

Go の呼び出しの行 595 のうち、標準ライブラリの名前 588、それ以外(grpc・protobuf・x/* など)7。

**Dart と Rust は未**(形が決まってから。Dart の `dart:*` は型のライブラリ URI から、Rust の `std` は rustdoc の JSON から、同じ基準で列挙できる見込み)。

## 2. (B) 需要: 読んだコードから、本体のないライブラリ関数への呼び出し

**Go の注意**: 解析は、標準ライブラリ以外の依存のソースを読んで、その関数の本体を解析する(`Deps`)。モデルのない依存の関数への呼び出しは「本体のない呼び出し」ではないので数えない(表の「本体を読む依存」の列)。Dart と Rust は `--no-deps` で抽出しているので、依存と SDK への呼び出しは、本体がない側に数える(bench の設定と同じ)。

**呼び出しの箇所**(IR の `call` 命令)のうち、呼び先が IR に本体のないもの(静的な呼び出しで、呼び先の関数が IR にない。動的な呼び出しで、候補が IR にない)を「ライブラリへの呼び出し」とし、呼び先の名前に**組み込みの行があるか**を数える。IR だけから数える(解析は要らない)ので、**汚れた値が届いたかは見ていない**(下の「届いた」を参照)。形は、行があれば行の形、なければ型(Go の標準ライブラリ)か名前(コールバック・書き込みの類)から。形の記号: `-` = 型では運ぶものがない(`reflect.Value` のように、型に出ない書き込みを含む)、`?` = 分からない、`ii?` = 書き先になりうる(型がスライス・マップで、データを返さない関数。または名前が push / add / write の類)。

### Go

| 入力 | ライブラリへの呼び出し(箇所) | モデルがある箇所 | うち標準ライブラリへ(箇所 / モデルあり) | 異なる呼び先 | モデルがある呼び先 | (Go)本体を読む依存への呼び出し(数えない) | 解析の `unmodeled_library_call` | `unmodeled_top`: 汚れが届いた(上位 20 の合計) |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| casdoor | 1668 | 1551 / 1668 (93%) | 1551 / 1668 (93%) | 182 | 130 / 182 (71%) | 1253 | 128 | 3894 (20 個) |
| gin-vue-admin | 1247 | 1139 / 1247 (91%) | 1139 / 1247 (91%) | 207 | 166 / 207 (80%) | 1547 | 112 | 6268 (20 個) |
| gogs | 2918 | 2777 / 2918 (95%) | 2777 / 2918 (95%) | 347 | 272 / 347 (78%) | 4766 | 291 | 4306 (20 個) |
| gophish | 564 | 481 / 564 (85%) | 481 / 564 (85%) | 146 | 108 / 146 (74%) | 543 | 92 | 1068 (20 個) |

形ごと(実コードの合計。参考の入力は入れない):

形は、モデルがある箇所は行の形、ない箇所は型(Go の標準ライブラリ)か名前(コールバック・書き込みの類)からの推定。**Dart / Rust と Go の標準ライブラリ以外では、モデルのない箇所の形の大半が「分からない」**ので、(ii)(iii) の「割合」は、分かった分の中の割合で、実際より高い。

| 形 | ライブラリへの呼び出し(箇所) | モデルがある箇所 | 割合 |
| --- | --- | --- | --- |
| (i) 値を返すだけ | 3730 | 3672 | 98% |
| (ii) 引数・受け手に書く | 494 | 421 | 85% |
| (ii?) 書き先になりうる型(スライス・マップ)だけ。上界 | 13 | 0 | 0% |
| (iii) コールバックを呼ぶ | 11 | 0 | 0% |
| (iv) source / sink / sanitizer / boundary | 343 | 343 | 100% |
| pure(流れを切る) | 1512 | 1512 | 100% |
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
| dart-pub | 4421 | 2867 / 4421 (65%) | 2517 / 2959 (85%) | 522 | 217 / 522 (42%) | - | 597 | 840 (20 個) |
| dart-pub-tests(参考。合計に入れない) | 14428 | 4774 / 14428 (33%) | 3701 / 4481 (83%) | 679 | 251 / 679 (37%) | - | - | - |

**Dart の入口(ADR-073。この表の入力の数ではなく、Flutter のアプリの測定の記録)**: フレームワークが呼ぶメソッド(依存の型のメソッドの上書き。`build`、`initState`、`dispose` など)を入口に足した。`flutter/samples`(`a5bccd24`)の 6 つのアプリ(`--no-deps`)で、入口から届く関数は、411 のうち 65(16%)から 403(98%)に。LocalSend の `app/` は 20,522 のうち 1,546(7.5%)から 19,750(96%)に。finding の `reachability` は `not_from_entry` から `from_framework_callback` に変わる(finding の集合は同じ)。dart-pub(`main` のある CLI。フレームワークの上書きがない)は変わらない。

形ごと(実コードの合計。参考の入力は入れない):

形は、モデルがある箇所は行の形、ない箇所は型(Go の標準ライブラリ)か名前(コールバック・書き込みの類)からの推定。**Dart / Rust と Go の標準ライブラリ以外では、モデルのない箇所の形の大半が「分からない」**ので、(ii)(iii) の「割合」は、分かった分の中の割合で、実際より高い。

| 形 | ライブラリへの呼び出し(箇所) | モデルがある箇所 | 割合 |
| --- | --- | --- | --- |
| (i) 値を返すだけ | 1437 | 1437 | 100% |
| (ii) 引数・受け手に書く | 401 | 401 | 100% |
| (ii?) 書き先になりうる型(スライス・マップ)だけ。上界 | 1 | 0 | 0% |
| (iii) コールバックを呼ぶ | 303 | 276 | 91% |
| (iv) source / sink / sanitizer / boundary | 107 | 107 | 100% |
| pure(流れを切る) | 646 | 646 | 100% |
| 形が分からない(モデルがなく、型も引けない) | 1526 | 0 | 0% |

**dart-pub**: 汚れた値が届いた、モデルのない呼び先の上位(`reached` = 届いた回数、`on_path` = finding の経路に載った数):

| 呼び先 | reached | on_path | 形 |
| --- | --- | --- | --- |
| `package:args/command_runner.dart#Command.usageException` | 158 | 0 | ? |
| `package:args/command_runner.dart#Command.addSubcommand` | 74 | 0 | ? |
| `dart:core#Set.contains` | 70 | 0 | ? |
| `package:yaml/src/yaml_node.dart#YamlNode.span` | 60 | 0 | ? |
| `package:path/path.dart#dirname` | 50 | 4 | ? |
| `package:args/command_runner.dart#CommandRunner.addCommand` | 46 | 0 | ? |
| `package:path/path.dart#canonicalize` | 36 | 0 | ? |
| `dart:core#List.removeWhere` | 31 | 0 | iii |

### Rust

| 入力 | ライブラリへの呼び出し(箇所) | モデルがある箇所 | うち標準ライブラリへ(箇所 / モデルあり) | 異なる呼び先 | モデルがある呼び先 | (Go)本体を読む依存への呼び出し(数えない) | 解析の `unmodeled_library_call` | `unmodeled_top`: 汚れが届いた(上位 20 の合計) |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| parseable | 69234 | 38151 / 69234 (55%) | 35652 / 37019 (96%) | 1516 | 302 / 1516 (20%) | - | 20383 | 7490 (20 個) |

形ごと(実コードの合計。参考の入力は入れない):

形は、モデルがある箇所は行の形、ない箇所は型(Go の標準ライブラリ)か名前(コールバック・書き込みの類)からの推定。**Dart / Rust と Go の標準ライブラリ以外では、モデルのない箇所の形の大半が「分からない」**ので、(ii)(iii) の「割合」は、分かった分の中の割合で、実際より高い。

| 形 | ライブラリへの呼び出し(箇所) | モデルがある箇所 | 割合 |
| --- | --- | --- | --- |
| (i) 値を返すだけ | 28956 | 28956 | 100% |
| (ii) 引数・受け手に書く | 666 | 666 | 100% |
| (ii?) 書き先になりうる型(スライス・マップ)だけ。上界 | 173 | 0 | 0% |
| (iii) コールバックを呼ぶ | 1436 | 1305 | 91% |
| (iv) source / sink / sanitizer / boundary | 141 | 141 | 100% |
| pure(流れを切る) | 7083 | 7083 | 100% |
| 形が分からない(モデルがなく、型も引けない) | 30779 | 0 | 0% |

**parseable**: 汚れた値が届いた、モデルのない呼び先の上位(`reached` = 届いた回数、`on_path` = finding の経路に載った数):

| 呼び先 | reached | on_path | 形 |
| --- | --- | --- | --- |
| `tracing::field::FieldSet::value_set_all` | 1260 | 0 | ? |
| `tracing::__macro_support::__tracing_log` | 941 | 0 | ? |
| `tracing::field::debug` | 758 | 0 | ? |
| `tracing::field::display` | 758 | 0 | ? |
| `serde::de::MapAccess::next_value` | 577 | 0 | ? |
| `serde::ser::SerializeStruct::serialize_field` | 477 | 0 | ? |
| `tracing::Event::dispatch` | 426 | 0 | ? |
| `serde::de::SeqAccess::next_element` | 382 | 0 | ? |

**届いた割合の限界**: `-pass-stats` の `unmodeled_top` は**モデルのない関数だけの上位 20**で、モデルがある関数に汚れが届いた回数と、`reached` の総数は出ない。このため「汚れが届いた呼び出しのうちモデルが当たった割合」は、まだ出せない(上の「`unmodeled_top`」の列は、モデルのない側の上位 20 の合計で、分母ではない)。出すには、コアの診断に、モデルがある呼び出しの `reached` の合計と、モデルのない呼び出しの `reached` の合計(20 件で切らない)を足す必要がある(コアは触っていない。助言者への依頼事項)。

## 3. (C) 正しさの現状

|  | Go | Dart | Rust |
| --- | --- | --- | --- |
| `pure` の行(実数。行 = 呼び出しの名前ごと) | 272 | 93 | 82 |
| `pure` の実行の検査(`TestZooPure*`): 呼んだ結果の部分 / うち誤り | (`-zoo` なし) | 未(検査がない) | 未(検査がない) |
| 表 S(組み込みの全行を、行ごとに実行で確かめる) | `pure` の一部だけ(上)。propagator / sanitizer / source / sink の行は未 | 未 | 未 |
| zoo(共通の形のマス): マス / ケース / 出る / `@miss` | (`-zoo` なし) | (`-zoo` なし) | (`-zoo` なし) |

(zoo のテストの出力は `-zoo` で渡す)

## 4. (D) 当たっているか: どの IR にも当たらなかった行

行の鍵(`symbol`)が、読んだ IR の呼び先の名前(`callee.symbol`)か、関数の名前(`kind: param` の行)と**文字列で一致**するか。**IR に出ないだけ(使われていないだけ)の行と、綴りの誤りで当たらない行は区別できない**ので、次に分ける: **API 不在**(Go のみ: `go/types` にその名前がない = 綴りの誤りか、消えた API)、**綴りの疑い**(同じ API の別の綴りが IR にある: 末尾が同じで経路・レシーバの `*` などが違う)、**別の型の同名メンバ**(Dart と Rust のみ: 同じライブラリ(crate)の別の型に、同じメンバ名の呼び出しが IR にある。`List.join` に対する `Iterable.join` のように、宣言した型が違う: 継承・Deref・trait の疑い。`new`・`get` などの一般的な名前は除く)、**使われていない**(IR に似た名前もない)。一致は `match: implementations` の展開と、`ValueCands`(コアが作る関数値の候補)を見ない。

|  | Go | Dart | Rust |
| --- | --- | --- | --- |
| 組み込みの行 | 599 | 477 | 540 |
| 鍵が呼び出しの行(分母) | 595 | 474 | 517 |
| **当たった行**(どれかの IR) | 371 / 595 (62%) | 235 / 474 (50%) | 318 / 517 (62%) |
|   うち実コードの IR に当たった | 364 / 595 (61%) | 231 / 474 (49%) | 311 / 517 (60%) |
| **当たらなかった行** | 224 / 595 (38%) | 239 / 474 (50%) | 199 / 517 (38%) |
|   API 不在(go/types にない。Go のみ) | 0 | 0 | 0 |
|   **綴りの疑い**(別の綴りが IR にある) | 0 | 0 | 17 |
|   **別の型の同名メンバ**(同じライブラリの、継承・Deref・trait の疑い) | 0 | 25 | 33 |
|   使われていない(似た名前も IR にない) | 224 | 214 | 149 |

形ごと(行 / 当たった / 当たらず・綴りの疑い・API 不在・別の型の同名メンバ / 当たらず・使われていない):

| 形 | Go | Dart | Rust |
| --- | --- | --- | --- |
| i | 207 / 149 / 0 / 58 | 253 / 127 / 15 / 111 | 329 / 197 / 33 / 99 |
| ii | 40 / 26 / 0 / 14 | 39 / 22 / 3 / 14 | 24 / 11 / 6 / 7 |
| iii | 3 / 1 / 0 / 2 | 29 / 16 / 1 / 12 | 32 / 26 / 3 / 3 |
| pure | 272 / 158 / 0 / 114 | 93 / 50 / 5 / 38 | 82 / 57 / 1 / 24 |
| iv | 73 / 37 / 0 / 36 | 60 / 20 / 1 / 39 | 50 / 27 / 7 / 16 |

綴りの疑いと別の型の同名メンバの例(モデルの鍵 -> IR にある別の綴り。全部は [DEAD.md](DEAD.md)):

- Go
  - (なし)
- Dart
  - `dart:async#Stream.forEach` -> `dart:async#Future.forEach`
  - `dart:async#Completer.sync` -> `dart:async#Future.sync`
  - `dart:async#Stream.error` -> `dart:async#Future.error`
  - `dart:async#StreamIterator.current` -> `dart:async#Zone.current`
  - `dart:async#Stream.timeout` -> `dart:async#Future.timeout`
  - `dart:async#StreamIterator.cancel` -> `dart:async#StreamSubscription.cancel`
  - `dart:async#StreamSink.close` -> `dart:async#EventSink.close`
  - `dart:convert#HtmlEscape.convert` -> `dart:convert#JsonEncoder.convert`
  - `dart:core#num.parse` -> `dart:core#DateTime.parse`
  - `dart:convert#Base64Codec.decode` -> `dart:convert#Codec.decode`
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
| **組み込みの行(全部)** | 599 | 477 | 540 |
| (i) propagator: 値を返すだけ | 207 | 253 | 329 |
| (ii) propagator: 引数・受け手に書く | 40 | 39 | 24 |
| (iii) propagator: コールバック(名前の推定) | 3 | 29 | 32 |
| pure | 272 | 93 | 82 |
| (iv) source / sink / sanitizer / boundary / config_read | 73 | 60 | 50 |
| 呼び出しでない鍵(型・フィールド・変数・marker) | 4 | 3 | 23 |
|   うち source | 4 | 9 | 4 |
|   うち sink | 53 | 38 | 41 |
|   うち sanitizer | 3 | 4 | 0 |
|   うち propagator | 522 | 414 | 467 |
|   うち config_read | 13 | 9 | 5 |

## 6. 読んだもの

| 言語 | 入力(IR) | 実コード | 関数 | 呼び先の名前(種類) | 注記 |
| --- | --- | --- | --- | --- | --- |
| Go | casdoor | はい | 1107 | 1154 | 7b0b426a |
| Go | gin-vue-admin | はい | 1148 | 944 | 7bc7c3051067 |
| Go | go-curated | いいえ(fixture) | 280 | 194 |  |
| Go | gogs | はい | 2953 | 2258 | 8a3b8198a |
| Go | gophish | はい | 395 | 524 | 27d13a05 |
| Go | zoo-go | いいえ(fixture) | 511 | 249 |  |
| Dart | dart-curated | いいえ(fixture) | 340 | 109 |  |
| Dart | dart-pub-tests | はい | 4847 | 2011 | 5e24336e; --include-tests |
| Dart | dart-pub | はい | 2196 | 1628 | 5e24336e; the extractor leaves test/ out (its default, ADR-062) |
| Dart | zoo-dart | いいえ(fixture) | 512 | 209 |  |
| Rust | parseable | はい | 8557 | 4877 | 5c20b8e4; an IR that was made before (not extracted again) |
| Rust | rust-curated | いいえ(fixture) | 386 | 190 |  |
| Rust | zoo-rust | いいえ(fixture) | 709 | 295 |  |

## 7. 読み方の注意

- 「当たった」は、**読んだ IR にその名前の呼び出しがある**こと。当たらなかったことは、行が誤りであることではない(使われていないだけかもしれない)。実コードの入力を増やすと、「使われていない」は減る。
- (iii) の形は名前による推定。モデルの書式がコールバックを表せない(CB-LIST)ので、(iii) の行は、モデルがあっても、コールバックの中を運ばない。
- Rust は、自前のコード(rust-curated)と、すでに IR のある parseable だけを読んだ(C-19)。
