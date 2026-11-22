# 124: -resolve-func-values: 補った候補が、サイトの列挙に戻らない。本体のない候補では、補った注記が出ない

`-resolve-func-values`(docs/DESIGN-callouts.md 5.6)は、候補 0 件の値の呼び出しに、関数値のパスが運んだ関数を候補として書く(`Callee.ValueCands`、`Instr.CandidateFns`)。次の 2 つが、今の版にない。

1. **サイトの列挙**: `sites.Enumerate` は、値の呼び出しの候補ごとに、ルール(source / sink のシンボル)が当たるサイトを作る(ADR-045)。列挙はグラフを作る前に、IR を読んで行い、グラフを作るときに IR は捨てられるので、補った候補は列挙に戻らない。補った候補の関数を、ルールが名指す形(自分のコードの関数に source / sink のルールがある)では、そのサイトが出ない。測った 6 つの対象では、そういう候補はない。
2. **本体のない候補**: 補った候補が、本体を持たない関数(外部の関数への参照)のとき、`unresolved_callee`(`func_values_supplemented:`)の注記は、`applySummary` に渡す注記の経由で出るため、出ない(`external` の分岐の、外部の関数の注記だけが出る)。

修正の案: (1) 補った候補の分だけ、列挙を足す(`FuncValueSites` と同じ要領で、補ったあとに、`sg` から作る)。(2) `external` の分岐で、`ValueSupplemented` の呼び出しには注記を足す。

## 追記(2026-11-22)

補った辺がグラフの索引に入らない、という限り(この課題の番号つきの 2 つとは別)は直した。フラグ on の `rivus index` が、補ってから索引を書く(docs/DESIGN-callouts.md 5.6)。この課題の (1) サイトの列挙と、(2) 本体のない候補の注記は、まだ。
