# 138: `origin: llm` の `pure` と、流れを切る `flows` のモデルが流れを消しても、`suppressions` と `llm_influenced` に残らない

- 状態: **closed**
- 種類: 正確さ(サイレントな見逃し)
- 言語: Go
- ROADMAP の節: 0
- 守るもの: `cmd/rivus/resolve_test.go`(`TestLLMPureModelSuppressionIsRecorded`)
- 題: `origin: llm` の `pure` と、流れを切る `flows` のモデルが流れを消しても、`suppressions` と `llm_influenced` に残らない
- 見つけた経緯: Go の実際のプロジェクトでの利用から。

`origin: llm` の `pure: true` のモデル(と、source を載せた引数を結果に流さない `flows`)で finding が消えても、出力の `suppressions` は空で、残った finding の `llm_influenced` は false だった。`-deterministic-only` を付けると、その行が無視されて finding が戻る(消したのは、確かにそのモデル)。記録されていたのは `origin: llm` の sanitizer の `drops` だけで、README の「モデルが流れを消した場所は `suppressions` に残ります」と食い違っていた。LLM が書いた「流れを切るモデル」は、最も見逃しを生みやすい(MODELING.md の警告)ので、記録がいちばん必要な種類だった。

## 直した内容

- `internal/taint/calls.go` の `suppressedByCut`: `builtin` 以外のモデル(`origin: llm` と、`-rules` に人が書いたもの)の `pure`、と、結果に届かない operand のある `flows` で、source の載った operand が結果に流れなかったとき、`suppressions` に、モデル・位置・label・落とした source のサイトを記録する。結果が数値と bool のものは、`numeric_stop` の担当なので対象外。`builtin` の `pure` と `flows` は、確認済みで、全部残すと出力が膨らむので対象外。
- `internal/taint/engine.go`: 残った finding が、同じ source の別の流れが `origin: llm` のモデルで消えた関数を通るとき、そのモデルを `models` に入れ、`llm_influenced` を付ける。
- finding は変わらない(記録を足すだけ)。`-deterministic-only` の挙動と JSON のスキーマは変えない。
- テスト: `cmd/rivus/resolve_test.go` の `TestLLMPureModelSuppressionIsRecorded`、`TestLLMFlowsModelSuppressionIsRecorded`、`TestLLMModelThatLetsTheOperandThroughRecordsNoCut`、`TestFindingIsInfluencedByModelThatCutAnotherWay`。
- 文書: README、ARCHITECTURE、PRINCIPLES、AGENT-GUIDE を実際の動作に合わせた。
- 残る限界: 依存パッケージの本体の中の呼び出しでモデルが流れを消した場合は、`suppressions` に出ない(出力は、プロジェクトの関数のものに限る)。
