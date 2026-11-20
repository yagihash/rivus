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
rivus index -C path/to/project ./...       # コールグラフ(構造の層)を SQLite に保存する。速い(taint 解析はしない)。書き先は、ユーザーのキャッシュ(-graph <file> で指定)
rivus graph info -C path/to/project ./...  # 保存したファイルの中身(件数、作成日時、source_hash)と、今のソースと一致するか(fresh: true/false)
rivus graph callers -C . os/exec.Command   # この関数を呼ぶ関数(逆向き)。--depth N(既定 1、最大 6)、--limit K(既定 50 辺)
rivus graph callees -C . main.go:42        # その行の呼び出し点が呼ぶ関数(順向き)。複数あれば、候補の一覧を返して選ばせる
rivus graph path -C . --from cmd.run --to os/exec.Command  # 経路を k 本(--limit、既定 5)、最短から。静的 → 動的 → ハブ込みの順に探す(searched に、どこまで探したかを書く)
rivus graph path -C . --from-entry --to os/exec.Command    # main と init(ライブラリなら exported な関数)から届く経路。出力の entry に、何を entry としたかを書く
rivus graph path -C . --from a --to b --avoid log.Error --via db.Query --only-impl bytes.Buffer  # 辺のフィルタ(callers / callees でも使える。繰り返し可。filters に必ず書く)
rivus index -taint -o idx.json ./...       # taint 解析の結果を JSON で保存(-o だけでも同じ)。構造の層も、同じ実行で、キャッシュに書く
rivus query -index idx.json -site <ID>     # 保存した結果から、ソースを読み込まずに答える
rivus resolve -bookmarks seen.json ./...   # 前に見た箇所（サイト・検出結果）が、今のコードのどこにあるかを答える
rivus models coverage ./...                # どの関数にモデルを書くと効くか（検出結果の経路に載っている順）
rivus extract -text -func Handle ./...     # 中立 IR を、人が読める形で出す（-text なしで JSON Lines）
rivus analyze -ir package.jsonl            # 他言語の extractor（Dart）が書いた IR を解析する（sites / query / index / resolve / models coverage も -ir で動く）
rivus models coverage -yaml -top 20 ./...  # 埋めるためのルールのひな形を出す
```

- 共通のフラグ: `-C <dir>`（解析の起点）、`-rules <file>`（ルールの追加。繰り返し可）、`-no-builtin-rules`、`-tests`、`-k <depth>`、`-v`（段階ごとの時間と遅い関数）、`-timeout <時間>`（時間切れでも、解析できた範囲を出して `depth_limit` として記録）、`-max-passes <n>`
- **`rivus index` と `rivus graph info`**（設計: [docs/DESIGN-graph.md](docs/DESIGN-graph.md)）。`index` は、コードを 1 回読んで、関数・呼び出し点・辺（静的 / インターフェース / 関数値の別つき）・サイトを、1 つの SQLite ファイルに保存する（純 Go。cgo なし）。使い捨てのキャッシュで、マイグレーションはしない（スキーマ版が違えば、エラーで、作り直しを案内する）
  - 書き先は `-graph <file>`。なければ、ユーザーのキャッシュ（`os.UserCacheDir()/rivus/<-C の絶対パスのハッシュ>/graph.db`。`RIVUS_CACHE_DIR` で変えられる）。**解析対象のディレクトリには作らない。読むのも、`-graph` で明示したファイルか、`-C` から導いたキャッシュだけ**（読み取り専用で開く）
  - **構造の層だけ**を作る（taint の層、つまり findings と不確実性は、このファイルにはない。`index` はそれを出力に書く。`-taint` でも、taint の結果は、これまでの JSON の index（`-o`）に入り、`query -index` / `resolve -index` はそれを読む）
  - `graph info` は、スキーマ版、rivus の版、作成日時、`source_hash`、層、件数、サイズ、言語を JSON（`rivus.graph/1`）で出す。**`-C <dir>` を付けると、鍵を今のソースから作り直して照合し、`fresh: true/false` と、変わった点（`stale_reasons`）を返す**。`-C` なしのときは、`verified: false`（今のソースとの一致は確認していない）。照合には、`index` に渡したのと同じフラグ（`-tests`、`-no-deps`、`-k`、`-lang`、`-rules`、パッケージなど）を渡す（違えば、`fresh: false`）
  - `source_hash` には、結果に影響しうるものを全部入れる: 実際に読んだファイルの内容（Go は `go/packages` の一覧。依存、標準ライブラリ、ビルドタグで除かれたファイル、`go.mod`、`replace` の中身を含む）、ツリーのファイルの一覧（新しいファイルを見逃さない）、`go.sum` / `go.work` / `pubspec.lock` / `Cargo.lock`、ルール（組み込みの内容、`.rivus/rules`、`-rules`）、フラグ、rivus の版（実行ファイルのハッシュ）、`go env` の全項目、extractor と SDK の版。Dart と Rust は、extractor が読んだファイルを言わないので、プロジェクトのツリー全体（`.git`、ビルド出力などを除く）の内容を鍵にする。**疑わしいときは、`fresh: false`**
- **`rivus graph callers | callees | path`**（段階 2。DB だけを読む。ソースの再読み込みも taint もしない）。`-graph <file>` か `-C <dir>`（そこから導くキャッシュ）で DB を指す。`-C` を付けると `graph info` と同じ照合をして、出力の `snapshot.verified` / `fresh` に入れ、**古ければ `warnings` に書く**（答えは返す。古いソースの答えである）。`index` に渡したフラグは、`-C` で照合するとき、与えなければ DB に記録されたものを使う
  - **指定（target）は 4 通りを形で判別する**: シンボル（`os/exec.Command`、`(*pkg.T).Method`、Rust は `crate::f`。前を省いた短い指定(`GetUserByID`、`user.User.GetName`、`Widget.build`、`Printer::handle`)も、**識別子の境界で末尾が一致し、グラフ全体で 1 つに決まるときだけ**通る(`ByID` は `GetUserByID` に当たらない)。複数なら、エラーにせず `ambiguous`(自分のコード、本体あり、名前の順に最大 20 件)と `ambiguous_total`、`pkg.Name` のヒントを返す）、`file:行:桁`（その位置から始まる呼び出し点。桁がずれていれば、その行でそれより前に始まる最後の呼び出し点）、`file:行`（その行の呼び出し点。複数あれば、**エラーにせず `ambiguous` に一覧を返す**）、`site:<ID>`（`rivus sites` のサイト。`callers` / `callees` は `--site <ID>` でも）。ファイルは、DB の名前（解析対象からの相対）でも、絶対パスでも、末尾だけでも（1 つに決まるなら）良い。見つからなければ、近い候補を添えたエラー
  - **指定が何を指すか**（関数 F と、呼び出し点 C = 関数の中の、別の関数を呼ぶ場所）: `callers F` は F を呼ぶ関数、`callees F` は F が呼ぶ関数。`callers C` は **C を含む関数**の呼び元、`callees C` は **C が呼ぶ関数**（C の呼び先の候補）。`path --from C` は C を通って始まり、`path --to C` は C を含む関数で終わる（最後のステップが C）。例: `main.go:42:7` が `exec.Command(...)` の呼び出しなら、`callees main.go:42:7` は `os/exec.Command`、`callers main.go:42:7` は、それを含む関数の呼び元
  - **builtin**（`len`、`append` など）の呼び出し点は、既定で結果から外す（呼び出し点の約 4 割）。外した数は `omitted.builtin`。`--include-builtin` で `builtin:len` のノードとして戻す
  - **動的な辺**（インターフェース、関数値）は、`kind` と `candidates`（その呼び出し点の候補の数）を必ず持つ。`callers` では「呼び元は、候補の 1 つとして、これを呼びうる」と読む。`callees` は静的な辺をたどり、**候補が 3 以下の動的な呼び出しは展開し、それより多いもの（と候補が 0 のもの）で止めて `frontier` に入れる**（`frontier_total` に総数）。`path` は 3 段で探す: 静的な辺だけ（`certainty: certain`）、なければ動的な辺（候補が 20 以下。`uncertain`）、まだなければ**候補が 20 を超える呼び出し点（ハブ）も含めて**（`certainty: hub`。ハブのステップに `hub: true` と `candidates`。ハブの少ない経路、次に短い経路の順）。`searched` が、どの段まで探したかを言い、`reachable: false` で `exhaustive: true` なら、その範囲では経路がない（3 段目まで探していれば、DB の全部の辺について）。2 段目で見つかったときは、ハブを使う経路は探しておらず、出会ったハブの数を `hub_skipped_total` に言う（`--hubs` で、ハブ込みの探索を頼める）
  - 切ったものは必ず言う: `truncated`（`count` と `limit`、`unit`。`at_least` は、数え切れていないとき。`beyond_depth` は、深さの端のノードに、見せていない先がある数）、`omitted`、`frontier`、`hub_skipped_total`、`warnings`。各出力に `schema: "rivus.graph/1"`、`relation: "calls"`（呼び出しの関係。データの流れではない）、`flows: "not_checked"`（データの流れは確認していない）、`snapshot`、`query`、`filters`、`hints`（次の一手）
  - **辺のフィルタ**（`callers` / `callees` / `path` のすべてで、繰り返し可）: `--avoid <シンボル>`（その関数を通らない）、`--via <シンボル>`（その関数を経由するものだけ。`path` は、与えた順）、`--only-impl <型>`（インターフェースの呼び出しは、その型のメソッドだけに届くとする。静的な呼び出しと関数値は絞らない）。**絞り込んだことは、必ず `filters` に書く**。`path` で `reachable: false` のときは、フィルタなしで 1 回探し直して、`without_filters: {reachable: ...}` と `hints` で、条件が経路を塞いでいるかを言う
  - **`path --from-entry --to X`**: `--from` の代わりに、プログラムの起点（Go の `main.main` と各パッケージの `init`、Dart の `main`、Rust の `entry_fn`。起点がなければ、読んだコードの exported な関数。taint の `reachability` と同じ定義）から探す。出力の `entry` に、定義(`definition`)、`source`(`entry` / `exported`)、件数、例を書く。経路の最初のステップが、届いた entry。「届かない」は、コードが死んでいるという意味ではない（フレームワークに登録されるハンドラーなど）
  - `path` の最後のステップ(呼び先)には、宣言の位置 `defined_at`、本体がなければ `no_body: true` が付く（呼び出し点が終点のときは `at`）
  - 出力は、既定で 1 行の JSON（`--pretty` で整形）。`callers` / `callees` は、ノードと辺に分ける。`path` は、ステップの列（各ステップは関数と、そこから次へ進む呼び出し点の `at`、`kind`、`candidates`、`sites`）
- `analyze` と `query` の `-detail summary` は、経路とスニペットを省いた概要だけを出す
- 出力の `summary` は、不確実性を kind と `attributable_to`（`target` / `rivus` / `mixed`）ごとに数え、モデルを書くと減るシンボルの上位（`top_symbols`）も出す
- 注意: `(error).Error` のように至る所で呼ばれるメソッドを sink にすると、解析が大幅に遅くなる（[ADR-032](docs/DECISIONS.md)）。ルールは狭く書く
- ルールに `match: implementations` と書くと、インタフェースのメソッドを、自モジュールの実装に展開する（上限 `-max-implementations`、既定 300）
- シンボルは `go/types` の形式（`(*net/http.Request).FormValue`、`os/exec.Command`）。ルールの書式は [組み込みルール](internal/model/builtin) を参照
- `resolve` には、前に出力された検出結果やサイトの JSON をそのまま渡せる。状態は `unchanged` / `changed` / `rematched` / `ambiguous` / `unresolved`。検出結果には、今も届くか（`flow`）も返す。出力には解析したコードの `target`（git の revision。作業ツリーの未コミットの変更は見ない）が入り、ブックマークも `revision` を持てる（出力の `target` を `{"target": ..., "bookmarks": [...]}` で渡す）。結果の `revision_change` が `same` / `different` / `unknown` を示す（ADR-038）
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
