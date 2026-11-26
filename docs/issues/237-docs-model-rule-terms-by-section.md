# #237 文書で「モデル」と「ルール」の使い方が、定義とずれている所を、節の名前に直す

- 状態: **open**
- 優先度: 低
- 種類: 文書の用語の整合
- 言語: 全
- ROADMAP の節: -
- 守るもの: -
- 進める完了条件: -

## 決めた用語(2026-11-27、ユーザー。advisor の案)

- **ルールのファイル**: `sources`、`sinks`、`sanitizers`、`propagators`、`calls` などの節を持つ YAML。組み込みは `internal/model/builtin/`、利用者は `-rules` と `.rivus/rules/`。
- **ルール**: ルールのファイルに書く項目の総称。どの節の項目も、ルール。
- **モデル**: 本体を読まない関数(ライブラリなど)の振る舞いを書いた項目。主に `propagators` と `calls`。`sanitizers` を含めてもよい。
- **`sources` と `sinks`**: 何を入力とみなし、どこを危険な箇所とみなすかを決める項目。モデルではなく、「何を検査するか」を決めるルール。

この定義は PR #621(`-no-builtin-sources-sinks`)の `docs/GLOSSARY.md` に入る(2026-11-27 の時点で #621 は未マージ。マージされたら GLOSSARY に載る)。この課題は、#621 のマージのあとに進める。新しい用語は作らない。

## 何が起きるか(事実。`origin/main`(5ff6322b)の実物で、ファイルと行を確かめた)

「モデル」が `sources` や `sinks` を含む使い方が残っている。

- `docs/PRINCIPLES.md:92`: 「sink のモデルは検出に要る」、「組み込みのモデルの範囲」。「モデル」に `sinks` を含めている。
- `docs/PRINCIPLES.md:95-97`: 「モデルの種類で扱いを分ける」の下位に、流れのモデル(`flows`、`pure`、`calls:`)と並べて、source / sink(何が危険か)がある。
- `docs/PRINCIPLES.md:103`: 「モデル: `internal/model/builtin/*.yaml`、... `RuleSet` の `Sources` / `Sinks` / `Sanitizers` / `Propagators` / `Globals` / `ConfigReads`」。「モデル」が `sources` と `sinks` を含む。
- `docs/PRINCIPLES.md:165`: 「LLM の判定から作ったモデルは `origin: llm`」。`origin` はルール全般に付くので、「ルール」が合う。
- `docs/MODELING.md:1`: 題が「Writing models for rivus」。`sources`、`sinks` を含む全部を model / row と呼ぶ。
- `docs/MODELING.md:209`: 「a sink model is needed for detection」。
- `docs/MODELING.md:212`: 節の題(「A sink that is one field of an argument: `parts`」)は sink の話。

定義と合っている所(直さない):

- `docs/PRINCIPLES.md:94`(「ルールの書式の安定」の「ルール」)。
- `docs/MODELING.md:352`(「A row that is only a source or only a sink is not a model」)。
- `docs/GLOSSARY.md:532`(「モデル: 関数 1 つの振る舞いの記述」)。
- `docs/GLOSSARY.md` の `origin` と `--deterministic-only` の「モデル」は、#621 で「ルール」に直した。

## 進め方(案。決定ではない)

- 「モデル」が `sources` や `sinks` を含む所を、節の名前(「`sources` と `sinks`」、「`propagators` など」)に直す。
- `docs/MODELING.md` の題「Writing models」は、`sources` / `sinks` も書き方を説明する文書なので、題の言い直し(「Writing rules and models」など)が要るかを、ユーザーと決める。**未決**。
- 直す範囲は、上の 7 か所から始め、`grep` で同じ使い方を拾い直す(行番号は動く)。

## 別の課題(触れるだけ)

wording.rego が禁じる「〜の行」の語が、既存の `docs/AGENT-GUIDE.md`、`docs/DESIGN-graph.md`、`docs/PRINCIPLES.md:198` などに残っている。用語の課題とは別の、wording の課題。

## 関連

- PR #621(`-no-builtin-sources-sinks`。用語の定義が `docs/GLOSSARY.md` に入る)
