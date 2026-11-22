# rivus

> ⚠️ 開発中です。コマンド、フラグ、出力の形は変わります。
> **rivus の結果を「通れば安全」の関門として使わないでください。** 見つけられない流れがあります（[何を言わないか](#何を言わないか)）。

`rivus` は、**Go / Dart / Rust のコードについて、「外から来た値が、危険な処理に届くか」の経路を、関数・ファイル・パッケージを跨いで組み立てるツール**です（taint 解析）。

脆弱性を判定する SAST ではありません。**判定は、LLM や人が行う**前提で、rivus は、その材料になる事実を出します。

- 届く経路と、その各段のコード
- 経路の途中で、確かめられなかったこと（不確実性）と、その理由
- 条件によって結果が変わる所（値によってサニタイズされる関数、ガード）と、その条件の出どころ

名前の `rivus` はラテン語で「小川・水路」。英語 *derive* の語源です。

## 何が出るか

次のコードを解析します。

```go
func run(name string) error {
	return exec.Command("sh", "-c", "convert "+name).Run()
}

func handler(w http.ResponseWriter, r *http.Request) {
	if err := run(r.FormValue("file")); err != nil {
		http.Error(w, "failed", 500)
	}
}
```

`rivus analyze ./...` の出力（JSON）から、1 件の finding を、要点だけ抜き出したものです。

```jsonc
{
  "id": "f1:7892e83c11a703ca28f1",
  "category": "command_injection",
  "source": { "symbol": "*net/http.Request", "func": "example.com/app.handler",
              "position": { "file": "main.go", "line": 12, "column": 37 } },
  "sink":   { "symbol": "os/exec.Command", "func": "example.com/app.run",
              "position": { "file": "main.go", "line": 9, "column": 21 },
              "snippet": "return exec.Command(\"sh\", \"-c\", \"convert \"+name).Run()" },
  "paths": [ { "steps": [
    { "func": "example.com/app.handler", "desc": "source: parameter *net/http.Request" },
    { "func": "example.com/app.handler", "desc": "propagated by (*net/http.Request).FormValue",
      "model": { "symbol": "(*net/http.Request).FormValue", "role": "propagator", "origin": "builtin" } },
    { "func": "example.com/app.handler", "desc": "call example.com/app.run" },
    { "func": "example.com/app.run", "desc": "operation" },
    { "func": "example.com/app.run", "desc": "store",
      "notes": [ { "kind": "container_element_imprecision", "attributable_to": "rivus" } ] },
    { "func": "example.com/app.run", "desc": "sink: os/exec.Command" }
  ] } ],
  "uncertainties": [ { "kind": "container_element_imprecision", "attributable_to": "rivus",
                       "detail": "the elements of the container are not told apart, ..." } ],
  "reachability": "from_entry",
  "confidence": "requires_review"
}
```

読み方:

- **`paths`**: source から sink までの段。各段に、関数、位置、その行のコード（`snippet`）、何が起きたか（`desc`）、使ったモデルがあります。
- **`uncertainties`**: その経路で、rivus が確かめられなかったこと。種類（`kind`）と、原因が対象のコードにあるか rivus の側にあるか（`attributable_to`: `target` / `rivus` / `mixed`。`mixed` は、言語の機能に由来するが、rivus の精度向上でも減らせるもの）を言います。種類は、`dynamic_dispatch`（呼び先が実行時に決まる）、`unmodeled_library_call`（中身もモデルもない関数を通った）、`dependency_body_unread`（モデルのあるインターフェースの呼び出しの候補に、依存の本体があったが、読まなかった: 依存が包んだ先の自前の実装へは進んでいない。Go）、`depth_limit`（解析の上限で止めた）、`assumed_call`（渡したクロージャが呼ばれると仮定した）、`container_element_imprecision`、`reflection`、`config_read`、`global_var` などです。
- **`confidence`**: 経路に不確実性が 1 つもなければ `no_uncertainty_on_path`、あれば `requires_review`。**本物かどうかの判定ではありません。**
- **`reachability`**: 経路の関数が、プログラムの起点（`main` など）から届くか（`from_entry` / `from_framework_callback` / `not_from_entry`）。`from_framework_callback` は、起点からは届かないが、依存の型が宣言するメソッドを上書きしている関数（Flutter の `build`、`initState` など。フレームワークが呼ぶ）から届く、という意味です（Dart）。`not_from_entry` は「死んだコード」の意味ではありません（フレームワークに値として登録されたハンドラは、起点から辿れないことがあります）。
- **`conditional_nodes`**（上の例にはありません）: 引数の値でサニタイズするかが変わる関数（`render(s, escape bool)` のような形）を通ったとき、条件、条件ごとの結果、呼び元が渡した値の出どころ（定数、設定の読み出し、など）を書き出します。どちらが起きるかは、判定しません。
- **`id` と `fingerprint`**: コードが少し変わっても同じ箇所を指せるようにするための鍵。`rivus resolve` が使います。

出力の全体は、`findings` のほかに、プログラム全体の `uncertainties`、`stats`（関数の数、収束したか、など）、`summary`（不確実性の集計、モデルを書くと減る関数の上位）を持ちます。

細かい点:

- 解析が上限で止まった所は `depth_limit` として記録され、理由が付きます: `budget`（仕事量。どのマシンでも同じ所で止まる）、`func_timeout` / `run_timeout`（時計。マシンの負荷で止まる所が変わる）。時計で止まったものがあれば、`summary.load_dependent` が `true` になります。
- Go で、型エラーのあるパッケージと、それを import するパッケージは、SSA が作られず、解析から落ちます(`packages` には載ったままです)。読んだが作れなかったパッケージの数、落ちた関数の数、上位 10 のパッケージ(と理由)は、`summary.unread` に出ます(1 つもなければ、欄はありません)。パッケージごとに `missing_source` の不確実性も出て、stderr に 1 行出ます。終了コードと finding は変わりません。
- 数値と真偽値になった値は、既定では追いません（`numeric_stop` として記録。`-numeric-taint` で追う）。
- `rivus resolve` の答えは、`unchanged` / `changed` / `rematched` / `ambiguous` / `unresolved` と、finding については今も届くか（`flow`）です。

## 使い方

```sh
go build -o rivus ./cmd/rivus          # Go 1.27 以降

rivus analyze ./...                    # source から sink への経路を全部
rivus analyze -C path/to/project       # ディレクトリを指す。言語は自動で判別（go.mod、pubspec.yaml）
rivus sites ./...                      # source / sink になりうる箇所の一覧（解析なし。速い）
rivus query -sink 'os/exec.Command' ./...                    # この sink に届きうる source
rivus query -source '(*net/http.Request).FormValue' ./...   # この source が届きうる sink
rivus query -site <ID> ./...           # sites の 1 件から始める
```

| コマンド | 何をするか |
|---|---|
| `sites` | source と sink のルールが当たる箇所を並べる。解析はしない |
| `analyze` | source から sink への経路を、関数とパッケージを跨いで探す |
| `query` | 片側からの問い（この sink に届く source / この source が届く sink / この箇所について） |
| `index` | コードを 1 回読んで、呼び出しグラフを保存する（`-taint` で、解析の結果も） |
| `graph` | 保存した呼び出しグラフを調べる（呼び元、呼び先、経路、インターフェースの実装）。**呼び出しの関係であって、データの流れではない** |
| `resolve` | 前に見た箇所（finding や site）が、今のコードのどこにあるか、今も届くかを答える |
| `models coverage` | どの関数にモデルを書くと効くかを、効く順に出す |
| `extract` | 中立の IR を書き出す（調査用） |

フラグは `rivus <command> -h` で出ます。よく使うもの: `-C <dir>`、`-rules <file>`（繰り返し可）、`-no-deps`（依存のソースを読まない。軽いが、依存への呼び出しは推測になる）、`-tests`（テストのコードも読む）、`-detail summary`（経路とコードを省く）、`-timeout`、`-v`。

`index` と `graph` の詳しい使い方は [docs/GRAPH.md](docs/GRAPH.md) にあります。

### 言語ごとの準備

| 言語 | どう読むか | 要るもの |
|---|---|---|
| Go | `go/packages` と `go/ssa` で直接読む | Go のツールチェーン。コードがビルドできること |
| Dart | 別プロセスの extractor（`package:analyzer`）が IR を書き、rivus が読む | Dart SDK。**解析の前に、そのパッケージで `dart pub get` を実行する**（rivus は実行しません。していない、または古いとき、rivus は `summary.unread` と stderr でそう言います）。コード生成（`build_runner`）も済んでいること。[extractor/dart](extractor/dart/README.md) |
| Rust | 別プロセスの extractor（rustc のドライバ。MIR を読む）が IR を書き、rivus が読む | Docker（既定。固定した nightly を入れたイメージを、このリポジトリから作る）か、`-unsafe-host-exec` のときは固定した nightly の Rust。コードがビルドできること、`Cargo.lock` があること。**読むと、対象のビルドが走る**（下の「信頼できないコード」）。[extractor/rust](extractor/rust/README.md) |

- Dart と Rust の extractor（`rivus-dart`、`rivus-rust`）は、PATH に置くか、`-dart-extractor` / `-rust-extractor`（または環境変数 `RIVUS_DART_EXTRACTOR` / `RIVUS_RUST_EXTRACTOR`）で指します。extractor が書いた IR のファイルを、`rivus analyze -ir file.jsonl` で読むこともできます。
- Rust は、`Cargo.toml` があるだけでは読みません。`-lang rust` と書いたときだけ読みます。
- **テストと、開発用の依存は、既定で読みません**（Go の `_test.go`、Dart の `test/` などと `dev_dependencies` だけから使われるパッケージ、Rust のテスト・例・ベンチと `[dev-dependencies]`）。本番のビルドに入らないコードだからです。読まなかった数は、出力の `stats.tests_skipped` に出ます。`-tests` で読みます。
- 対象のコード生成（`go generate`、`build_runner`）は、rivus はしません。先に済ませてください。ビルドも、Go と Dart ではしません。**Rust は例外で、読むときに `cargo check` が走ります**（上の表と「信頼できないコード」）。

## 今の状態

Go から作り始めたので、**Go が最も進んでいて、Dart と Rust は、それより見逃しが多い**状態です。差を測って縮める作業をしています。

| | Go | Dart | Rust |
|---|---|---|---|
| 読み込み | 直接（`go/ssa`） | extractor（最初の版） | extractor（最初の版） |
| 動的な呼び出しの候補 | 型に基づく解析で絞る | rivus 自身が値を追う。追えないと候補なし（不確実性として出る） | extractor が型で候補を出す（型が合うだけの候補も入る）。一部は rivus 自身が値を追う |
| 実コードでの確認 | 複数のプロジェクト | 少ない（Flutter のアプリは、これから） | 少ない |

- 言語間の差は、**同じ流れの形を 3 言語で書いたテスト**（[bench/zoo](bench/zoo/README.md)）で数えています。`go test ./cmd/rivus -run Zoo -v` が、言語ごとに「出る / 痕跡つきで見逃す / 黙って見逃す / おとりを誤って出す」の数を出します。
- 既知の見逃しの一覧と、その原因は [bench/MISSES.md](bench/MISSES.md) にあります。

## 何を言わないか

rivus が出さなかったことは、「流れがない」という意味ではありません。

- **見逃しがあります。** 流れを運べない形（一部の別名、フレームワークの中を通る状態の受け渡し、など）があります。運べなかったことが分かるときは、不確実性として出しますが、**何も出さずに見逃す形も残っています**。数と形は、上の zoo と MISSES.md で公開しています。
- **判定はしません。** サニタイズが十分か、ガードが効いているか、その入口に認証があるかは、判定役が、出力の経路とコードを見て決めます。
- **ルールとモデルにないことは、知りません。** どの関数が source / sink かは、ルールで決まります（次の節）。中身もモデルもないライブラリの関数は、「引数は結果に伝わる」と仮定して進み、`unmodeled_library_call` を記録します。
- **読まなかったコードの中は、見ていません。** テスト、ビルドの条件で外れたファイル、読めなかったパッケージ。出力の `stats` と `load_errors` を見てください。`stats.converged` が `false` のときは、解析が上限で止まっていて、結果は途中のものです。
- **`rivus graph` は、呼び出しの関係だけを答えます。** 経路が見つかっても、値がそこを流れるとは限りません。

## ルールとモデル

- **ルール**: どの関数や型が、source（外から来る値）、sink（危険な処理）、sanitizer（無害にする処理）か。
- **モデル**: 中身を読まないライブラリの関数が、引数を結果にどう伝えるか（propagator、`pure`）。

どちらも YAML で書き、組み込みのものと同じ書式です（[internal/model/builtin](internal/model/builtin)）。

**組み込みで持つ範囲**は、各言語の標準ライブラリと、広く使われているライブラリです。**何を組み込むかは、その都度判断します**(組み込みのモデルは、ある程度充実させる方向です。[ADR-063](docs/DECISIONS.md) の 2026-11-22 の追記)。今の組み込みは、標準ライブラリ、代わりのないパッケージ(Rust の `serde`、`tonic`、`tokio`、`futures`、`bytes`、`libc`、HTTP の型の `http` / `url` など)、SQL のドライバ(`sqlx`、`rusqlite`、Dart の `sqflite`。あらゆるプロダクトで要る)、Rust の一部の Web フレームワークと Dart の `shelf`、Google Cloud のクライアント(今は Go の Cloud Spanner と BigQuery。`internal/model/builtin/go-gcp.yaml`。[ADR-077](docs/DECISIONS.md))です。プロジェクト固有のラッパーや、組み込みでないフレームワークは、使う側がルールで足します。見本は [examples/rules](examples/rules) に置いています: Go の gin と macaron、Rust の actix-web(`examples/rules/rust-actix-web.yaml`)。組み込みでないフレームワークの入口は、ルールを足すまで source として見えず、その分、検出が減ります。コードがそのフレームワークを呼んでいるのに、モデルがルールにないときは、`analyze` が stderr に 1 行知らせます([ADR-063](docs/DECISIONS.md))。そのための仕組み:

1. 置き場所: 解析するディレクトリの `.rivus/rules/*.yaml`（フラグなしで読まれる）か、`-rules file.yaml`。書き方の見本は [examples/rules](examples/rules)。
2. 何を書けばよいか: `rivus models coverage` が、finding の経路に載っているのにモデルがない関数を、効く順に出します。`-yaml` で、埋めるためのひな形が出ます。

   ```sh
   rivus models coverage -yaml -top 20 ./...
   ```

3. 確信のない流れ: モデルの `flows` に `approximate: "理由"` を書くと、流れは消さずに、通った finding に不確実性 `model_approximation` が付きます（書き方は [docs/MODELING.md](docs/MODELING.md)）。
4. 誰が書いたか: ルールの `origin: builtin | human | llm` で区別できます。`-deterministic-only` は `llm` のルールを無視します。LLM 由来のモデルが効いた finding には `llm_influenced` が付き、モデルが流れを消した場所は `suppressions` に残ります。

LLM にモデルを書かせ、人や別の手段で確かめる、という使い方を想定しています。書式の手引きは [docs/MODELING.md](docs/MODELING.md)(英語。まだ全部の節は無い)。モデルを実行で確かめる道具は、これから整えます。

ルールで書けること（抜粋）:

- `kind: param`: 関数の引数を source にする（フレームワークが呼ぶハンドラの入口）。その値は、関数の中では追いますが、呼び元には書き戻しません。
- `match: implementations`: インターフェースのメソッドに書いたルールを、自分のコードの実装に広げる（上限は `-max-implementations`）。
- `pure: true`: その関数を通って、値は伝わらない。
- `accepts` / ラベル: sink が、どの種類の source に反応するかを絞る。

書くときの注意:

- **流れを切るルール（sanitizer、`pure: true`）は、間違えると、見逃しになります。** 迷ったら、切らないでください。伝わると書いて間違えても、余分な finding が出るだけです。
- `(error).Error` のように、至る所で呼ばれるメソッドを sink にすると、解析が大幅に遅くなります。ルールは狭く書いてください。
- シンボルの書き方は言語ごとに違います（Go は `go/types` の形: `(*net/http.Request).FormValue`、`os/exec.Command`。Dart と Rust は各 extractor の README）。

## 信頼できないコードを解析するとき

**解析する対象のコードが、解析する人のマシンで動くことがあります。** 採用前の第三者のコードの確認や、怪しいリポジトリの調査にも使う道具なので、先に書いておきます。

| 言語 | 何が動くか |
|---|---|
| Rust | extractor が、対象に `cargo check` を走らせます（**対象をコンパイルします**）。対象と依存の `build.rs`、proc macro が実行されます。**既定は、Docker のコンテナの中で動かします**（依存を取る段階は、ネットワークがあるが、対象の cargo の設定は読ませない。抽出の段階はネットワークなし、対象は読み取り専用、非 root、権限なし。Docker がなければ、動かさずに止まります）。ホストで直接動かすのは `-unsafe-host-exec`（信頼できるコードだけ）。手順と限界は [docs/rust-isolation.md](docs/rust-isolation.md) |
| Go | `go/packages` が `go list` を走らせます。依存の取得（ネットワーク）が起きることがあります。cgo のあるパッケージは、cgo の前処理（C コンパイラ）が走ります。**対象のソースそのものは実行しません**が、「Go はコードを実行しない」とは言えません。隔離はありません（Rust と同じ作りに載せる予定） |
| Dart | extractor は、静的に読むだけです。ただし、前提の手順（`dart pub get`、`build_runner`）は、対象の依存と生成器のコードを実行します |

- **Rust は、対象のビルドが、解析の結果そのものも書き換えられます。** IR の断片を書く場所が、対象のビルドから見えるためです（コンテナの中でも同じです。隔離が守るのは、マシンと秘密で、結果の正しさではありません）。
- **対象のディレクトリに付いてくるルール（`.rivus/rules/*.yaml`）は、フラグなしで読まれます。** 対象を書いた人は、そこで finding を消せます。読んだファイルと、消せるものの数は、出力の `project_rules` に出ます。**信頼できないコードでは `-no-project-rules` を付けてください。**
- Dart の依存は、プロジェクト、pub のキャッシュ、SDK の下のものだけ読みます（`package_config.json` は対象の一部で、どこでも指せるため）。ほかの場所は `-allow-deps-under DIR` で足します。

Rust 以外の信頼できないリポジトリは、使い捨ての環境（コンテナや VM）で解析してください。依存を先に取得してから、ネットワークを切り、ソースを読み取り専用で渡す形です。

```sh
docker run --rm --network none -v "$PWD":/src:ro -w /src my-rivus-image rivus analyze ./...
```

`my-rivus-image` は、rivus と、対象の言語のツールチェーンを入れた、自分で用意するイメージです。

## 仕組み

```
Go / Dart / Rust の extractor ──→ 中立の IR（CFG、JSON Lines）──→ コア（Go）
                                                                SSA → 関数ごとの要約 → taint → 出力
```

- 言語ごとの extractor が、ソースを、制御フローグラフの水準の IR に下ろします。コアは、言語を知りません。
- コアは、関数ごとに「引数や戻り値のあいだで、何が何に伝わるか」の要約を作り、呼び出しグラフの下から順に組み合わせます。
- `rivus index` は、呼び出しグラフを SQLite のファイルに保存します（利用者のキャッシュに置き、解析の対象のディレクトリには書きません）。

詳しくは [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)。設計の判断の記録は [docs/DECISIONS.md](docs/DECISIONS.md)。

## 開発

```sh
go test ./...                                   # Go だけで走るテスト
RIVUS_DART="dart" go test ./cmd/rivus           # Dart の extractor を使うテストも
RIVUS_RUST=$PWD/extractor/rust/target/debug/rivus-rust go test ./cmd/rivus   # Rust の extractor を使うテストも（先に extractor/rust をビルド）
```

正しさは、役目の違うテストとベンチで確かめています（[bench/README.md](bench/README.md)）。

| 置き場 | 何を見るか |
|---|---|
| `testdata/`、`bench/*-curated` | 小さいケース。ソースの `@source(tag)` / `@sink(tag)` / `@sink(none)` が期待値 |
| `bench/zoo-go`、`zoo-dart`、`zoo-rust` | 流れの形を 1 つずつ。**期待値は、ケースを実際に動かした結果**（rivus の出力に合わせて書けない）。脆弱な版、直した版、おとりの 3 つ組 |
| `bench/go-scale` | 大きなコードで起きる現象（要約の膨張、収束しない）を、数秒で再現する生成コード |
| `bench/wild` | 公開された実世界の脆弱性。その件の source と sink を与えたとき、本物の経路を組み立てられるか |
| 実在のプロジェクト（`bench/gogs` ほか） | 変更の前後で、finding の増減を比べる（`bench/compare.sh`）。コードは置かず、版の指定だけ |

解析の精度に関わる変更は、`bench/compare.sh <前> <後>` で前後を比べ、消えた finding が本物でなかったことを確かめてから入れます。

## ドキュメント

読み手ごとの入口です。全体の目次は [docs/README.md](docs/README.md)。

| 読み手 | 読む文書 |
|---|---|
| 使い手 | この README。信頼できないコードの扱いは [docs/rust-isolation.md](docs/rust-isolation.md)。用語は [docs/GLOSSARY.md](docs/GLOSSARY.md) |
| ルールを書く人 | [docs/MODELING.md](docs/MODELING.md)(書式)、[examples/rules](examples/rules)(見本)、[docs/GCP-MODELS.md](docs/GCP-MODELS.md) |
| rivus を使う AI エージェント | [docs/GRAPH.md](docs/GRAPH.md)(`rivus index` と `rivus graph`)。出力の読み方はこの README の「何が出るか」 |
| 開発者 | [docs/PRINCIPLES.md](docs/PRINCIPLES.md)(設計思想)、[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)(今の構成)、[docs/DECISIONS.md](docs/DECISIONS.md) と [docs/ADR-INDEX.md](docs/ADR-INDEX.md)(判断の経緯と一覧)、`docs/DESIGN-*.md`(個別の設計。冒頭に状態がある)、[bench/README.md](bench/README.md)(テストと測定) |
| レビューする人 | [docs/PRINCIPLES.md](docs/PRINCIPLES.md)(原則と、まだ届いていない所)、[bench/MISSES.md](bench/MISSES.md)(既知の見逃し) |

[docs/history/](docs/history/) の BRIEF.md と QUESTIONS.md は、最初の構想と、それへの指摘です(歴史的な文書。今の設計とは異なる所があります)。

## ライセンス

[Apache-2.0](LICENSE)
