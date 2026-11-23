# rivus

> ⚠️ 開発中です。コマンド、フラグ、出力の形は変わります。
> **rivus の結果を「通れば安全」のゲートとして使わないでください。** 見つけられない流れがあります（[何を報告しないか](#何を報告しないか)）。

`rivus`(リーヴス、/ˈriː.vus/。ラテン語で「流れ」)は、**Go / Dart / Rust のコードについて、「外から来た値が、危険な処理に届くか」の経路を、関数・ファイル・パッケージを跨いで組み立てるツール**です（taint 解析）。

脆弱性を判定する SAST ではありません。**判定は、LLM や人が行う**前提で、rivus は、その材料になる事実を出します。

- 届く経路と、その各ステップのコード
- 経路の途中で、確かめられなかったこと（不確実性）と、その理由
- 条件によって結果が変わる所（値によってサニタイズされる関数、ガード）と、その条件の出所

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

- **`paths`**: source から sink までのステップ。各ステップに、関数、位置、その行のコード（`snippet`）、何が起きたか（`desc`）、使ったモデルがあります。
- **`uncertainties`**: その経路で、rivus が確かめられなかったこと。種類（`kind`）と、原因が対象のコードにあるか rivus の側にあるか（`attributable_to`: `target` / `rivus` / `mixed`。`mixed` は、言語の機能に由来するが、rivus の精度向上でも減らせるもの）を示します。種類は、`dynamic_dispatch`（呼び出し先が実行時に決まる）、`unmodeled_library_call`（中身もモデルもない関数を通った）、`dependency_body_unread`（モデルのあるインターフェースの呼び出しの候補に、依存の本体があったが、読まなかった: 依存が包んだ先の自前の実装へは進んでいない。Go）、`depth_limit`（解析の上限で止めた）、`assumed_call`（渡したクロージャが呼ばれると仮定した）、`container_element_imprecision`、`pointer_alias`（ポインタ越しの書き込みを別名で近似した所を通った。Rust）、`reflection`、`config_read`、`global_var` などです。
- **`confidence`**: 経路に不確実性が 1 つもなければ `no_uncertainty_on_path`、あれば `requires_review`。**本物かどうかの判定ではありません。**
- **`reachability`**: 経路の関数が、プログラムの起点（`main` など）から届くか（`from_entry` / `from_framework_callback` / `not_from_entry`）。`from_framework_callback` は、起点からは届かないが、依存の型が宣言するメソッドを上書きしている関数（Flutter の `build`、`initState` など。フレームワークが呼ぶ）から届く、という意味です（Dart）。`not_from_entry` は「死んだコード」の意味ではありません（フレームワークに値として登録されたハンドラは、起点からたどれないことがあります）。
- **`conditional_nodes`**（上の例にはありません）: 引数の値でサニタイズするかが変わる関数（`render(s, escape bool)` のような形）を通ったとき、条件、条件ごとの結果、呼び出し元が渡した値の出所（定数、設定の読み出し、など）を書き出します。どちらが起きるかは、判定しません。
- **`id` と `fingerprint`**: コードが少し変わっても同じ箇所を指せるようにするための鍵。`rivus resolve` が使います。

出力の全体は、`findings` のほかに、プログラム全体の `uncertainties`、`stats`（関数の数、収束したか、など）、`summary`（不確実性の集計、モデルを書くと減る関数の上位）を持ちます。

細かい点:

- 解析が上限で止まった所は `depth_limit` として記録され、理由が付きます: `budget`（作業量。どのマシンでも同じ所で止まる）、`func_timeout` / `run_timeout`（時計。マシンの負荷で止まる所が変わる）。時計で止まったものがあれば、`summary.load_dependent` が `true` になります。
- Go で、型エラーのあるパッケージと、それを import するパッケージは、SSA が作られず、解析から落ちます(`packages` には載ったままです)。読んだが作れなかったパッケージの数、落ちた関数の数、上位 10 のパッケージ(と理由)は、`summary.unread` に出ます(1 つもなければ、フィールドはありません)。パッケージごとに `missing_source` の不確実性も出て、stderr に 1 行出ます。終了コードと finding は変わりません。
- **依存の本体を読むのは、流れを追うためです。source と sink は、プロジェクトのコードの中にだけ付きます**(依存の関数の中では探しません。ADR-099。プロジェクトの範囲は ADR-114)。依存を読んだ実行では、`summary.dependency_sites`(`searched: false`、`dependency_functions`: 読んだ依存の関数の数)が、毎回そのことを言います(`-no-deps` や、依存の関数が 0 のときは出ません)。**利用者の入力が、ライブラリの中で危険な処理(ファイルを開く、コマンドを実行する、など)に届く形は、プロジェクトのコードがそのライブラリの API を呼ぶ所に sink のモデルがない限り、検出されません。** sink のモデルは、解析のコストを減らすためだけでなく、検出のためにも必要です(`rivus models coverage` で、モデルのない呼び出しを多い順に見られます)。
- 数値と真偽値になった値は、既定では追いません（`numeric_stop` として記録。`-numeric-taint` で追う）。
- `rivus resolve` の答えは、`unchanged` / `changed` / `rematched` / `ambiguous` / `unresolved` と、finding については今も届くか（`flow`）です。

## 使い方

### インストール

**Homebrew(macOS)**:

```sh
brew install yagihash/tap/rivus
rivus version
```

- Go のツールチェーンと Dart SDK(`dart-sdk`)は、formula の依存としてインストールされます。
- Rust を読むための固定した nightly と extractor(`rivus-rust`)も、formula がインストール時に用意します。初回だけ、static.rust-lang.org と crates.io から約 1.1 GB を取得します。Xcode のコマンドラインツールが必要です。実行時に rustup や `~/.cargo` は使いません。
- nightly は `$(brew --prefix)/var/rivus` に置かれます。rivus を更新しても、nightly のバージョンが同じなら取得し直しません。`brew uninstall rivus` では削除されないので、不要になったら `rm -rf "$(brew --prefix)/var/rivus"` で削除してください。
- 更新は `brew upgrade rivus` です。

設計は [docs/DESIGN-install.md](docs/DESIGN-install.md) にあります。

**ソースからビルドする**(Go 1.27 以降):

```sh
go build -o rivus ./cmd/rivus
```

Dart と Rust を読むには、それぞれの extractor が必要です(下の「言語ごとの準備」)。開発用のコマンドは「開発」にあります。

**zsh の補完**(任意): コマンド、サブコマンド、フラグ、決まった値を補完します。スクリプトは、入れた rivus 自身が作るので、コマンドやフラグが増えても追従します(rivus を更新したら、作り直します)。

```sh
source <(rivus completion zsh)                    # ~/.zshrc に書く(compinit のあと)
rivus completion zsh > "${fpath[1]}/_rivus"       # または、fpath のディレクトリに保存する
```

### コマンド

```sh
rivus analyze ./...                    # source から sink への経路を全部
rivus analyze -C path/to/project       # ディレクトリを指す。言語は自動で判別（go.mod、pubspec.yaml）
rivus sites ./...                      # source / sink になりうる箇所の一覧（解析なし。速い）
rivus query -sink 'os/exec.Command' ./...                    # この sink に届きうる source
rivus query -source '(*net/http.Request).FormValue' ./...   # この source が届きうる sink
rivus query -site <ID> ./...           # sites の 1 件から始める
```

| コマンド | 何をするか |
|---|---|
| `sites` | source と sink のルールが一致する箇所を並べる。解析はしない。読む言語は `analyze` と同じ(`-lang`)で、読まなかった言語は出力の `skipped_languages` に出る |
| `analyze` | source から sink への経路を、関数とパッケージを跨いで探す |
| `query` | 片側からの問い（この sink に届く source / この source が届く sink / この箇所について） |
| `index` | コードを 1 回読んで、呼び出しグラフを保存する（`-taint` で、解析の結果も） |
| `graph` | 保存した呼び出しグラフを調べる（呼び出し元、呼び出し先、経路、インターフェースの実装）。**呼び出しの関係であって、データの流れではない** |
| `resolve` | 前に見た箇所（finding や site）が、今のコードのどこにあるか、今も届くかを答える |
| `models coverage` | どの関数にモデルを書くと効果があるかを、効果の大きい順に出す |
| `completion` | シェルの補完のスクリプトを出す(今は `completion zsh`)。コマンドとフラグの表から作るので、増減に追従する |
| `debug` | 調査と開発のためのコマンド。普段は使わない（今は `debug extract`: 中立の IR を書き出す） |

フラグは `rivus <command> -h` で出ます。`-h` は、普段使うフラグだけを出します。診断と調整のフラグ（プロファイル、解析の上限、サンドボックスの調整など）は、`-h -help-all`（または `-help-all` だけ）で全部出ます。フラグそのものは、どちらの表示でも同じように使えます。よく使うもの: `-C <dir>`、`-rules <file>`（繰り返し可）、`-no-deps`（依存のソースを読まない。軽いが、依存への呼び出しは推測になる。読んだ場合も、source と sink は依存の関数の中では探しません）、`-body-replaced <パッケージ>`（そのパッケージの依存の本体を読まず、モデルを代わりにする。繰り返し可。モデルのファイルの `body_replaced_packages` と同じ。解析の時間とメモリが減る。`-read-declared-bodies` で、本体を読む実行と比べられる。`summary.body_replaced_packages` に一覧が出る）、`-tests`（テストのコードも読む）、`-detail summary`（経路とコードを省く）、`-timeout`、`-max-memory <サイズ>`（メモリの上限。`6GiB`、`512MiB`、`off`。指定がなければ `GOMEMLIMIT`、それもなければ物理メモリ(cgroup があれば小さいほう)の 1/4。やわらかい上限で、合計が近づくと GC を強くする。結果は変わらない。生きている量より低いと守れず、GC が回り続けて CPU が増える。`summary.memory_limit_*` に出る)、`-v`。`-resolve-func-values`（実験。既定 off）は、呼び出し先が分からない関数値の呼び出しに、自分のコードで作った関数を候補として補ってから解析します（`stats.func_values` に、補った数が出ます）。`rivus index` に付けると、補ったエッジが索引に入り、`graph callers / callees / path` の答えに `supplemented` で出ます（このフラグの有無は索引のキーに入るので、索引を作ったときと同じ指定で問い合わせます）。`-timeout` は、補う解析と、そのあとの解析を合わせた上限です。

