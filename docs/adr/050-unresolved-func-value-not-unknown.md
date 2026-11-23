# ADR-050: 関数値の呼び出しを、rivus の上限で解決しなかったときは、「知らない」と報告しない（CORE-D）

- 状態: 一部覆った(→ADR-070)
- 関連: -
- 状態の詳細: 拡張: 070。追記(2026-11-22): 候補の上限を 8 から 32 にした
- 要旨: `candidate_limit` を区別。32 を超える形はサイレントな見逃し(#77)

**背景:** `internal/valuecalls` の `resolve` は、関数値の呼び出しの候補を、コピー・φ・パラメータ（呼び出し元が渡す引数、`maxDepth` = 3 段まで）を通って求める。見つかった関数が 8 個（`most`）を超えると、候補を 1 つも付けずに戻る。何も見つからなかったときと同じ形だったので、解析は不確実性 `unresolved_callee`（「the called function is not known」、`attributable_to: mixed`）を出した。**関数は分かっているのに、rivus が自分の上限で捨てた**ので、これは誤った注記だった（読む人は、コードの側の問題（動的で分からない）と読む）。呼び出し元を 3 段より上にたどらないで 0 件のときも同じ。

**決めたこと:**
1. `ir.Callee` に、IR のストリームに出ない `ValueLimit` と `ValueFound` を足した（`ValueCands` と同じ扱い）。`resolve` が、候補を付けないときに理由を残す。
   - `candidate_limit`: 見つかった関数が 8 個を超えた（`ValueFound` = 見つけた数）。
   - `caller_depth_limit`: 何も見つからず、呼び出し元をたどる歩みが `maxDepth` で切られた（`resolver.cut`）。切られていなければ（本当に分からない）、従来どおり `unresolved_callee`。
2. 解析（`calls.go` の呼び出しの処理、`provenance.go` の来歴）は、`ValueLimit` があるとき、`unresolved_callee` ではなく、**`depth_limit`**（`attributable_to: rivus`）を出す。文面は理由で始まる（`candidate_limit: the function value may be any of N functions, more than the 8 that rivus keeps; ...`、`caller_depth_limit: ...`）。場所は、呼び出しの位置と関数。kind を `depth_limit` にした理由: 「上限を超えたので、解析が切った」という意味に合うのは `depth_limit`（`guard_limit`、`path_limit` と同じ系列）で、`unresolved_callee` は「関数がどれか分からない」ことだから。
3. **上限そのもの（8、3 段）は変えない**。候補を持たない呼び出しの扱い（候補なしで、モデルもなし）は同じ。finding の集合は変わらない（不確実性の kind と文面と attributable_to だけが変わる）。

**限界:** 3 段で切れたが、途中で 1〜8 個が見つかったとき（候補が足りないかもしれない）は、まだ、何も報告しない。呼び出し元の 3 段より先にある関数は、候補に入らない（`resolver.cut` は、この場合も立つが、不確実性の注記は 0 件のときだけに付けた）。8 と 3 を外すのは、案 2（巨大 SCC の整理）の ADR で。

**検証:** `bench/dart-curated/lib/limits.dart`（9 個の関数を渡すヘルパー、5 段のヘルパーに渡すクロージャ。`TestCuratedDart` が、`depth_limit` の `candidate_limit` と `caller_depth_limit` を要求する）。Go は、VTA が候補を持つので、この経路を通らない。curated の `@miss(silent)` の数え方（source と sink の関数にある不確実性だけを不確実性の注記とする）では、ヘルパーの中の注記は数えない。そのため、足した 2 つの見逃しは `@miss(silent)` のまま（Dart 27/34 → 29/36）。数えているのは、足したケースの 2 件だけで、既存の見逃しの注記は変わらない。

**追記(2026-11-22、相談役の判断): 上限を 8 から 32 にした(`ir.ValueCandidateMost`)。** 本文の「3. 上限そのもの(8、3 段)は変えない」は、候補の数の上限については、この追記で変わった。理由: zoo の Dart で、同じヘルパーに 10 種類のクロージャを渡す形(`F/clos/helper-overflow`)が、source の関数にも sink の関数にも何も出ずに流れが消えていた(サイレントな見逃し。不確実性の注記は途中のヘルパーにだけ付く)。測定(`RIVUS_MOST` で変えた試作の bin): 今のベンチで 8 を超える呼び出し点は 4 か所で、全部 Dart(zoo-dart の 1 か所〔候補 12〕、dart-curated の 3 か所〔9、10、22〕)。8 → 16 / 32 / 上限なしで、増えた finding は本物の流れだけ(zoo-dart +10、dart-curated +2。消えた本物の流れは 0)、Rust の rust-curated は過検出が 10 件消える(`narrow.rs` の `lim9`。絞り込みが効く)、Go と Rust の zoo、go-curated、gcp は差 0。時間とメモリは同じ(zoo-dart 5.5 → 5.6 秒、最大 RSS 1088 → 1085 MB。gcp 39.9 → 40.0 秒)。Go と Rust は、コールグラフが候補を持つので、この経路を通らない(10 種類のクロージャでも流れが残る)。32 にした理由: 上限なしは、候補が数百の呼び出し点でのコストが未測定(go-scale と wild は取得済みがない)。33 以上は、これまでどおりサイレントな見逃し(`F/clos/helper-overflow-many`、`@miss(silent)`、`dart-curated` の `limit_many` は 33 種類に増やした)。注記は途中のヘルパーにしか付かない(渡した側の関数には付かない)ので、要約に「どのパラメータの値の呼び出しが上限に達したか」を持たせて渡した側に注記する修正が残る(要約の形が変わるので、相談のうえで)。

