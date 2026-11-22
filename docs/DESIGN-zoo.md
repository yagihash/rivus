# 設計: zoo(意図的に脆弱性を作り込んだベンチ)と wild(実世界の脆弱性を育てるベンチ)

> **状態: 一部実装**(確認: 2026-11-22、9c26a39)
> - 実装済み: 段階 1〜4(`bench/zoo`、`bench/zoo-go`、`bench/zoo-dart`、`bench/zoo-rust`、`TestZoo*`)。
> - 未完: 段階 5(組み合わせ)と表 S の全体。`wild` は Go の 3 件のみで、`TestWildStagesInZoo` は無い。
> - 注意: `@cell(` を含むファイルは Go 51、Dart 55、Rust 69(設計の見積りは Go 80 弱)。zoo 専用の ADR は無い。
> - 使い方の正本は [bench/zoo/README.md](../bench/zoo/README.md) と [bench/wild/README.md](../bench/wild/README.md)。

設計時の状態の記述(今の状態は上の枠が正本): **案(設計だけ。実装はしていない)**。アーキテクチャ助言者との合意の骨子を、実装できる粒度に書いた。この文書は公開リポジトリに置かれ、issue や ADR から参照される。外部の人が読む前提で書く(個人情報、手元のパスは書かない)。

## 0. 要約

- **zoo** は、「流れの形」を体系的に並べた、小さな、動くコードのベンチ。`bench/zoo-go`、`bench/zoo-dart`、`bench/zoo-rust`。
- **正解は、注釈を実行で裏付けたもの**。source は目印の文字列を返す関数、sink は受け取った引数を記録するだけの関数(本物の exec や SQL は呼ばない)。ケースを実際に動かし、「目印が sink に届いた / 届かなかった」を観測し、それがソースの注釈(`@sink(tag)` / `@sink(none)`)と一致することを CI で見る。**rivus の出力を見て注釈を合わせることはできない**。rivus が出せないものは `@miss`。緑にするために注釈は変えない。
- **1 マス = 3 ケース**: 脆弱な版(`@sink`)、直した版(`none`)、おとり(同じ形で、汚染された値が別のオブジェクト・引数・フィールド・ハンドラに行く。`none`)。
- **立方体にしない**: 表 F(流れの形、言語ごとに手で書く)、表 S(伝播のモデル = `pure` / propagator / sanitizer の 1 行ごと。機械で生成。正解は YAML からでなく実行から。source / sink のルールは対象外)、少しの組み合わせ(形 × 族の被覆)。
- **wild** は、実世界の脆弱性を 1 件ずつ足して育てるベンチ。1 件に 2 つの形(W1 = 本物の切り出し、W2 = 経路の骨組みの書き起こし)を持つ。流れの向きは wild → zoo(wild で見つけた新しい形は、zoo にマスとして足す)。
- 段階: (1) Go の仕組みと 20 形、(2) Dart の仕組みと 20 形(同時)、(3) 残りと表 S、(4) Rust、(5) 組み合わせ。**実行での裏付けを先に作る**。

## 1. 何のためか、何でないか

今の curated(`bench/go-curated`、`dart-curated`、`rust-curated`)は、実プロジェクトの形と CVE の形を、注釈つきで小さく写したもの。注釈は人間(と AI)が書いた。実際の運用で分かったこと:

- 注釈は間違う。全体レビュー(2026-11-21)の R-03(組み込みの `pure` に、入力を出力に伝える関数がある。`(*url.URL).RequestURI`、`(time.Time).Format` など)は、「モデルが正解を決める」構図の誤りだった。モデルや rivus の出力から注釈を作ると、同じ誤りが正解に入る。
- 体系がない。どの形が試されていて、どの形が試されていないかが、一覧にならない。今日のレビューの見逃しは、形の抜け(CORE-J のポインタレシーバ、R-14 の tear-off、R-17 の Drop など)だった。
- デグレの網として、実コード(gogs など)は「正解のない差分」にしかならない。ラベルつきの小さい網がもう 1 つほしい。

zoo は、それを埋める。**zoo が見ないもの**は §10 に書く。

**設計の目的(1 文)**: この設計の評価の中心は、**実在の脆弱性を発見するための経路情報(source → 途中の関数 → sink)を、エンジンが組み立てられるか**である。ルールは差し替えられる入力。ただし**標準ライブラリとデファクトスタンダードのライブラリのモデルは、整備の対象**で、その質(伝播、source、sink)も見る。

**線引き**: 整備する = 標準ライブラリと、デファクトスタンダードのライブラリのモデル(言語を問わない。基準は §4.3a)。軸にしない = 特定のプロジェクト固有のラッパーのルール、特定の CVE に合わせたルール(ベンチ用のルールとして、試験の入力に置く)。範囲の外の API にルールがないだけの不検出は、エンジンの見逃しに数えない。範囲の中の API(標準とデファクト)にモデルがないのは、見逃しとして数える。標準ライブラリの伝播のモデル(`pure` / propagator / sanitizer)の誤りは、経路を切るので、**優先**(R-03)。

## 2. curated との関係(決めること a)

- 別のディレクトリ: `bench/zoo-go`、`bench/zoo-dart`、`bench/zoo-rust`。curated は、「実プロジェクトの形」と「CVE の形」を持つものとして、そのまま残す。
- **注釈の書き方と採点の仕組みは 1 つ**にする。今の `cmd/rivus/bench_curated_test.go` の `checkCurated`(`checkCuratedIR` / `checkCuratedReport`)を `checkAnnotated(dir, options)` に一般化し、`TestCuratedGo/Dart/Rust` と `TestZooGo/Dart/Rust` と `TestWild*` が同じ関数を呼ぶ。zoo 用の追加(マス・役割の検査、`@ref` の検査、実行との照合)は options で足す。curated の既存の振る舞いは変えない(カウントの下限、`@miss` / `@fp` / `@noisy` の両側検査、`offSourcePath` の上限)。
- curated のケースが、zoo のマスに当たるとき(例: `go-curated/api/capture.go` の `cap-run-a` / `cap-run-b` は、`closure/two-callers` のマス)は、注釈に `@cell(<マスの名前>)` を付ける。テストは「マスごとに、curated のどのケースが当たるか」の対応表をログに出す(重複を削る判断の材料。強制はしない)。
- 新しい見逃しや誤検出が見つかったときの置き場の順: **最小の形は zoo のマス、実プロジェクトの形が要る話だけ curated、実世界の再現は wild**。

## 3. 実行での裏付け(設計の中心)

### 3.1 原則

1. 注釈(正解)は、ケースを**実行した結果**と一致していなければならない。一致しなければ、`TestZooTruth*` が落ちる。
2. 注釈を直すときの根拠は**実行の結果だけ**。rivus の出力、モデルの YAML、人の直感は根拠にならない。rivus の出力と注釈が違うなら、それは `@miss` か `@fp`(エンジンの課題)であって、注釈の誤りではない。
3. `@miss` と `@fp` には、理由と参照(`@ref(R-03)`、`@ref(#101)`、`@ref(CORE-J)`)が要る。参照のない `@miss` は落とす。
4. 実行で確かめられない形(並行の順、実際の時間、外部の入力)だけ、手の注釈 `@manual(理由)` を許す。数に上限を置く(§3.8)。
5. 緑にするためだけに注釈を変えた疑いは、CI の「真理の変更の一覧」(§3.9)で、レビューに見える形にする。

### 3.2 目印と記録の仕様(3 言語で共通)

**目印(marker)**。`src(tag)` が返す文字列。`tag` はケースごとに一意(`[a-z0-9-]+`)。目印は、sink のクラスごとの**セグメント**の連結:

```
Z<tag>.SH[;|$]  Z<tag>.SQL[']  Z<tag>.HTML[<>&]  Z<tag>.PATH[../]
```

(実際は区切りなしの連結。見やすさのために空白を入れた)。クラスは `Cmd`(SH)、`SQL`、`HTML`、`Path`、`Any`。`[...]` の中が、そのクラスにとって**危険な並び**。

**「届いた(reached)」の定義: 目印の全体の一致ではなく、危険な部分の生存**。`strings.ToUpper`、`s[3:]`、`TrimPrefix`、`Split` → `Join`、無関係な文字の `Replace` で tag の部分が変わっただけでは、reached でなくならない。

- reached(クラス)= **そのクラスの危険な並び(`[` `]` の中身)が、連続して無傷で、sink に届いた**こと。
- **どの tag かの帰属**: (a) tag の文字が残っていれば、それで決める。(b) 残っていなければ、**variant 1** で危険な並びが variant 1 のものに入れ替わること(差分)で決める。variant 0 と 1 を別プロセスで走らせ、sink が受け取った文字列を比べる。
- **クラスごとの判定は、文字の一致で足りないものがある。判定を `spec.json` にクラスごとに 1 つずつ書く**(下)。