`index` と `graph` の詳しい使い方は [docs/GRAPH.md](docs/GRAPH.md) にあります。AI エージェントに rivus を使わせるときの手引き(どのコマンドを、どのフラグで実行し、出力のどこを読むか)は [docs/AGENT-GUIDE.md](docs/AGENT-GUIDE.md) にあります。

### 依存を浅く読む

依存の関数の中のフィールドのたどり方(アクセスパス)は、既定で、深さ 2 で切ります(自分のコードは 3、`-k`)。リクエストの構造体が深い依存(AWS SDK など)を含む解析が、終わるようになります。切った所より下は、まるごと汚染として扱うので、依存の中の隣のフィールドへの流れが、多めに出ることがあります(見逃しにはなりません。その流れには `access_path_truncated` の注記が付き、使った深さは `summary.access_path_depth` に出ます)。前の動作(依存も深さ 3)に戻すには、`-dep-k 3` を付けます。

### 自分のプロジェクトで、どの依存が重いかの表を得る

依存を含む大きいプロジェクトで、`analyze` が時間切れや収束しないで終わるとき、どの依存が重いかを、パッケージ名と数字だけの表で出せます。

```sh
rivus analyze -pass-stats -pass-stats-top 0 -pass-stats-sort time -timeout 600s -detail summary > out.json
jq -r '["package","dep","functions","reanalyses","spent_ms","summary_elems"], (.stats.packages_top[] | [.package, (.dep // false), .functions, .reanalyses, .spent_ms, .summary_elems]) | @tsv' out.json > table.tsv
jq '.stats | {timed_out, converged, largest_scc, largest_scc_packages, summary_elems, functions, summary_sizes}' out.json
```

