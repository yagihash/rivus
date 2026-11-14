# rivus

> ⚠️ 設計段階です。実装はまだありません。

`rivus` は **Dart / Go / Rust に対応予定の taint 解析（汚染解析）ツール**です。
ただし「脆弱性を報告する SAST」ではなく、**到達可能性に関する事実を問い合わせるクエリエンジン**として設計します。

最終的な正誤判定は LLM や人間が行う前提で、rivus は**決定論的に確定できる事実だけを構造化して提示**します。
確定できなかった箇所は、なぜ確定できなかったかを分類して必ず記録します。

名前の `rivus` はラテン語で「小川・水路」。英語 *derive* の語源です。

## 動機

プライベートリポジトリのコードに対して、ファイルを跨ぐ taint 解析を無償で行える選択肢がほぼありません。

- CodeQL: プライベートコードには GitHub Code Security が必要。Dart 非対応
- Opengrep / Semgrep CE: 関数は跨げるがファイルは跨げない
- Joern: ファイル跨ぎはできるが Dart と Rust に非対応
- SonarQube: Dart 対応は Developer Edition 以上（有償）

対象言語の優先度は Dart > Rust > Go です。

## 特徴

- **ファイルを跨ぐ** source → sink の経路解析
- **片側だけのクエリ**: 「この sink に到達し得る source の列挙」「この source が到達し得る sink の列挙」
- source / sink / sanitizer / propagator を **YAML のルール**で定義（組み込みルールも同じ形式）
- **LLM 検証を支援する出力**
  - 経路の各ノードと周辺のソースコードスニペット
  - 値依存のサニタイズの明示（条件と結果の対応を書き出す。判定はしない）
  - 引数の由来（provenance）の追跡
  - 不確実性の所在の分類（`literal` / `config_read` / `dynamic_dispatch` / `unmodeled_library_call` / `depth_limit` など）と、それが対象由来か rivus 由来か（`attributable_to`）

## 設計の概要

```
Go / Rust / Dart extractor ──→ 中立 IR (CFG, JSON Lines) ──→ Go 製コアエンジン
                                                             (SSA → サマリ → taint → 出力)
```

- 言語ごとの extractor がソースを CFG レベルの中立 IR に下ろす
- コアが SSA 化、bottom-up の関数サマリ、ラベル付き taint 解析、出力を担う
- 本体の出力は構造化 JSON。SARIF は副次出力（GitHub のプライベートリポジトリの Security タブは Code Security が必要なため）

詳細は [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) を参照してください。

## ステータス

| フェーズ | 内容 | 状態 |
|---|---|---|
| Phase 1 | Go のみで完結（`go/ssa` から直接 taint 解析） | 未着手 |
| Phase 2 | 中立 IR の切り出し | 未着手 |
| Phase 3 | Rust 対応（固定 nightly 前提） | 未着手 |
| Phase 4 | Dart 対応 | 未着手 |

## 出力のイメージ

```json
{
  "sink":   { "symbol": "...", "location": "handler.go:88", "snippet": "..." },
  "source": { "symbol": "...", "location": "handler.go:71", "snippet": "..." },
  "path": ["..."],
  "conditional_nodes": [
    { "function": "render", "kind": "value_dependent_sanitization",
      "branch_condition": "escape",
      "outcomes": { "escape == true": "sanitized by html.EscapeString",
                    "escape == false": "tainted (passthrough)" },
      "resolved": false }
  ],
  "provenance": {
    "resolved":   [ { "value": false, "call_site": "handler.go:40", "certainty": "literal" } ],
    "unresolved": [ { "terminal_kind": "config_read", "attributable_to": "target" } ]
  },
  "confidence": "requires_review"
}
```

## 使い方（予定）

```sh
rivus analyze ./...                       # source/sink/sanitizer ルールで検出
rivus query --sink 'os/exec.Command' ./...  # この sink に到達し得る source を列挙
rivus models coverage ./...               # モデル未定義の外部関数を頻度順に表示
rivus extract --lang=go --output=ir.jsonl ./   # 中立 IR を出力
```

コマンド体系は仮置きで、実装時に変わります。

## 前提条件

- 対象言語のビルド・コード生成（`go generate`、`build_runner` など）は rivus の責務外。事前に済ませてください
- Dart / Rust の解析には、それぞれのツールチェイン（Dart SDK、固定した nightly の Rust）が必要です

## ドキュメント

- [docs/BRIEF.md](docs/BRIEF.md): 初期ブリーフ（改訂注記つき）
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md): 全体構成、IR、パッケージ構成、データ構造、Phase 1 の分解
- [docs/DECISIONS.md](docs/DECISIONS.md): 設計判断の記録（ADR）
- [docs/GLOSSARY.md](docs/GLOSSARY.md): 用語集
- [docs/QUESTIONS.md](docs/QUESTIONS.md): 初期ブリーフへの質問と指摘

## ライセンス

Apache-2.0（予定）
