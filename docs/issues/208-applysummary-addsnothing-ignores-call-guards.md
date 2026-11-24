# #208 applySummary の addsNothing が、call の guards(extra)を見ずに追加を飛ばす(疑い、再現せず)

- 状態: **closed** 再現しない(読んで確かめた。動かして再現は試していない)。起きても、飛ばす側が正しいか、結果が変わらない
- 種類: 過検出・見逃しのどちらでもない(疑いのみ)
- 言語: 全
- ROADMAP の節: -
- 守るもの: `internal/taint/add_test.go` の `TestAddAndItsLookAheadsAgree`(`extra` つきの場面は入っていない)
- 題: applySummary の addsNothing が extra を見ない疑い

## 疑い

`internal/taint/summary.go` の `applySummary` は、要約の出力の要素を足す前に、`len(oe.Guards) == 0 && s.addsNothing(...)` なら足すのを飛ばす(コミット 6cdc646c。`origin/main` にある)。見ているのは、要約側の guards(`oe.Guards`)だけで、call の guards(`extra`。仮想呼び出しの候補を選ぶ `dispatchGuards` と、関数の値の呼び出しの `valueGuards`)は見ない。`extra` があると、hop に `setGuards(extra)` が付く。`add` なら、既に plain な hop がある要素に guards つきの hop が来たとき、`differ` が立って `old.markUnguarded()` と `guardEpoch++` をする。飛ばすと、それが起きない。

## 確かめたこと

- `addsNothing` は、既にある hop が plain(guards も callee guards も call も無い)で、`sameChainGuards`(新しい hop の出どころまでの鎖の guards が同じ)のときだけ true を返す。`extra` で hop が guarded でも、既にあるのが plain でなければ飛ばさない。
- 飛ばすのは、既に plain な hop があり、鎖の guards が同じ U のとき。新しい到達は U かつ extra なので、和は U。飛ばして U のままにするのが正しい。`add` の `markUnguarded` は、U を捨てて無条件にする(保守的すぎる)。
- U が空のとき(普通)は、`markUnguarded` しても鎖の guards は空のままで、結果は同じ。
- `copyLoc`、`put` も、`plainExists` で guards を見ずに飛ばす。`extra` を見ない形は、既存の流儀と同じ。

## 結果

- finding が消える: 起きない(要素は既にある)。
- 条件つきとして誤って出る: 起きない。起きるとしても、U が空でない稀な形で、`add` の方が無条件になる(conditional_nodes が減る側。飛ばす方が正しい)。
- 6cdc646c の「結果は同じ」は、この稀な形では厳密には違う(飛ばす方が正しい側)。zoo と curated の差としては、まだ見ていない。

## 直すか

直さない。`extra` を `addsNothing` に入れる必要は無い。守るなら、`TestAddAndItsLookAheadsAgree` に `extra` つきの場面を足す(今は無い)。再現(U が空でない鎖 + plain な既存 hop + `extra`)が作れたら、状態を `open` に戻す。
