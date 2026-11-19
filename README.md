# rivus

> ⚠️ 開発初期です。Phase 1（Go のみ）の途中で、API とコマンドは変わります。

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
- **疑わしい箇所の軽量な列挙**（`rivus sites`）と、そこを起点にした詳細分析（`rivus query --site`）。LLM エージェントの道具として使うことを想定
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
| Phase 1 | Go のみで完結（`go/ssa` から直接 taint 解析） | 完了（ステップ 1.0〜1.12） |
| Phase 2 | 中立 IR の切り出し | 完了（ステップ 2.0〜2.6）。コアは `go/ssa` を知らない。15.6 万行の x/tools を約 29 秒・2.4 GB で解析（[ADR-040](docs/DECISIONS.md)） |
| Phase 3 | Rust 対応（固定 nightly 前提） | 最初の版を実装済み（[`extractor/rust`](extractor/rust/README.md)、[ADR-042](docs/DECISIONS.md)）。MIR を `rustc_public` で読む。`async` とフレームワークの入口は未対応 |
| Phase 4 | Dart 対応 | 最初の版を実装済み（[`extractor/dart`](extractor/dart/README.md)、[ADR-041](docs/DECISIONS.md)）。`rivus analyze -ir` で読む。パターンと関数値のコールグラフは未対応 |

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

## プロジェクトに合わせてモデルを足す

source・sink・sanitizer と、使っているライブラリの伝播（モデル）は、解析するプロジェクトの側に置ける。

1. 解析するディレクトリの `.rivus/rules/*.yaml` に、ルールを書く（組み込みと同じ形式。[models](docs/ARCHITECTURE.md)）。フラグなしで読まれる（`-no-project-rules` で外せる）。`-rules file.yaml` でも足せる
2. 何を書くとよいかは `rivus models coverage` が、検出結果の経路に載っているのにモデルがない関数を、効く順に出す。`-yaml` で、埋めるひな形が出る

```sh
rivus models coverage -yaml -top 20 ./...                       # Go
rivus models coverage -yaml -top 20 -ir package.jsonl           # Dart など
```

モデルはライブラリの API についての知識なので、書いたものは、そのライブラリを使う他のプロジェクトでも使える。広く使えるものは、組み込み（`internal/model/builtin`）に入れる。

## 使い方（Go）

```sh
go build -o rivus ./cmd/rivus

rivus sites ./...                          # source / sink のサイトの列挙（解析なしで高速）
rivus analyze ./...                        # source から sink への経路を、関数・パッケージを跨いで検出（Go。依存も読む。外すなら -no-deps）
rivus analyze -C path/to/project           # ディレクトリの言語を自動で判別（go.mod、pubspec.yaml）。複数あれば全部読んで 1 つの結果にする。-lang go,dart,rust で指定。Rust（Cargo.toml）は、読むとビルドが走るので、自動では読まず、-lang rust のときだけ読む
                                           # Dart と Rust は extractor（rivus-dart、rivus-rust）が PATH にいること。-dart-extractor / -rust-extractor か、RIVUS_DART_EXTRACTOR / RIVUS_RUST_EXTRACTOR で指定
rivus query -sink 'os/exec.Command' ./...  # この sink に届き得る source を列挙
rivus query -source '(*net/http.Request).FormValue' ./...  # この source が届き得る sink を列挙
rivus index -o idx.json ./...              # 解析結果を保存
rivus query -index idx.json -site <ID>     # 保存した結果から、ソースを読み込まずに答える
rivus resolve -bookmarks seen.json ./...   # 前に見た箇所（サイト・検出結果）が、今のコードのどこにあるかを答える
rivus models coverage ./...                # どの関数にモデルを書くと効くか（検出結果の経路に載っている順）
rivus extract -text -func Handle ./...     # 中立 IR を、人が読める形で出す（-text なしで JSON Lines）
rivus analyze -ir package.jsonl            # 他言語の extractor（Dart）が書いた IR を解析する（sites / query / index / resolve / models coverage も -ir で動く）
rivus models coverage -yaml -top 20 ./...  # 埋めるためのルールのひな形を出す
```

- 共通のフラグ: `-C <dir>`（解析の起点）、`-rules <file>`（ルールの追加。繰り返し可）、`-no-builtin-rules`、`-tests`、`-k <depth>`、`-v`（段階ごとの時間と遅い関数）、`-timeout <時間>`（時間切れでも、解析できた範囲を出して `depth_limit` として記録）、`-max-passes <n>`
- `analyze` と `query` の `-detail summary` は、経路とスニペットを省いた概要だけを出す
- 出力の `summary` は、不確実性を kind と `attributable_to`（`target` / `rivus` / `mixed`）ごとに数え、モデルを書くと減るシンボルの上位（`top_symbols`）も出す
- 注意: `(error).Error` のように至る所で呼ばれるメソッドを sink にすると、解析が大幅に遅くなる（[ADR-032](docs/DECISIONS.md)）。ルールは狭く書く
- ルールに `match: implementations` と書くと、インタフェースのメソッドを、自モジュールの実装に展開する（上限 `-max-implementations`、既定 300）
- シンボルは `go/types` の形式（`(*net/http.Request).FormValue`、`os/exec.Command`）。ルールの書式は [組み込みルール](internal/model/builtin) を参照
- `resolve` には、前に出力された検出結果やサイトの JSON をそのまま渡せる。状態は `unchanged` / `changed` / `rematched` / `ambiguous` / `unresolved`。検出結果には、今も届くか（`flow`）も返す。出力には解析したコードの `target`（git の revision と dirty）が入り、ブックマークも `revision` を持てる（出力の `target` を `{"target": ..., "bookmarks": [...]}` で渡す）。結果の `revision_change` が `same` / `different` / `unknown` を示す（ADR-038）
- 値によってサニタイズされるかが変わる関数（`render(s, escape bool)` のようなもの）は、検出結果の `conditional_nodes` に、条件・条件ごとの結果・呼び出し側が渡している式・引数の由来（`provenance`。定数、設定の読み出し、パッケージ変数、公開関数の引数など、辿れなくなった理由つき）を書き出す。どちらが起こるかは判定しない。引数が定数のときは、起こりえない側の流れを捨てる
- ルールの `origin: builtin | human | llm` で、モデルを決めた主体を区別できる。`-deterministic-only` で `llm` のルールを無視する。LLM 由来のモデルが影響した検出結果は `llm_influenced`、検出結果を消した場所は `suppressions` に残る
- ルールには `pure: true`（その関数を通って汚染は流れない）と、フレームワークの入口を宣言する `kind: param` が書ける
- 出力は構造化 JSON。SARIF は未対応（副次出力として後で作る。GitHub のプライベートリポジトリの Security タブには Code Security が必要）

