# コードに埋まった API の一覧(完了条件 D(1))

> **状態: 棚卸し(2026-11-27、main `70026385` で調べた)**。完了条件 D(1)の成果物。条件の原文(`.claude/work/QUEUE.md` の「現在のゴール」の表、領域 D の列):
>
> 「(1) コードに埋まった API の一覧を棚卸しして、YAML に移すか課題に。」
>
> 外部の設計者の返事(`2026-11-25-next-goal-languages-syntax-models-reply.md` の領域 D の表)の文言は、「(1) コードにハードコードされた API の一覧(コールバックを呼ぶ API の一覧など)を棚卸しし、YAML に移すか課題にしている。」

ここでいう「API」は、**ライブラリや標準ライブラリの関数・型の名前**で、解析のコードか extractor のコードが、名前を文字列で持っているもの。ルールの YAML(`internal/model/builtin` の `*.yaml` 44 ファイル)に書けば、利用者が足せて、`TestBuiltinRowsAreHitByTheZoo` で一致も確認される。コードに埋まっていると、どちらもできない。この文書は、その名前の表を全部挙げて、各項目に処置(移す / 課題 / 移さない)を付ける。

この文書が扱わないもの: ルールの書式で何が書けないか(別の文書 [YAML-LIMITS.md](YAML-LIMITS.md)、完了条件 D(3))、rivus のコマンドやスキーマの一覧([AGENT-GUIDE.md](AGENT-GUIDE.md)、[ARCHITECTURE.md](ARCHITECTURE.md))。

## 調べ方と限界

`internal/`、`cmd/`、`extractor/rust/src`、`extractor/dart/lib` の、テスト以外のコードを、次の 3 つで探した。(1) `case "..."`、`matches!(.. "..")`、`map[string]` と `[]string{` のリテラル。(2) `std::`、`dart:`、`package:` と、`sort.`、`strings.` のようなパッケージ名のつく文字列。(3) 型名の文字列(`Vec`、`bytes.Buffer`、`context.Context`)。読んで、API の名前の表と、言語の構文の表(演算子、型の大きさの名前)を分けた。**漏れがありうる**(推測): 名前を文字列で持たず、関数名の末尾の一致(`HasSuffix`)や型の部分文字列の一致で持つ形は、見つけたものだけを挙げた。見つけたら、この表に足す。

数え方: 1 項目 = 名前の表 1 つ(関数の名前や型の名前を並べたもの)。「件数」は、表の中の名前の数。

## A. 解析の本体(Go)が持つ表