- `-pass-stats-top 0` は全パッケージ、`-pass-stats-sort time` は、かかった時間の順です(`time` は時計なので、実行ごとに少し違います。時計に依らない並びは `reanalyses`(既定)、`functions`、`elems`)。
- 表の列は、`dep`(依存のパッケージか)、`functions`(関数の数)、`reanalyses`(関数の解析の回数)、`spent_ms`(かかった時間)、`summary_elems`(要約の要素の数)です。
- `summary_sizes` は、関数の要約の要素(解析が関数ごとに持ち運ぶ、taint の単位)が、関数にどう散らばっているか、何でできているかの数字です(関数名やコードは出ません)。要約の大きさの偏り(少数の関数が大きい要約を持つのか、多くの関数が同じくらいずつ持つのか)を、数字で見られます。
  - 大きさの分布: `functions`(要約が空でない関数)、`empty`(空の関数)、`median`、`p90`、`p99`、`max`、`over_100` / `over_1000` / `over_10000`(その個数より大きい要約を持つ関数の数)、`top_1pct_share_permille`(要約が大きい上位 1% の関数が、全要素の何‰を持つか。1000 に近いと少数の関数に集中、10 に近いと均等)。`own_elems` と `dep_elems` は、プロジェクト自身と依存の関数の要素の数です。`summary_elems` を、関数の数で割ると、1 関数あたりの平均です。
  - 内訳: `by_output`(要素が行き着く先。`result`、`param`(引数の指す先への書き込み)、`global`、`throw`)、`by_element`(`real` は source 由来、`param` は引数の中身(呼び出し側が渡すもの)、`param_0` はそのうち 0 番目の引数(メソッドではレシーバ)、`with_path` は引数の中のパスつき、`with_guards` は引数の条件つき)、`by_path_depth`(引数の中のパスの深さ。`3+` は 3 以上。深さの上限は `-k`)。`distinct` / `distinct_dep`(依存の関数だけ)は、出力ごとに数えた要素を、すべて(`all`)、ガードを無視(`ignoring_guards`)、パスを無視(`ignoring_path`)、パスとガードを無視(`ignoring_path_and_guards`)の 4 通りで数えたもの。`all` と最後の数の比が、パスの深さと条件が要素を何倍にしているかの目安。
  - source の広がり: `real_origins`(要約に載っている、source の箇所の種類の数)と、`origin_reach`(その source の箇所 1 つ 1 つが、何個の関数の要約に載っているかの分布。`max`、`median`、`p90`、`over_100` / `over_1000`(それより多い関数に載っている source の箇所の数))。少数の箇所が全関数に広がっているのか、各箇所が少数の関数にだけ載っているのかが分かります。`globals_top`(要約が各グローバル変数に入れた要素の数の上位 10。名前は出ません)と、`globals_with_elems`(要素を持つグローバルの数)も出ます。