### 開発

```sh
go test ./...                                              # 単体テストとマイクロベンチマーク
RIVUS_TEST_MODULE=/path/to/module go test ./internal/taint -run TestRealCode -v   # 実在のモジュールで頑健性を確認
RIVUS_TEST_MODULE=/path/to/module go test ./internal/ssacheck -run TestAgainstGoSSAOnAModule -v   # 自作の SSA 化を go/ssa と比べる
```

マイクロベンチマークは `testdata/go/cases` にあり、ソース中の `// @source(tag)` と `// @sink(tag)` が期待値になります。
`@sink(none)` は「検出されないこと」を表します。

## 前提条件

- 対象言語のビルド・コード生成（`go generate`、`build_runner` など）は rivus の責務外。事前に済ませてください
- Dart / Rust の解析には、それぞれのツールチェイン（Dart SDK、固定した nightly の Rust）が必要です

## 信頼できないコードを解析するとき

**解析する対象のコードが、解析する人のマシンで動くことがあります。** 採用前のサードパーティの確認や、怪しいリポジトリのトリアージにも使うツールなので、先に書いておきます。

| 言語 | 何が動くか |
|---|---|
| Rust | extractor が、対象のワークスペースに `cargo check` を走らせます（rustc の代わりに driver を差し込むだけで、**対象をコンパイルする**）。`build.rs`、proc macro、`[build-dependencies]` が実行されます |
| Go | `go/packages` が `go list` を走らせます。依存モジュールの取得（ネットワーク）が起きることがあります。cgo のあるパッケージは、cgo の前処理が走ります |
| Dart | extractor 自身は、`package:analyzer` で静的に読むだけです。ただし、前提の手順（`dart pub get`、`build_runner` などのコード生成）は、対象の依存と生成器のコードを実行します |

- **Rust は、対象のビルドが、解析結果そのものも書き換えられます。** extractor は、IR の断片を書くディレクトリ（`RIVUS_IR_DIR`）を、対象のビルドに見える環境で渡すので、対象の `build.rs` や proc macro は、断片を足したり消したりできます。「ビルドが実行される」ことの、もう1つの帰結です。
- **Rust は、`-lang rust` と書いたときだけ読みます。** `Cargo.toml` があるだけでは、読みません（Rust だけのディレクトリは、明示を求めるエラーになり、Go や Dart と一緒のときは、Rust を除いて、stderr に書きます）。
- **ディレクトリに付いてくるルール（`<dir>/.rivus/rules/*.yaml`）は、フラグなしで読まれます。** それは、解析されるコードを書いた人が、そのコードについて言わせたいことなので、`sanitizers` や `pure: true` で、検出を消せます。読んだファイルと、消せるものの数（sanitizer、`drops: ["*"]` の sanitizer、`pure` のモデル）は、結果の `project_rules` に出ます。信頼できないコードでは、`-no-project-rules` を付けてください。
- **Dart の依存は、プロジェクト、pub のキャッシュ、SDK の下のものだけ読みます。** `package_config.json` は、対象の一部で、どこでも指せるからです。ほかの場所（モノレポの隣のパッケージなど）は、`-allow-deps-under DIR` で足します。

信頼できないリポジトリは、使い捨ての環境（コンテナや VM）で解析してください。たとえば、依存を先に取得してから、ネットワークを切ったコンテナで、ソースを読み取り専用で渡す形です（Rust の extractor は、作業用のディレクトリを、一時ディレクトリに作ります）。

```sh
# 例: 依存の取得は先に済ませ、解析はネットワークなしで、ソースは読み取り専用
docker run --rm --network none -v "$PWD":/src:ro -w /src my-rivus-image rivus analyze ./...
```

`my-rivus-image` は、rivus と、対象の言語のツールチェインを入れた、自分で用意するイメージです。

## ドキュメント

- [docs/BRIEF.md](docs/BRIEF.md): 初期ブリーフ（改訂注記つき）
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md): 全体構成、IR、パッケージ構成、データ構造、Phase 1 の分解
- [docs/DECISIONS.md](docs/DECISIONS.md): 設計判断の記録（ADR）
- [docs/GLOSSARY.md](docs/GLOSSARY.md): 用語集
- [docs/QUESTIONS.md](docs/QUESTIONS.md): 初期ブリーフへの質問と指摘

## ライセンス

Apache-2.0（予定）