| # | 場所 | 中身(件数) | 何に使うか | 処置 |
|---|---|---|---|---|
| H1 | (移した)`internal/model/callwarn.go` の `knownCallbackParams` | Go の標準ライブラリで、コールバックを受ける関数と、その引数の数(12): `(*sync.Once).Do`、`sort.Slice`、`sort.SliceStable`、`sort.Search`、`slices.SortFunc`、`slices.SortStableFunc`、`slices.IndexFunc`、`slices.ContainsFunc`、`strings.IndexFunc`、`strings.Map`、`strings.TrimFunc`、`strings.FieldsFunc` | `calls:` のエントリの `with` の数が、コールバックの引数の数と合うかの助言(`CallWarnings`)。結果は助言で、解析は変わらない | **移した**(課題 #243、closed)。12 件に `params` を `go-callbacks.yaml` に書き、表を消した。`(*sync.Once).Do` の 0 を書くため、`CallRow.Params` を `*int` にした(`params: 0` は引数のないコールバック。書いていない行は検査しない)。`TestCallbackParamsKnown`(`internal/model/builtin/callbackparams_test.go`)は、YAML の値を go/types の署名と比べる |
| H2 | `internal/taint/guards.go:92-101` `stdLeafType` | Rust の標準ライブラリで、ほかの書き込み先を持たない型の名前(12): `Vec`、`String`、`VecDeque`、`File`、`Stdout`、`Stderr`、`StdoutLock`、`StderrLock`、`Sink`、`Cursor`、`TcpStream`、`ChildStdin` | `dyn Write` のような呼び出しの候補から、この型を外す(`dispatchSpec.excludes`、`guards.go:76`)。外すと、プロジェクトの `impl Write` の本体を読まない | **課題**。今の書式に、「この型は書き込み先を持たない」と宣言する場所がない(推測: ルールのファイルに型の宣言の節を足す案)。課題 #244 |
| H3 | `internal/taint/calls.go:1148-1150` `contentOnly` | 書き込み先が `bytes.Buffer` か `strings.Builder` のときだけ、書き込みを中身だけに限る(2)。Rust は `isStdType` の全部 | `flows` の `to: recv` で、書き込みが入れ物の全体に届くか、中身だけかを決める(ADR-082) | **課題**(H2 と同じ課題 #244)。書き込み先の型の性質の宣言になる |
| H4 | `internal/taint/callrows.go:564` `isStdType`、`:467` `sameTraitPath` | 標準ライブラリの名前の接頭辞(`std::`、`alloc::`、`core::`、`&std::`、`&mut std::` など 9) | Rust の標準ライブラリの型か、トレイトの同一性か | **移さない**。名前の空間の規則で、API の表ではない。同じ規則が extractor にもある(`extractor/rust/src/lower.rs:219`、`:1525`)ので、3 か所の食い違いに注意(推測: 今は一致している) |
| H5 | `internal/taint/calls.go:808-851` `builtin` | Go の組み込み関数(`append`、`copy`、`min`、`max`、`len`、`cap`、`complex`、`real`、`imag`、`delete`、`clear`、`close`、`print`、`println`、`recover`)と、go/ssa の `ssa:wrapnilchk`、`ssa:deferstack`、`unsafe.String` / `Slice` / `StringData` / `SliceData` / `Add`(合計 22) | 組み込み関数の流れ。知らない名前は、引数から結果への流れとして扱い、uncertainty を出す(`calls.go:857`) | **移さない**。言語の組み込みで、シンボルが `builtin.<名前>` の関数で、ライブラリの API ではない。`min` / `max` は `flows` で書ける形(引数のどれかが結果)だが、`append` と `copy` は入れ物の要素への書き込みで、書式にない([YAML-LIMITS.md](YAML-LIMITS.md) の E1) |
| H6 | `internal/taint/state.go:513` `charReadSymbols`、`:509` `isTextSequence`、`:446` | 文字を 1 つずつ読む API(Dart の `.codeUnitAt`、`String.codeUnits`、`String.runes`、Rust の `::as_bytes`、`std::str::Bytes` / `Chars` / `CharIndices` と `std::iter::Iterator::next`、`iter.start` / `iter.next`)(約 9) | 数や bool に止まる流れを、「文字を 1 つずつ読む形」として数える(`summary.numeric_stops` の数え方だけ。何が流れるかは決めない。コメントに「#185 で消す」とある) | **課題(既存)**: #185。extractor が `CharRead` を IR に付ければ、この表は要らない(Go はすでにそう) |
| H7 | `internal/valuecalls/narrow.go:41` `isFnCall`、`:461` `isDynamicClone` | Rust の `std::ops::Fn::call`、`FnMut::call_mut`、`FnOnce::call_once`、`std::clone::Clone::clone`(4) | 関数値の呼び出しの候補を、クロージャの型で絞る | **移さない**。Rust の言語の機能のシンボルで、IR の約束(extractor が付ける名前)。ただし extractor 側にも同じ名前の表がある(H13)ので、名前は IR の仕様として [ARCHITECTURE.md](ARCHITECTURE.md) に書くのがよい(推測: 今は書いていない) |
| H8 | `internal/report/coverage.go:140` `contextType` | `context.Context`(1) | `rivus models coverage -yaml` が、`context.Context` を書き込みの候補から外す | **移さない**。Go の標準の型で、テンプレートを作る道具の中の規則。`MODELING.md` の 0 節に理由がある |
| H9 | `internal/graphq/path.go:1402` | Go のビルドの設定の鍵 `env.go.GOOS`、`env.go.GOARCH`、`env.go.CGO_ENABLED`(3) | `graph` の答えに、索引を作った設定を書く | **移さない**。API ではない |

## B. Rust の extractor(`extractor/rust/src/lower.rs`)

| # | 場所 | 中身(件数) | 何に使うか | 処置 |
|---|---|---|---|---|
| H10 | `lower.rs:1374` `call_back`(`:1381-1397` と `:1428-1440`) | クロージャを呼ぶ標準ライブラリのメソッド(25): `Iterator` の `for_each`、`map`、`filter`、`filter_map`、`flat_map`、`inspect`、`any`、`all`、`find`、`find_map`、`position`、`take_while`、`skip_while`、`map_while`(14)。`Option` の `map`、`and_then`、`filter`、`inspect`、`is_some_and`(5)。`Result` の `map`、`and_then`、`inspect`、`map_err`、`inspect_err`、`unwrap_or_else`(6)。各メソッドに、レシーバのどの部分をクロージャの引数にするか(`Element`、`Some.0`、`Ok.0`、`Err.0`)と、結果をクロージャの結果にするか(11 件)が付く | extractor が、型でクロージャが分かるとき、その呼び出しを IR に書く(`rust-std.yaml` の `calls:` と別に) | **課題**。`calls:` の `with` に、レシーバの要素(`recv.elem`、`recv.Some.0`)を書く書式がない(MODELING.md の 3 節の「Not yet」。[YAML-LIMITS.md](YAML-LIMITS.md) の L2)。書式ができたら、`rust-std.yaml` に移す。課題 #245 |
| H11 | `lower.rs:185-205` `class_of` | 値の種類(`string`、`container`、`pointer`)に分ける標準ライブラリの型(18): `String`、`Vec`、`HashMap`、`HashSet`、`VecDeque`、`BTreeMap`、`BTreeSet`(各 `std::` と `alloc::` の別名を含む)、`Box`、`Rc`、`Arc` | IR の `class`。ルールの `when`、コピーの扱い、`stdLeafType` が読む | **移さない**。型の分類は IR の約束で、extractor が決める(Dart は `names.dart:81` の `typeClass` が同じ役)。3 言語の表は別々にあり、同じ型の分類が言語で食い違うかは未確認(推測)。言語間の差は [PARITY](../bench/parity/README.md) の台帳で測る |
| H12 | `lower.rs:332-337` `is_smart_pointer` | `Box`、`BoxRaw`、`Unique`、`NonNull`、`Rc`、`Arc`(6) | この型のフィールド(ポインタの仕掛け)を、データとして扱わない | **移さない**(H11 と同じ理由) |
| H13 | `lower.rs:1212`、`:1279`、`:1359-1366`、`:1414`、`:1172`、`:260`、`:288` | `std::ops::Fn::call`、`FnMut::call_mut`、`FnOnce::call_once`、`std::ops::Drop::drop`、`std::iter::Iterator::next`、`std::iter::IntoIterator::into_iter`、`std::arch::asm`(7) | 言語の機能(クロージャの呼び出し、スコープの終わりの `drop`、`for` の展開、インラインのアセンブリ)に、シンボルを付ける | **移さない**。IR の約束(H7 と対になる) |
| H14 | `lower.rs:1356`、`:1630` | クレートの名前 `std`、`core`、`alloc`、`proc_macro`、`test`(5) | 標準ライブラリの本体を読まない(`-no-deps` と別に) | **移さない**。toolchain の構造で、API の表ではない |

## C. Dart の extractor(`extractor/dart/lib`)

| # | 場所 | 中身(件数) | 何に使うか | 処置 |
|---|---|---|---|---|
| H15 | `exprs.dart:456` `_callbackMode` | コールバックを呼ぶ SDK のメソッド(14): `Future.then`(値)、`Stream.listen`、`Stream.forEach`、`Iterable.forEach`、`map`、`where`、`any`、`every`、`expand`、`firstWhere`、`singleWhere`、`lastWhere`、`takeWhile`、`skipWhile`(要素) | extractor が、レシーバが持つ値で、コールバックを呼ぶ呼び出しを IR に書く | **課題**。`dart-core.yaml` に、同じ形の `calls:` が 24 件あるが、ここの 14 件は表で先に処理される(二重)。H10 と同じ書式(レシーバの要素)が要る。#221 の「見直す条件」に、「extractor が表をモデルに移したとき」とある。同じ課題 #245 にまとめる。表にない API(`catchError`、`whenComplete`、`putIfAbsent`、`Map.forEach` など)は #80(open) |
| H16 | `names.dart:12` `_publicLibrary` | `dart:_http` を `dart:io` と呼ぶ(1) | SDK の内部のライブラリの名前を、利用者が import する名前にそろえる | **移さない**。シンボルの綴りの規則(ルールの YAML が読む名前)で、API ごとの振る舞いではない。増えるなら、ルールの書式の「別名」にする(推測) |
| H17 | `names.dart:81-111` `typeClass` | `dart:core` の型の分類(`bool`、`String`、`int`、`double`、`num`、`List`、`Set`、`Map`、`Iterable`、`Iterator`、`Object`、`Null`、`Type`、`Symbol`、`Enum`、`Record` の 16)と、`dart:async`、`dart:collection` の全体 | IR の `class` | **移さない**(H11 と同じ) |
| H18 | `exprs.dart:414`、`:488`、`:495`、`lower.dart:422`、`:429` | `dart:core#dynamic.invoke`、`iter.start`、`iter.next`(3) | `dynamic` のメソッド呼び出しと、`for-in` の展開に付けるシンボル | **移さない**。IR の約束。`dynamic.invoke` のモデルは `dart-core.yaml:18` にある(YAML に出ている分) |

## D. Go の frontend

| # | 場所 | 中身(件数) | 何に使うか | 処置 |
|---|---|---|---|---|
| H19 | `internal/frontend/goir/ops.go:103-112` | go/ssa の組み込み関数に、`builtin.<名前>`(`unsafe` のものは `builtin.unsafe.<名前>`)のシンボルを付ける | H5 の入力 | **移さない**(H5 と対) |

## E. 数えなかったもの(API の名前ではない)

調べた表のうち、次のものは、API の名前でなく、言語の構文や、道具の設定なので、一覧に入れなかった。

- 演算子と型の大きさの名前: `internal/taint/provfold.go`(整数の型の名前と演算子)、`internal/taint/guards.go:656` `comparisons`、`internal/ir/validate.go:10` `shapes`(命令の形)、`extractor/rust/src/lower.rs:395` `bin_op`、`extractor/dart/lib/exprs.dart` の演算子。
- コマンドの値の表: `cmd/rivus/values.go`(`-lang` と並べ替えの値)、`cmd/rivus/main.go:122` `oldIsolationFlags`、`cmd/rivus/emptyinput.go:32` `sourceSuffix`。
- 隔離の設定: `cmd/rivus/isolate_sandbox_darwin.go:68`、`isolate_sandbox_linux.go:75-83`(許可するプログラムとディレクトリ)、`cmd/rivus/isolate_sources.go:22` `cratesIOSources`。
- 文書の検査の語彙: `internal/docsgen/issues.go:18-21`、`internal/docsgen/adr.go:14-17`。

## 件数と処置

| 処置 | 項目 | 件数 |
|---|---|---|
| 移した(YAML に。H1 は #243 で、`params` を 0 でも書けるように 1 点だけ直した) | H1 | 1 |
| 課題(書式か仕組みが要る) | H2、H3(#244)、H10、H15(#245)、H6(#185、既存) | 5 |
| 移さない(言語や IR の約束、設定。理由つき) | H4、H5、H7、H8、H9、H11、H12、H13、H14、H16、H17、H18、H19 | 13 |
| 合計 | | 19 |

新しい課題は 3 件: #243(H1: `params` を YAML に書く)、#244(H2、H3: 型の性質の宣言)、#245(H10、H15: レシーバの要素を渡す `calls:`)。残りの項目は、既存の課題か、「移さない」の理由が項目に書いてある。

### 前のコードとの関係(参考)

- モデルのファイルは、`.dead-allow` と `TestBuiltinRowsAreHitByTheZoo` で、zoo のセルとの一致を確認している。この文書の表の項目には、その確認がない。H1 は、移したので、その対象になる(`params` を足した行は、もとからある `calls:` の行)。
- `internal/model/builtin` の `calls:` は、`go-callbacks.yaml` 15 件、`rust-std.yaml` 24 件、`dart-core.yaml` 24 件、`dart-async.yaml` 8 件など。コールバックを呼ぶ API の表の大半は、すでに YAML にある。表に残ったのは、**レシーバが持つ要素を、コールバックの引数にする**形だけ(H10、H15)。
