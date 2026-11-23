# ADR-070: 関数値の呼び出しの候補が「一部の呼び出し元しか追えていない」ことを、不確実性 `unresolved_callee`(`partial`)で示す

- 状態: 有効
- 関連: -
- 要旨: `unresolved_callee`(`partial`)

**背景:** `valuecalls` の `funcsOf` は、値の出所が追えない形(フィールドの読み出し、戻り値、グローバルなど。`classify` の `refOther`)に該当すると、**何も報告せずに戻って**いた。見つかった関数が 1 つ以上あれば、それが「全部」として候補になり、`found` が 0 のときだけ不確実性が出た。Dart の再現: ヘルパー `apply(f, s) { f(s); }` を `apply((s){}, 'x')`(追える)と `apply(h.cb, source())`(フィールドの読み出し)で呼ぶと、候補は前者の 1 つだけになり、`h.cb` の中の sink の流れは、見逃しなのに、finding も不確実性も出なかった(追えない呼び出し元だけのときは `unresolved_callee` が出る。つまり「全部が追えないときは報告し、一部が追えないときは報告しない」)。呼び出し元の絞り込み(Rust の `Fn::call` の候補をレシーバの出所で絞る案)が、このマークなしでは、見つかった分だけに絞ってしまう。

**決めたこと(設計の段階 1):**
1. `funcsOf` が、(a) `refOther`、(b) 呼び出し元の引数が足りない、(c) 呼び出し元の `form` が作れない、(d) 呼び出し元をたどる深さの上限(3)で切った、のどれかに該当したとき、**不完全**のマークを付ける。見つかった関数は、**候補に残す**(finding は減らさない)。
2. マーク: `Callee.ValueLimit = "partial"`、`ValueFound`(見つかった数)、`ValueWhy`(原因: `untraced_value`、`missing_argument`、`no_ssa_form`、`caller_depth`)。候補が 0 のときの `candidate_limit` と `caller_depth_limit` は、そのまま(候補が 0 なら `partial` にしない)。
3. 不確実性: 候補が 1 つでも複数でも、その呼び出しに `unresolved_callee`(detail は `partial: ...`、原因と見つかった数)を、**呼び出しの位置に**足す。その呼び出しを通った finding の経路にも付く(確信度は `no_uncertainty_on_path` にならない)。
4. **候補を絞る案は、このマークのある呼び出しには使わない**(マークは、そのための前提)。いまの絞り込み(ガード)は、候補が複数あるとき、呼び出し元が直接渡した関数だけで絞るもので、変わらない。
5. zoo に共通のセル `clos/partial-callers`(3 言語。脆弱、修正済みのケース、デコイ)を足した。Go は VTA の候補があり、`valuecalls` に来ないので、見つかる(マークも出ない)。Dart は、脆弱が `@miss(silent)`(見逃しは今までと同じ。不確実性はヘルパーの中に出るので、見逃した流れの source と sink の関数の不確実性の注記にはならない: `TestZooPartialCallersSaysItIsPartialDart` が、ヘルパーの `unresolved_callee`(`partial:`)を検査する)。Rust は、関数ポインタの呼び出しが `valuecalls` を通らず、追えない呼び出し元がなくても、脆弱とデコイが見つからない(`@miss(unmodeled_library_call)`。別の抜け)。
6. スキーマは変えない(不確実性の `kind` は既存。`detail` の追加)。

**確かめたこと(`compare.sh b0ed28b 732de0b`、軽い層):** finding の追加 0・削除 0(実コードの全対象)、経路・到達性の変化 0。追加の finding は、zoo の新しいセルの分だけ(zoo-go +2、zoo-dart +1。base にはセルがない)。不確実性: Go の実コード(gogs 2 つ、gin-vue-admin、go-test-bench、gophish、casdoor)は **0 件の変化**(Go は候補が VTA で付く)。Dart: dart-pub の `unresolved_callee` が 66 → 71(`mapMap`、`Package.load`、`_handleAccessTokenResponse`、`_handleErrorResponse`、`_parseIgnorePatterns`: 関数値の呼び出し元の一部が追えない)、dart-curated が 21 → 22(`withDefault`)。parseable(Rust)は 0。

**限界:** 候補が見つかった分だけであることは変わらない(広げない。広げると、過検出が増える。別の判断)。マークの `incomplete` を使う絞り込みは、まだない(その案を入れるとき、このマークのある呼び出しを除く)。