- `summary.on_findings_top` と `summary.on_findings_dispatch_candidates`(`-pass-stats` のときだけ)は、finding の経路に付いた注記が、どこに集中しているかの数字です。`on_findings_top` は、注記の種類(`dynamic_dispatch`、`assumed_call`、`container_element_imprecision` など)ごとに、注記が最も多いシンボルの上位 10 件(`notes`: 注記の数、`findings`: それが付いた finding の数)で、`on_findings_dispatch_candidates` は、インターフェースや関数値を通る呼び出しの注記が、候補 0、1、2-5、6-20、21 以上のどれに当たるかの数です。多くの finding が同じ呼び出しを通ると、その呼び出しの注記は finding ごとに数えられます。どの呼び出し(どの依存のメソッド)が、注記を集めているかが分かり、モデルで切る候補になります。シンボルは、コードの名前です。共有するときは、依存のものだけにしてください。
- `largest_scc_packages` は、互いに呼び合う関数の最大のかたまり(`largest_scc`)が、どのパッケージの関数でできているかの上位 10 件です。かたまりの大きさが解析の重さを決めるので、ここに出る依存が、モデルで切る候補です。
- `stats.timed_out` が true でも、途中までの内訳が出ます。`packages_top` と `largest_summary` は、時間切れまでに解析した分で、`largest_scc_packages` は呼び出しグラフから解析の前に作るので、解析が始まる前に切れても出ます。`-timeout` は、互いに呼び合う大きいかたまりの中でも、関数ごとに見ます(1 つの関数の解析の中は `-func-timeout`)。
- `stats.timed_out` は、時間切れでないとき、出力にありません(jq では `null`)。
- 共有してよいのは、この表(パッケージ名と数字)だけです。コードや経路(`-detail summary` で省いています)は共有しないでください。パッケージ名が非公開のものなら、共有の前に確認してください。

### 言語ごとの準備

| 言語 | どう読むか | 必要なもの |
|---|---|---|
| Go | `go/packages` と `go/ssa` で直接読む | Go のツールチェーン。コードがビルドできること |
| Dart | 別プロセスの extractor（`package:analyzer`）が IR を書き、rivus が読む | Dart SDK。**解析の前に、そのパッケージで `dart pub get` を実行する**（rivus は実行しません。していない、または古いとき、rivus は `summary.unread` と stderr でそう報告します）。コード生成（`build_runner`）も済んでいること。[extractor/dart](extractor/dart/README.md) |
| Rust | 別プロセスの extractor（rustc のドライバ。MIR を読む）が IR を書き、rivus が読む | 固定した nightly の Rust と extractor（`rivus setup rust -source extractor/rust` か Homebrew で用意する。macOS は既定でサンドボックスの下、Linux は `-unsafe-host-exec` のときだけ読める）。`-lang` なしでは、macOS で `[Y/n]` と聞いてから読む(`-yes` で聞かない)。コードがビルドできること、`Cargo.lock` があること。**読むと、対象のビルドが実行される**（下の「信頼できないコード」）。[extractor/rust](extractor/rust/README.md) |

- Dart と Rust の extractor（`rivus-dart`、`rivus-rust`）は、PATH に置くか、`-dart-extractor` / `-rust-extractor`（または環境変数 `RIVUS_DART_EXTRACTOR` / `RIVUS_RUST_EXTRACTOR`）で指します。`rivus setup dart -source extractor/dart` で extractor をコンパイルしておくと（約 10 秒）、名前（フラグ、環境変数）を指していないときに、PATH の `rivus-dart` より先にそれを使います（`dart run` の起動の 5 秒以上が要らなくなります。ソースか Dart SDK が変わったら使わず、そう言います）。extractor が書いた IR のファイルを、`rivus analyze -ir file.jsonl` で読むこともできます。
- `-lang` を書かない(auto)とき、`Cargo.toml` のある対象では、**読む前に聞きます**(macOS で、標準入力が端末のとき): 「読むにはビルドが要り、`build.rs` と proc macro が動く。macOS のサンドボックスの下で動く。依存を crates.io から取得する」と言って、`[Y/n]`(Enter は Yes)。聞かれたくないときは `-lang rust`(または `-yes`)、読みたくないときは `-lang go,dart` のように指定します。**答えによって別の index になります**(Rust ありと、なしは、別のもの)。端末でないとき(`claude -p`、CI、パイプ)は聞けないので、`-yes` がなければ Rust は読みません(Rust だけの対象はエラー)。サンドボックスのない環境(Linux)も、auto では読みません(`-lang rust -unsafe-host-exec` と明示したときだけ)。**読まなかった言語は、結果に残ります**: `summary.unread.skipped_languages`(と、index の `meta.unread`)に `{"lang":"rust","why":"declined"|"no-terminal"|"not-installed"|"no-sandbox"}`。この index の `path` の「経路なし」は、`exhaustive: false` になります。Rust だけの対象で断ると、何も読まずに終了コード 3 で終わります(0 = 成功、1 = エラー、2 = 使い方の誤り、3 = Rust を断った、4 = 完全でない解析(時間切れ、未収束。結果は出ている。`summary.incomplete_reasons` に理由)、130 = 中断。`-h` は、どのコマンドでも 0 です)。
- **テストと、開発用の依存は、既定で読みません**（Go の `_test.go`、Dart の `test/` などと `dev_dependencies` だけから使われるパッケージ、Rust のテスト・例・ベンチと `[dev-dependencies]`）。本番のビルドに入らないコードだからです。読まなかった数は、出力の `stats.tests_skipped` に出ます。`-tests` で読みます。
- 対象のコード生成（`go generate`、`build_runner`）は、rivus はしません。先に済ませてください。ビルドも、Go と Dart ではしません。**Rust は例外で、読むときに `cargo check` が実行されます**（上の表と「信頼できないコード」）。

