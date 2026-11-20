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
rivus graph path -C . --from-entry --to routers/web/repo.Activity  # ルータに登録しただけの関数(m.Get("/x", repo.Activity))にも、4 段目(参照)で届く。--refs で 4 段目から、--no-refs で使わない
rivus graph callers -C . repo.Activity --refs   # この関数を、値として持つ関数(登録、保存、受け渡し)も、kind: ref の辺として足す。callees --refs は、持っている関数
rivus graph path -C . --from a --to b --avoid log.Error --via db.Query --only-impl bytes.Buffer  # 辺のフィルタ(callers / callees でも使える。繰り返し可。filters に必ず書く)
rivus graph path -C . --from a --to b --avoid-call server/handler.go:42:7  # 呼び出し点を 1 つだけ切る(--avoid は関数を丸ごと切る)
rivus graph impls -C . '(io.Writer).Write'        # インターフェースのメソッドの、実装(関数)の一覧。短い指定(io.Writer.Write、Writer.Write)も可。--limit で切り、総数と by_package(上位 10)を言う
rivus graph dispatchers -C . '(*bytes.Buffer).Write'  # その実装を候補として呼ぶ、動的な呼び出し点(位置、含む関数、kind、candidates、通るインターフェースのメソッド = callee)
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
  - 切ったものは必ず言う: `truncated`（`count` と `limit`、`unit`。`at_least` は、数え切れていないとき。`beyond_depth` は、深さの端のノードに、見せていない先がある数）、`omitted`、`frontier`、`hub_skipped_total`、`warnings`。各出力に `schema: "rivus.graph/1"`、`relation: "calls"`（呼び出しの関係。データの流れではない。参照を含む経路は、経路ごとに `"calls+refs"`）、`flows: "not_checked"`（データの流れは確認していない）、`snapshot`、`query`、`filters`、`hints`（次の一手）
  - **辺のフィルタ**（`callers` / `callees` / `path` / `impls` / `dispatchers`、繰り返し可）: `--avoid <シンボル>`（その関数を通らない。**その関数の呼び出しも、その関数への呼び出しも、全部**）、`--avoid-call <file:line:col>`（**呼び出し点を 1 つだけ**切る。その呼び出しを含む関数と、その関数のほかの呼び出しは残る。位置は、出力の `at` が言う形で、`file:line` が複数の呼び出しに当たるときはエラーで候補を返す。ハブの経路は「本物らしい順」ではないので、1 本ずつ確かめて、疑わしい呼び出し点を切っていくために使う）、`--via <シンボル>`（その関数を経由するものだけ。`path` は、与えた順。`impls` / `dispatchers` では使えない）、`--only-impl <型>`（インターフェースの呼び出しのうち、**候補に、その型のメソッドが 1 つでもある呼び出し点だけ**を、その型のメソッドに絞る。候補にその型がない呼び出し点（関係のない `Stringer` や `error` の呼び出し）は絞らない。静的な呼び出しと関数値も絞らない。型は、型の名前（`Buffer`）、またはパッケージや経路を付けた形（`bytes.Buffer`）で、**型の名前に当たる**: Rust の `<T as Trait>::m` は `T` だけ（trait の名前では当たらない）、Go はパッケージ名だけでは当たらない、型引数の中（`Vec<Foo>` の `Foo`）では当たらない）。**絞り込んだことは、必ず `filters` に書く**（`--only-impl` は、出会った呼び出し点のうち、絞った数と、絞らなかった数も）。`path` で `reachable: false` のときは、フィルタ（`--via` も）なしで 1 回探し直して、`without_filters` と `hints` で、条件が経路を塞いでいるかを言う。フィルタが 2 つ以上なら、1 つずつ外した探索もして、外すと届くものを `without_filters.culprits` に書く。フィルタつきの探索が上限で切れたときは、「フィルタが原因とは言えない」と言う
  - **`path` が並べた範囲**: `listed`（`stage`: static / dynamic / hub、`hubs`（hub の段: 並べた経路のハブの数 = 最少）、`shortest_hops`、`max_hops` = 最短 + 2）と、`beyond: "not_searched"`（これより長い経路、ハブの多い経路は、探していない。`truncated.count` は数えない）。`--limit` で切ったものだけが `truncated` に入る
  - **`path --from-entry --to X`**: `--from` の代わりに、プログラムの起点（Go の `main.main` と各パッケージの `init`、Dart の `main`、Rust の `entry_fn`。起点がなければ、読んだコードの exported な関数。起点の決め方は taint の `reachability` と同じ。辿る辺も、`sg.Program.Reachable` に合わせた（呼び出し、インターフェース・関数値の候補、**値として名前を挙げた関数 = 参照**。参照は `path` の 4 段目で辿る。下の「参照」）。出力の `entry` に、定義(`definition`)、`source`(`entry` / `exported`)、件数、例を書く。経路の最初のステップが、届いた entry。「届かない」は、コードが死んでいるという意味ではない（フレームワークに登録されるハンドラーなど）。**届かないとき**: `exhaustive` は常に `false`(読んでいないコード〔依存、フレームワーク〕が、インターフェース経由で呼ぶ関数は、辺にない)。探索が呼び出しと参照の辺を尽くしたことは `edges_exhausted: true` が言う(`exhaustive_of`: 呼び出しと参照の辺について。`sg.Program.Reachable` と同じ規則)。`limits`(リフレクション、フレームワークからの呼び出し〔依存の型のメソッドの上書き〕は、辺にない)と、`target_facts`(終点の呼び元の呼び出し点の数〔静的、動的〕、`exported`、`has_receiver`〔メソッドである。メソッドで呼び元の辺がないものは、依存から呼ばれうる〕、`has_body`、`dep`、`value_references`〔参照の数〕、`value_reference_owners`〔参照を持つ関数の上位〕)を付ける。`--no-refs` のときは、従来どおり `exhaustive` は `false`(関数値の保存は辺にない、と `limits` が言う)。`edges_exhausted` は、辺についての探索が上限で切れなかったこと。`path` の `exhaustive` は、`exhaustive_of` の範囲についてのもの
  - **参照(`refs` 表。スキーマ版 2)と、`path` の 4 段目**(設計: DESIGN-graph.md「参照」): `rivus index` は、値として名前を挙げた関数(`m.Get("/x", repo.Activity)` の `repo.Activity`、変数や構造体に入れた関数、関数ポインタ、Dart の tear-off、キャプチャのあるクロージャ)を、`refs` 表に入れる(持っている関数 `owner_func`、書かれた位置、参照される関数 `target_func`、引数に取っている呼び出しの呼び先 `via_callee`、`kind` = `arg` / `closure` / `other`)。**スキーマ版が 1 から 2 に上がった**ので、前の版のファイルは、エラーで、`rivus index` での作り直しを案内する(マイグレーションはしない)。`path` は、静的 → 動的 → ハブ → **参照** の順に探し、4 段目は、3 段目までで見つからないときだけ(`--refs` で 4 段目から、`--no-refs` で使わない)。参照のステップは `kind: "ref"`(呼び出しではない。**位置 `at` は、参照が書かれた位置 = 登録の行**。`via_callee`、`ref_kind`、`note` つき)。参照を含む経路は `certainty: "reference"`、経路ごとの `relation: "calls+refs"`(呼び出しだけの経路は `"calls"`)で、経路の `note` が「届くかもしれない、の広い近似(登録しただけで、呼ばれるかは分からない)」と言う。並べる順は、参照が少ない → ハブが少ない → 短い。`searched` に `ref`、`listed.stage` が `ref`(`listed.refs` は参照の数)。`callers` / `callees` は、既定で参照を辿らず、数を `omitted.refs` に言う。`--refs` で、`kind: "ref"` の辺として足す(`filters` に書く。関数値の呼び出しの候補 = `kind: "value"` とは別)。`impls` / `dispatchers` は参照を使わない
  - **`rivus graph impls <インターフェースのメソッド>` / `dispatchers <実装のメソッド>`**（段階 3b。具体的な実装から始める使い方の入口。DB の辺は変えず、`edges` の `kind = interface` を、呼び出し点の静的な型 `callsites.callee` ごとにまとめて読む）。`impls` の指定は、呼び出し点が持つ名前（`(error).Error`、`(io.Writer).Write`、Rust は `pkg::Trait::method`、Dart は `dart:core#List.length`）か、その末尾（`io.Writer.Write`、`Writer.Write`。識別子の境界で、**インターフェースのメソッド全体で 1 つに決まるときだけ**。複数なら `ambiguous` に、呼び出し点の多い順で最大 20 件）。`dispatchers` の指定は、`callers` と同じ規則の関数（シンボル、短い形、`ambiguous`）
    - **`impls` の出力は「DB の中の、その呼び出し点の候補の和」で、「型としての実装の全部」ではない**。Go の候補は VTA が呼び出し点ごとに絞ったもので、どの呼び出し点からも届かない型は載らない。これを `basis` に必ず書く。`call_sites`（そのメソッドの、インターフェース呼び出し点の数）、`zero_candidate_sites`（そのうち候補が 0 のもの。受け手の型が見つからなかったので、その先の実装は載っていない）、`impls_total`、`impls`（`id`、`at`、`dep`、`call_sites` = その実装を候補に持つ呼び出し点の数。自分のコード、呼び出し点の多い順）。`--limit`（既定 50）で切ったら、`truncated`（総数との差）と `by_package`（全体の、パッケージごとの内訳の上位 10。残りは `by_package_other`）を言う
    - そのメソッドの呼び出し点が DB に 1 つもないとき（関数としては存在する、など）は、エラーにせず、`impls: []`、`call_sites: 0`、`reason`（なぜ何も言えないか）と `hints` を返す。「空 = 実装なし」ではない
    - **`dispatchers`** は、その関数を候補に持つ**動的な呼び出し点**（`kind` が `interface` または `value`）だけを返す（静的な呼び元は `callers`）。各呼び出し点に、位置 `at`、含む関数 `in`、`kind`、`candidates`、**そこから届くインターフェースのメソッド `callee`**（静的な型）、`sites`。候補の少ない順（その実装を名指しに近い順）。`--limit` で切ったら `truncated` と、全体の `by_callee`（インターフェースのメソッドごと、上位 10）。同じ symbol の関数が複数あるとき（型の埋め込みによるラッパー）は、`id` が symbol と同じ本物のメソッドだけを見て、ほかがあることを `hints` で言う
    - `--avoid`、`--avoid-call`、`--only-impl`（`filters` に書く）は使える。`--via` は、意味がないので、エラー。`snapshot`、`-C` の照合、`relation: "calls"`、`flows: "not_checked"` は、ほかと同じ
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
- `kind: param` の source は、「その関数が受け取った値についての宣言」。その種は関数の中（sink、戻り値）に効くが、呼び元には書き戻さない（関数が自分で行った書き込み、たとえばグローバルへの保存は、出る）。呼び元が同じオブジェクトを別の関数に渡して sink に使う流れは、その呼び元を入口にしないと出ない（ADR-048）
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