| クラス | reached の判定(意味) | 文字の一致で足りない理由 |
| --- | --- | --- |
| SH | `;` `\|` `$` の並びが連続して無傷で、**引用の外**に届く | 引用符の中に入ったものは、危険でない(`quote` のサニタイザ) |
| SQL | 引用符 `'` が、**文字列リテラルを閉じる位置**に届く(前に `\` や `''` の対がない) | `''` に置き換えたものは、閉じない |
| HTML | `<` が、**エスケープされずに**残る(`&lt;` になっていない) | `&` を含む並びは、エスケープで形が変わる |
| Path | sink は、**基準のディレクトリ**(`base`)を受け取り、`Clean(Join(base, 値))` が**基準の外に出る**、または `../` の並びが残る | `filepath.Join` と `Clean` は `x/../` を畳む。危険な並びは消えるが、結果は基準の外に出ている(本物の横断) |
| Any | 上のどれか | |

設計の段階で、クラスごとに「文字の一致」と「意味」が食い違う入力の表(`spec.json` の `probes`)を先に書き、3 言語の `mark` が同じ答えを出すことを自己検査する。

**サニタイザ**(`CleanCmd` など)は、そのクラスの危険な文字の集合を `_` に置き換え、reached にならなくする。HTML のエスケープのあと `SinkCmd` に行く形は、SH の並びが無傷なので、reached(= 出るべき。「種類の違うサニタイザ」)。

**carry(届いてはいないが、依存はある)**。差分(variant 0 と 1)で、受け取った文字列が危険な並びの中身に依存して変わるが、reached でないもの。**taint 解析が追うのはデータの依存であって、「危険な文字がそのまま残るか」ではない。宣言されたサニタイザを通っていない依存は、出るのが既定**。carry を採点の外に置くと、hex、base64、URL エスケープ、大文字化、部分文字列、書式を通る流れが「どちらでもよい」になり、`pure` に化ける退行(R-03: `hex.AppendEncode`、`(time.Time).Format`)を zoo が見逃す。だから carry は**種類に分け、種類ごとに期待を決め、採点に入れる**:

| 種類 | 例 | 期待(rivus) | 注釈 |
| --- | --- | --- | --- |
| `carry(encode)` | hex、base64、URL エスケープ、引用、大文字・小文字、切り出し、置換、書式 | **出るべき**。出なければ `@miss`(`@sink` と同じ扱い) | `@carry(tag, encode)` |
| `carry(digest)` | ハッシュ、HMAC | **方針の行**。今の rivus の動き(出る / 出ない)を注釈に固定する | `@carry(tag, digest)`(出ないなら、その行に `@nosink(tag)` を併記して固定) |
| `carry(numeric)` | 長さ、`Atoi`、比較の結果 | **出ないべき + `numeric_stop` の不確実性の注記** | `@carry(tag, numeric)` |

どの種類も、**今の rivus の出方を注釈に固定する**(`encode` は出るのが正で、出なければ ratchet つきの `@miss`)。方針の行(digest)も、変わったら「真理の変更の一覧」(§3.9)に出る。**採点の外に置くものを作らない**。見出しの数字には carry の内訳を足す(§6.3)。

**3 つの結果と注釈**: `reached`(`@sink(tag)`)、`none`(`@sink(none)`、`@nosink(tag)`)、`carry`(`@carry(tag, 種類)`)。

表 S のサニタイザの行などが、「宣言と実際の食い違い」を見つけるのは、ここ(sanitizer と宣言されているのに、危険な並びがそのまま届く = 脆弱。rivus が出さなければ `@miss`)。

**記録(hit)**。sink を呼ぶたびに 1 件: `(呼び出し位置(file, line)、クラス、見えた tag の一覧、そのうち reached の一覧、受け取った文字列の写し(差分用))`。呼び出し位置は、sink を呼んだ行(Go は `runtime.Caller(1)`、Dart は `StackTrace.current` の 2 番目の frame、Rust は `#[track_caller]` と `Location::caller()`)。**注釈は sink を呼んだ行に付ける**(今の curated と同じ。rivus の finding の位置と一致する)。型のある sink は、受け取った文字列をそのまま記録する。`SinkAny` の値だけ、深く歩く(Go は reflect で、非公開フィールド、ポインタ、スライス、map のキーと値、構造体。Dart はケース側の `toString`。Rust は `Debug` の出力。目印に `"` と `\` を使わないので、Debug の出力で壊れない)。深さの上限 8、循環の検出つき。

**記録は、ケースごとに初期化しない**。tag と行が、ケースを一意に決める。順序の依存は、逆順で実行して、観測が同じことで検査する(§3.6)。

### 3.3 Go: `zoo/mark`

```go
package mark // module zoo (bench/zoo-go/go.mod)

type Class uint8

const (Any Class = iota; Cmd; SQL; HTML; Path)

// source。目印を返す。tag はケースで一意。
func Src(tag string) string
func SrcBytes(tag string) []byte

// source その 2: 引数の型による。rules.yaml の param_type に zoo/mark.In を置く。
type In struct{ v string }
func NewIn(tag string) In
func (i In) Value() string

// sink。**既定は型のある sink**。受け取った文字列を記録するだけ。何も実行しない。
func SinkCmd(s string)        // 同じ形で SinkSQL, SinkHTML。
func SinkCmdBytes(b []byte)   // バイト列
func SinkPath(base, s string) // Path は基準のディレクトリを受け取る(§3.2)
// SinkAny(v any) は、構造体やコンテナを渡す形のマスだけが使う(reflect で歩く)。
func SinkAny(v any)
// サニタイザ。そのクラスの危険な文字の集合を `_` に置き換える。
func CleanCmd(s string) string   // 同じ形で CleanSQL, CleanHTML, CleanPath

// ケースの登録。id は "F/call/depth3/vuln" の形。
func Case(id string, fn func())
```

ケースは 1 マス = 1 ファイル(または 1 パッケージ)。

```go
// bench/zoo-go/cells/call/depth3/depth3.go     @cell(F/call/depth3)
package depth3

import "zoo/mark"

func init() {
	mark.Case("F/call/depth3/vuln", vuln)
	mark.Case("F/call/depth3/fixed", fixed)
	mark.Case("F/call/depth3/decoy", decoy)
}

func f1(s string) { f2(s) }
func f2(s string) { f3(s) }
func f3(s string) { mark.SinkCmd(s) } // @sink(call-depth3-vuln)  ← 実行で裏付けられる

func vuln() {
	v := mark.Src("call-depth3-vuln") // @source(call-depth3-vuln)
	f1(v)
}

func fixed() { // @fixed
	v := mark.CleanCmd(mark.Src("call-depth3-fixed")) // @source(call-depth3-fixed)
	g1(v)                                              // g1..g3 は上と同じ形、別の関数
}
// g3 の sink 行: mark.SinkCmd(s) // @sink(none)

func decoy() { // @decoy
	t := mark.Src("call-depth3-decoy")       // @source(call-depth3-decoy)
	h1(t, "constant")                        // h1(a, b) は b だけを深さ 3 で sink に渡す
}
// h3 の sink 行: mark.SinkCmd(b) // @sink(none) @nosink(call-depth3-decoy)
```

規約:

- **ケースの関数は引数なし、戻り値なし**。状態はケース専用の変数(`g_<ケース名>_`)だけを使う(ケース間で共有しない。lint で検査)。
- 同じ sink の呼び出し行を、複数のケースで共有してよいのは、形がそれを要求するとき(= 共有そのものが形。例: `dyn/xtalk-n-by-m`)だけ。
- おとりは、**汚染された値の行き先を持つ**: 汚染された値は別の場所に行く。別の sink 行(`@sink(tag)`)に届くのが望ましい(エンジンが全部切る、という退化を、おとりが見逃さないため。これは「共存する流れ」と呼ぶ)。判定の対象の行が `@sink(none)`。
- 非同期(goroutine、`select`)は、ケース自身が待つ(`sync.WaitGroup`)。待てないものは `@manual`。
- **sink の既定を `any` にしない**。`SinkCmd(v any)` が既定だと、全ケースの最後の段が make_interface + reflect の深い走査になり、その扱いの誤り(CORE-J の近く)が、全マスを一斉に赤か緑に変える。`SinkAny` は、構造体やコンテナを渡す形のマスだけ。**「構造体を丸ごと sink に渡したら、中のフィールドの汚染は届いたとみなすか」は、それ自体を 1 つのマス**(`F/cont/struct-whole-to-sink`)にする。
- **解析が見る `mark` は、本体のない宣言だけ**。`mark` の本体(記録のグローバルへの追記、reflect)を rivus が読むと、全ケースの値が 1 つのグローバルに書かれる合流点になる(今日の `os.Stderr` と同じ形)。Go: ビルドタグ。既定(解析)は空の本体(`func SinkCmd(s string) {}`)、実行のときだけ `-tags zooexec` で本物の記録。rivus は、ルールだけで source / sink / sanitizer を与える(`symbol: zoo/mark.SinkCmd` など)。Dart / Rust も、解析の対象から `zoo_mark` / `zoo-mark` の実装を外す(実装は別ファイルにし、rivus の入力のパターンから外す。公開の宣言だけを解析に見せる)。解析の結果が、`mark` の実装の有無で変わらないことを、段階 1 で確かめる。
- harness(`cmd/observe`)は解析の対象に含めない(登録の `fn()` は動的な呼び出しで、全ケースの汚染源になる)。**登録(`init` で関数値をグローバルの表に入れる)**は、全ケースの関数が「保存された関数値」になる。C-7(`assumed_call`)や `Reachable` に触れないこと(捕獲がないので当たらないはず)を、段階 1 の最初の表で確かめる。
- 入口の形で `In`(引数の型の source)を使うとき、ケースの関数は引数なしなので、内側のハンドラ `h(in mark.In)` をケースが呼ぶ形になる(`h(mark.NewIn("tag"))`)。rivus から見ると source は `h` の引数、実行では `NewIn(tag)` の値で、**両者が同じ tag で対になる**。これを、`call/direct` の次の 1 マス(`F/call/direct-in`)で先に確かめる(CORE-A の N×M は、この形でしか出ない)。

### 3.4 Dart: `package:zoo_mark`

```dart
// bench/zoo-dart/packages/zoo_mark/lib/mark.dart
String src(String tag);                 // source その 1
class In { In(String tag); String get value; }   // その 2: rules.yaml の param_type
void sinkCmd(String s);                 // 同じ形で sinkSql, sinkHtml, sinkPath(base, s)。型のある sink が既定
void sinkAny(Object? v);                // 構造体やコンテナを渡す形のマスだけ
String cleanCmd(String s);              // 同じ形で cleanSql ...
void zooCase(String id, FutureOr<void> Function() fn);   // 登録
```

- 呼び出し位置は `StackTrace.current` を解析して、`package:zoo_dart/` を `lib/` に戻す。
- `sinkAny` の値の歩き方: **既定は、ケース側の `toString`**(`dart:mirrors` は将来の SDK で使えなくなる可能性があるので、依存を減らす)。`sinkAny` に渡すケースの値の型は、`toString` で全フィールドを出す。`dart:mirrors` は、`toString` を書けない型の補助に限り、使うときは理由を書く。
- 解析が見る `zoo_mark` は宣言だけ(実装は別ファイルで、rivus のパターンから外す)。
- async のケースは `await` される。`Isolate` のケースは、isolate の中の hit を `SendPort` で親に送り、`mark.relay` で記録に足す。
- **Flutter の本体は入れない**(SDK が重く、`pub get` が不安定)。`build()` / `onPressed` / `initState` は、50 行ほどの自前の `StatefulWidget` 相当と、「押す」を呼ぶ harness で、形だけを写す。限界は §10 に書く。

### 3.5 Rust: クレート `zoo-mark`

```rust
// bench/zoo-rust/zoo-mark/src/lib.rs
pub fn src(tag: &str) -> String;                       // source その 1
pub struct In(String);  impl In { pub fn new(tag: &str) -> In; pub fn value(&self) -> &str; }
#[track_caller] pub fn sink_cmd(s: &str);   // sink_sql, sink_html, sink_path(base, s)。型のある sink が既定
#[track_caller] pub fn sink_any<T: std::fmt::Debug + ?Sized>(v: &T);   // 構造体やコンテナを渡す形のマスだけ
#[track_caller] pub fn sink_cmd_own(s: String);                        // 所有権を渡す形(移動の形のため)
pub fn clean_cmd(s: &str) -> String;                   // clean_sql ...
// 解析が見る zoo-mark は宣言だけ(実装は feature `exec` のときだけ。解析の対象から外す)
pub fn case(id: &'static str, f: fn());                // 登録(明示的な一覧 `pub fn cases() -> Vec<..>`)
```

- ワークスペース: `zoo-mark`(記録)、`zoo-cases`(ケース。マスごとに `mod`)、`zoo-run`(harness、`observe` のバイナリ)。ケースごとのテストは `libtest-mimic`(`harness = false`)で、登録 1 件を 1 つのテストにする。
- 登録の一覧は明示的に書く(`inventory` やマクロを使わない: マクロは解析する側の形になり、形 `macro` を汚すため)。登録に漏れたケースは、`@cell` 注釈の関数との照合で見つかる(§3.6)。

### 3.6 実行と照合の流れ

1. 各言語の harness が、登録された全ケースを実行する。**variant 0、variant 1、variant 0 の逆順**の 3 回(別プロセス)。出力は `observed.json`: `{ ケース id: [ {file, line, class, seen: [tag], reached: [tag], carry: [tag]} ] }`。逆順の観測が variant 0 と違えば、ケース間の依存として落とす。
2. 言語ごとのテストが、それを走らせる。Go: `go test`(`t.Run` のサブテストが、ケースごとの小さいテスト)、Dart: `dart test`(登録から `test()` を作る)、Rust: `cargo test`。各テストは、ケースが panic せず、記録が空でないことを見る。
3. rivus 側のテスト `TestZooTruth{Go,Dart,Rust}`(`cmd/rivus`)が、`observed.json` と、**ソースの注釈**を照合する(注釈の構文解析は、既存の 1 つのパーサだけ。3 言語で書き直さない)。規則:
   - `@sink(tag)` の行: その行の hit に、`reached` の tag がある。carry だけなら、注釈は `@carry(tag, 種類)`(種類ごとの期待は §3.2)。
   - `@sink(none)` の行: その行の hit が **1 件以上ある**(実行されていること。実行されていない `none` は、空の合格)で、`reached` も `carry` も空(`carry(digest)` で `@nosink` を併記した行を除く)。
   - `@nosink(tag)`: その行の `reached` と `carry` に tag がない。
   - 実行で reached の hit がある行に、注釈がなければ落ちる(注釈の抜けも許さない)。実行されない注釈の行も落ちる(`@manual` を除く)。
   - `@miss` / `@fp` の行: `@miss` は実行で reached(流れは本物)、`@fp` は none。
   - **`@sink(none)` は、ケースの全部の枝を実行したときだけ正解**。実行は 1 本の経路の証人で、「届かなかった」は「その実行では届かなかった」でしかない。だから照合の条件に、**ケースのコード(`cells/`)の文のカバレッジが 100%** を入れる(Go は `-cover` と `-coverprofile`、Dart は `--coverage`、Rust は `-C instrument-coverage`。段階 1 は Go と Dart)。実行されない文があるケースは落とす(`@manual` を除く)。枝のある形(`guard/one-branch`、`allowlist-early-return`、`ctl/type-switch`、`select`)は、**ケースが両方の枝を自分で通る**(条件を変えて 2 回呼ぶ)。観測は「どれかの実行で届いた」を reached とする。
   - **非決定な形**(goroutine の順、map の走査順)は、variant 0 を数回(5 回)走らせて、観測が同じことを見る。違えば `@manual`。
   - 各マスは、`@vuln`(または役割なしの本体)、`@fixed`、`@decoy` の 3 つを持つ。`@cell` の名前が表と対応する。表 F にない `@cell` と、`@cell` のない表 F の行は、落ちる。
4. そのあと、rivus を `rules.yaml` で走らせ、`checkAnnotated` の既存の検査(出る・出ない・`@miss` / `@fp` の両側検査、不確実性の注記の検査)を行う。**失敗したマスは、そのマスだけで解析し直して**(`./cells/<family>/<shape>/...`)、「単独では緑 / 赤」をログに出す(全体の中での混ざりと、そのマスの誤りを分けるため)。

### 3.7 `@miss` の書き方(zoo の追加)

```go
x := Src("a") // @source(a) @miss(silent) @ref(R-03)
```

- `@miss(silent)` と `@miss(<kind>)` は curated と同じ(§不確実性の注記)。zoo は、`@ref(...)` を必ず要求する。
- **`@miss(norule)`**: **範囲の中の API**(標準ライブラリとデファクトのライブラリ、§4.3a)に、モデルがなくて経路が出ないもの。エンジンが不確実性の注記を出しようがないので、`silent` とは別の欄で数える。`@ref(CWE-78,gosec:G204)` のように、外の一覧の見出しを付ける。**範囲の外の API(プロジェクト固有のラッパー、特定の CVE のためのルール)は、`@miss` ではない**(ケースにそのルールを与える。ルールは試験の入力)。それでも経路が出ないときだけ、`@miss`。
- 表の列に出す数(§6.3 の見出し)は、`silent` / 注記あり / `norule` の 3 つ。

### 3.8 手の注釈 `@manual(理由)`

実行で確かめられない形(並行の順、実際の時間、外部の入力)だけ。sink の行に `@sink(tag) @manual(<理由>)` と書くと、その行の実行との照合を免除する(行の存在と、rivus との照合は残る)。上限: ケース全体の 3% と、マスごとに 1 つ。一覧を報告に出す。理由が「面倒」なものは受け付けない(レビューで落とす)。

### 3.9 真理の変更の一覧

`@sink(x)` と `@sink(none)` の入れ替え、`@miss` / `@fp` の削除、`@carry` の追加は、**エンジンの変更がなければ起きないはず**のもの(`@miss` の削除は、エンジンが直ったときだけ)。CI が、ベースとの差分から、これらを「真理の変更」として列挙し、PR の要約に出す(強制ではなく、見えるようにする)。実行の結果が変わる変更(ケースのコードの変更)と、注釈だけの変更を、別に数える。

**「ケースを弱くして緑にする」を監視する**: 既存のマスのコードを変え、かつ `@miss` / `@fp` が減るコミットは、「形の変更」として一覧の**先頭**に出す。`internal/` と既存の `cells/` を**同時に**変えるコミットも同じ。そのために、**各マスは、形を 1 行で表す説明**(`@shape: インターフェース越しのポインタレシーバのフィールドを読む`)を持つ。形の変更は、説明が変わらないか、変わった理由が書かれていること、をレビューで見る。

## 4. マスと表

### 4.1 1 マス = 3 ケース

| 役割 | 内容 | 注釈 | 何を見るか |
| --- | --- | --- | --- |
| 脆弱 | source の値が、その形を通って sink に届く | `@sink(tag)` | 見逃し(`@miss`) |
| 直した | 前(または途中)でサニタイザ / ガードを通す | `@sink(none)` | 誤検出、サニタイザの効き |
| おとり | 同じ形。汚染された値は、別のオブジェクト・別の引数・別のフィールド・別のハンドラに行き、判定の行には届かない。共存する流れを持つ | `@sink(none)`(+ 共存する流れは `@sink(tag)`) | 混ざり: XTALK、N×M、tear-off の receiver と引数の入れ替わり、構造体全体への持ち上げ |

source は 2 種(戻り値: `src(tag)`、引数の型: `In`)。**既定は戻り値**。入口の形(ハンドラ、receiver、グローバルを書く入口、クロージャ)の約 20 形は、2 種の両方を持つ。

### 4.2 表 F(流れの形)

形ごとに手で書く。sink は代表 1 つ(`SinkCmd`。記録するだけ)。言語ごとに、その言語らしい形で書く(Go の写しにしない)。ID は `F/<族>/<形>`。「参照」は、今日のレビュー(R-xx)と backlog の項目、バックテストの行(#6 など)。**太字は段階 1(Go)の 20 形**。

#### 4.2.1 Go

| 族 | 形(ID の末尾)と、おとりの入れ替え | 参照 |
| --- | --- | --- |
| call | **direct**、depth1、**depth3**、depth10、**file**(ファイル越し)、pkg(パッケージ越し)、**variadic**(可変長に束ねて展開。おとりは別の位置)、**multiret**(複数の戻り値。おとりは別の戻り値だけを使う)、named-ret-defer(名前付き戻り値を defer が書く)、recursion、mutual-rec | |
| dyn | **iface-val-recv**(値レシーバ)、**iface-ptr-recv-read**(ポインタレシーバのフィールドを読む)、iface-ptr-recv-write(書く)、**sink-in-method**(sink がメソッドの中)、embed(埋め込み。おとりは外側のフィールド)、stdlib-writer-own(`io.Writer` の自前の実装)、stdlib-error-own(`error` の自前の実装)、http-middleware-chain(`http.Handler` の連鎖)、**method-value**(メソッド値。おとりは receiver と引数の入れ替え)、method-expr、func-returning-func、**func-field**、func-map、func-slice、**xtalk-n-by-m**(N 個のハンドラが同じ型の引数を共有。おとりは別のハンドラ) | R-01(CORE-J)、R-04、バックテスト #7 |
| clos | **inline**(その場で呼ぶ)、**to-own-helper**(自分のヘルパーに渡す)、to-lib(`sort.Slice`、`sync.Once` など)、**returned**(返す)、stored(保存して後で呼ぶ)、capture-write-by-ref(参照で捕獲して中で書く)、loop-var(ループ変数)、two-callers(同じヘルパーに、2 つの呼び出し元が別のクロージャ。おとりは相手のクロージャ) | C-7 |
| state | **global-direct**、global-accessor、global-ptr、global-field(構造体のフィールド)、**save-restore**(保存して戻す。おとりは別のフィールド)、global-init(`init`)、**handler-then-reader**(ハンドラが書いて、別の入口が読む)、struct-copy(構造体の丸ごとのコピー)、deep-field(アクセスパス 4 段以上)、alias(別名) | CORE-H(#6)、C-7 の雑音(#8)、R-19 |
| cont | **slice**、map-key-vs-value(2 つのキーで別の値。おとりは別のキー)、chan、array、range、append-copy、sync-map、atomic-value、ctx-value(`context.WithValue`)、map-two-keys | |
| ctl | defer、go、select、panic-recover、type-switch、type-assert、err-wrap(エラーの包み)、range-over-func(返されたイテレータ)、builtin-unsupported(未対応の builtin) | R-08、R-18 |
| conv | concat、sprintf、builder、bytes-string(`[]byte` と string)、**buffer-writestring**(`io.WriteString(&b, s)` と `b.String()`)、buffer-fprintf、json-decode、reflect-set、reflect-typeof-decoy(型の記述子だけでつながる組。おとり)、generics(`*G[string]`)、stderr-logger-decoy(`os.Stderr` を通るロガー。おとり) | R-02(CORE-K)、バックテスト #1、#3 |
| guard | **sanitize-before**、**one-branch**(片方の枝だけ。出るべき)、resanitize-after-taint(サニタイズのあとで再び汚す。出るべき)、allowlist-early-return、const-cond(定数の条件のガード)、guards-5plus(ガード 5 個以上)、kind-mismatch(HTML のエスケープのあとで `SinkCmd`。出るべき)、sanitize-copy-only(コピーだけをサニタイズ。出るべき) | R-30、R-32 |

形の数: 80 弱。ケースは形 × 3 に、source の 2 種目の分(約 30)を足して 240 前後。**段階 1 の形**: call/direct、**direct-in**(`In` の対。`direct` の次)、depth3、multiret、dyn/iface-val-recv、iface-ptr-recv-read、sink-in-method、method-value、xtalk-n-by-m、**stdlib-writer-own**、**http-middleware-chain**(この 2 つは R-04。未修正の第 1 群を全部入れる)、clos/inline、to-own-helper、returned、state/global-direct、save-restore、handler-then-reader、cont/slice、map-key-vs-value、conv/buffer-writestring、guard/sanitize-before、one-branch。**後ろに回してよい順**: file、variadic、func-field。(表の太字は、この一覧に合わせて読み替える。)

**R-03 の先出し**: 表 S(段階 3)を待たず、**`pure` の行だけの実行の検査**を段階 1 で切り出す(引数が文字列 / バイト列で、呼ぶだけで安全な行に限る)。R-03 の修正の受け入れ条件になる。

追加の形: call/`direct-in`、cont/`struct-whole-to-sink`(構造体を丸ごと sink に渡す。中のフィールドの汚染は届いたとみなすか。`SinkAny` を使うマス)。

#### 4.2.2 Dart(共通の族を Dart の言葉で + 固有)

共通の族の対応(Go の写しにしない): interface → `abstract class` / `implements`、embedding → `extends` / `mixin`、`defer` → `try` / `finally`、`go` → `Future` / `Isolate`、channel → `StreamController`、panic/recover → `throw` / `catch`、type switch → パターンの `switch`、range-over-func → `sync*` ジェネレータ、`io.Writer` の自前の実装 → `StringSink` / `Sink` の自前の実装、ミドルウェアの連鎖 → `shelf` 風の `Handler` の連鎖。表 F の共通の形は、この対応で言い直して書く。

固有の形(`F/dart/<形>`): **async-await**、**future-then**(`then` / `catchError` / `whenComplete`)、stream(`listen` / `map` / `await for`)、cascade(`..`)、**extension**、mixin、**named-args**(名前付き引数。おとりは位置の入れ替え)、**getter-setter**、**patterns-record**(パターン、record、`if-case`、`switch` の case)、**for-in**、late(`late` の遅延初期化)、**interp-tostring**(補間の `toString`)、**callable-class**(`call`)、dynamic(`dynamic` の呼び出し)、isolate、**tearoff-instance**(インスタンスメソッドの tear-off。receiver と引数の入れ替わり)、tearoff-static、super-mixin-impl(mixin や上位クラスから来る実装)、flutter-build / flutter-onpressed / flutter-initstate(自前の骨組み)。参照: R-14(tear-off)、R-15(パターン / for-in)、R-16(mixin / 補間)、DART-12。**段階 2 の 20 形**: 太字の 11 に、共通の族から 9(直接、深さ 3、クロージャ 2 つ、グローバル、ハンドラ → 読む入口、リスト、map、サニタイズ、片方の枝)。

#### 4.2.3 Rust(共通の族 + 固有)

共通の族の対応: interface → trait と `dyn`、embedding → 合成と `Deref`、`defer` → `Drop`、`go` → `thread::spawn` / `async`、channel → `mpsc`、panic/recover → `catch_unwind`、type switch → `match` と `Any::downcast_ref`、エラーの包み → `?` と `From`、`io.Writer` の自前の実装 → `io::Write` / `fmt::Write` の自前の実装。

固有の形(`F/rust/<形>`): ownership-move(所有権の移動)、mut-out-param(`&mut` の出力引数)、option-result-chain(`Option` / `Result` の連鎖、`?`、`map` / `and_then`)、iterator-chain(`iter().map().filter().collect()`)、closure-fn / fnmut / fnonce、dyn-vs-mono(`dyn` と静的な単相化)、assoc-fn(`S::assoc()`。receiver のない trait 関数)、drop(入れ子の値の `Drop`)、deref、async、arc-mutex(`Arc<Mutex<_>>`)、channel、static-lazylock(`static` / `LazyLock`)、macro(`macro_rules!`)、from-into、display-format(`Display` と `format!`)、cfg-feature(feature / cfg)。参照: R-17。

#### 4.2.4 形の追加の規則

新しい形は、(a) wild で見つけた経路のステップ、(b) 層 C や探索で見つけた見逃し、(c) レビューの項目、から足す。1 つの形を足す = 表に 1 行 + 3 ケース。形を足したとき、同じ形が curated にあれば、`@cell` を付ける。

#### 4.2.5 言語間の parity と、2 つのマーク `@na` / `@todo`

Go のマスには、Rust と Dart の同じ名前のマスか、次の 2 つのマークのどちらかが要る(`TestZooRust` / `TestZooDart` の parity の検査。どちらも無いと落ちる)。**2 つのマークは、検査では同じ扱いで、表では別に数える**(`bench/parity/TABLE.md` の 3 節)。

| マーク | 意味 | 書き方 |
| --- | --- | --- |
| `@na(F/<族>/<形>: 理由)` | **その言語に、本当に無い形**。書こうとしても、同じ形にならない(言語の機能がない) | Rust: `cells/src/lib.rs` の先頭。Dart: `bench/zoo-dart/na.txt` |
| `@todo(F/<族>/<形>: PARITY: 理由)` | **その言語でも書ける形で、まだマスが無い(未実装)** | 同じ場所。Dart でマークの無い Go のマスは、`@todo` と同じに数える |

使い分けの基準(迷ったら `@todo`。ただし「迷う」ものは、`@na` のまま理由に書いて助言者に聞く):

1. **近い形が書けるなら `@todo`**。例: Go の `fmt.Fprint` を自前の `io.Writer` に → Rust の `write!` を自前の `io::Write` に。形の見かけが違っても、エンジンが試される経路(自前の実装の呼び出し先、書いた値を別のところで読む)が同じなら、書ける。
2. **`@na` は、言語に無い機能だけ**。例: Go のゴルーチンの形を持たない言語、Rust の借用だけの形、Go のポインタレシーバを Dart に。
3. **「まだ測っていない」「エンジンの側が未対応」は `@na` の理由にならない**。書けるのだから `@todo`。エンジンが出せないなら、書いて `@miss` にする(`@na` は、見逃しを表から消す)。
4. 逆向き(Dart だけにあるマス)は、`@na(go: 理由)`(Go に無い形)。Go で書けるが、まだ無いものは、同じ名前のマスを zoo-go に足すまで「まだ」として数える。

数は、`TestZoo*` の出力の `zoo-<言語>: parity marks: @na N [名前] | @todo N [名前]` の行に出て、`bench/parity` の表の 3 節に、言語ごとの件数と名前が並ぶ。**`@todo` の件数を 0 に近づけるのが、言語間の差の是正の進み具合**。

### 4.3 表 S(組み込みのモデルの網羅)

**範囲**: 組み込みのモデルの**全行**: sink / source / sanitizer / propagator / pure。組み込みに入っているものは、標準ライブラリかデファクトのライブラリ(§4.3a)のはず。`pure` / propagator / sanitizer の誤りは、経路を切る(R-03、R-02)。source / sink の行の誤りは、経路の端を誤らせる。どちらも整備の対象。

**目的**: 組み込みの 1 行ごとに、「直接の流れ」のケースが 1 つある。ケースのない行があれば、CI で落とす。

**生成**: `bench/zoo-go/gen`(Go)が、`internal/model/builtin/*.yaml` を読み、行ごとにケースを出す。Dart と Rust も同じ(`dart-*.yaml`、`rust-*.yaml`)。出した `gen_*.go` はコミットし、CI が「再生成して差分がない」ことを見る。**標準ライブラリの行を先に、デファクトのライブラリの行は、一覧(§4.3a)が決まってから**(依存の取得が要るので)。

**正解を YAML から作らない(R-03 の再発の防止)**。生成の 2 段階:

1. 行から、**呼び出しのコードだけ**を出す。呼び方は、型の情報から作る(文字列の引数に目印、`io.Writer` に `bytes.Buffer`、など)。作れない行と、安全でない行(`os.Exit`、`(*exec.Cmd).Run`、`syscall.*`、`os.RemoveAll`、ネットワーク)は、`recipes.yaml` で手書きの呼び方を与えるか、`skip: <理由>` を書く(理由つきの skip の一覧を、報告に出す)。
2. **harness で実行**し、結果(戻り値と書き込み先の全部を `SinkAny` に渡して、目印が残るか)を観測し、**観測から、ケースの注釈を書き込む**。注釈の根拠は、実行だけ。

| 行の種類 | ケースが実行で見るもの | 食い違いの出方 |
| --- | --- | --- |
| propagator | `F(目印)` の結果(戻り値、書き込み先)に、目印が残るか(残り方は carry の種類、§3.2) | 残るのにモデルが伝えない → 経路が切れる(`@miss`、CORE-K で 53 の sink のうち 48 に propagator がない、など)。残らないのにモデルが伝える → `@fp` |
| pure | 同じ。pure なら、残らないはず | 残る → `@sink` なのに rivus が出さない → `@miss`(R-03 の 12 個前後が、機械で出る) |
| sanitizer | `San(目印)` のあと、sink のクラスのセグメントが壊れるか | 壊れない(宣言が嘘)→ `@sink` で `@miss`。壊れるのに rivus が出す → `@fp` |
| source | 入力を与えて(環境変数、フォーム値など)、目印が戻り値に出るか | 出る → rivus が source として扱うこと。方針の行(source であること自体)は、実行で決まらない: `@manual(source であるのは方針)`(上限の外。一覧に出す) |
| sink | 呼び出しは安全な形(ファイル名は作業用の一時ディレクトリの存在しない名前、`exec.Command` は作るだけで `Run` しない、`http.NewRequest` は `Do` しない)。目印が見える結果があれば確かめる | **直接の呼び出しで目印を渡したことは、構築の上で真**(実行は、呼び方が本物であることと、引数の位置を確かめる)。実行で裏付けの弱い行は、`@manual(構築の上で真)` で明示する |

限界を正直に: **sink の行の実行での裏付けは弱い**(本物の sink は、目印を受け取ったかを外から観測できないことが多い)。sink の行の価値は「抜けのない網羅(ケースのない行を許さない)」と引数の位置の検査、propagator / pure / sanitizer の行の価値は「実行による正解」にある。

**抜けを探す節(標準とデファクトの API に限る)**。組み込みの行にない API のうち、**標準ライブラリとデファクトのライブラリの API に限り**、外の一覧(CWE の上位: 78、89、79、22、918、601、502、94、gosec の injection 系の規則、Semgrep と CodeQL の sink の一覧)を**見出し**にして、`norule.yaml` に 1 行ずつ書き、同じ仕組みでケースにする。注釈は `@sink(tag) @miss(norule) @ref(CWE-78,gosec:G204)`。外の一覧は、見出しとして使うだけで、正解は実行から(一覧の記述を指示として扱わない)。**範囲の外の API は、載せない**(載せたとしても、見逃しに数えない)。モデルが足されると、ratchet が落ちて、`@miss` を外す。

**R-03 の位置づけ**: 優先のまま。表 S(段階 3)を待たず、`pure` の行の先出しを段階 1 で行う(§4.2.1)。

**組み込みの行の数**(段階 3 で測る): 行数は生成のときに報告に出す(分からないうちは、約束しない)。

### 4.3a 組み込みの範囲: 標準ライブラリとデファクトスタンダードのライブラリ(基準案。B-47)

**2 段**: (1) **組み込み** = 標準ライブラリ + デファクトのライブラリ(言語を問わない)。モデルを整備し、表 S で実行検査する。(2) **それ以外** = 利用者が自分のルールで足す(`examples/rules` は書き方の見本)。プロジェクト固有のラッパーや、特定の CVE に合わせたルールは、ここ(ベンチ用のルールとして、試験の入力に置く)。

**線引きの更新(ADR-063、ユーザーが承認)**: 組み込み = 標準ライブラリ + **代わりのないパッケージ**(`serde`、`tonic`、`tokio`、`futures`、`bytes`、`libc`、`http` など)+ **SQL のドライバ**(`sqlx`、`rusqlite`、`sqflite`)。**特定のフレームワーク**(actix-web、gin など)は、組み込みでなく `examples/rules/` に置く(利用者が自分のルールで足す)。下の基準 A / B は、組み込みの候補を選ぶ目安で、特定のフレームワークは、基準を満たしても組み込みにしない。

**デファクトの基準**(A か B のどちらかを満たす):

- **基準 A(客観的な数)**: 言語ごとのレジストリの上位で、外からの入力・ファイル・プロセス・SQL・HTTP・テンプレート・直列化・ログ・設定・状態管理・非同期に関わるもの。Go: pkg.go.dev の被 import 数。Dart: pub.dev の人気とダウンロード。Rust: crates.io のダウンロード数。
- **基準 B(手元の証拠)**: 層 C と探索の対象の 2 本以上で使われていて、`unmodeled_top`(汚染された値が実際に届いた未モデルの呼び出し)の上位に出るもの。

**一覧**: `docs/DEFACTO-LIBRARIES.md` に 1 つの文書(ライブラリ、版の範囲、モデルの有無、根拠(A か B)。作るのは実装の段階。この設計では作らない)。載ったものに、モデルを足す。モデルの正しさは、**表 S の実行での検査**で守る。

**順序**: 表 S の生成は標準ライブラリを先、デファクトは一覧が決まってから(依存の取得が要る)。デファクトの依存の取得は、zoo の `go.mod` / `pubspec.yaml` / `Cargo.toml` で SHA(版)を固定し、取得したコードのテストやスクリプトを実行しない(§9.7 と同じ扱い)。

### 4.4 組み合わせ(形 × 形の抜き取り)

全組(80 × 79 / 2 = 3,160)は作らない。**形 × 族の被覆**: 各形が、**ほかの各族と最低 1 回**組む(8 族なら、各形は 7 つの別の族と)。

- 組み合わせ = 4 つの**段**(stage)の直列。`sink(S4(S3(S2(S1(src)))))`。段は、別々の族から選ぶ。
- 各形に、**段の形**を手で書く(`stage_*.go`): `S(v) string`(値を、その形で通す)と、`D(v) string`(おとりの形: 汚染された値を、別の場所に置き、きれいな値を返す)。段にできない形(sink-in-method、入口の形)は `stage: no` とし、一覧に出す。
- 生成器(決定的、シードつき、貪欲な被覆)が、`combo_gen.go` を出す。各組み合わせは 3 ケース: 脆弱(全段 `S`)、直した(ランダムな 1 段の前にサニタイザ)、おとり(1 段を `D`)。**正解は、実行から**(§3.6)。
- 数: 段になる形が約 55 なら、50 前後。言語ごとに 30〜50。

### 4.5 おとりの設計の注意

おとりが「何でも通る空のテスト」にならないための規則: (1) 共存する流れを持つ(汚染された値が、確かに存在し、別の sink に届く)。(2) 判定の行が、実行で hit を持つ。(3) 「入れ替える対象」(別のオブジェクト、別の引数、別のフィールド、別のハンドラ)が、形ごとに表に書いてある(上の表の括弧)。書いていない形は、別の引数(位置の入れ替え)を既定とする。

## 5. エンジンの未修正の項目を、直す前にマスにする(決めること c の後半)

今日のレビュー(2026-11-21)の未修正の項目は、**直す前に zoo のマスを先に置き**、`@miss` で赤が出ることを確かめてから直す。手順:

1. 項目の再現(レビューの一時ディレクトリにある。消えうる)を、表 F の形に直して、3 ケースにする(脆弱、直した、おとり)。
2. 実行で、注釈を確かめる(§3.6)。
3. rivus で走らせる。落ちるなら(`@sink` の注釈のまま `TestZoo` が落ちる)、それが「赤」。`@miss(<silent|kind>) @ref(<項目>)` を付けてコミット(赤が出ていた証拠を、コミットメッセージに書く)。
4. 直す。ratchet が落ちる(`@miss` が真でなくなる)。同じコミットで `@miss` を外す。
5. 直す前後で、`compare.sh` の差分を分類する(`bench/README.md` の手順どおり)。

| 項目 | 置くマス |
| --- | --- |
| R-01 / CORE-J | dyn/iface-ptr-recv-read、iface-ptr-recv-write、sink-in-method、method-value、generics |
| R-02 / CORE-K | conv/buffer-writestring、buffer-fprintf、表 S の propagator の行 |
| R-03 | 表 S の pure の行(機械で出る) |
| R-04 | dyn/stdlib-writer-own、stdlib-error-own、http-middleware-chain |
| R-11 | dyn/sink-behind-own-interface(自前のインターフェース越しの sink。vuln2 は標準の `database/sql`) |
| R-08 | ctl/builtin-unsupported、panic-recover |
| R-14 | Dart tearoff-instance |
| R-15 | Dart patterns-record、for-in |
| R-16 | Dart super-mixin-impl、interp-tostring |
| R-17 | Rust drop、assoc-fn、static-lazylock、cfg-feature |
| R-18、R-19 | ctl/range-over-func、state/alias |

## 6. 数と段階(決めること b)

### 6.1 規模

| 言語 | 形 | マス × 3 | 2 種目の source | 組み合わせ | 表 S | ケースの目安 |
| --- | --- | --- | --- | --- | --- | --- |
| Go | 80 弱 | 約 240 | 約 30 | 約 50 × 3 | 生成 | 400〜600(表 S を除く) |
| Dart | 55 前後 | 約 165 | 約 20 | 30〜40 × 3 | 生成 | 300 前後 |
| Rust | 55 前後 | 約 165 | 約 20 | 30〜40 × 3 | 生成 | 300 前後 |

### 6.2 段階

| 段階 | 内容 | 終わりの条件 |
| --- | --- | --- |
| 1 | Go の仕組み(`mark`、harness、`TestZooTruthGo`、`checkAnnotated` の一般化)と、表 F の 20 形 | 実行と注釈が一致。`TestZooGo` が curated の `go test` と同じ流れで走る。バックテストに要る 3 マス(#6 handler-then-reader、#7 xtalk-n-by-m、#8 save-restore)がある。`mark` の有無で結果が同じ |
| 2 | Dart の仕組み(`zoo_mark`、`dart test`、`dart:mirrors`)と、20 形(段階 1 と**同時**: 仕組みを 2 言語で試すため) | 同上。Go と Dart で、目印の仕様が同じ(`spec.json` の自己検査) |
| 3 | Go / Dart の残りの形と、表 S の生成(標準ライブラリの行。デファクトは一覧が決まってから) | 組み込みの全行にケース(skip は理由つき)。再生成の差分なし |
| 4 | Rust(仕組みと形) | 同上 |
| 5 | 組み合わせ(3 言語) | 各形が、ほかの各族と最低 1 回組む(被覆の検査) |

各段階の終わりに、§6.3 の見出しの数字を報告する。**実行での裏付けを、他のどのケースよりも先に作る**(段階 1 の最初の作業)。

### 6.3 見出しの数字(ユーザーへの報告の定型)

言語ごとに 1 行(`go test ./cmd/rivus -run Zoo -v` が出す):

```
zoo-go: マス 78 / ケース 234 | 出る 61 / 78 | @miss 17(痕跡あり 6、黙っている 8、ルールなし 3) | 直した版で誤って出る 1 | おとりで誤って出る 4 / 78 | carry: encode 出る 9 / 10、digest 方針 2、numeric 出ない 3 / 3(numeric_stop あり 3) | 手の注釈 2(0.9%) | 実行と注釈の不一致 0 | 実行されない文 0
```

- **マス**: 3 ケースそろったマスの数。
- **出る**: 脆弱なケースのうち、rivus が出したもの(`@miss` でないもの)。
- **`@miss`**: 注記あり(`@miss(<kind>)`)、何も報告していない(`@miss(silent)`)、ルールなし(`@miss(norule)`。**範囲の中の API、つまり標準ライブラリとデファクトのライブラリだけ**。範囲の外の API は数えない)の内訳。
- **おとりで誤って出る**: おとりの `@sink(none)` の行を、rivus が報告した数(`@fp`)。直した版の分は別に数える。
- **手の注釈**: `@manual` の数と割合。
- **実行と注釈の不一致**: **常に 0**。0 でなければ、CI が落ちる。

- **carry の内訳**: 種類(encode / digest / numeric)ごとに、期待どおりだった数。encode は「出る」に、numeric は「出ない」(+ `numeric_stop` の不確実性の注記)に、それぞれ数える。**採点の外に置かない**。
- **実行されない文**: ケースのコードのカバレッジの欠け(常に 0)。

表 S と wild は別の行(件数、`@miss` の内訳、skip の件数)。

## 7. バックテストと台帳への組み込み(決めること c)

`advisor` のバックテスト(Gitea 離脱のためのもの)の台帳に、**zoo の列**を足す。入力は head のものに固定し、エンジンだけを古い版にする(同じ決まり)。

| # | 現象 | 悪い版で赤になるべき zoo のマス | 赤の種類 |
| --- | --- | --- | --- |
| 6 | 明示のグローバルへの書き込みを落とす(CORE-H。`623b620` 〜 `ae5fdf6`) | state/handler-then-reader の脆弱 | **MISS** |
| 7 | param の source の書き戻しで N×M(CORE-A。`8e8b866`) | dyn/xtalk-n-by-m のおとり | **FAKE** |
| 8 | 保存して戻すでサブセルが根に持ち上がる(C-7 の案 D の前) | state/save-restore のおとり | **FAKE** |
| 1、3 | reflect のモデルなし、`holds: nothing` なし(`2dd5022`) | conv/reflect-typeof-decoy、stderr-logger-decoy のおとり | FAKE(小さい形でも出るなら。出なければ、規模の現象として層 A に任せる) |

- 良い版では、同じマスが緑であること(悪い版が赤、良い版が緑の両方。赤の指標が期待した指標であること)。
- zoo が悪い版で赤にならない行は、zoo の形が足りない(足す)。
- `compare.sh` の差分の分類で、zoo の finding は**ラベルがある**ので、`compare.py` が注釈と突き合わせ、追加 / 削除された finding を「脆弱が出た / 脆弱が消えた(MISS)/ おとりが出た(FAKE)/ 直した版が出た」に自動で分類して、`details/zoo-*.md` に出す(分類が機械的になる。実コードの NOISE の分類は、今までどおり人)。
- 古い版が読めない IR(Dart / Rust)は「対象外」と書く(緑に数えない)。

## 8. 軽い層に入る(決めること d)

- **予算**: 1 言語あたり、解析 + 実行で 20 秒以内、メモリ 2 GB 以内(curated は 1〜2 秒)。段階 1 で、20 形(60 ケース)の実測から外挿し、予算を超える見込みなら、`zoo-go-a` / `zoo-go-b` に分ける(族で分ける)。時計が結果を変えない(`-func-timeout` で切らない。切られたら赤)。
- **`compare.sh` の spec**(curated の `go-curated|dart-curated|rust-curated)` の節に並べる):

```
zoo-go)   DIR="$R/bench/zoo-go";   RULES="$DIR/rules.yaml"   # 解析するパターンは ./cells/... (harness を含めない)
zoo-dart) KIND=dart; DIR="$R/bench/zoo-dart"; RULES="$DIR/rules.yaml"
zoo-rust) KIND=rust; DIR="$R/bench/zoo-rust"; RULES="$DIR/rules.yaml"
```

  `BENCHES` と `--light` の一覧に足す。基準と head は、それぞれの版のフィクスチャで解析される(R-41。head で足したマスは `+N` に出る)。IR のキャッシュのキーは、抽出器の tree と対象の tree(既存の仕組み)。
- 軽い層の合計は、zoo の分で +60 秒を超えないこと。超えたら、重い層に移す前に、サイズを削る(ケースを削るのでなく、形の整理を先に)。
- CI: `test` ジョブが `TestZoo*` を走らせる(`RIVUS_DART` と `RIVUS_RUST` の既存の設定で)。生成物の再生成の差分、`spec.json` の自己検査、grep の検査(§11)も、同じジョブ。

## 9. wild(実世界の脆弱性を取り込んで育てるベンチ)

`bench/wild/`。zoo が「体系」なら、wild は「現実」。**公開リポジトリであることを前提に書く**(§9.7)。

### 9.1 1 件に 2 つの形

「脆弱な関数の周りだけを書き起こす」だけにすると、現実の難しさ(sink の関数でなく、そこまでの**経路**: フレームワークのコールバック、インターフェース、グローバル、設定、何段もの関数)が消え、zoo と同じものになる。だから、1 件は 2 つの形を持てる。

| 形 | 中身 | 何を答えるか |
| --- | --- | --- |
| **W1(本物の切り出し)** | 脆弱な版の**完全な SHA** + 取得のスクリプト(`fetch.sh`)+ 解析するパッケージのパターン + ルール。**コードは写さない**。Gitea の層 B1(`bench/gitea-cve-paths.md`: 7 パターン、`-no-deps`、21 秒)と同じ形 | **現実性の正解**。現実の経路がそのまま残る |
| **W2(経路の骨組みの書き起こし)** | 本物の経路の**各ステップを 1 つも省かずに**、自分の名前、自分の構成で書き直したコード。`vuln/` と `fixed/`。zoo と同じく**実行で裏付ける**(目印が届く / 直した版では届かない) | zoo に近い、軽くて、実行で裏付けた再現 |

- **W1 だけ、W2 だけの件があってよい**(ライセンスや大きさ。GPL / AGPL は W1 だけ)。
- 両方ある件は、**「W1 で出て W2 で出ない」「その逆」の食い違いを、調べる価値のある信号**として扱う(W2 が経路を省いたか、W1 の経路がエンジンにだけ見えているか)。
- **段の列の一致が、取り込みの検査**: `meta.yaml` に、経路のステップの列を、**zoo の表 F の形の名前**で書く。例:

```
stages: [param_source, dyn/iface-ptr-recv-read, clos/to-lib, state/global-field, sink]
```

  段の数と形が、W1 の本物の経路の文書(`gitea-cve-paths.md` のような、経路を 1 本ずつ読んだ文書)と一致していること。一致しない W2 は取り込まない(省かれた段があるということ)。

### 9.1a 評価するのは経路

- **各件に、その件のルール(`rules.yaml`)を必ず付ける**。source と sink は、その件のルールで与える(組み込みに頼らない)。このルールは、ベンチ用の入力で、評価の対象ではない(プロジェクト固有のラッパー、その CVE に合わせたルールを含む)。**その件のルールが、標準ライブラリかデファクトのライブラリのものなら、あとで組み込みのモデルに移す候補としてマークを付ける**(`meta.yaml` の `rules` 欄に `promote_candidate: true`)。移すときは、表 S の実行検査に載せる(§4.3)。
- 評価は**経路の一致**: source + 途中の関数 + sink、段の形の列。ルールを与えたのに経路が出ない、または経路の途中の段が違うとき、`@miss`(理由つき)。**ルールがないだけで出ない件は、`@miss` ではない**(件に付けるルールで、それは起きない)。件のルールが範囲の中の API(標準、デファクト)にモデルがないことを示していたら、`promote_candidate` に載せる。
- 「直した版でガードが証拠に出るか」(§9.6)は、経路の情報の質の話なので残す。

### 9.2 zoo との関係: 流れの向きは wild → zoo

- 現実の経路を、**形の列に分解**する。表 F にない形が見つかったら、**zoo にマスを足す**(§4.2.4)。
- wild の 1 件ごとに、「この経路のステップの形は、全部 zoo にあるか」を検査する: `meta.yaml` の `stages` の形の名前が、zoo の表 F に存在すること。**CI で検査**する(`TestWildStagesInZoo`)。ない形は、同じ変更で zoo に足す(足せなければ、`stages` に `todo:<名前>` と書き、一覧に出して、期限つきの課題にする)。
- 逆(zoo → wild)は作らない。

### 9.3 取り込みの基準と順序

- **taint の形をした脆弱性だけ**: コマンド、SQL、パス、SSRF、XSS、リダイレクト、テンプレート、デシリアライズ。
- **対象外**: 認可の抜け、ロジック、暗号、DoS、メモリ安全。**対象外にした件も、ID と理由を 1 行で `bench/wild/OUT-OF-SCOPE.md` に残す**(守備範囲の外の、正直な数)。
- **rivus が見つけられるかで選ばない**。見つからない件も、ルールを付けたうえで経路が出なければ `@miss(理由) @ref(...)` で入れる。**順番**(手順に書く): (1) 報告を読んで、取り込むかを決めて、`meta.yaml` に書く(rivus を走らせる前) (2) W2 を書き、実行で裏付ける (3) その後に、rivus を走らせる (4) 結果を注釈にする(`@miss` など)。(1) の決定は、(3) の結果で変えない。
- ライブラリの報告は、使う側のコードを書き足さず、**脆弱な公開関数の引数を source(kind: param)にして、ライブラリの中の sink まで、で 1 件**。使う側を含めるのは、入口からの経路が報告に書かれているときだけ。

### 9.4 探し方

- Go: vuln.go.dev(`GO-YYYY-NNNN`)、GitHub の Security Advisories(GHSA)。
- Rust: RustSec(advisory-db)、GHSA。
- Dart: GHSA の pub の項目。**Dart は公開された報告が少ないはず(推測)**。足りなければ、OSS の Flutter / Dart のアプリの「セキュリティの修正」のコミットから拾う(公開 ID がなければ、コミットの SHA を ID にする)。
- OSV(osv.dev)は、3 言語の横断の検索に使う。
- アプリ(ハンドラ → sink の経路がある)の報告を優先する(rivus の用途に近い。道の段がある)。
- **Dart が薄いことは、数字に出して隠さない**: `bench/wild/README.md` に、**言語 × 種類(コマンド / SQL / パス / SSRF / XSS / リダイレクト / テンプレート / デシリアライズ)の件数の表**を置き、見出しの数字に含める。
- 最初の目標: **Go 10、Dart 5、Rust 5**。最初の 3 件は Gitea の 3 件(GHSA-4p89-xcq8-xfhm、GHSA-x77v-q46j-393g、GHSA-8vmh-w35x-73qr)。W1(切り出し。`bench/gitea-cve-paths.md` がある)と W2(骨組み。`go-curated/gitea` を出発点にし、段の列を経路の文書と照合する)の両方。3 件目(oauth)の本筋(プロバイダの応答 → `AvatarURL`)は `@miss`(rivus に HTTP の応答を source にする規則がない。B-41)。

### 9.5 1 件の形式

```
bench/wild/<lang>/<id>/
  meta.yaml     # 下
  vuln/         # W2: 脆弱な版(自分の名前、自分の構成)。実行で裏付ける
  fixed/        # W2: 直した版
  rules.yaml    # その件のルール(source、sink、必要なら sanitizer)。必須
  fetch.sh      # W1: 取得(SHA 固定)と、解析のパターン、ルールの場所
```

1 件足す = ディレクトリ 1 つと、`bench/wild/INDEX.md` の 1 行(手順は `bench/wild/README.md`)。`<id>` は公開 ID(`GHSA-…`、`GO-…`、`RUSTSEC-…`)。公開 ID がなければ、`commit-<SHA の 12 桁>`。

`meta.yaml`:

| 項目 | 内容 |
| --- | --- |
| `id`、`url` | 公開 ID と、報告の URL(リンクだけ。本文は引用しない) |
| `cwe`、`kind` | CWE と、種類(command / sql / path / ssrf / xss / redirect / template / deserialize) |
| `repo` | リポジトリの URL |
| `vuln_sha`、`fixed_sha` | 脆弱な版と直した版の**完全な SHA** |
| `published` | 報告の公開日(§9.7) |
| `license` | 元のコードのライセンス |
| `forms` | `[W1, W2]` のどちらか、または両方 |
| `rules` | その件のルールのファイル(`rules.yaml`)。必須。`promote_candidate: true` は、標準かデファクトのライブラリのルールで、組み込みへ移す候補(§9.1a) |
| `source`、`path`、`sink` | source、途中の関数、sink(関数と行) |
| `stages` | **段の形の列**(zoo の表 F の形の名前) |
| `fix_kind` | **修正方法の種類**(下) |
| `summary` | 自分の言葉の要約 1〜2 行 |
| `imported` | 取り込んだ日 |
| `copy` | W2 が「元のコードの写しではない」こと(`rewritten: true`) |

注釈と採点は、curated / zoo と同じ(`checkAnnotated`)。**W2 は実行で裏付ける**(zoo の `mark` を使う)。W1 は、外部のコードなので、実行しない(§9.7)。W1 の正解は、`truth.json`(既存の `score.py` と同じ形)。

### 9.6 直した版の期待値は、修正方法の種類で分ける

| 種類 | 例 | 直した版での期待 |
| --- | --- | --- |
| (i) **サニタイザを通す / API を替える** | プレースホルダ、`exec` の引数の分離、エスケープ | finding が**消えるべき**(消えなければ、`@fp`) |
| (ii) **検証して拒否する** | 許可リスト、`path.Clean` と接頭辞の検査、ホストの照合 | rivus は判定役でないので、**消えなくてよい**。ただし、そのガードが**証拠**(`conditional_nodes` / ガード)として出るべき。出ないなら `@miss(ガードが証拠に出ない) @ref(...)` |

**「現実の修正方法のうち、rivus の証拠に表れる割合」**が、今どこにもない精度の数字になる(見出しの数字に足す)。

### 9.7 ライセンス、公開、個人情報(rivus は公開リポジトリ)

**前提: rivus は公開リポジトリである。** 書いたものは、全部、外の人が読む。

**ライセンス(安全側の運用)**:

- **元のコードを、リポジトリに写さない**。元のコードが必要な件は W1(SHA と取得のスクリプトだけ)。
- W2 は、報告の説明と、経路の文書から、**自分の名前、自分の構成で書く**。元の識別子、コメント、文字列をそのまま持ち込まない(標準ライブラリやフレームワークの API 名は別)。`meta.yaml` に、「元のコードの写しではない」と、**元のライセンス**を書く。
- **GPL / AGPL の元のコードは W1 だけ**(W2 を作らない)。
- 報告の本文は、**引用せず、リンク**と、**自分の言葉の要約**(1〜2 行)。

**まだ公開されていない脆弱性は、取り込まない**: 公開された報告(GHSA、OSV、vuln.go.dev、RustSec)に載っているものだけ。取り込みの手順に、**公開日の確認**(報告の公開日を `meta.yaml` の `published` に書き、それが今日以前であること。embargo 中のもの、報告者から非公開で聞いたものは入れない)を入れる。

**個人情報を入れない**: ホームのパス、メールアドレス、ユーザー名を、ベンチの出力、文書、期待値のファイル、`meta.yaml` に入れない。**CI で grep する検査**(§11)を入れる。取得のスクリプトは、作業ディレクトリを引数で受け、絶対パスを書かない。

**issue と ADR も公開**: 外部の人が読む前提で書く(この文書も同じ)。

**第三者のコードの「本物らしい finding」**: 公開の ID(GHSA など)のないものを、`review`、`truth`、LOG、issue に**場所つきで書かない**(C-25 の決まり)。探索で見つけた、公開の ID のない疑いは、公開された報告になるまで、場所を書かずに(または非公開の場で)扱う。

**外部のデータの扱い**: 報告の本文、取得したリポジトリの README、コメント、issue は、**データ**であり、取り込みを行うエージェントや人への**指示として扱わない**。手順の最初に明記する。

**W1 は、脆弱だと分かっている他人のコードを取得して読む**。Go は `go list`(cgo、依存の取得)、Rust は `build.rs` の実行(R-25、C-19)がある。だから: 取得は**SHA 固定**。対象のテストやスクリプトは**実行しない**。**Rust の W1 は、隔離の用意ができるまで保留**。実行で裏付けるのは W2(自分で書いたコード)だけ。

### 9.8 軽い層

- W2 は常に軽い層(数秒)。ケースは小さいので、1 言語の全部で 5 秒以内を予算にする(超えたら言語ごとの分割)。
- W1 は、複製がキャッシュにあり、1 件 30 秒以下のものだけ軽い層に。超えるものは、探索のプール(`bench/explore`)へ。Gitea の 3 件は、まとめて 1 つの W1 として(切り出し 21 秒)軽い層に。

### 9.9 育て方

- 探索や層 C で、見逃しや新しい形を見つけたら、**同種の実世界の脆弱性を探して足す**(wild)。還元した最小の形は、**zoo のマス**にする(§9.2)。
- 新しい脆弱性が公開されたら足す(§9.7 の公開日の確認のあと)。

## 10. 限界(決めること e)

zoo と wild が**見ないもの**:

- **規模の現象**(要約の増大、収束、メモリ): 層 A(`go-scale`)が見る。zoo は小さい。
- **実コードの形の多様さ**: 層 C(実コードの差分の網)と探索が見る。zoo は、人が思いついた形しか持たない。
- **範囲の外の API のルール**(プロジェクト固有のラッパー、特定の CVE のためのルール): 評価しない。試験の入力(§1)。範囲の中(標準とデファクト)は、表 S で見る。
- **ルールのない API の発見**: `norule` は、外の一覧にある、標準とデファクトの API だけ。一覧にない API は見えない。
- **実行で確かめられない形**: `@manual` に落ちる(並行の順、時間、外部の入力)。
- **Flutter の本物**: `build` / `onPressed` / `initState` は自前の骨組みの写し。フレームワークの中の呼び出しの順は見ない。
- **種類(クラス)の区別**: Dart / Rust のルールが、sink の種類で分けられない間(R-13: すべて `accepts: [user_input]`)、kind-mismatch の形は Dart / Rust で対象外(`N/A`)。
- **数値や型のある経路**(ポート番号、ID)は、目印(文字列)が通れないので、zoo の対象外。`carry(numeric)` の期待(出ない + `numeric_stop`)で、ふるまいを見るだけ。

**zoo を緑にするためだけの、エンジンへの特別扱いの危険**: エンジンが、`zoo/mark` の名前、目印の文字列(`Z<tag>.SH[`)、ケースの名前を見て、判定を変えること。監視:

1. **CI の grep**: `internal/` に、`zoo`、`mark.Sink`、`ZOO` の目印のパターンが出たら落とす(§11)。
2. zoo のルールは、**公開のルールの仕組みだけ**で書く(ルールのファイル `rules.yaml` と、組み込みのモデル)。エンジンの内側に、zoo 専用の経路を作らない。
3. **層 C と探索**が、zoo にない形での新しい見逃し・誤検出を見る(zoo が緑でも、実コードが赤なら、zoo の形が足りない)。
4. wild の W1(本物のコード)が、zoo の形と無関係に走る。
5. 将来の案(段階 5 のあと): **変異テスト**(意味を変えない書き換え: 識別子の改名、ファイルの並べ替え、素通しの関数の挿入)の後で、判定が同じであること。zoo の名前や配置への過学習を見る。

## 11. CI の検査の一覧

| 検査 | 内容 |
| --- | --- |
| 実行と注釈の照合 | `TestZooTruth{Go,Dart,Rust}`(§3.6)。不一致 0 |
| マスの完全性 | 全マスが 3 ケース。表 F と `@cell` の一致 |
| `@ref` | `@miss` / `@fp` に参照がある |
| 手の注釈 | `@manual` が上限(3%、マスごとに 1)以内。一覧を出す |
| 表 S | 組み込みの全行にケースか理由つきの skip。再生成の差分なし |
| 目印の仕様 | 3 言語の `mark` の定数が `spec.json` と同じ(自己検査) |
| 順序の依存 | 逆順の観測が同じ |
| 被覆(組み合わせ) | 各形が、ほかの各族と最低 1 回 |
| wild の段の形 | `meta.yaml` の `stages` の形が zoo にある(`TestWildStagesInZoo`) |
| 公開日 | `published` があり、今日以前 |
| **個人情報の grep** | `bench/zoo-*`、`bench/wild`、`docs/DESIGN-zoo.md`、期待値のファイル、出力の例に、ホームのパス(`/Users/`、`/home/` で始まるもの)、メールアドレスの形、取得時のユーザー名の形が**ない**(例外は一覧ファイルで明示。ライセンスの著作権表示は、元のコードを写さないので持たない) |
| **エンジンへの特別扱いの grep** | `internal/`、`cmd/` に、`zoo`、`mark.Sink`、目印のパターンが出ない(テストファイルを除く) |

## 12. 実装の手順(最初の 1 本)

1. `bench/zoo/spec.json`(目印の仕様)と、`bench/zoo-go/mark` と harness、`TestZooTruthGo`(まず、3 ケースだけ。`call/direct`)。
2. `checkAnnotated` の一般化(curated の既存のテストが、同じ結果で通ること)。
3. 同じ仕組みを Dart(`zoo_mark`、`dart test`)で。目印の仕様が同じことを自己検査。
4. 段階 1・2 の 20 形ずつ。そのたびに、見出しの数字を報告する。
5. 採用が決まったら、ADR を 1 つ(`docs/DECISIONS.md`)書く: 「注釈は実行で裏付ける。rivus の出力で注釈を直さない」。

## 13. 決まっていないこと(実装で測る)

- 解析時間とメモリの実測(段階 1 の 20 形で外挿し、予算を確かめる)。
- ケースの関数が、harness を含めない解析で、到達性(`reachability`)をどう扱われるか(zoo では到達性の欄を判定の対象にしない。別の課題)。
- 組み込みのモデルの行の数と、`recipes.yaml` の手書きの量(段階 3 で測る)。
- `dart:mirrors` で歩けない値(record の中身など)の扱い(`toString` で足りなければ、ケースの値の型に `toString` を書く)。
- Dart / Rust の `spec.json` の読み込み(定数に写して、自己検査にするか、実行時に読むか)。既定は、定数 + 自己検査。