## 今の状態

Go から作り始めたので、**Go が最も進んでいて、Dart と Rust は、それより見逃しが多い**状態です。差を測って縮める作業をしています。

| | Go | Dart | Rust |
|---|---|---|---|
| 読み込み | 直接（`go/ssa`） | extractor（最初のバージョン） | extractor（最初のバージョン） |
| 動的な呼び出しの候補 | 型に基づく解析で絞る | rivus 自身が値を追う。追えないと候補なし（不確実性として出る） | extractor が型で候補を出す（型が合うだけの候補も入る）。一部は rivus 自身が値を追う |
| 実コードでの確認 | 複数のプロジェクト | 少ない（Flutter のアプリは、これから） | 少ない |

- 言語間の差は、**同じ流れの形を 3 言語で書いたテスト**（[bench/zoo](bench/zoo/README.md)）で数えています。`go test ./cmd/rivus -run Zoo -v` が、言語ごとに「出る / 注記つきで見逃す / 何も報告せずに見逃す / デコイを誤って出す」の数を出します。
- 既知の見逃しの一覧と、その原因は [bench/MISSES.md](bench/MISSES.md) にあります。

## 何を報告しないか

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

**組み込みで持つ範囲**は、次の 5 つです([ADR-096](docs/adr/096-builtin-scope-four-parts.md)、[ADR-097](docs/adr/097-builtin-scope-service-clients.md))。(a) Google Cloud のクライアント(Firebase を含む。範囲の 1 つ目。Go の Cloud Spanner と BigQuery は `internal/model/builtin/go-gcp.yaml`、Dart は `dart-gcp.yaml`)。(b) 外部のサービスのクライアント(クラウド、認証、決済、メールと通知、検索、監視、メッセージング、ストレージ、AI の API など。公開の指標の上位から選ぶ)。(c) 各言語の標準ライブラリ。(d) 代わりのないパッケージ(Rust の `serde`、`tonic`、`tokio`、`futures`、`bytes`、`libc`、HTTP の型の `http` / `url` など)と、SQL のドライバ(`sqlx`、`rusqlite`、Dart の `sqflite`)。(e) pub の人気の上位(Flutter に依存しないサーバ側と基本のパッケージ。Dart の `shelf`、`http` など)と、Rust の一部の Web フレームワーク。選ぶ基準は公開の指標だけで、特定の利用者が何を使うかでは選びません。候補と進める順は [docs/DART-MODEL-CANDIDATES.md](docs/DART-MODEL-CANDIDATES.md)(今は Dart から)です。**何を組み込むかは、その都度判断します**([ADR-063](docs/adr/063-frameworks-in-examples-rules.md) の 2026-11-22 の追記)。組み込みを増やすのは、依存を API の境界のモデルにして、解析のコストを下げるためと、依存の中に届く流れを検出するためです(sink のモデルは、検出に必要です。ADR-099)。モデルがあるメソッドは、呼び出しの所でモデルが使われ、そのメソッドの本体の要約は使われません。実装の多いインターフェースのメソッド(Dart `package:http` の `Client.send` など)では、実装全体をモデルで切り、読まなかった依存の本体を `dependency_body_unread` で示します([ADR-098](docs/adr/098-cut-delegation-clique-methods.md)。条件は [docs/MODELING.md](docs/MODELING.md) の 4a 節)。パッケージ単位で本体を読まない宣言(`body_replaced_packages`。ADR-100)は、解析の側にあります(落とした関数を、読み込みのあとに外す)。extractor が、宣言のあるパッケージを、そもそも読まない段階は、進行中です(`docs/DESIGN-package-models.md` の 7 節)。 プロジェクト固有のラッパーや、組み込みでないフレームワークは、使う側がルールで足します。見本は [examples/rules](examples/rules) に置いています(個々のフレームワークは、組み込みになったものから見本が空のルールになるので、名指しせず、見本の一覧は `examples/rules/README.md`、組み込みの一覧は `rivus models versions` か `docs/MODEL-QUEUE.md` を見てください)。組み込みでないフレームワークの入口は、ルールを足すまで source として見えず、その分、検出が減ります。コードがそのフレームワークを呼んでいるのに、モデルがルールにないときは、`analyze` が stderr に 1 行知らせます([ADR-063](docs/adr/063-frameworks-in-examples-rules.md))。そのための仕組み:

1. 置き場所: 解析するディレクトリの `.rivus/rules/*.yaml`（フラグなしで読まれる）か、`-rules file.yaml`。書き方の見本は [examples/rules](examples/rules)。
2. 何を書けばよいか: `rivus models coverage` が、finding の経路に載っているのにモデルがない関数を、効果の大きい順に出します。`-yaml` で、埋めるためのテンプレートが出ます。

   ```sh
   rivus models coverage -yaml -top 20 ./...
   ```

3. 確信のない流れ: モデルの `flows` に `approximate: "理由"` を書くと、流れは消さずに、通った finding に不確実性 `model_approximation` が付きます（書き方は [docs/MODELING.md](docs/MODELING.md)）。
4. 誰が書いたか: ルールの `origin: builtin | human | llm` で区別できます。`-deterministic-only` は `llm` のルールを無視します。LLM 由来のモデルが効いた finding には `llm_influenced` が付きます。モデルが流れを消した場所は `suppressions` に残ります(`origin: llm` の `sanitizers` の `drops`、`builtin` 以外のモデル(`llm` と、人が `-rules` に書いたもの)の `pure: true` と、結果に届かない `flows`。組み込み(`builtin`)の `pure` と `flows`、組み込みと人が書いた `sanitizers` の `drops` は、確認済みなので残しません)。同じ source から別の流れが消えた関数を通る finding にも、その LLM 由来のモデルを `models` に入れ、`llm_influenced` を付けます。

LLM にモデルを書かせ、人や別の手段で確かめる、という使い方を想定しています。書式の手引きは [docs/MODELING.md](docs/MODELING.md)(英語。まだ全部の節は無い)。モデルを実行で確かめる道具は、これから整えます。

ルールで書けること（抜粋）:

- `kind: param`: 関数の引数を source にする（フレームワークが呼ぶハンドラの入口）。その値は、関数の中では追いますが、呼び出し元には書き戻しません。
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
| Rust | extractor が、対象に `cargo check` を実行します（**対象をコンパイルします**）。対象と依存の `build.rs`、proc macro が実行されます。**macOS では既定で、OS のサンドボックス（`sandbox-exec`）の下で動かします**（依存を取る段階は、ネットワークがあるが、対象の cargo の設定は読ませない。抽出の段階はネットワークなし、対象は読み取り専用、書き込みは作業ディレクトリだけ。サンドボックスが作れなければ、動かさずに止まります）。**Linux には隔離の手段がなく、既定では何も実行せずに止まります。** ホストで直接動かすのは `-unsafe-host-exec`（信頼できるコードだけ。Linux ではこれが普通の使い方です）。どちらで読んだかは、出力の `summary.isolation` に出ます。**信頼できないコードの解析の結果とログは、そのコードが書き込んだものを含みえます。そのまま公開の場所に出さないでください。**手順と限界は [docs/rust-isolation.md](docs/rust-isolation.md) |
| Go | `go/packages` が `go list` を実行します。依存の取得（ネットワーク）が起きることがあります。cgo のあるパッケージは、cgo の前処理（C コンパイラ）が実行されます。**対象のソースそのものは実行しません**が、「Go はコードを実行しない」とは言えません。隔離はありません（Rust と同じ作りに載せる予定） |
| Dart | extractor は、静的に読むだけです。ただし、前提の手順（`dart pub get`、`build_runner`）は、対象の依存と生成器のコードを実行します |

