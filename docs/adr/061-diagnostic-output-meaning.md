# ADR-061: 診断の出力（`-pass-stats`、`-trace-element`）の意味（結果は変えない）

- 状態: 有効
- 関連: -

**背景:** Gitea の収束しない原因（ADR-055 / 056 / 057）は、要約の変化をパスごとに数える診断と、source の要素が関数から出る最初の場所を印字する診断で見つけた。この診断は、計画 2（ADR-047）のブランチに閉じていた。収束の調査は、また必要になるので、実験のコードから切り出して、main に置く。**診断は、結果（finding、不確実性、`stats` の既存のフィールド）を変えない**。

**決めたこと:**
1. **`-pass-stats`**（開発用のフラグ。既定は切）: `stats` に、2 つのフィールドを足す。
   - `pass_changes`: パスごとに、`changed`（要約が変わった関数の数。最初の要約は、変化に数えない）、`new_flow`（要素が増えた関数の数）、`added_elems`（増えた要素の数。ガードは見ない）、**`new_elems`**（その関数の、それまでのどの要約にもなかった要素。解析が、何かを見つけている）、**`returned_elems`**（前の要約にあったが、最後のにはなく、戻ってきた要素。要素が増えたり減ったりする往復）、`guard_only`（同じ要素で、ガードだけ変わった関数の数）、`shrunk`（要素が減った関数の数。要約は前のものと join するので、0 のはず）。収束しない実行では、`new_elems` が続くなら、解析が増え続けており、`returned_elems` が続くなら、往復している（ADR-057 の形）。
   - `unmodeled_top`: モデルのない関数（`(unmodeled)` の呼び出し）の上位 20 件。**呼び出しの回数ではなく**、2 つの列: `reached`（汚染された値が、その関数の引数に、実際に届いた回数。呼び出し側の関数の解析ごとに数える。引数に汚染されたものがない呼び出しは数えない）と、`on_path`（結果に残った finding のうち、表示される経路に、その呼び出しがあるものの数）。並びは、`reached`、`on_path` の降順、シンボル名。`on_path` は、表示される（最短の）経路だけを見るので、「その関数のモデルがないために見える finding」の下限。
   - `packages_top`（アドバイザーの提案）: パッケージ（Go は import パス、Dart は `package:名前`、Rust は crate。`report.UnitOf` と同じ `taint.UnitOf`）ごとの、`reanalyses`（その関数の解析の回数）、`spent_ms`（かかった時間。時計）、`functions`、`summary_elems`（要約の要素の合計）、`largest_summary` と `largest_summary_elems`。上位 20 件で、**並びは `reanalyses`（時計に依存しない）、`summary_elems`、名前の順**（時間で並べると、実行ごとに違う一覧になる）。`own_spent` / `dep_spent` は 2 分割しかできず、mock や生成コードが時間を伸ばしている仮説（候補への mock の混入、生成コードの大きさ、`ctrl.Call` / `Called` の可変長 `any` が全部を混ぜる合流点）を分けられないので、パッケージごとに見る。
   - `dispatch_candidates`: interface、trait、関数値を通る呼び出し（コードの呼び出し箇所を 1 回ずつ。解析の前の、静的な候補）を、候補の数で数える（`one`、`two_to_8`、`9_to_16`、`over_16`、候補なし `none`）と、最大の数とその関数。候補が多い呼び出しが、合流点。
2. **`-trace-element <site id>`**（開発用。既定は切）: その source の要素が、関数の要約へ最初に出た場所（関数ごとに 1 度）と、パッケージ変数の格納に入った場所を、stderr に印字する（`TRACE`、`TRACEWRITE`）。環境変数（`RIVUS_TRACE_ELEM`）だった。フラグにしたのは、`-memprofile` と同じく、開発用のフラグを、`analyze` の一覧に集めるため。**印字の行の順は、解析の順で、実行ごとに違いうる**（並列の解析）ので、結果には出さない（`stats` にも入れない）。
3. **スキーマバージョンは上げない**（`rivus.result/1`）。足すだけで、フィールドは `omitempty`、既定では出ない（ADR-034 の、足すだけの変更の扱い）。
4. **決定的であること**: 新しいフィールドは、時計に依存しない（要素の数と、呼び出しの回数だけ）。`TestDeterministicPassStats`（Go）と、Dart・Rust の `TestDeterministicCurated*`（`-pass-stats` つき）は、出力全体を、`GOMAXPROCS` と `-workers` を変えて比べる（`stable()` が落とすのは、時間のフィールドだけ: 3 フィールドと、`packages_top` の `spent_ms`）。`TestPassStatsChangeNothing` は、`-pass-stats` ありなしの出力が、新しい 2 フィールドを除いて同じことを見る。時間の上限（`-timeout`、`-func-timeout`）が効いた実行は、`load_dependent` が示すとおり、ほかのフィールドと同じく、時計に依存する。

**限界:** 診断は、実行のコストを増やす（`pass_changes` は、要約が変わるたびに、要素の集合を作って比べる。`unmodeled_top` は、ロックを取る）ので、既定では切。数えるのは、実行がたどった解析で、要約の値ではない。

**測定:** `compare.sh 6c5c28c <この変更>`: finding の追加・削除 0、新しいフィールド以外に差なし（コミットに記す）。

**追記(`-pass-stats-top`、`-pass-stats-sort`、`largest_scc_packages`):**
- `-pass-stats-top N`(既定 20、0 は全件)は `packages_top` の件数、`-pass-stats-sort KEY` は並びのキー(`reanalyses`(既定)、`time`、`functions`、`elems`。同じ値は名前順)。`time` は時計なので、実行ごとに違う。上の「上位 20 件で、並びは `reanalyses`…」は、既定の動き。
- `largest_scc_packages`: 最大の SCC の関数を `taint.UnitOf` ごとに数えた上位 10 件(`package`、`functions`)。呼び出しグラフから解析の前に作る(`-pass-stats` のときだけ、SCC の関数を覚える)ので、**時間切れの実行にも出る**(解析が 1 つも進む前に切れても出る)。`packages_top`(解析した分)と `largest_summary` も、途中までの分が出る。0 件のときは出さない。
- `-timeout` は、8 関数以上の SCC の並列ラウンドの中でも、各関数の計算の前に見る(課題 #126)。期限がなければ結果は変わらない。
- 使い方(自分のプロジェクトで表を得る手順)は README の「使い方」。


