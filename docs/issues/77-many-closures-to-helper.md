# #77 Dart: 同じヘルパーに渡すクロージャが 9 種類以上だと、呼び出しが未解決になり、コールバックの中の sink・throw・書き込みが落ちる(サイレントな見逃し)

- 状態: **partial**
- 種類: 注記つきの見逃し(push 後)
- 言語: Dart
- ROADMAP の節: 1 / 2
- 守るもの: zoo `F/clos/helper-overflow`(32 まで)、`F/clos/helper-overflow-many`(33 以上。`@miss(depth_limit)`)、`cmd/rivus/dart_limit_test.go`
- GitHub: #77(移行前の番号は同じ。2026-11-20 に開いた。移行時の状態: OPEN)

## 今の状態(2026-11-22 の見直し)

**残っている形: 同じヘルパーに渡すクロージャが 33 種類以上のとき**。候補の上限は 32 に上げた(32 までは直った)。33 以上は見逃しで、不確実性の注記(depth_limit、candidate_limit)は、渡した側の関数に付く(`wip-limit-trace` が入ったとき)。関数値の第 2 歩で閉じる見込み。

## 本文(GitHub から移した写し)

## 現象
同じヘルパーに渡す**異なるクロージャが 9 種類以上**になると、ヘルパーの中の呼び出し `f(s)` が未解決になり、**コールバックの中の sink、throw、捕獲変数への書き込みが、どの呼び出し元でも届かなくなる**。ヘルパーが広く使われるほど(`retry`、`withLock`、`transaction` など)起きやすい。不確実性は、ヘルパーの `f(s)` に `unresolved_callee` が 1 つ出るだけで、source / sink の関数には出ない(**サイレントな見逃し**)。

## 再現(Dart)
```dart
String transform(String s, String Function(String) f) => f(s);

void c0() { final a = source(); transform(a, (x) { sink(x); return x + '0'; }); }
// c1 ... c9 も同じ形(クロージャは別々)
```
* c0〜c3 の 4 種類だけのとき: 4 件の流れが出る。
* 10 種類のとき: **0 件**(クロージャの中の `sink(x)` に届かない)。
同様に、`transform(a, (x) => throw AppError(x))` の `on AppError catch (e) { sink(e.detail) }` も、11 種類のときだけ落ちる。過検出も出る: `transform(c, (x) => 'fixed')` の結果に、`c` の汚染が残る(引数 → 結果の保守側)。

## `@miss` / `@fp`
`bench/dart-curated`: `through`(何も報告しなかった)、`ret-clean` と `tear-drop`(`@fp`)。`nest-a/b` と `bound-prefix` も `transform` が未解決で、捕獲(別 issue の `CAP`)と重なる。`bench/MISSES.md` の型 `OVERFLOW`。

## 落ちる場所の推定
`internal/valuecalls/valuecalls.go:124`: `if len(found) == 0 || len(found) > most`(`most = 8`)で候補を捨てる → `internal/taint/calls.go` の `callBody` で `len(fns) == 0` → `s.unmodeled`(引数を結果へ流すだけ)。
ADR-047(案 2)で直る見込み(ヘルパーが候補を持たず、呼び出し元が渡したクロージャで具体化する)。それまでは、上限を超えたとき、せめてユーザーに分かる不確実性(source / sink の関数にも)を出すべき。

## コメント(GitHub から移した写し)

別セッションのレビューの追記(識別子: CORE-D)。確度: コード読解(`internal/valuecalls/valuecalls.go` の `resolve`: `len(found) == 0 || len(found) > most`、`most` = 8。`maxDepth` = 3 も同じ扱い)。

不確実性の中身が誤っている: 9 個以上の関数が見つかったのに、候補を 1 つも付けず、出る不確実性は `unresolved_callee`「the called function is not known」。関数は分かっていて、rivus が上限で捨てた。kind と `attributable_to` が誤り。Dart と Rust はこの経路しかない(Go は VTA の候補が先)。

修正の方向:
1. すぐ: kind を `depth_limit`(または `candidate_limit`)、`attributable_to: rivus`、見つけた数つきにする。`maxDepth` で切ったときも区別して書く。source / sink の関数にも不確実性の注記が出ること
2. 案 2 で: 記号的にする条件を「値がパラメータに由来する」で決め、候補の一覧に依らない。上限は外すか、超えたら記録する。ADR-045 に上限の記述があれば、ADR の変更として明記する
3. 測定: dart-corpus(55 パッケージ)で、上限に当たる呼び出し点の数

## コメント(GitHub から移した写し)

再開後の確認(2026-11-22): zoo の `F/clos/helper-overflow`(10 種類のクロージャを 1 つのヘルパーに)で、見逃しを確認した(`@miss(recursion_approximation) @ref(#77)`)。**不確実性の注記の kind が、この Issue の記述(`unresolved_callee`)と違い、`recursion_approximation` だった**。再帰のない形で `recursion_approximation` が出るのは、不確実性の注記の kind が実態と合っていない疑い。なぜこの kind になるかを追う。

## コメント(GitHub から移した写し)

再開後の追跡(2026-11-22): zoo の `F/clos/helper-overflow` の `@miss` の不確実性の注記が `recursion_approximation` になっていた理由は、**zoo 全体の解析が収束していない(`stats.converged: false`、12 パス、`state/global_chain` の 2 関数が不安定)ことで出る、全体の `recursion_approximation`** を、不確実性の注記の判定が拾っていたため。そのセルだけを解析すると、`converged: true` で、不確実性は `depth_limit`(`candidate_limit: the function value may be any of 12 functions, more than the 8 ...`)、`unresolved_callee`、`dynamic_dispatch`。**本当の理由は `depth_limit(candidate_limit)`**で、この Issue の推定(候補が 8 を超えると捨てる)と合う。zoo の注釈の kind の付け替えは、別に扱う。

