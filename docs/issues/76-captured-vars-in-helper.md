# #76 Dart / Go: ヘルパーに渡したクロージャがキャプチャした変数の読み書きが、ヘルパーの中の呼び出しで落ちる(サイレントな見逃し)

- 状態: **partial**
- 種類: サイレントな見逃し
- 言語: Dart / Go
- ROADMAP の節: 2
- 守るもの: zoo `F/clos/*`、`bench/rust-curated`
- GitHub: #76(移行前の番号は同じ。2026-11-20 に開いた。移行時の状態: OPEN)

## 今の状態(2026-11-22 の見直し)

主な形は直った(2026-11-22)。残りは `write-param`(`@miss(depth_limit)`、注記つき)と holder 系の `@miss(unresolved_callee)`。関数値の第 2 歩とキャプチャセルで閉じる見込み。

## 本文(GitHub から移した写し)

## 現象
クロージャが**キャプチャした値**(読む・書く)を、そのクロージャを別の関数(ヘルパー)に渡して、ヘルパーが呼ぶ形で、流れが失われる。`uncertainties` に、この見逃しに関わるものは出ない(**サイレントな見逃し**)。ヘルパーの `f()` に出るのは、成功する流れにも出る `dynamic_dispatch` だけ。同じクロージャを、作った関数の中で呼ぶと見つかる。

## 再現(Dart、`rivus analyze -ir`。source() / sink() はルールで指定)
```dart
void run(void Function() f) => f();

void read() {
  final a = source();
  run(() => sink(a));        // 期待: source → sink   実際: 出ない
}

void write() {
  String? out;
  final t = source();
  run(() { out = t; });
  sink(out);                 // 期待: source → sink   実際: 出ない
}
```
`final f = () => sink(a); f();`(作った関数の中で呼ぶ)と、キャプチャしないクロージャ(`fireAndForget(a, (x) => sink(x))`)は見つかる。呼び出し元が 1 つでも落ちる(実験)。
Go でも同じ形: `apply(func(q *Q){ q.name = n }, q)`(`n` は source)が失われる。

## 期待と実際
期待: 呼び出し元が渡したクロージャのキャプチャ値が、ヘルパーの中の呼び出しに届く。実際: キャプチャ値が、何も無い値(`KUndef`)として扱われ、流れが消える。

## `@miss`
`bench/dart-curated`: `run-a` `run-b` `write-a` `write-b` `write-param` `add-list` `nest-a` `nest-b` `twice-args` `across-cap`(何も報告しなかった)、`builder` `bound-prefix`(弱い注記)。`bench/MISSES.md` の型 `CAP`(Dart 12 件)。

## 落ちる場所の推定
* `internal/valuecalls/valuecalls.go:319` `funcsOf`: 束縛(`make_closure` の引数)を付けるのは、クロージャを作った関数 `home` の中だけ(コメント: 「from another function they are not at hand」)。引数で渡された候補には束縛が付かない。
* `internal/sg/build.go:362` `resolveValueCalls`: `c.Bind != nil` のときだけ `CandidateBindings`。
* `internal/taint/summary.go:283-286` `applySummary`: 束縛が無いと `FreeVars` を `KUndef` で埋める(不確実性なし)。
ADR-047(案 2)で直る見込み(呼び出し元が、自分が渡したクロージャの束縛で具体化する)。そのとき `ExactFuncs` が束縛も返す必要がある。

## コメント(GitHub から移した写し)

別セッションのレビューの追記(識別子: DART-15)。確度: 実行で確認(見逃しと、不確実性が出ないこと)、原因はコード読解。

根は同じ(`applySummary` が、足りない引数を `KUndef` で何も報告せずに埋める)。dart-curated の `@miss` 32 件のうち、「source か sink のある関数に不確実性が 1 つも出ない」ものが 19 件、何か出る(`dynamic_dispatch` / `unresolved_callee`)ものが 13 件。19 件のうち、キャプチャの読み書き(run、write、add-list、nest、relay-c、twice-args、across-cap、when-complete)の 13 件前後がこの型。残り(catch-error、completer、future-error、on-error、through、put-if-absent)は別の原因のはず(#80、#77 など)。集計は関数の範囲を行の見当で取ったので、±1 件の誤差がありうる。

提案の 2 ステップ:
1. すぐ: 足りないのが、クロージャのキャプチャの分(`Captured` のパラメータ)のときは、何も報告せずに埋めることはせず、不確実性を出す(`attributable_to: rivus`、「このクロージャがキャプチャした値は、ここでは分からない(クロージャは別の関数で作られた)」)。省略できる引数のための埋め方は今のままでよい
2. 本体: 案 2(ADR-047)で、呼び出し元が自分のクロージャを具体化するときに `make_closure` の引数をキャプチャの値として渡す。受け入れのフィクスチャに `run-a` / `run-b`

渡しただけのクロージャ(呼ぶところが見えない形)は #88 に分けた。Go と Rust で同じ形が起きるかは未確認(rust-curated の README はキャプチャの書き込みだけを見逃しに挙げている)。

## コメント(GitHub から移した写し)

再開後の確認(2026-11-22): Dart の dart-curated で、run-a / run-b / write-a / write-b など、この Issue の主な形は `@miss` が外れて見つかる。残り: `write-param`(`bench/dart-curated/lib/capture.dart`、`@miss(depth_limit)`、注記つき)と、holder 系の `@miss(unresolved_callee)`。Go の `apply(func(q *Q){ q.name = n }, q)` の形は今回は実行していない。この Issue は、残りの形を指すものとして開けておく。

