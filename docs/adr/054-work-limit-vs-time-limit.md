# ADR-054: 作業量の上限と、実時間の上限を、区別して報告する（C-10 の uncertainty の部分）

- 状態: 有効
- 関連: README
- 要旨: `budget:` / `func_timeout:` / `run_timeout:`、`load_dependent`

**背景:** 関数の解析が上限で止まったとき、不確実性は、どの上限でも同じ文面だった（「the analysis budget of this function was exhausted; its summary may miss flows」、`depth_limit`）。止める上限は 3 つあり、性質が違う: **作業量**（`Options.Budget`: 追加した要素の数。どのマシンでも同じ）、**関数ごとの実時間**（`FuncTimeout`: 時計。繰り返しの解析の合計が 4 倍を超えたときも）、**全体の実時間**（`-timeout`）。時計で止まった関数は、**マシンの負荷で、止まる場所が変わる**（同じ入力でも、結果が変わりうる）。これは、読む人に、決定的でない結果だと、伝える必要がある。

**決めたこと:**
1. 理由を分ける。不確実性（`depth_limit`、`attributable_to: rivus`）の文面の先頭が、`budget:`（作業量。決定的）、`func_timeout:`（実時間。負荷に依る）、`run_timeout:`（全体の `-timeout`。負荷に依る）。時計が関わるものを先に報告する（両方のとき）。
2. `stats` に、件数を足す（フィールドの追加だけ。既存の意味は変えない）: `budget_exhausted`、`func_timeouts`、`run_timeouts`（1 つの関数は 1 回、時計で止まったなら時計のほうに数える）、`load_dependent`（時計で止まった関数が 1 つでもある、または全体が止まった）。
3. 出力の先頭の要約（`summary`）に、`load_dependent` と `load_dependent_note`（「この結果は、マシンの負荷に依存する: 関数ごとの実時間で止まった関数 n、全体の実時間で止まった関数 m、…。静かなマシンで実行すると、もっと見つかるかもしれない」）を出す。時計で止まったものが無いときは、出さない（作業量だけなら、決定的）。
4. `stats.unstable` の意味を直した。以前は、最後のパスで要約が変わった関数（10 件まで）を、**収束したときも**出していた（収束した最後のパスで変わった関数は、落ち着いただけ）。今は、**収束しなかったときだけ**出し、`unstable_count` に、その数（全部）を持つ。
5. **上限そのもの（実時間か作業量か、30 秒、400000）は変えない**（C-10。ユーザーの判断待ち）。

（`stats.unstable` の意味を変えた: 前は収束しても入っていた、今は収束したとき空。読む側は今は自分たちだけなので、スキーマバージョンは上げない。）

**変わらないもの:** finding の集合と、どの関数が止まるか（止める条件は、そのまま）。

**検証:** `internal/taint/limits_test.go`（`Budget = 1` で `budget` だけ、`FuncTimeout = 1ns` で `func_timeout` と `load_dependent`、収束した実行で `unstable` が空）、`internal/report/loaddependent_test.go`（要約の文面）。

