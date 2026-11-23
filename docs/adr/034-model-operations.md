# ADR-034: モデルの運用機能（ステップ 1.11）

- 状態: 有効
- 関連: MODELING, README
- 要旨: `models coverage`、`pure`、`origin`、`--deterministic-only`。複数経路は見送り

- **状況:** 不確実性の大半（`rivus` に帰属する 74〜93%）は、モデルを書けば減る（ADR-032）。そのための道具と、LLM が書いたモデルを監査できる仕組みが必要（ADR-010）。
- **決定:**
  - **`rivus models coverage`:** モデルのない呼び出しを、**検出結果の経路に載っている数（`findings`）、呼び出しの数、関数の数**の順に並べる。対象は `unmodeled_library_call`、`reflection`、実装が分からない `dynamic_dispatch`。
    `-yaml` で、埋めるためのルールファイルのテンプレートを出す（流れ方は仮置きで `TODO` を付ける。人が決める）。`-index` で保存した結果から出せる。出力したテンプレートは、そのままルールとして読み込める
  - **`pure: true`（propagator）:** その関数を通って汚染は流れない、というモデル。ロック、時計、ログ、述語など。
    「引数が戻り値に流れる」という保守的な既定と、それに伴う不確実性をなくす。組み込みに、よく出る標準ライブラリの関数（`sync`、`time`、`log`、`strings` の述語、`sort` など）を足した
  - **モデルの出所（`origin: builtin | human | llm`）:** どのルールにも付けられ、ルールセットの先頭に書けば既定になる（書かなければ、組み込みは `builtin`、ほかは `human`）。
    `llm` のルールは `decided_at`、`rationale`、`model_version` を必須とし、欠けるとルールの読み込みでエラーになる
  - **`--deterministic-only`:** `origin: llm` のルールをすべて無視して解析する。出力に `deterministic_only` と、無視した数（`ignored_models`）が残る
  - **出力に残す記録:**
    - 検出結果の `models`: 経路で何かを決めたモデル（source と sink のルール、通った sanitizer と propagator）とその出所。`llm_influenced` は、そのうち 1 つでも `llm` なら true
    - `suppressions`: `origin: llm` の sanitizer が、実際に汚染を除去した場所（モデル、位置、除去したラベルと、その発生元のサイト）。検出結果自体がなくなるので、検出結果には書けない
  - **`param` 型の source:** 関数のシンボルと位置（`params`、`recv`）で、フレームワークが汚染を渡す入口を宣言できる。`match: implementations` と組み合わせられる。
    ARCHITECTURE 6 節にある `entrypoints:` セクションの実体はこれで、source の 1 種として実装した（別のセクションは作っていない）
- **効果（組み込みモデルの追加前後。検出結果は変わらない）:**

  | 対象 | 不確実性の件数 | `rivus` の比率 | 検出 |
  |---|---|---|---|
  | ghmint | 288 → **170** | 87% → 80% | 15 件のまま |
  | rivus 自身 | 390 → 337 | 93% → 92% | 0 件のまま |
  | x/tools | 12,350 → 10,638 | 78% → 74% | 4 件のまま |

  `pure` を外すと対応するテストが失敗することを確かめてある（モデルが効いている）
- **見送ったもの:** 検出結果ごとの複数経路と経路数の上限（現状は経路 1 本で足りている）。LLM の判定をモデルのファイルに書き戻す機能（形式は決めたが、書き戻す道具は作っていない）。
  `models coverage` のテンプレートは `propagator` の仮置きのみで、sanitizer や sink のテンプレートは出さない
- **帰結:** ルールを書く人（と、書かせる LLM）は、`models coverage` で優先順位を決め、`-yaml` のテンプレートから始め、`origin` を付け、`--deterministic-only` で影響を確かめられる

