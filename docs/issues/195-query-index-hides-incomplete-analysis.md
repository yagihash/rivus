# 時間切れの index に対する問い合わせの出力に、元の解析が完全でなかったことが出ない

- 状態: closed(修正した。`query` と `resolve` の出力の最上位に、`incomplete` と `incomplete_reasons` を、いつも出す。ADR-120 の追記)
- 種類: サイレントな見逃し(問い合わせの出力)
- 言語: 全
- ROADMAP の節: -
- 守るもの: `TestQueryIndexOfATimedOutAnalysis`、`TestQueryIndexOfACompleteAnalysis`、`TestQueryOfTheSourceSaysIncomplete`、`TestResolveIndexSaysIncomplete`、`TestQueryIndexOfTheOlderForm`(`cmd/rivus/queryincomplete_test.go`)
- 題: 時間切れの index に対する問い合わせの出力に、元の解析が完全でなかったことが出ない

## 何が起きたか

`rivus index -taint -timeout 1ns` は終了コード 4 で終わり、保存した index の `summary` に `incomplete: true`、`incomplete_reasons: ["timeout"]`、`stats` に `converged: false`、`timed_out: true` が入る(ADR-120)。ところが、その index に対する `rivus query -index <index> -sink <symbol>` の出力には、`summary` も `incomplete` もなく(キーは `schema_version`、`query`、`matched`、`groups`、`findings`、`stats` だけ)、終了コードは 0 だった。

`matched` に sink が 1 件あり、`groups` と `findings` が 0 件、という出力は、「この sink に届く source は無い」と読める、完全な答えに見える。実際は、探していない(サイレントな見逃し)。同じ意味の情報は `stats` にあったが、`stats` を読まない呼び出し側(パイプ、ファイルに保存してから読むエージェント)は気づけない。`resolve -index` も、同じ index を読むので同じ。

## 再現(合成の対象。実測)

1. 小さい Go のモジュール(source `Source`、sink `Sink`、`F` で `Sink(Source())`)を作る。
2. `rivus index -taint -rules rules.yaml -graph g.db -o idx.json -timeout 1ns ./main`(終了コード 4)。
3. `rivus query -index idx.json -sink <Sink のシンボル>`: `matched` あり、`findings` 0 件、`incomplete` の項目なし、終了コード 0。

## 修正の方法

- `query` と `resolve` の出力の最上位に、`incomplete`(bool)と `incomplete_reasons`(配列。完全なら `[]`)を、`-index` のときも、今の解析から答えるときも、いつも出す(`report.Report.IncompleteState`)。
- `-index` は、保存した `summary` の値を引き継ぐ。古い形(`summary.incomplete` がない)の index は、`stats` から作る。`stats` もないときは `incomplete: true`、`["unknown"]`。
- 終了コードは変えない(`-index` は 0。今の解析は 4)。既存の項目は変えない(`schema_version` はそのまま)。

コミット: この課題を修正した PR のコミット(`feat/query-incomplete`)。
