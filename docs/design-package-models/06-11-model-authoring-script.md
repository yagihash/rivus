### 6.11 モデル作成の手順のスクリプト化(設計案)

背景: クラウドで 1 パッケージ分のモデルを作る作業(手順書 [MODEL-RUNBOOK.md](../MODEL-RUNBOOK.md))は、Dart(dio、github、shelf、googleapis_auth)、Go(gin、storage、sqlx、go-github、`os` と `database/sql` の行)、Rust(actix-web、axum、google-cloud-storage、octocrab)の 10 本あまりで、ほぼ毎回、手順書への追記が要った。クラウドで確実に回すには、手順を全部スクリプトにして、人や LLM の判断を「sink、source、sanitizer をどれにするか」だけに絞る必要がある。最初の要件(決まった手順はスクリプト、LLM は sink / source / sanitizer の選択だけ)に戻る。この節は設計案で、実装はしない。事実は、手順書の履歴(`git log --follow docs/MODEL-RUNBOOK.md`)と、リポジトリのファイルで確認した。

#### 1. 手でやっている段と、スクリプトにできるか

追記の原因が「スクリプトがなかった」ものか、「本当に判断が要る」ものかを分ける。

| 順 | 段 | 手順書の追記(根拠) | 分類 | 理由 |
| --- | --- | --- | --- | --- |
| 1 | 取得(`go mod download`、`dart pub get`、`cargo fetch`。取得だけ) | 0 節の「取得と実行の範囲」の表 | スクリプトにできる | 一覧(MODEL-QUEUE.md)の行から決まる |
| 2 | 公開の API の列挙と、粗い flows の生成(型から) | 2 節の 1 は「未実装」のまま。`rivus models coverage -yaml` で代用している | スクリプトにできる(新規。4.3a の規則 a〜f) | 型だけで決まる。判断は要らない |
| 3 | 骨組み(skel)の作成。公開の署名どおり、sink の行の関数は記録を呼ぶ | 4 節の補足(Go は `zoo/mark`、Rust は `zoo-mark`)。dio、shelf の骨組みと本物のずれ。sqlx の埋め込み | スクリプトにできる(新規。6.8 の (b) と生成器の要件) | 本物の署名と、埋め込み・実装の関係から機械的に作れる。手書きは、本人がずれに気づけない |
| 3a | Rust の骨組みで、記録に至る呼び出しの連鎖のすべてに `#[track_caller]` を付ける | 4 節の「骨組みの `#[track_caller]`」(actix-web などで、1 つ欠けて記録の場所がずれた) | スクリプトにできる(骨組みの生成に含める) | 規則が 1 つで、生成なら漏れない |
| 3b | Rust の `[workspace] exclude = ["skel"]`(`kind: field` の source を使うとき) | 4 節の Rust の注意(octocrab) | スクリプトにできる | 条件を付けず、骨組みに常に足す(3 つの言語とも、骨組みを依存として扱う) |
| 3c | 埋め込み型・インターフェースの骨組みの形(sqlx の `DB` が `*sql.DB` を埋め込む) | 4 節の補足と 6.8 の生成器の要件 | スクリプトにできる(型の情報から) | 型の埋め込みは `go/types` などで読める |
| 4 | セルの置き場所(Go は `bench/zoo-go` か `bench/zoo-go-pkg/<名前>/`。パッケージの設定、`go.mod`、`rules.yaml`) | 4 節の「さわってよいファイル」。標準ライブラリの行を、`zoo-go-pkg` のセルで確認するか `zoo-go` のセルでするか、の迷い(`os`、`database/sql`) | スクリプトにできる(規則を決めれば) | 規則: 第三者のパッケージは `zoo-<lang>-pkg`。標準ライブラリは、対のセルの確認を満たせれば `zoo-<lang>`、満たせなければ `zoo-<lang>-pkg/<標準ライブラリの名前>`(#219、#223 の形)。コマンドが決める |
| 5 | セルの本体(vuln / fixed / decoy の呼び出し) | 2 節の 4 | 骨組み(テンプレート)はスクリプトにできる。中身は判断が要る | 行ごとに「届く / 届かない」のセルが要る。テンプレートは、行から作れる。`fixed` は Rust で 4 つの `clean_*` を通す決まり(4 節)なので、テンプレートに入れる |
| 6 | マークの固定(`ZOO_MARKS_UPDATE=1 go test ./cmd/rivus -run '^TestZooMarksLock$'`) | 4 節の補足 | スクリプトにできる | コマンドが決まっている。新しいセルだけが増えたことを確認する |
| 7 | Dart の依存の解決(`--dep-dir`、`pubspec.yaml` の `dependency_overrides`、`zoo_exec_override`、`make zoo-dart-gen`) | 4 節、6.8(shelf と `test` の名前のぶつかり。github の `field` の source を `--dep-dir` で読む) | スクリプトにできる | `zoo_exec_override` の行は、`cmd/rivus/zoo_dart_gen_test.go` が読む。骨組みの `pubspec.yaml` に書くだけ。`--dep-dir` は条件を付けず、骨組みに常に足す |
| 8 | Go の major バージョンの接尾辞(`/v2` など)を、シンボルでどう書くか(ワイルドカード) | 手順書の追記の履歴(go-github)。手順書の本文には、いまは規則がない(**要確認**) | スクリプトにできる(モジュールのパスから) | モジュールのパスの `/vN` は、`go list -m` で決まる |
| 9 | Rust の、本物のクレートでの記号の綴りの確認(axum の `Request` など) | 4 節の「Rust のモデルの標準の確認」。サンドボックスの下だけ | スクリプトにできる(ただし、extractor の実行が要る。3 節) | 手順(最小の crate、`cargo generate-lockfile`、`rivus debug extract`、`ir.jsonl` の照合)が決まっている |
| 10 | `SKIP` のまま通る `TestZooPkgRust`(`RIVUS_RUST` がないと SKIP で `ok`) | 4 節の Rust の注意。実際に「確認していない」のに通った | スクリプトにできる | スクリプトが `RIVUS_REQUIRE_RUST=1` を付け、未設定なら失敗にする。**これは「スクリプトの欠け」の典型** |
| 11 | `.dead-allow` の書き方(`symbol \| 理由`、重複なし、同じ形の行を 1 行ずつ) | 4 節、6 節(書式の注意) | スクリプトにできる | 「新しい行がどこにも一致しない」と出たものを、理由の定型で書き出す。理由の定型は `dependency` / `debt` / `spelling`(`bench/parity/README.md`)から選べる |
| 12 | `verified_versions` | 2 節の 3 | スクリプトにできる | 取得したバージョンから書ける(6.7) |
| 13 | sink / source / sanitizer の選択 | 2 節の 4、ADR-108(source は「誰が書いたか」で選ぶ)、5 節の 7(案と違う判断) | **判断が要る** | 危険かどうかは、API の意味で決まる。ここだけ、人か LLM が決める |
| 14 | `calls:`(コールバックと、利用者が実装するインターフェース) | 手順書の 8 節(手で書く) | 候補の一覧はスクリプトにできる。採用は判断が要る | 関数を引数に取る関数と、インターフェースを引数に取る関数は、型から列挙できる。呼ぶタイミングは、読んで決める |
| 15 | 既知の見逃し(`@miss`)と誤検出(`@fp`)の課題の起票(`docs/issues/model-*.md`、`@ref`) | 3 節 | テンプレートはスクリプトにできる。内容は判断が要る | テンプレート(`docs/issues/README.md`)は固定。原因の説明は人が書く |
| 16 | 差分テスト(`make model-diff`)の実行と、PR で動かす対象(`-changed`) | 9 節 | スクリプトにできる(済み) | 既にある |
| 17 | 行がない関数の一覧(`-coverage-gaps`) | 9 節 | スクリプトにできる(Go の標準ライブラリだけ済み) | 第三者のパッケージと Dart、Rust は未対応(下の 4) |
| 18 | PR の本文(5 節の 7 項目)、題、署名の確認 | 5 節 | スクリプトにできる | 数字(行の数、MISS / EXTRA、セルの数、`verified_versions`)は、出力から埋める。「手順のとおりにできなかった所」だけ人が書く |

**「スクリプトの欠け」と「本当の判断」の仕分け**(追記の中身で数える): 追記の大半は、3、3a、3b、3c、6、7、8、9、10、11 で、どれもスクリプトにできる。判断が残るのは、13(sink / source / sanitizer)、14 の採用、15 の内容、5 のセルの中身(届く / 届かない)だけで、手順の 2 割ほど。つまり、毎回追記が要ったのは、判断の難しさより、決まった手順を文章で渡していたことによる。

#### 2. 目指す形: 1 つのコマンドと、モデルのファイルの中の判断

```sh
make model PKG=<パッケージ> VERSION=<バージョン> LANG=<go|dart|rust>   # 案。名前は未定
```

終了コードで、失敗した段を言う(0: 成功、10: 取得、20: 生成、30: 骨組みのビルド、40: セル、50: 差分テストの MISS、60: zoo のゲート、70: 未決の候補がある(`-expand` では、決めの衝突、`covers` を超える増加、代表が生成した行でない)、80: 実行できない段(Rust の extractor)で止まった)。数字は例で、決めるのは実装のとき。

| 順 | やること | 出力(コミットするか) | 使う既存の部分 |
| --- | --- | --- | --- |
| 1 | 取得(取得だけ。実行しない) | なし(一時の場所) | `go mod download`、`dart pub get`、`cargo fetch` |
| 2 | 型から flows を生成する(4.3a の規則) | `internal/model/builtin/<lang>-<package>.gen.yaml`(コミットする。毎回上書き。先頭に生成器のバージョンと入力) | 新規(生成器) |
| 3 | 骨組みを生成する(公開の署名、埋め込み、実装、記録の呼び出し、`#[track_caller]`、`exclude`) | `bench/zoo-<lang>-pkg/<パッケージ>/skel/`(コミットする。先頭に生成器のバージョン) | 新規(骨組みの生成器) |
| 4 | 候補を出し、モデルのファイルと照合る(下) | 未決があれば、作業ファイルに一覧(コミットしない)。終了コード 70 | 新規(候補の一覧)。`rivus models coverage`、`-coverage-gaps` |
| 5 | zoo のセルのテンプレート、`rules.yaml`、`pubspec.yaml` の設定 | `bench/zoo-<lang>-pkg/<パッケージ>/cells/`(テンプレートはコミットする。中身は人か LLM が足す) | 新規(テンプレート)。`make zoo-dart-gen` |
| 6 | マークの固定、`.dead-allow`、`verified_versions` | `bench/zoo/marks.d/`、`*.dead-allow`、モデルの先頭(コミットする) | `TestZooMarksLock`、`readDeadAllow` |
| 7 | 差分テスト(MISS が 0。EXTRA は理由を注釈にする)。PR のジョブにも入れる(段 1 と同時) | なし(結果は PR の本文へ) | `bench/model-diff` |
| 8 | 行がない関数の一覧 | なし(PR の本文へ) | `-coverage-gaps`(第三者のパッケージへの拡張は新規) |
| 9 | zoo のゲート、`regomark check --no-judge` | なし | `scripts/zoo-loosened-check.sh`(`make zoo-loosened`)、`regomark` |
| 10 | PR の題と本文の材料 | `.claude/work/` などの一時の場所(コミットしない) | 新規(数字の集計) |

**判断の置き場: モデルのファイルそのもの**。入力ファイルを別に作って、そこからモデルを生成する形(`models-src/`)は採らない。手の行は、モデルの書式の全部(`named:`、`categories`、`callback_param`、`calls:`、`checked_by`、コメント)を使う。入力ファイルから生成すると、この書式を入力ファイル側でもう一度定義することになり、既存のモデルの移行も要る。代わりに:

- **採用した候補 = モデルの行そのもの**。今までどおり手で直してよい。
- **採用しなかった候補 = 同じファイルの `rejected:` の節**。1 件は `symbol`、`kind`(sink | source | sanitizer | calls)、`why`、必要なら `position`。今、モデルのファイルの先頭に「採用しなかった候補と理由」をコメントで書いているものを、データにする。ローダーは読んで検証するだけで、解析は使わない。

```yaml
rejected:
  - symbol: github.com/jmoiron/sqlx.LoadFile
    kind: sink
    why: "引数はファイルのパスで、クエリではない"
  - symbol: (*github.com/jmoiron/sqlx.NamedStmt).Exec
    kind: sink
    position: arg0
    why: "クエリは準備のときに決まる。sink は Preparex / PrepareNamed"
```

- **コマンドの動き**: 毎回、候補の一覧を作り直す。各候補を「行がある」か「`rejected:` にある」かに照合る。どちらでもない候補(未決)が 1 つでもあれば、終了コード 70 で止まり、未決の一覧を作業ファイル(コミットしない)に書く。人か LLM は、一覧を見て、行を足すか `rejected:` に足す。`rejected:` にあって候補から消えたもの(API が消えた)は、警告として出力に一覧する(止めない)。
- **状態はモデルのファイルだけにある**。同じ入力(パッケージとバージョン)と同じモデルのファイルなら、同じ結果になる。型から作る `*.gen.yaml` は、別のファイルで、毎回上書きする(手で直さない。直したいときは生成器を直す)。
- **既存のモデルは移行しない**。そのパッケージを次にコマンドに通したときに、未決の一覧を `rejected:` に書いて、足す。

**候補の出し方(決定的。2 層)**: 公開の関数は多い(go-github は公開のメソッドが 1,000 以上あるのに、sink の行は 57)ので、全部に判断を求めるのは無理。そこで、判断を必須にする層と、しない層に分ける。順序は、シンボルの辞書順。**同じ入力なら、同じ候補になる**。

- **層 1(採用か不採用かの判断が必須)**: 規則の表で決める。(a) 引数の名前と型(SQL、コマンド、パス、ファイル名、URL、ホスト、HTML、テンプレート、クエリ)。`resource_name` もここ(サービスのクライアントの名前の引数: bucket、object、owner、repo、project、topic など)。(b) 本文や中身を運ぶ型の戻り値とフィールド(ADR-108 の source の候補)。(c) (string → string)の関数で、名前に `escape` / `quote` / `sanitize` / `clean` を含むもの(sanitizer の候補)。(d) 関数またはインターフェースを引数に取る関数(`calls:` の候補)。規則の表はコードの中にあり、PR で足していく。
- **層 2(判断は要らない)**: ほかの公開の関数で、string / `[]byte` / `any` を取るもの。件数と一覧を、PR の本文の材料に出すだけ。
- 規則の外の行(手の行)は、今までどおり足してよい。

**層 1 の規則の最初の評価(段階 1 の最初の作業)**: 前の晩に採用した 12 個のモデル(dio、github(Dart)、shelf、googleapis_auth、gin、go の storage、sqlx、go-github、actix-web、axum、google-cloud-storage、octocrab)の採用した行のうち、層 1 に入るものがいくつかを数える(再現率)。入らない行は、規則の表に足すか、手の行として残す。この評価は、まだしていない。出発点として、モデルのファイルごとの行の数(`symbol:` の行を節ごとに数えた。2026-11-23)を置く: `dart-dio` sinks 44、`dart-github` sources 1・sinks 15、`dart-googleapis_auth` sinks 4、`go-gin` sinks 7・propagators 51、`go-storage` sinks 7、`go-sqlx` sinks 52、`rust-actix-web` propagators 19、`rust-google-cloud-storage` sinks 23、`rust-octocrab` sinks 21、`rust-web`(axum を含む)sinks 3・propagators 30。shelf、go-github、axum の行は、ファイル名から特定できていない(評価のとき、行の出所を確認する)。

**候補の元**: 段階 1 は、**ライブラリ単体の型の規則だけ**を使う。「ライブラリをプロジェクトとして解析して、公開の関数の引数が、すでにモデルのある sink に届くかを見る」案は、課題 #163([docs/issues/163-library-as-project-candidates.md](../issues/163-library-as-project-candidates.md))に分ける。層 1 の強い根拠になる。段階 3 のあとに入れる。課題 #144(利用者のプロジェクトで、汚染された値が届いた入口の API の一覧)は、別の課題のまま。

**コミットするものと、生成するもの**: モデルのファイル(`rejected:` を含む)、`*.gen.yaml`、骨組み、テンプレートのセル、マークの固定、`verified_versions` は、コミットする(ビルドに要る、または CI が読む。4.3 と同じ理由)。PR の本文の材料、未決の一覧、取得したソースは、コミットしない。セルの中身(人か LLM が足したもの)は上書きしない(テンプレートの部分にマークを付け、その外は触らない。**要確認**: マークの形)。

#### 3. 言語ごとの違い

| | Go | Dart | Rust |
| --- | --- | --- | --- |
| 型の取り方(4.2) | `go/packages`(`NeedTypes`) | analyzer の `LibraryElement` | extractor を使う(段階 5) |
| 実行する場所 | クラウドで全段が可能(取得だけ。実行しない) | クラウドで全段が可能 | ローカルのサンドボックスの下。クラウドは Rust を扱わない |
| 骨組みの形 | `skel/`、`zoo/mark` | `cells/packages/<名前>`(`zoo_exec_override`、`--dep-dir`) | `skel/`、`zoo-mark`、`#[track_caller]` |
| 差分テスト | あり | あり | 段階 5 で(extractor を動かす。サンドボックスの下) |

Rust は、extractor を動かす段(型の取得、セルの解析の確認、本物のクレートでの記号の綴りの照合、差分テスト)が、macOS のサンドボックスの下でしか許されていない(手順書 0 節、6.5)。クラウドには隔離がなく、クラウドは Rust を扱わない。**構文だけで署名を読む道具は作らない**(再エクスポートや型の別名を解けず、精度が下がるため)。Rust は、同じコマンドを、サンドボックスの下でローカルに走らせる(段階 5)。

**骨組みの共通の決め**: `--dep-dir` と Rust の `exclude = ["skel"]` は、条件付きにしない。骨組みには常に足す(3 つの言語とも、骨組みを依存として扱うため)。

**セルの置き場所の決め**: 第三者のパッケージは `zoo-<lang>-pkg`。標準ライブラリは、対のセル(parity)の確認を満たせるなら `zoo-<lang>`、満たせないなら `zoo-<lang>-pkg/<標準ライブラリの名前>`(#219、#223 の形)。コマンドが決める(選択は残さない)。

**言語を横断する順**(2026-11-23。言語の間の機能の差を大きくしない。[bench/parity/README.md](../../bench/parity/README.md) の機能の差の表で見る): 言語ごとに段 1〜3 を終えるのでなく、段ごとに 3 言語を回す。段 1(枠、候補、検証、差分テスト)は Go → Dart → Rust(Rust はローカルで、サンドボックスの下)。次に段 2(骨組みの生成)を Go → Dart → Rust。段 3(型からの flows の生成)も同じ。Rust を急ぐ必要はないが、段の途中で差が開いたままにしない。Rust の段 1 の前提は、次の 4 つ(見積もりは、あとで足す): (a) Rust の extractor が宣言のある本体を読まない(宣言の段階 4。ADR-105 の続き。7a 節)。(b) Rust の差分テスト。(c) `rust-web.yaml` をパッケージごとのファイルに分ける。(d) `summary.unverified_versions` を Dart と Rust に出す(課題 #162)。

#### 4. 実装の順と、最初の段の範囲

| 段 | 内容 | 規模(見積もり) |
| --- | --- | --- |
| 1(最初) | **Go だけ。`make model` の枠、候補の一覧(層 1 と層 2)、`rejected:` の読み込みと検証、照合と終了コード 70、`.dead-allow` と `verified_versions`、差分テスト(PR のジョブにも入れる)、PR の本文の材料**。最初の作業は、層 1 の規則の評価(2 節)。型からの flows の生成と、骨組みの生成は含めない(今の `rivus models coverage -yaml` と、手で書いた骨組みを使う) | 中。コマンド 1 つ(Go の小さいプログラム。`bench/model-diff` と同じ形)と、ローダーの検証と、テスト |
| 2 | Go の骨組みの生成(署名、埋め込み、`zoo/mark` の記録)と、セルのテンプレート。**既存の骨組みに `--dep-dir` と `exclude` を足して揃える作業を、1 つの PR で行う** | 中 |
| 3 | Go の型からの flows の生成(4.3a の規則 a〜f。段階 3 の生成器) | 中。`-coverage-gaps` の拡張(第三者のパッケージ)を含む |
| 3 のあと | 課題 #163(ライブラリをプロジェクトとして解析する候補。Go が先) | 中 |
| 4 | 段 1〜3 の Dart 版(下の「言語を横断する順」。骨組み、`zoo_exec_override`、`--dep-dir`、型の取得) | 中〜大 |
| 5 | 段 1〜3 の Rust 版(クラウドでできる段だけ。3 の表。ローカルはサンドボックスの下) | 大(構文だけの署名の取得) |

**段階 1 の状態(2026-11-23): 実装済み(`bench/model-author`、`make model`。Go だけ)。型からの flows の生成、骨組みとセルの生成、Dart、Rust は含めない。差分テストを PR のジョブに入れる部分は、入れた(2026-11-23。`model-diff-changed`。06-03)。CI の `model-diff` は、定期の実行と手動のまま。**

**段階 1(Dart)の状態(2026-11-23): 実装済み(`make model PKG=<パッケージ名> VERSION=<v> LANG=dart`。`bench/model-author/dart*.go`。骨組みの生成は含めない)。** 取得は、空のプロジェクトで `dart pub get` するだけ(`hook/build.dart` か `hook/link.dart` を持つパッケージが 1 つでもあれば、終了コード 10 で止める)。公開 API は、`bench/model-author/dartapi`(Dart の analyzer。型を読むだけで、パッケージのコードは実行しない)が、`lib/` の `src/` の外の各ライブラリの export から読む。シンボルは、extractor と同じ形(宣言したライブラリの `package:x/src/a.dart#Class.member`)なので、行とそのまま照合する。規則の表は Go と同じ(同じ語、同じ形)で、Dart の署名の形に読み替える: 位置は、位置引数が `arg<i>`、名前つき引数が `named:<名前>`、クラスの引数のフィールドは `arg0.owner`(3 段まで)。`dynamic` と `Object` の引数は、名前が決める(dio の `savePath`)。`Map` を返すもの(ヘッダー、メタデータ)は、本文と同じに source の候補。`static` の定数は source の候補にしない。`calls` の候補は、`body_replaced_packages` にあるパッケージだけ(Go と同じ決め。Dart の組み込みのモデルには、今ない)。終了コードは Go と同じ(10: `dart pub get` の失敗かフック、20: API を読めない、差分テストが動かない、50、70)。`-write-dead-allow` は、zoo のテストが Dart の extractor を使うので `RIVUS_DART` が要る(未設定なら 20)。再現率は、`bench/model-author` の `-recall`(`-dart-libs storage/,pubsub/` で、`googleapis` のように大きいパッケージを一部のライブラリに絞れる)。

**段階 2 の状態(2026-11-23): Go の骨組みの生成とセルのテンプレートを実装した(`make model PKG=<パッケージ> VERSION=<v> ARGS="-gen-skeleton"`。`bench/model-author`)。** 入力は、取得した本物のパッケージの公開 API(`go/packages` で型だけ読む。実行しない)と、モデルのファイルの行。出力は、次のとおり(同じ入力なら同じ結果。毎回上書きする)。
- **置き場所(コマンドが決める)**: 第三者のパッケージは `bench/zoo-go-pkg/<モジュールの名前>/`(モジュールの path の最後の要素。major は除く。`go-github/v93` は `go_github`)に、依存の形(独立したモジュール。`skel/` を `replace` で読む)で置く。標準ライブラリは、常に `bench/zoo-go-pkg/<名前(`/` は `_`)>/` に、骨組みなしで置く(`os`、`database_sql` の形)。
- **骨組み(`skel/**/<パッケージ名>.go`。先頭の行は `// Code generated by make model; DO NOT EDIT.`)**: 本物の公開の型(フィールド、埋め込み、メソッド)、関数、定数、変数のうち、**モデルの行が指すもの、その署名に出る型、埋め込みとインターフェースの実装の関係(インターフェースのメソッドと同じ名前のメソッド)、セルと手で書いたファイルが使う名前**だけを、本物の署名のまま書く(パッケージ全体は書かない。go-github で 43,000 行になったため)。型のフィールドは、埋め込み、行とセルが指すもの、ほかの型を引かないものを書く。モデルの行が指すものが骨組みに作れないとき(別の module の型を使う、名前がない)は、警告でなく失敗(終了コード 30)にして、何も書かない。行が指すが流れを再現できないもの(スライスのレシーバ。#164)は、警告に出す。`sqlx.DB` が `*sql.DB` を埋め込む形、インターフェースを満たす関係、型の別名、型パラメータは、本物と同じになる。本体は、sink の行(カテゴリから `mark.SinkSQL`、`SinkCmd`、`SinkHTML`、`SinkPath`、それ以外は `SinkAny`)の危険な引数を記録するだけで、ほかはゼロ値を返す(構造体へのポインタは nil にせず、埋め込みのポインタも入れる)。レシーバから結果への流れの行(`gin.Context` の getter)は、レシーバの `v` に入れたマーカーを返し、レシーバから引数への流れ(`Bind*`)は、引数の文字列のフィールドにマーカーを入れる。`call_result` の source は、ケースのマーカー(`zooV`)を返し、`field` の source が指定するフィールドは、その型を返す関数が、マーカーを入れて作る。本物の module の外の型(第三者の依存)は書けないので、それを使うメンバーは省いて一覧にし(`.claude/work/model-author/<パッケージ>-skeleton-omitted.txt`)、その型の可変長の引数は外す。同じ module の別のパッケージ(`gin/binding`、`sqlx/reflectx`)の型は、同じ規則で `skel/` の下に書く。
- **手で書く部分**: 骨組みの `skel/` に、生成するファイルと違う名前のファイル(`zoo_extra.go`)を置くと、コマンドは触らない。zoo のためだけの関数(`gin.NewContext(tag)`、`github.NewClientFor(tag)`)は、そこに書く。
- **モジュールのファイル**: `go.mod`(セルのモジュール)と `skel/go.mod` は生成する(先頭に同じ行。手で別の内容にしたものは、上書きせず、終了コード 30 で止める)。`rules.yaml` と `cells/` は、なければ作り、あれば触らない。
- **セルのテンプレート**: `cells/` に Go のファイルが 1 つもないときだけ、sink の行のカテゴリごとに `cells/<カテゴリ>/<カテゴリ>.go`(vuln / fixed / decoy。`@source`、`@sink`、`@nosink` のマーク付き)を作る。作ったあと、マークの固定(`ZOO_MARKS_UPDATE=1 go test ./cmd/rivus -run '^TestZooMarksLock$'`)が要る。セルがあれば、一切触らない。
- 終了コード: 30(骨組みかモジュールのファイルを作れない、骨組みがビルドできない、同じ module の別のパッケージが骨組みを持っている)、40(セルのテンプレートを作れない、またはビルドできない)。どちらも、取得だけで、取得したコードは実行しない(骨組みのビルドは、骨組み自身と `zoo/mark` だけ)。
- 再生成の結果: 既存の `sqlx`、`gin`、`storage`、`go_github` の骨組みを、このコマンドの出力に替えた。zoo の結果(`TestZooPkgGo`、マークの固定、`TestBuiltinRowsAreHitByTheZoo`、緩めの検査)は変わらない。本物と違っていた手の骨組みの箇所(`go_github` の `Blob.Content` は、本物は `string` で、手の骨組みは `*string` だった)は、生成で見つかった。

**モデルを確認する対象(ユーザーの方針。2026-11-23)**: 上流が今もサポートしている major(公開の事実で決める。根拠をモデルのファイルの冒頭か PR の本文に 1 行)。既定は最新で、並行してサポートされている major は、`make model` を `VERSION` を替えて複数回実行する。Go の `/v*` の行は、実行した major の具体的なシンボルに直して照合し、片方の major にしかない候補は、もう片方の実行で「消えた候補」の警告にならない(`/v*` で書いた `rejected:` の項目は、警告しない)。`verified_versions` は、ローダーが同じ `package` の重複を拒むので、major ごとに具体的な module の path(`x/v2`、`x/v3`)の記録を並べる。Dart と Rust は、パスに major が入らないので行は共通で、並行サポートのバージョンがあれば、それぞれのバージョンで確認する。

**最初の段の範囲(正確に)**:

- 入るもの: (i) `make model PKG=<Go のモジュール> VERSION=<v> LANG=go`。(ii) `rejected:` の書式の確定と、ローダーでの読み込み、検証(`symbol`、`kind`、`why` の有無、解析には使わない)。(iii) 候補の一覧(層 1 は規則の表から、層 2 は件数と一覧だけ)と、行と `rejected:` との照合。未決があれば、終了コード 70 と、作業ファイルへの一覧。消えた候補の警告。(iv) 層 1 の規則の評価(再現率)。(v) `verified_versions`、`.dead-allow` の書き出し。(vi) 差分テストの実行(`bench/model-diff`)と、PR のジョブへの組み込み。結果を、PR の本文の材料にまとめる。(vii) 終了コードの表。
- 入らないもの: 型からの flows の生成、骨組みの生成、セルの生成、Dart と Rust、マークの固定の自動実行(まず、手順書に 1 行で書く)、既存のモデルの移行。
- 完了の基準: main にある Go のモデル 1 つをコマンドに通すと、未決の一覧が出る。それを `rejected:` に書くと、終了コードが 0 になり、モデルの行は変わらない。

**今ある部分で足りるものと、新しく要るもの**:

| 部分 | 状態 | 使い方 |
| --- | --- | --- |
| `rivus models coverage`(`-yaml`、`-index`) | 足りる | 候補の一覧の、source と呼び出しの候補の元。スタブの形 |
| `bench/model-diff`(`make model-diff`) | 足りる | 段 7。`-changed`、`-changed-reviewed` も |
| `-coverage-gaps` | 一部(Go の標準ライブラリだけ) | 第三者のパッケージと、Dart、Rust への拡張が新規 |
| `zoo_exec_override`、`make zoo-dart-gen` | 足りる(Dart) | 骨組みの `pubspec.yaml` に 1 行。生成した骨組みに入れる |
| `scripts/zoo-loosened-check.sh`(`make zoo-loosened`) | 足りる | 段 9 |
| `TestZooMarksLock`(`ZOO_MARKS_UPDATE=1`) | 足りる | 段 6 |
| `regomark check --no-judge` | 足りる | 段 9 |
| 骨組みの生成器 | **新規** | 公開の署名、埋め込み、実装、記録、`#[track_caller]` |
| 型からの生成器(4.3a) | **新規** | 4.2 の足場の上 |
| 候補の一覧(規則の表) | **新規** | 上の層 1、層 2 |
| オーケストレータ(`make model`) | **新規** | 段の順と終了コード |
| `rejected:` のローダー | **新規** | 読み込みと検証だけ |

**相談役の判断で決まったこと**: 判断はモデルのファイルの中に置く。`models-src/` は作らない。候補は 2 層。段階 1 の候補の元は、ライブラリ単体の型の規則だけ。Rust はクラウドで扱わない。セルの置き場所、`--dep-dir` と `exclude` の扱い、差分テストを PR のジョブに入れる時期は、上のとおり。

**残る確認**: (1) 層 1 の規則の再現率(2 節。段階 1 の最初の作業)。(2) セルの中身を守るマークの形。