- **Rust は、対象のビルドが、解析の結果そのものも書き換えられます。** IR の断片を書く場所が、対象のビルドから見えるためです（隔離の下でも同じです。隔離が守るのは、マシンと秘密で、結果の正しさではありません）。
- **対象のディレクトリに付いてくるルール（`.rivus/rules/*.yaml`）は、フラグなしで読まれます。** 対象を書いた人は、そこで finding を消せます。読んだファイルと、消せるものの数は、出力の `project_rules` に出ます。**信頼できないコードでは `-no-project-rules` を付けてください。**
- Dart の依存は、プロジェクト、pub のキャッシュ、SDK の下のものだけ読みます（`package_config.json` は対象の一部で、どこでも指せるため）。ほかの場所は `-allow-deps-under DIR` で足します。
  - 許可の外の依存は読まず、**読まなかったことを `summary.unread`(理由は `dependency_not_allowed`)と標準エラーに出します**(結果が、その依存を読んだものに見えないように)。
  - 複数のパッケージを 1 つのリポジトリに置いている(モノレポ)とき、`-C` は**解析の始まり**です。解析したいアプリ(サーバ)のディレクトリを指します: `rivus analyze -lang dart -C <リポジトリ>/app -allow-deps-under <リポジトリ>/packages`。アプリの依存が、同じリポジトリの別のパッケージ(兄弟)を `path:` で指しているとき、その兄弟は `-allow-deps-under` で足さないと読まれません(読まなかったことは `summary.unread` に出ます)。`-allow-deps-under` の下のパッケージ(`path:` の依存、`workspace:` のメンバー)は、読むだけでなく、プロジェクトとして扱います(source と sink を探します)。pub のキャッシュと SDK の下は、含まれていても依存のままです(ADR-114)。
  - `-C` に何を指すかで、解析するものが変わります。リポジトリのルートを指すと、`workspace:` のメンバーの全部が始まりになり、読む範囲が広くなって、解析が終わらないことがあります。`packages` のような、パッケージを束ねたディレクトリを指すと、そこに並ぶ別のプロジェクトが対象になり、アプリではなくなります。サーバのアプリを読みたいときは、そのアプリのディレクトリを指してください。読み込みが長いときに、何を読んでいるかはまだ出ません([#174](docs/issues/174-extraction-progress.md))。
  - `workspace:` に Flutter のメンバーがあると、Dart SDK だけの `dart pub get` が通りません。読みたいのがサーバ側のパッケージだけのときは、`pub get` の前に、Flutter のメンバーをルートの `pubspec.yaml` の `workspace:` から外してください(解析する側のコピーで。[#176](docs/issues/176-dart-flutter-workspace-members.md))。
  - Go でプロジェクトの範囲を狭めるには、パターンか `-C` を使います: `rivus analyze -C <module> ./cmd/...`、または `-C <module>/cmd/app`。**プロジェクトは、パターンで挙げたパッケージだけではなく、同じ module のパッケージの全部**です(`go.work` に並ぶ module と、`replace` がディレクトリを指す module も)。パターンが決めるのは入口と、頼んだ範囲で、狭めた先のコードが届く、同じ module の他のパッケージの source と sink も探します。module キャッシュの module と、`replace` がバージョンを指す module は依存です。取り込んだ第三者のコピーを `replace` で置いているときは、`-dep-dir <ディレクトリ>` で依存として読ませます(指定は `summary.dep_dirs` に出ます)。狭めた指定に `-no-deps` を付けると、挙げなかった同じ module のパッケージは読まれず、そのことは出力に出ません(ADR-114)。
  - 依存は `package:` の ID で解決されます。同じ依存の別のバージョンを 2 つのパッケージが使うときは、1 つしか読めず、もう 1 つは `summary.unread`(`dependency_not_linked`)に出ます。

Rust 以外の信頼できないリポジトリは、使い捨ての環境（VM など）で解析してください。依存を先に取得してから、ネットワークを切り、ソースを読み取り専用で渡す形です。

## 仕組み

```
Go / Dart / Rust の extractor ──→ 中立の IR（CFG、JSON Lines）──→ コア（Go）
                                                                SSA → 関数ごとの要約 → taint → 出力
```

- 言語ごとの extractor が、ソースを、制御フローグラフの水準の IR に下ろします。コアは、言語を知りません。
- コアは、関数ごとに「引数や戻り値のあいだで、何が何に伝わるか」の要約を作り、呼び出しグラフの下から順に組み合わせます。
- `rivus index` は、呼び出しグラフを SQLite のファイルに保存します（利用者のキャッシュに置き、解析の対象のディレクトリには書きません）。`index -taint` の JSON の index も、`-o` を省くと、同じキャッシュの下（`graph.db` の隣の `index.json`）に書きます。

詳しくは [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)。設計の判断の記録は [docs/adr/](docs/adr/README.md)(1 件 1 ファイル)。

## 開発

新しいマシン(Linux)では、先に `make setup` を 1 回実行します。go.mod の Go(1.27 系の最新のパッチを固定)と、CI と同じバージョンの Dart SDK がなければ、バージョンとチェックサムを固定して `~/.local/rivus-dev` に入れ(sudo は不要)、Dart の各パッケージの `dart pub get` を実行します。インストール先は環境変数 `RIVUS_DEV_TOOLS` で変えられます(未設定なら `~/.local/rivus-dev`)。入っていれば何もしないので、何度実行しても同じです。終わりに PATH の設定が表示されます。Rust の nightly と隔離の仕組みは入れません(`make test` に必要。`.github/workflows/ci.yml` の `rust` ジョブを参照)。

`make setup` は、`git config core.hooksPath .githooks` も設定します(`make setup` を使わないときは、自分で実行します)。`.githooks/pre-push` は、PR がマージ済み(または閉じている)で、開いている PR がないブランチへの push を止めます(コミットが main に入らないため。`gh` がなければ警告だけで通します)。意図した push は `RIVUS_ALLOW_PUSH_TO_MERGED=1 git push` にします。`core.hooksPath` の相対パスは、各 worktree の最上位から読まれます。`.githooks/` がない古い worktree では、clone の設定に絶対パス(`git config core.hooksPath <main の checkout>/.githooks`)を入れます。

```sh
make setup                                      # 新しいマシンで 1 回(Go、Dart SDK、dart pub get)。そのあと make check
make check                                      # push の前(約 1 分): ビルド、gofmt、go vet、短いテスト(go test -short。TestDogfood は週 1 回の CI で、前回から差分があるときだけ回す。手元は `RIVUS_DOGFOOD=1`)
make test                                       # 全部のテスト(Dart と Rust の extractor つき。CI の main と同じ)
go test ./...                                   # Go だけで走るテスト
RIVUS_DART="dart" go test ./cmd/rivus           # Dart の extractor を使うテストも（extractor はテストが 1 回コンパイルして使う。RIVUS_DART_FROM_SOURCE=1 で dart run に戻す）
RIVUS_RUST=$PWD/extractor/rust/target/debug/rivus-rust go test ./cmd/rivus   # Rust の extractor を使うテストも（先に extractor/rust をビルド）
```

PR の CI は `go test -short` で実行します(重いテストは、`-short` のときにスキップされます)。main への push、週 1 回の実行、tagpr のリリース PR では、全部のテストを実行します。main の実行が失敗したら、次の作業より先に、修正するか、原因の PR を取り消します。

言語に閉じた変更の PR(`extractor/dart` や `bench/zoo-dart` だけの変更など)では、ほかの言語の extractor のジョブ(`dart` または `rust`)をスキップします。共通の場所(`cmd`、`internal`、`testdata`、共通の `bench` など)を変えた PR は、両方を実行します。

正しさは、役目の違うテストとベンチで確かめています（[bench/README.md](bench/README.md)）。

| 置き場 | 何を見るか |
|---|---|
| `testdata/`、`bench/*-curated` | 小さいケース。ソースの `@source(tag)` / `@sink(tag)` / `@sink(none)` が期待値 |
| `bench/zoo-go`、`zoo-dart`、`zoo-rust` | 流れの形を 1 つずつ。**期待値は、ケースを実際に動かした結果**（rivus の出力に合わせて書けない）。脆弱なケース、修正済みのケース、デコイの 3 つ組 |
| `bench/go-scale` | 大きなコードで起きる現象（要約の膨張、収束しない）を、数秒で再現する生成コード |
| `bench/wild` | 公開された実世界の脆弱性。その件の source と sink を与えたとき、本物の経路を組み立てられるか |
| 実在のプロジェクト（`bench/gogs` ほか） | 変更の前後で、finding の増減を比べる（`bench/compare.sh`）。コードは置かず、バージョンの指定だけ |

解析の精度に関わる変更は、`bench/compare.sh <前> <後>` で前後を比べ、消えた finding が本物でなかったことを確かめてから入れます。

## ドキュメント

読者ごとの入口です。全体の目次は [docs/README.md](docs/README.md)。

| 読者 | 読む文書 |
|---|---|
| 使い手 | この README。信頼できないコードの扱いは [docs/rust-isolation.md](docs/rust-isolation.md)。用語は [docs/GLOSSARY.md](docs/GLOSSARY.md) |
| ルールを書く人 | [docs/MODELING.md](docs/MODELING.md)(書式)、[examples/rules](examples/rules)(見本)、[docs/GCP-MODELS.md](docs/GCP-MODELS.md) |
| rivus を使う AI エージェント | [docs/AGENT-GUIDE.md](docs/AGENT-GUIDE.md)(コマンドの選び方、フラグ、出力の読み方、ユースケース別の手順)、[docs/GRAPH.md](docs/GRAPH.md)(`rivus index` と `rivus graph`)。出力の読み方はこの README の「何が出るか」にもあります |
| 開発者 | [docs/PRINCIPLES.md](docs/PRINCIPLES.md)(設計思想)、[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)(今の構成)、[docs/adr/](docs/adr/README.md)(判断の経緯。1 件 1 ファイル。一覧は `make docs-gen`)、`docs/DESIGN-*.md`(個別の設計。冒頭に状態がある)、[bench/README.md](bench/README.md)(テストと測定) |
| レビューする人 | [docs/PRINCIPLES.md](docs/PRINCIPLES.md)(原則と、まだ届いていない所)、[bench/MISSES.md](bench/MISSES.md)(既知の見逃し) |

[docs/history/](docs/history/) の BRIEF.md と QUESTIONS.md は、最初の構想と、それへの指摘です(歴史的な文書。今の設計とは異なる所があります)。

## ライセンス

[Apache-2.0](LICENSE)
