# 手順書: 1 パッケージ分の組み込みモデルを作る

読む人: 前の記憶のない、新しいセッション。**1 パッケージ = 1 つの新しいセッション = 1 つの PR。** 起動の指示文は数行で、「この手順書を読んで、その手順で 1 パッケージ分だけ作り、PR を 1 本立てて終わる」。ユーザーに承認を聞けない。別のセッションが、同じ時間に走っていることがある。上から順に、迷わず進める。

必要なときだけ読むもの: 設計 [DESIGN-package-models.md](DESIGN-package-models.md)(節の一覧。本文は [design-package-models/](design-package-models/) の節ごとのファイル。設計メモの足し方: 節のファイルに足す。新しい節は、新しいファイルと、一覧に 1 行)、モデルの書き方 [MODELING.md](MODELING.md)、候補の一覧 [DART-MODEL-CANDIDATES.md](DART-MODEL-CANDIDATES.md)(Dart。Go と Rust は、同じ種類の一覧があればそれ)。

道具の状態(2026-11-23): `make model`(`bench/model-author`。Go と Dart の段階 1、Go の段階 2)、`make model-diff`(差分テスト。Go と Dart)、一括採用(`adopt_rules:`)、`verified_versions`(major ごとの記録を含む)、パッケージ単位の宣言(`body_replaced_packages`。ADR-100、105)の書式と解析は**実装済み**。**まだない**: 型から flows の行を作るステップ(段階 3)、Dart の骨組みの生成、Rust の `make model` と差分テスト(隔離が要る。9 節)。ないステップは、`rivus models coverage` と zoo のセルと手で書く行で進め、PR の本文に「そのステップは道具がないため手で行った」と書く。**宣言は、いまのモデルの PR では書かない**(2 節の 5)。

## 0. 環境

- `make setup` 済み。`go`、`dart`(3.13.6)、`regomark` が PATH にある。`RIVUS_DART=dart`、`GOMODCACHE`、`PUB_CACHE` は環境変数で設定済み。Dart の extractor は、`rivus setup dart -source extractor/dart` でコンパイルしておくと、起動が速い(README)。
- **クラウドのセッションでは、Rust は、いまは CI に任せる(ローカルは下の表)。** rivus の Rust extractor は、読むときに `build.rs` と proc macro を実行する(隔離の下でだけ許される)。この環境には、Rust の nightly も、Linux の隔離もない(`make setup` は対象外。理由と案: [DESIGN-package-models.md](DESIGN-package-models.md) 6.5)。**隔離が動かないので、Rust のパッケージは、「モデルは書くが、extractor を使う確認は CI に任せる」形にする。取得したクレートのコードを、隔離なしで実行しない**(`cargo check`、`cargo build`、`cargo test`、`rivus-rust`、`-unsafe-host-exec` を、取得したクレートに対して使わない)。モデルの YAML は Go のテスト(`internal/model/builtin`)で確認し、zoo のセルは、自前のスタブ(`path:`、第三者のコードなし)で作って、CI の `rust` ジョブに確認させる。差分テストの Rust 版は、隔離が入るまで「未実施」と書く。CI が失敗したときは、ログを読んで修正して push し直す。手元で試せないので、Go と Dart のパッケージを先に取る。
**取得と実行の範囲(ユーザーが承認した上限。2026-11-23)**: 指示文に書かれていなくても、この範囲が上限になる。これより広いことは、先にユーザーに確認する。クラウドのセッションと、ローカルのサブエージェントは、確認が要る場面では、止まって、理由を書いて終わる。

| 行為 | 認められているか | 条件 |
| --- | --- | --- |
| `docs/MODEL-QUEUE.md` に載っている公開パッケージ(Go のモジュール、Rust の crate、pub のパッケージ)のソースの取得(`go mod download`、`cargo fetch`、`dart pub get`) | 認められている | そのつど聞かない。**取得だけ** |
| 取得したコードの実行(`dart run`、`dart test`、`go run`、`go test` を取得したパッケージに対して行う。取得したコードのビルドスクリプトを動かす) | 認められていない | 例外は次の行だけ |
| 一覧の Rust の crate に対する、rivus の Rust の extractor の実行(ビルドスクリプトとマクロの実行を含む。解析だけ。モデルのシンボルの確認のため) | 認められている | **macOS のサンドボックスの下だけ**([DESIGN-macos-sandbox.md](DESIGN-macos-sandbox.md) 8 節と同じ線) |
| クラウドのセッションでの Rust の extractor の実行 | 認められていない | 隔離(macOS のサンドボックス)がない。Rust のモデルは、当面ローカルで作る。クラウドでは、モデルは書き、extractor を使う確認は、CI かローカルに任せる |
| 一覧にないパッケージの取得、サンドボックスの外での実行、実在のプロジェクトをまるごと解析する長い実行 | 先にユーザーに確認 | 確認が取れるまで、始めない |

**`rivus setup rust` を実行しない。共有の場所(`~/Library/Application Support/rivus/` など、ユーザーの環境)に書かない。extractor を試すときは、作業ツリーの中でビルドして、`RIVUS_RUST` でそのパスを指す。** 理由: 共有の extractor を入れ替えると、インストール済みの rivus を使うほかのセッションの挙動も変わる。

- ブランチは `claude/` で始まる名前になる(セッションごとに決まる)。名前は変えない。
- 新しいセッションは、その時点の `main` から始まる。
- コミットには、GitHub の上で署名が付く。手元の `gpgsig` の確認は要らない。push のあと、API(`gh api repos/<owner>/<repo>/commits/<sha> --jq .commit.verification.verified`)で `true` を確認する。

## 1. 次の 1 つを取る(着手の判定と宣言)

ブランチ名は `claude/…` で自動で決まるので、**ブランチ名では宣言しない**。着手の宣言は、**下書き(ドラフト)の PR を先に立てる**ことで行う。判定は、**開いている PR(ドラフトを含む)とマージ済みの PR の題と、main にあるモデルのファイル**で行う。題の形(固定): `feat(model): <言語> <パッケージ> の組み込みモデル`(例: `feat(model): dart dio の組み込みモデル`)。

1. 候補の一覧の順(先頭から)に、1 つずつ見て、**取られていないもの**を選ぶ。取られている = 次のどれか。
   - `gh pr list --state all --search 'feat(model): <言語> <パッケージ> in:title' --limit 20` に、題が一致する PR がある(開いている、ドラフト、マージ済み)。ただし、**閉じて(マージせずに)あるもの、24 時間以上更新のないドラフト**は、取られていないとみなす。
   - `ls internal/model/builtin/` に、`<言語>-<パッケージ>*.yaml` がある(別の題で入った)。
2. 選んだら、**作業の最初に、ドラフトの PR を先に立てて宣言する**。空に近いコミット(`docs/` や `internal/` を触らない。例: 空のコミット `git commit --allow-empty -m 'chore(model): <言語> <パッケージ> に着手'`)と、上の題と、本文 1 行(「着手の宣言。作業中のドラフト」)で、`/pr` のスキルでドラフトにする。
3. ドラフトを立てたあと、もう一度 `gh pr list --state all --search '<題> in:title'` を見る。**同じ題の PR が、自分のより先(番号が小さい)にあったら、自分のドラフトを閉じて、次の候補を取る。**
4. 作業が終わったら、そのドラフトを、同じ PR のまま、完成の PR にする(コミットを足して push、本文を書き直し、ドラフトを外す)。
5. リポジトリのファイル(候補の一覧、`docs/ISSUES.md` など)に、着手のマークを書かない。

取得してよいもの: 候補の一覧にある公開のパッケージだけ(`go mod download`、`dart pub get`、`cargo fetch` まで)。取得したコードは実行しない(`go run`、取得物の `go test`、`dart run`、ビルドスクリプトなし)。

### 新規と、広げる、の違い

候補の一覧([MODEL-QUEUE.md](MODEL-QUEUE.md))の「新規 / 広げる / 例から昇格」の列で、作業の種類が決まる。

- **新規**: そのパッケージのモデルのファイルが、まだ無い。2 節の標準の 4 ステップで作る。
- **広げる**: 今ある行を残して、足りない API を足す作業。差は次のとおり。
  - 既存の行を変えない。変えるときは、理由と、差分テストでの確認を PR の本文に書く。
  - 既存のセルのマークを緩めない(3 節の zoo のゲートと同じ)。
  - 既存のモデルのファイルの冒頭の注釈を読んで、重複や矛盾を避ける(同じ API を、別のファイルに二重に書かない)。
  - 広げる対象のファイルは、もともと main にあるので、1 節の `ls internal/model/builtin/` の判定は使わない。判定は PR の題だけで行う。
  - PR の題は `feat(model): <言語> <パッケージ> のモデルを広げる`。1 節の判定も、この題の形で `gh pr list` を検索する。

### 例からの昇格

`examples/rules/*.yaml` と `bench/*/rules.yaml` の例を、組み込みに昇格する作業(一覧の「例から昇格」の行)。

- 例のまま移さない。ほかのモデルと同じ確認を通す: 差分テスト(入っていれば)、`result` / `results` の点検、zoo のセル、すでに組み込みにある行との重複の確認。
- 昇格したら、**例のファイルは、リポジトリの中に、ファイルを名指しで使う参照が残る場合は、中身が空のルール(「組み込みになった。`internal/model/builtin/<ファイル>`」のコメントだけ)として残し、参照がなければ消す**([ADR-102](adr/102-promoted-example-empty-rules.md))。決め方: `grep -rn` で、ファイル名と `examples/rules` を、リポジトリ全体(`bench/`、`cmd/`、`internal/`、`extractor/`、`docs/`、`.github/`、`Makefile`)で探し、結果を PR の本文に書く。参照の側を直す必要があるとき(例: `cmd/rivus/notbuiltin.go` とそのテスト)は、その PR でさわってよい。`examples/rules/README.md` の、そのファイルの行は、「組み込みになった(`internal/model/builtin/<ファイル>`)」の 1 行にする(ファイル名で探す利用者のため)。`bench/*/rules.yaml` の例も同じ。リポジトリの直下の README は、昇格の PR ではさわらない(例の一覧は `examples/rules/README.md` を見る、という形にしてある。README に、昇格するファイルを名指しした記述が残っていたら、書き直さず、PR の本文の「手順のとおりにできなかった所」に書く)。
- 例の位置づけ(ADR-063)の上書きは、**[ADR-101](adr/101-promote-example-rules-to-builtin.md) で済んでいる**。昇格の PR は ADR を書かない(ADR-101 を参照するだけ)。
- PR の題は `feat(model): <言語> <パッケージ> の組み込みモデル(例から昇格)`。判定は、この題の形で行う(1 節)。昇格のあと、例のファイルは無いか短くなるので、`ls internal/model/builtin/` の判定は、`<言語>-<パッケージ>*.yaml` で使える。

作業の種類は、一覧の「新規 / 広げる / 例から昇格」の列ではなく、**main にそのパッケージのモデルのファイルがあるか**(`ls internal/model/builtin/<言語>-<パッケージ>*.yaml`)で決める。ファイルがなければ、一覧が「広げる」でも新規として扱う(標準ライブラリの行に乗る薄いラッパーなど)。

## 2. 作る(標準の 4 ステップ)

1. **候補と生成(Go と Dart。`make model`)**: `make model PKG=<パッケージ> VERSION=<v> LANG=<go|dart>` が、公開 API の候補(第 1 層)を、モデルの行、`rejected:`、`deferred:` に照合する(9 節の最後。`MODELING.md` の 4c-1 節)。どれにもない候補があると、終了コード 70 で止まり、`.claude/work/model-author/` に一覧を書く。一覧の項目ごとに、行を足すか、理由(`why`)つきで `rejected:` に足すか、決められないものを `deferred:`(`ref` で課題のファイルを指す)に置く。同じ形の候補が多いときは、`adopt_rules:`(一括採用の決め。`MODELING.md` の 4c-2 節)を書き、`ARGS="-expand"`(Go だけ)で行に展開して `*.adopt.gen.yaml`(コミットする。`origin: generated`)に書く。`ARGS="-recall"` は第 1 層の規則の再現率、`-breakdown` は候補の内訳を出す。**型から flows の行を作るステップ(段階 3)は、まだない**: flows と `calls:` の行は手で書く(8 節)。手で行う場合(Rust は `make model` がない): `rivus models coverage` で未モデルの呼び出しを見て、手で書く。
2. **差分テスト(Go と Dart)**: モデルの行を本体と照合する。MISS は 0。EXTRA は、意図した差だけ、手の部分のファイルの冒頭のコメントに、理由つきで書く(9 節)。`make model ... ARGS="-diff"`(全部の候補が決まったあとに走る)か、`make model-diff PKG=...`。Rust は、隔離が入るまで「未実施」と書く。
3. **確認したバージョンを書く**: モデルのファイルの冒頭に `verified_versions`(形式と検証: [DESIGN-package-models.md](DESIGN-package-models.md) 6.7)。パッケージ(Go: モジュールのパス、Dart: パッケージ名、Rust: クレート名)、確認したバージョンの範囲、実際に確認したバージョン(`checked_with`)、日付(`checked`)。**Go と Dart は、差分テストが通ったとき `ARGS="-diff -write-verified"` が書く**。Rust と、手で書いた行だけのファイルは、手で書く。生成物があれば、先頭の入力のバージョンと同じにする。解析した対象のバージョンが範囲の外のときは、`summary.unverified_versions` に注記が出る(Go は `go.mod`、Dart は `pubspec.lock`、Rust は `Cargo.lock` から。6.10)。
4. **追加**: `<lang>-<package>.yaml`(手の部分)に、sink、source、sanitizer、`calls:`(利用者のコードを呼び返す API: コールバック、利用者が実装するインターフェース。パッケージの行には各行に `checked_by: cell` を書く。実行の確認の道具は標準ライブラリだけを呼ぶ。代わりに、その API を呼び、`@sink` か `@miss` のマークがあるセルが要る。[MODELING.md](MODELING.md) 3 節、ADR-107)を足す。パッケージの行を足したら、**その行を空にして(または消して)セルが落ちること**を確認する。セルが行を使っていることの確認で、実行の確認より弱いのを補う)。**`origin` は、組み込みのファイルの行(propagator、source、sink)には書かない**: 書かなければ、組み込みの行は `builtin` になる(`internal/model/model.go` の `fillOrigins`)。`origin` は「誰がその行を正しいと決めたか」で、`--deterministic-only` が `origin: llm` の行を無視すること、finding の `models` に `origin` が載ること、`llm` の行が経路にあれば `llm_influenced` が立つこと(`internal/taint/engine.go`)にだけ使う。**書くのは、LLM が決めた行だけ**: `origin: llm` に、`decided_at`、`rationale`、`model_version` を付ける(足りないと、ファイルが読めない)。組み込みのファイルに `origin: human` を書くことはできるが、書かなくてよい。zoo に、肯定と否定のセルを足す(Go は、骨組みとセルのテンプレートを `-gen-skeleton` で作れる。下)。`rivus models coverage` で、未モデルの呼び出しを確認する。zoo のセルが当たらないパッケージの行は、`ARGS="-write-dead-allow"` が、モデルのファイルの `.dead-allow` に、理由(`dependency`)つきで足す(4 節の範囲で)。
5. **宣言(`body_replaced_packages`)**: パッケージの本体を読まない宣言は、書式と解析の側は**実装済み**(ADR-100、105。Go、Dart、Rust の段階 2 と 4)だが、**組み込みのモデルには、まだ 1 つも書いていない**。**この手順書は、新しいモデルに宣言を求めない。宣言なしで PR を出す。** 宣言は、次の条件がすべて成り立ったパッケージに、あとから、別の作業として足す([DESIGN-package-models.md](DESIGN-package-models.md) 2 節 5): (i) 差分テストの MISS が 0(EXTRA は意図した差だけ)、(ii) `calls:` の行が、利用者のコードを呼び返す形をすべて表している(または、ないと確認した)、(iii) 照合できなかった行(`unverified`)が少なく、公開の API の範囲が揃っている、(iv) ほかのパッケージから呼ばれる扱いが決まっている。**最初の宣言は、別に計画している**。モデルの PR のチェックリストには、「宣言は、上の条件が揃うまで書かない」を入れる(5 節)。

**source の選び方**: サービスのクライアントは、「その値を誰が書いたか」で選ぶ([ADR-108](adr/108-service-client-source-criterion.md))。迷う値は入れず、モデルのファイルのコメントに、入れなかった理由を書く。

**骨組みは、公開のシグネチャと、記録するだけの本体にする**: rivus は、骨組みの本体も読む(依存として)。interceptor を呼ぶ、検査をするなど、凝った本体にすると、本物と違う流れになり、モデルの確認が、骨組みの形に引きずられる(実例: Dart の `dio` の最初の骨組み)。本体は、引数を記録する(`zoo_mark` の `recordAny` など)か、空にする。骨組みの公開のコンストラクタは、互いを `this(...)` で呼ばない。非公開の 1 つを通す(互いを呼ぶと、その呼び出しが sink 行のサイトになり、zoo に想定外の流れが出る。実例: shelf の `Response`)。

**開発用の依存とぶつかるパッケージの骨組み(Dart)**: `bench/zoo-dart/exec` の `test` が持ってくる本物のパッケージ(`shelf`、`shelf_static`、`shelf_web_socket`、`web_socket_channel`、`async`、`collection`、`http_parser`、`stream_channel`、`path` など)と同じ名前の骨組みは、`bench/zoo-dart/cells/packages/<名前>`(path の依存。本物の依存は書かない)に置き、骨組みの `pubspec.yaml` の末尾に `zoo_exec_override: <本物の範囲>` の 1 行を書く(`exec/pubspec.yaml` の `dependency_overrides` は、これから生成される。`exec/pubspec.yaml` と `cells/pubspec.yaml` は生成物で、コミットしない。`make zoo-dart-gen`)。解析(cells)は骨組みを読み、実行(exec)は、`test` がすでに動かしている本物で動く。cells の `pubspec.yaml` の依存に本物の名前を hosted で書かない(zoo のプログラムに本物の依存が入り、依存の関数を検査するテストが失敗する)。骨組みは、本物の公開のシグネチャに合わせる(セルのコードが、両方で型が合うこと)。

**骨組みの置き場(Go と Rust)**: 外部のパッケージを使うセルは、パッケージごとに独立したモジュールに置く([DESIGN-package-models.md](DESIGN-package-models.md) 6.8 の表)。Go は `bench/zoo-go-pkg/<パッケージ>/`(`go.mod`、`skel/`、`cells/`、`rules.yaml`)、Rust は `bench/zoo-rust-pkg/<パッケージ>/`(`Cargo.toml`、`Cargo.lock`、`skel/`、`cells/src/`、`rules.yaml`)。**テストは、そのディレクトリを探す(一覧はない)ので、ディレクトリを足すだけで、実行、真の注釈の照合、rivus の照合が働く**(合成の `zoo_synth` が、見本)。セルの名前は `F/pkg/<パッケージ(`_` は `-`)>/<セル>`、マークの固定は `bench/zoo/marks.d/<lang>/pkg/<パッケージ>/`。Rust の `Cargo.toml` は、zoo-mark を `path = "../../zoo-rust/zoo-mark"` で指す(テストが、サンドボックスのために、対象に写して書き換える)。 `kind: field` の source を使うモデルのときは、パッケージの `Cargo.toml` の `[workspace]` に `exclude = ["skel"]` を書く(cargo は、ワークスペースの根の下の path の依存をメンバーにし、extractor はメンバーをプログラムとして読む。除かないと、骨組みの中のフィールドの読み出しが source の場所に出て、想定外の流れになる。除くと、骨組みは依存として読まれる)。

**Go の骨組みは、手で書かずに生成する**: `make model PKG=<パッケージ> VERSION=<v> ARGS="-gen-skeleton"`(置き場所、`skel/`、`go.mod`、`rules.yaml`、セルのテンプレートを作る。生成するファイルの先頭は `// Code generated by make model; DO NOT EDIT.` で、毎回上書きする。zoo のためだけの関数は、`skel/zoo_extra.go` のように別の名前のファイルに書く(触らない)。`cells/` にセルがあれば触らない。詳しくは [DESIGN-package-models.md](DESIGN-package-models.md) 6.11 の「段階 2 の状態」)。

**骨組みは、sink の行の関数が呼ばれたら、記録する**(zoo のセルで sink の行を確認するため。記録しない骨組みでは、実行の確認が、sink に届かない)。Go の骨組みは `zoo/mark` を、Rust の骨組みは `zoo-mark` を呼ぶ。記録の場所は、セルの中の、骨組みの関数の呼び出しの行になる(Go は、スタックの中の最初の `/cells/` のフレーム。Rust は、骨組みの関数に `#[track_caller]` を付ける)。最小の例(見本は、`bench/zoo-go-pkg/zoo_synth`、`bench/zoo-rust-pkg/zoo_synth` の `Run` と `run`):
- Go(骨組みの `go.mod` に `require zoo v0.0.0` と `replace zoo => ../../../zoo-go`): `func Run(cmd string) { mark.SinkCmd(cmd) }`
- Rust(骨組みの `Cargo.toml` に `zoo-mark = { path = "../../../zoo-rust/zoo-mark" }`): `#[track_caller] pub fn run(cmd: &str) { zoo_mark::sink_cmd(cmd); }`
sink の行は、パッケージの `rules.yaml` の `sinks:` に、骨組みの関数の記号で書く(`zoo/mark` の sink の記号は、zoo の `rules.yaml` が持つ)。Rust のテストは、パッケージの crate と骨組みの crate の `Cargo.toml` の zoo-mark の path 依存を、写した zoo-mark に書き換える。

**骨組みは、本物の型の埋め込みと、インターフェースの実装の関係を、本物と同じ形にする**(昇格したメソッドが、別のパッケージの行に当たるため。実例: Go の `sqlx.DB` は `*sql.DB` を埋め込む。骨組みが埋め込まないと、昇格したメソッドを呼ぶセルが、本物と違う形で通る)。骨組みの生成器の要件にも足してある([DESIGN-package-models.md](DESIGN-package-models.md) 6.8)。

**Rust のモデルの標準の確認: 本物のクレートでの記号の綴り(macOS のサンドボックスの下だけ)**: 骨組みは本物と綴りが違いうる(定義の経路と公開の経路、別のクレートの型の再エクスポート)ので、行を書いたら、本物のクレートで extractor が書く名前と照合する。**コマンド**: 先に、このリポジトリの extractor を別の場所にビルドする(`cd extractor/rust && CARGO_TARGET_DIR=<場所> cargo build`。共有の extractor と `rivus setup rust` は使わない)。そして `go run ./bench/model-diff -lang rust -pkg <crate> [-version <v>] [-features a,b] [-no-default-features] -rust-extractor <場所>/debug/rivus-rust`(`-version` の既定は、モデルの `checked_with`。`-model <ファイル>` で行のファイルを絞れる。`-out` に一時のディレクトリ)。クレートは、版を固定して取得だけし(`docs/MODEL-QUEUE.md` にあるクレートだけ)、行ごとのプローブを extractor に読ませて、行の 1 つずつを found、spelled-differently(extractor は別の綴りを書く)、resolves(コンパイラは経路を知るが、プローブでは名指しできない)、not-found に分ける。終了コード: 0 全部 found、1 spelled-differently か not-found がある、2 失敗、3 実行しなかった(macOS でない、extractor か cargo がない)、4 resolves だけがある(その行は、手で確認する)。`field` の行は、まだ確認しない。**手で確認するときの代わり(この道具が使えないとき)**: 一時の crate で本物のクレートを `=<版>` で使い、モデルの各行を 1 回ずつ通る関数を書き、`cargo generate-lockfile` のあと `rivus debug extract -lang rust -C <dir>`(サンドボックスの下。`-unsafe-host-exec` は使わない)の IR の `callee.symbol` と型を行と照合する。クラウドの実行には隔離がないので、**Rust のモデルは、当面ローカルで作る**。実例と、骨組みと本物の違い(定義の `id` は、別のクレートの型だと本物は `actix_http::...`、`http::...` になる。呼び出しの `symbol` は、公開の経路で同じ): PR #204(actix-web)。

テスト: `go test ./internal/model/... ./cmd/rivus/ -run 'Builtin|Zoo'`、`regomark check --no-judge`(0)。Dart のセルは、Dart のテスト(`dart analyze` を含む)も通す。手元で実行する最小の組は、次の節。

### 実行するテスト(最小の組。全部のテストは CI に任せる)

**`go test ./cmd/rivus`(全部)は、手元で実行しない。** 全部で 10 分を超える(`TestDogfood` などの重いもの、シャードに分けて CI が実行する)。モデルの YAML、zoo のセル、`.dead-allow`、`@ref` の課題のファイルを足す PR が、手元で通すのは、次の組だけ(`-short` は、PR の CI と同じ)。秒数は、**2026-11-23 の測定**(macOS、同時の重い処理なし)で、測り直していない。CI のランナーでは、これより遅いことがある。

| 確認すること | コマンド | 時間(2026-11-23 の測定) |
|---|---|---|
| モデルの YAML の読み込み、行の整合(`calls:`、標準ライブラリの記号の存在) | `go test -count=1 ./internal/model/...` | 約 2 秒 |
| 行が zoo の一致で使われているか(`.dead-allow` の確認を含む)、マークのファイルと元の一致、承認の行の検査、課題のファイルの形、`models coverage` | `go test -short -count=1 -run 'TestBuiltinRowsAreHitByTheZoo\|TestZooMarksLock\|TestZooLoosenedCheck\|TestIssuesFiles\|TestModels' ./cmd/rivus` | 約 13 秒(`TestZooLoosenedCheck` が 9 秒) |
| Dart のセル(真偽の確認、セルの解析、生成物) | `RIVUS_DART=dart go test -short -count=1 -run 'TestZooTruthDart\|TestZooDart' ./cmd/rivus`(先に `make zoo-dart-gen`) | 約 22 秒 |
| Go のセル | `go test -short -count=1 -run 'TestZooTruthGo\|TestZooGo\|TestZooPureGo' ./cmd/rivus` | 約 5 秒 |
| 差分テスト(行が変わったパッケージ) | `go run ./bench/model-diff -changed origin/main` が出す `<lang> <pkg>` ごとに `go run ./bench/model-diff -lang <lang> -pkg <pkg>`(Dart は `RIVUS_DART=dart`) | Go の標準ライブラリで 1〜2 秒、`package:http` で約 30 秒 |
| 言い回し | `regomark check --no-judge`(0) | 数秒 |
| マークのゲート | `git fetch origin` のあと `scripts/zoo-loosened-check.sh origin/main HEAD`(`make zoo-loosened BASE=origin/main HEAD=HEAD` でも同じ) | 約 7 秒 |

- **重い実行(数 GB の解析。`bench/` の測定の道具)は、`bench/heavylock.py` のロックの下で 1 本ずつ**(同時に 2 つ走らせない)。道具は自分でロックを取る。シェルのコマンドは `python3 bench/heavylock.py -- <command...>`(`bench/README.md` の「One lock for the heavy runs」)。上の組は、重くないので、ロックは要らない。
- 同じ組を、CI も実行する(全部のテストの中で)。手元で通っても、CI が見つける問題(別のテストの `-short` でない側、重いテスト)がある。**push のあと、CI の結果を読んで修正する**(5 節)。

## 3. 既知の見逃しと誤検出は、セルと課題のファイルで残す

PR の本文は、マージのあと誰も見ない。**見逃し(`@miss`)と誤検出(`@fp`)は、セルとして残す。**

1. セルを、新しいセルとして作る(既存のセルのマークは変えない)。
2. 課題を、**番号を取らないファイル名** `docs/issues/model-<パッケージ>-<名前>.md` で作る(GitHub の Issue は作らない)。
3. セルから `@ref(model-<パッケージ>-<名前>)` で結ぶ(新しい `@miss` / `@fp` には `@ref` が必須。ゲートが確認する)。
4. 課題のファイルは、決まったテンプレート(何が見逃されるか / 誤検出されるか、原因、再現するセル)を満たす。テンプレートは [docs/issues/README.md](issues/README.md) を見る(ゲートが確認する)。見本は、既存の `docs/issues/model-*.md`。
5. 課題の一覧(`make docs-gen`)は課題のファイルから作るので、表への反映は要らない。

zoo のゲート(`scripts/zoo-loosened-check.sh`、#167): 新しいセル(`bench/zoo/marks.lock` にまだないファイル)の `@sink(none)`、`@miss`、`@fp` の行の追加は、承認を求めない。`@noisy`、`@na`、`@todo`、`@manual` の新しいセルと、既存のセルへの行の追加は、承認の行が要る。**モデルの PR では作らない。**

## 4. conflict を避ける置き方

1 パッケージ = 1 つのモデルのファイル + 専用の zoo のセルのディレクトリ + 専用のマークのファイル(`bench/zoo/marks.d/`)。`cases.dart` と、zoo-dart の `cells/pubspec.yaml`、`exec/pubspec.yaml` は生成(コミットしない。`make zoo-dart-gen`)、zoo-dart の `pubspec.lock` もコミットしない。共有のファイルをさわらない。**さわってよいファイルは、次のものだけ**:
- `internal/model/builtin/<lang>-<package>*.yaml`(`verified_versions` も、このファイルに書く)と、そのモデルの `.dead-allow`(`internal/model/builtin/<lang>-<package>*.dead-allow`。**さわるのは、「新しい行がどこにも一致しない」という失敗が出たとき(その行を、理由つきで足す)だけ。「zoo から一致するようになったので外せ」の指摘は、無視する**(#187 のあと、通常は記録だけになる。掃除は定期の別の作業で、モデルの PR は、ほかのモデルの `.dead-allow` をさわらない))。**`.dead-allow` の書式は、1 行 1 シンボルの `symbol | 理由`(同じシンボルを 2 回は書けない。ワイルドカードや、まとめの行はない)。zoo のセルが当たらないメソッドで、同じクラスの同じ形のもの(例: 同じ形のオーバーロード、`get`、`post`、`put` のような同じ形の HTTP のメソッド)は、1 行ずつ、同じ理由の文(例: `debt | no cell of the zoo calls it yet`、または `not called: same shape as <当たっているメソッド>`)で書いてよい(書式の拡張は要らない)。1 行のグループにまとめる書式は、いまはない。**
- `bench/zoo-<lang>/` の自分のディレクトリ(セル、骨組み)、外部のパッケージのセルは、`bench/zoo-go-pkg/<パッケージ>/`、`bench/zoo-rust-pkg/<パッケージ>/`(ディレクトリを足すだけでよい。テストも `ci.yml` も足さない)、`bench/zoo/marks.d/<lang>/` の自分のファイル、`docs/issues/model-<パッケージ>-*.md`。
- **広げる PR**(1 節): もともと main にある、そのパッケージのモデルのファイル(と、その `.dead-allow`)。
- **例からの昇格の PR**(1 節): 昇格する `examples/rules/<その 1 つ>`(消すか、空のルールにする。1 節)と、参照の側(洗い出して必要なものだけ)、`examples/rules/README.md` のその行(「組み込みになった」の 1 行に)、`bench/*/rules.yaml` の例(あれば)。
- **例からの昇格の、さわってよい範囲の事実(洗い出し)**: 例のファイルを使っているのは、`bench/compare.sh`(gin: 145、147 行。`gin-vue-admin` と `go-test-bench` のルール)、`bench/parity/collect.sh`(103 行。gin)、`bench/README.md`(gin: 8、158〜160、258 行)、`cmd/rivus/notbuiltin.go` と `notbuiltin_test.go`(actix-web のファイルを名指し。テストが読む)、`bench/parseable/rules.yaml` と `bench/parseable/README.md`(actix-web の写し)、`internal/model/builtin/rust-web.yaml` の 3 行目のコメント、`extractor/rust/README.md` の 108 行。ファイルを消すと、`compare.sh` の gin の 2 つのベンチは、ルールのファイルがなく飛ばされる。**gin は、例のファイルを消さず、中身を空のルール(「組み込みになった」のコメントつき)にして残す案が、使っている所を壊さない**(`compare.sh` は、比べる 2 つのバージョンが、それぞれの木のファイルを読むので、古いバージョンは昔の gin のルール、新しいバージョンは組み込み、になり、昇格の前後の比較の意味が保たれる)。**actix-web は、`notbuiltin.go` の表の行と、そのテストを直す必要がある**(昇格すると、`actix_web::` が組み込みのルールにあるので、知らせは出ない)。この 2 ファイルは、actix-web の昇格の PR で、さわってよい。決める(相談役。ADR-101 の「消す」との関係)まで、gin は、消さずに残す。

- ほかのファイル(`docs/DECISIONS.md`、`docs/ADR-INDEX.md`、`docs/ISSUES.md`、直下の README、コード、ゲート、ワークフロー)は、さわらない。conflict が出たら、main 側の行を残して、自分の行を足し直す。
- **module の path に major が入る Go のパッケージ**(`/v2` 以上。`pgx/v5`、`go-redis/v9`、`go-github/v93` など): major ごとにファイルや行を写さない。シンボル、`field`、`type`、`verified_versions` の `package` に、major の要素を `/v*` と書く(`(*github.com/google/go-github/v*/github.Client).Do`、`package: github.com/google/go-github/v*`。ADR-110、設計 6.9)。`/v*` は `/v` と数字の 1 要素だけに当たる(`/vendor` などには当たらない)。`verified_versions` の `versions` は、**実際に差分テストを通した範囲だけ**を書く(`make model-diff PKG=github.com/google/go-github/v93/github` に `-version v93.0.0`: `/v*` の行は、`-version` の major の package として照合される)。zoo のセルの骨組みは、読んだ major 1 つ分のモジュールで足りる(`replace github.com/google/go-github/v93 => ./skel`)。確認する対象は、上流が今もサポートしている major(既定は最新。複数をサポートしていれば、それぞれで差分テストを通し、`verified_versions` に並べる。その major にしかない API の行も書く)。「サポートしている」は、上流の公開の事実(サポートの方針、README、リリースのブランチ、直近のリリース)で決め、根拠をモデルのファイルの冒頭か PR の本文に 1 行で書く(分からないときは最新だけにして、そう書く)。サポートを終えた major は対象にしない(行は当たり、確認していないことが `summary.unverified_versions` に出る)。`aws-sdk-go-v2` のように、`/vN` の要素でない名前は対象外(そのままの path で書く)。接尾辞のない v0 / v1 の path には `/v*` は当たらない: v1 と v2 以降の両方を対象にするパッケージは、接尾辞のない行と `/v*` の行の両方が要る。

### Rust の zoo のセルを足すとき(確認と骨組みの注意)

- **`RIVUS_RUST` を設定しないと、`TestZooPkgRust` は SKIP のまま通る**(`go test` は `ok`。SKIP は `-v` でしか見えない)。Rust のパッケージのセルを足したら、必ず `RIVUS_RUST=<rivus-rust のバイナリ>` と `RIVUS_REQUIRE_RUST=1` を付けて実行する(`RIVUS_REQUIRE_RUST=1` なら、未設定は SKIP でなく失敗になる。CI の `rust` ジョブは、これを設定してあるので、SKIP は失敗になる)。`bench/zoo-rust-pkg/` にパッケージがあって `RIVUS_RUST` がないときは、`-v` で「NOT checked」と出る。実行したら、`found N / N`、`@miss 0` の行を PR の本文に書く。行を使っているかは、sink の行を全部消して、vuln が見つからずに落ちることで確かめる。
- **骨組みの `#[track_caller]`**: 記録の場所をセルの行にするには、sink を記録する関数だけでなく、**呼び出しの連鎖のすべて**(骨組みの公開の関数、それが呼ぶ内部の記録の関数)に付ける。1 つでも欠けると、記録の場所が骨組みの中になる。
- **`fixed` のセルは、`clean_cmd`、`clean_sql`、`clean_html`、`clean_path` の 4 つを通す**: `zoo_mark::sink_any` は、全部の class で判断するので、1 つだけでは「清められていない」と判断される。例: `m::clean_path(&m::clean_html(&m::clean_sql(&m::clean_cmd(&m::src("..")))))`。
**4 節の補足(骨組みの記録、ロックの生成、Rust の型の綴り)**:
- **Go の骨組みが、名前の sink(`resource_name` など)で記録するもの**: `zoo/mark` の `mark.SinkCmd` で、その引数(名前)を記録する(fixed のセルでは、`mark.CleanCmd` で消した値が入るので、記録が空になる)。骨組みの本体は「公開のシグネチャ + 記録するだけ」で、それ以外はしない。見本は `bench/zoo-go-pkg/zoo_synth/skel/zoosynth.go` の `Run`(`func Run(cmd string) { mark.SinkCmd(cmd) }`)と、`bench/zoo-go-pkg/gin/skel/gin.go`(`mark.Src` を呼ぶ source の側)。
- **`bench/zoo/marks.d/` のロックの生成**: 新しいセルを足したら、`ZOO_MARKS_UPDATE=1 go test ./cmd/rivus -run '^TestZooMarksLock$'`(`bench/zoo/README.md`、`cmd/rivus/zoo_marks_test.go`)。新しいセルの `@sink(none)`、`@miss`、`@fp` の行は、承認なしで通る(`@miss` と `@fp` は `@ref` が要る。3 節)。既存のセルの行は変えない。
- **Rust の型の綴りは、extractor が書く名前で確認する**: 実例: axum の `axum::extract::Request` は、IR では `axum::http::Request` と書かれる。綴りを確認せずに `param_type` の `type:` を書くと、source が当たらない。

## 5. PR(完成)

- 題: `feat(model): <言語> <パッケージ> の組み込みモデル`(1 節の題と同じ)。`/pr` のスキルで作る(1 節のドラフトを更新する)。
- **本文の形は、`pr` スキルの形**(`## Summary` と `## Background / Motivation` の 2 つだけ。ほかの `##` の見出しは作らない)。下の必須の項目は、`## Summary` の下に、`###` の小見出しか箇条書きで書く。この節の「本文の最後」は、`## Summary` の最後の意味。理由と背景(なぜこのパッケージか、判断の理由)は、`## Background / Motivation` に書く。
- 本文は、公開の事実だけ(パッケージ名とバージョン、公開の API)。**次の項目を、必ず、この順で書く**(ないものは「なし」、できなかったものは「未実施」と理由を書く。空にしない):
  1. **モデルの行の数と種類**: sink / source / sanitizer / flows / pure / calls の、それぞれの数。生成した行(`*.adopt.gen.yaml`)と手で書いた行の内訳。
  2. **差分テストの結果**: MISS / EXTRA / 意図した差(理由つき)。Rust は、道具がないので「未実施(Rust の差分テストは隔離が入るまでない)」と書く。
  3. **zoo のセル**: vuln / fixed / decoy の数。
  4. **既知の見逃しと課題**: `@miss` / `@fp` のセルと、`docs/issues/model-<パッケージ>-<名前>.md` の一覧(なければ「なし」)。
  5. **公開のベンチでの前後の数字**: finding(件数と ID の集合が同じか)、`dynamic_dispatch` の注記の数、`summary_elems`、時間(`-pass-stats`)。中を読んだ場合(モデルなし)との照合と、違いの理由。公開のベンチに、そのパッケージを使うものがなければ、「該当するベンチなし」と書く。
  6. **確認したバージョン**: `verified_versions` に書いた、パッケージ、範囲、確認したバージョン。宣言のチェック: **宣言(`body_replaced_packages`)は書いていない**(2 節の 5 の条件が揃うまで書かない)。条件の成否(MISS 0 か、`calls:` の行、未検証の行の割合)も 1 行で添える。
  7. **手順のとおりにできなかった所**: 手順書が曖昧だった所、道具がなかった所、自分で判断した所。なければ「なし」。手順書の改善の入力になる。**`docs/MODEL-QUEUE.md` の案(境界の案)と違う判断をしたとき(source にしない、sink を足す・外すなど)は、ここに、理由を書く。**
- **最初の 3 本(言語ごとに 1 本)は、相談役が、中身を確認する。** そのために、上の 7 に加えて、**手順の改善案**(次のセッションの手順書に足したいこと)を、`## Summary` の最後に書く。
- 触るファイルは 4 節のとおり(広げる PR と、例からの昇格の PR の分も、4 節に書いてある)。それ以外(コード、ゲート、ワークフロー、共有の文書)は触らない。触る必要が出たら、完成にせず、ドラフトのまま、理由を本文に書いて終わる。
- push のあと、コミットの署名(0 節)と、CI の結果を確認する。CI が失敗したら、直して push し直す(自分で `gh pr merge` はしない)。

## 6. 守ること

- 公開の事実だけ。特定の利用者・プロジェクトを書かない。
- GitHub の Issue を作らない・閉じない・編集しない(課題は `docs/issues/` のファイル。3 節)。
- 自分でマージしない(CI が通れば自動で入る)。履歴は書き換えない。
- 既存のセルのマークを緩めない(既存の行の変更・削除は、ユーザーの承認が要るので、しない)。
- 言い回しは `.regomark/policy/wording.rego` に従う(`regomark check --no-judge` が 0)。
- ADR を書かない(昇格は、[ADR-101](adr/101-promote-example-rules-to-builtin.md) で済んでいる)。方針の変更が要ると思ったら、PR の本文に書く。
- 1 つの PR で、1 パッケージだけ。終わったら、次のパッケージには進まず、終わる。
- `.dead-allow` の書式: 1 行 1 シンボルで、`symbol | 理由`(`readDeadAllow`)。既存のファイルは `symbol | 種類 | 理由` の 3 列だが、テストは `|` の最初の区切りだけで分けるので、「種類 | 理由」の全体が理由の文字列として読まれる(2 列でも 3 列でも読める。種類は検査されない)。種類の意味は `bench/parity/README.md`: `dependency`(標準でないライブラリで、zoo にその依存がない)、`debt`(標準ライブラリで、まだ呼ぶセルがない。**減る一方で、新しい行には足さない**)、`spelling`(extractor が書く別の綴り)。新しいパッケージの行は、`symbol | dependency | 理由` か、`symbol | 理由` の形で足す(`debt` は、標準ライブラリのモデルの、いまある行だけ)。

## 7. 更新の PR(案。定期の照合が入ってから)

ライブラリが更新されて、モデルが古くなったとき(設計 6.7)の PR。1 回の実行 = 1 パッケージ = 1 つの PR。
- 題: `feat(model): <言語> <パッケージ> のモデルを更新する(<旧バージョン> → <新バージョン>)`。着手の判定は、1 節と同じ(題の一致)。
- 取るもの: `verified_versions` の `checked` が古いパッケージ、または、定期の照合が差分を報告したもの。
- 作業: 生成し直す(増えた API、消えた API、シグネチャが変わった API の一覧を、本文に書く)、差分テストを実行し直す、`calls:` の見直し(コールバックが増えていないか)、zoo の骨組みのシグネチャの更新。最後に、`verified_versions` の範囲、`checked_with`、`checked` を更新する。
- 本文: 5 節の項目に加えて、API の差分の一覧。

## 8. 生成と差分テストの対象外(手で書く / 未検証と書く)

次は、機械で作れないので、**手で書く行にして、PR の本文に「手で書いた」「未検証」と書く**(設計 6.7)。
- コールバックを取る関数(`calls:` の行は、手で書く)。
- ジェネリクスの関数(差分テストのコードを機械的に作れない。「未検証」と書く)。
- 差分テストで照合できなかった生成の行(複雑な型の引数など)は、行に `unverified: true` を付けたまま残す(消さない。PR の本文の「差分テストの結果」に、照合済みの行の割合を書く。設計 4.3a の e)。
- 生成した行の `origin` は `generated`(生成器のバージョンつき)。手で書いた行は、`origin` を書かない(LLM が決めた行だけ `llm`。2 節の 4)。設計 4.3a の f。


## 9. 差分テスト(`bench/model-diff`。実装済み。2 節の 2 の道具)

モデルの行を、依存の本体と照合する。**Go と Dart。Rust は、綴りの確認だけ(ローカル。上の「Rust のモデルの標準の確認」)。解析 2 回の比較は対象外**(クレートを読むには、`build.rs` と proc macro を動かすので、隔離が要る。上の 0 節)。

```sh
make model-diff PKG=strings                                   # Go の標準ライブラリ。行は、組み込みのモデルのすべてのファイルから取る
make model-diff PKG=package:http MODEL=internal/model/builtin/dart-http.yaml   # Dart。MODEL は、その 1 ファイルの行だけ
go run ./bench/model-diff -pkg google.golang.org/grpc -model mine.yaml -version v1.80.0   # -rules で渡すファイル、バージョンの指定
```

- **やること**: モデルの行にある関数を、source の値で、位置(`recv`、`arg0`、...)ごとに 1 つずつ呼ぶコードを作り、出てくるもの(戻り値ごと、書き込まれうる引数、レシーバ)を、**関数と場所ごとに別の sink** に渡す。同じコードを、(A) そのパッケージの行を外した組み込みモデルで、本体を読んで解析し、(B) モデルで解析して、比べる。出力は、表と、JSON(`-json`。既定は `<out>/report.json`)。
- **結果の種類**: `agree`(一致。`agree_no_flow` は、A も B も流れがなく、その位置について何も言えなかった数)、**`MISS`**(A にあって B にない流れ。モデルが落とした)、**`EXTRA`**(B にあって A にない流れ。モデルが広い)、`EXTRA-intended`(意図した差。下の注釈)、`unverified`(未検証。理由つき)。
- **終了コード**: MISS があれば 1。**行のシンボルがパッケージにない(関数、メソッド、型、フィールド)、行が指定する位置(`arg3`、`result2`、`recv`)が関数にない、も 1**(綴りの誤りと、新しいバージョンで消えた関数が、通らないようにする。Go)。EXTRA は、注釈のないものがあれば 1。注釈に理由がない、sink の行が存在しない位置を宣言している、生成したコードがビルドできない、も 1。**MISS は、原則、注釈でも許さない**(落ちた流れは、モデルを直す)。例外は、本体を読んで、データの流れではないと確認した 1 つの位置だけ(下の `miss-reviewed`)。
- **行がない関数の一覧: `-coverage-gaps`**: 上のプローブは、モデルの行にある関数だけを対象にするので、**行がない関数は原理的に見つからない**(`database/sql.Conn` の sink の行が足りなかったのはこの形)。`go run ./bench/model-diff -coverage-gaps -pkg database/sql` は、Go の標準ライブラリのパッケージについて、文字列・バイト列・`any` を受ける公開の関数とメソッドのうち、組み込みのモデルに行がないものを一覧にする(情報だけ。終了コードは 0)。型の単位で、SQL・コマンド・パスを受ける関数に行があるかを見るときに使う。
- **`miss-reviewed` の扱い**: 差分テストの出力は、毎回、`miss-reviewed` の件数と一覧(`miss-reviewed: N` と、1 行ずつ)を出す(JSON は `miss_reviewed`)。**モデルを作るセッション(クラウドのルーチン、ローカルの検証のサブエージェント)は `miss-reviewed` を足さない**。必要だと思ったら、下書きのまま、理由を書いて終わる(足すかどうかは、相談役が決める)。PR で増えた注釈は、`go run ./bench/model-diff -changed-reviewed origin/main` が、`miss-reviewed-added <ファイル> <symbol>@<位置> <種類>` の行で出す(ジョブは、これを失敗でなく目立つ注意として表示する。`ci-ok` は止めない)。
- **意図した差の置き方**: モデルの YAML の、その行の近くに、コメントで書く。理由は必須(ない注釈は失敗)。注釈は、コメントの行を読む(`internal/model` のパーサではない)。

  ```yaml
  # model-diff: extra <symbol> <理由>        # その関数の EXTRA は、意図した差
  # model-diff: miss-reviewed <symbol>@<位置> <種類> <短い説明>   # その 1 つの位置の MISS は、本体を調べて確認した、モデルに足さないもの。種類は下の表の 3 つ。種類のない理由は失敗。MISS-reviewed と表示され、失敗にしない。関数全体には付けない
  | 種類 | 意味 | 使える条件 | 必須の記述 |
  | --- | --- | --- | --- |
  | `type-only` | 値でなく、値の型の情報だけが出力に届く | エラーの文が `%T` や `reflect.TypeOf` で型名を入れるだけ | 短い説明 |
  | `non-payload` | 本体の流れは実在するが、届く値が、決まった狭い形に絞られる | 数値、bool、列挙に近い値、文字の種類と長さが固定で短い値(例: ゾーン名) | 何に絞られるか、と、標準ライブラリのソースの場所 `src=<ファイル>.go:<関数>`([MODELING.md](MODELING.md) の「Stopping at a number, and a value of a narrow shape」) |
  | `body-default` | 本体を分析する側の、未解決の呼び出しなどの既定の近似による見かけの流れ | 本体を調べて、データの流れがないと確認した | 直す課題の番号 `#N` |
  # model-diff: value <型> <Dart の式>       # Dart: その型の値(レシーバ、引数)の作り方。$S が source の位置。`<型>.<メンバ>` は、そのメンバのレシーバだけ
  ```

  `extra` に当たる EXTRA がなくなると、「注釈が要らなくなった」と出る(失敗はしない)。例は `internal/model/builtin/dart-http.yaml`。
- **`pure`、sanitizer、source の行(Go)**: 流れの行だけでなく、この 3 種類も読む。**確認した行と、確認できなかった行は、数を分けて出す(次の「確認した行の数」)。確認できなかった行を「一致」に数えることはしない。**
  - `pure: true` の行: 流れの行と同じプローブ(位置ごとに source を入れ、戻り値、書き込まれる引数、レシーバを見る)を、本体を読む側(A)で実行する。**A に流れがあれば MISS**(pure は、モデルが流れも注記も消す行なので、本体が流すものを 1 つでも消せば誤り)。コールバックを取る関数は、pure の行だけ、コールバックの関数リテラルを作って渡し、本体がコールバックに渡す値(`cb<位置>_<引数>`)を見る(`filepath.Walk` が root を渡す形)。流れの行のコールバックは、これまでどおり未検証(`calls:` の行は手で書く)。
  - sanitizer の行: **本体は、「ラベルを落とす」という約束を確かめられない**(本体を読む解析にはラベルを落とす考えがない)。決めたこと: (1) 本体で、値が結果(`flows:` があればその行き先)に届くことを確認する。届かない本体(定数を返す、引数を無視する)は、落とすものがないので、**確認しない行**として数える(失敗にはしない)。(2) `flows:` を書いた行は、**流れの行と同じに、本体と同じ流れでなければならない**。本体にあって行にないもの(`arg1` を運ぶのに `arg0` だけ書いた)は MISS、行にあって本体にないものは EXTRA(`extra` の注釈で許せる)。`flows:` のない行は、既定(すべての引数が `result0` へ)なので、`result0` に届くかだけを見る。(3) `drops` と `adds` の意味(どのラベルをどう落とすか)は、この検査では見ない。
  - source の行: 生成したコードは source を呼ばない(source は流れの始まり)ので、**パッケージを調べる**。`call_result` / `param` / `callback_param`: シンボルがパッケージにあり、`params`、`out`、`operand` の位置が関数にあり、`out` の引数が書き込める型(ポインタ、スライス、map、`any`)で、`recv` ならメソッド。`field`: 型と、そのフィールド(`*` は任意)。`param_type`: 型。通れば確認した行、通らなければ失敗。結果が数値か bool だけの関数の source は、データを持てないので、確認しない行として数える。ほかの種類(`marker` など)は読まない(確認しない行)。
  - source を入れられない型の引数(構造体のポインタ、`context.Context`、メソッドを持つインターフェースなど)は、**何も報告せずに飛ばさず、位置つきで未検証に出す**(`type that the source cannot be put into: <型>`)。数値、bool、関数、`io.Writer` は、データを持たない(または書き込み先)ので、出さない。`context.Context` に source を入れる(`WithContext` が値を結果に運ぶかを見る)ことは、まだしない。
  - **「どちらの実行にも流れがない」は、確認した行に数えない**: そのプローブでは、本体を読む側(A)にもモデルの側(B)にも、その位置の流れが出ていない。出ていない流れが一致しているだけで、行について何も言えない(行が間違って「流れなし」でも、正しくても、同じ結果になる)。この行は、確認していない行として、理由 `no flow in either run at any position that was tried` つきで出し、`agree_no_flow` の数にも出る。A か B のどちらかに流れが出た位置が 1 つあれば、確認した行になる。ただし、`pure` の行は、結果か書き込まれる引数かレシーバがあれば、見る場所があるので、流れがなくても確認した行に数える(本体が何も運ばないことを、その場所で見た)。
  - **構造体の引数とレシーバ(Go。2026-11-23)**: エクスポートされたフィールドに文字を持てる構造体(`Path string`、`Args []string`、`Keys map[string]any` など)は、**そのフィールドすべてに source を入れて**確認する(1 階層だけ。ポインタと構造体のフィールドはたどらない。流れの行、pure、sanitizer の行。sink の行は、位置をそのままにする)。モデルは値を 1 つのものとして扱うので、どのフィールドを本体が読んでも流れが出るように、まとめて入れる。**入れられなかった部分(エクスポートされていないフィールド、ポインタ)を本体が読むことがあるので**、このプローブは、本体にあって行にない流れは今までどおり MISS、**行にだけある流れは EXTRA にせず、その行の確認にも数えない**(`partial`)。文字を持てるフィールドがない構造体(ロック、時刻、データベースや接続のハンドル)には、今までどおり入れられない。
- **確認した行の数**: 出力の `rows: N judged / M not judged (flow 20/3, pure 5/1, ...)` は、パッケージの行(種類ごと)のうち、**本当に確認した行と、していない行**の数。確認した行 = 流れ、pure、sanitizer の行は、何かを示したプローブが 1 つ以上ある(流れの行は、A か B のどちらかに流れが出た。pure の行は、見る場所がある。sink の行は、宣言との照合がある)、source の行は、上の調べが通った。確認していない行は、理由つきで 1 行ずつ出る(`not judged ...`。JSON は `rows`、数は `rows_judged` と `rows_not_judged`)。**「未検証」は位置か関数の単位、「確認していない行」は行の単位**で、1 つの行は、一部の位置が未検証でも、ほかの位置が何かを示せば、確認した行になる。Dart は、流れの行と sink の行だけを見て、pure の行は確認していない行として出す。
- **`.dead-allow` の数**: `go run ./bench/model-diff -run-changed 4b825dc642cb6eb9a060e54bf8d69288fbee4904 -out <dir>` は、空の木を基準にして、すべての Go のパッケージを回す(時間がかかる)。`go run ./bench/model-diff -dead-allow <dir>` は、その `report*.json` を読み、`.dead-allow` の行のうち、差分テストが確認した行、パッケージは回したが確認していない行、報告がない(回していない)行の数を、ファイルごとに出す。
- **sink の行は、本体ではなく宣言と照合する**(依存の中の sink は finding にならない。ADR-099)。位置ごとに 1 つだけ source にした呼び出しで、宣言した位置では finding が出て、宣言していない位置と、定数だけの対照では出ないことを確認する。`parts` / `field` は、1 階層のフィールドだけ。
- **未検証として出し、何も報告せずに飛ばさないもの**: ジェネリクスの関数、コールバックを取る関数、作れない型の引数、実装のないインターフェースのメソッド、source を入れられないレシーバ(`new` で作れる値と `New...` の関数で作れない構造体のメソッドなど。Dart は `value` の注釈がないクラス)、`named` / `when_arg_type` / `match` のある sink の行。PR の本文の「差分テストの結果」に、MISS / EXTRA / 未検証の数と、確認した行 / 確認していない行の数を書く(5 節)。
- **Go の標準ライブラリ**: rivus は標準ライブラリの本体を読まないので、A は、パッケージと、それが import する `internal/...` を、一時のモジュールの中に写して読む(アセンブリは写さない。本体のない関数は、本体のない関数として読まれる)。写せないパッケージの flows の行は、未検証になる(sink の行は照合できる)。
- **バージョン**: `-version` がなければ、Go は、このリポジトリの `go.mod` にあるバージョン(なければ最新)、Dart は、`bench/` などの `pubspec.lock` にあるバージョン(なければ `any`)。標準ライブラリは、いまの Go のバージョン。`verified_versions` への記録は、別の作業。
- **取得と実行**: 依存の取得(`go get`、`dart pub get`)だけで、取得したコードは実行しない(Go は型検査と解析まで、Dart は analyzer まで)。ライブラリごとに分けるので、1 回は数秒〜十数秒、依存込みで 1 GB 前後。
- **PR で動かす対象の求め方**: `go run ./bench/model-diff -changed origin/main` が、`internal/model/builtin/` の行(flows、pure、sinks、sanitizers、sources)が変わったパッケージだけを、`go <pkg>` / `dart <pkg>` の形で出す。**PR のジョブ `model-diff-changed`**(`ci.yml`)は、モデルのファイル(`internal/model/builtin/*.yaml`)が変わった PR だけで動き、`make model-diff-changed BASE=<sha>`(`-run-changed`。手元では `BASE=origin/main`)で、その Go のパッケージを 1 つずつ差分テストにかけて、最後に 1 つの表(`ok` / `FAIL` / `ERROR` / `not run` / `nothing`)を出す。`not run` は、Go 以外のパッケージ(今は Dart。ジョブに Dart の SDK がない)で、**確認していない**という意味。`nothing` は、行が全部消えたパッケージ。`/v*` のパッケージは、`verified_versions` の `checked_with` のバージョンで回す(ないと失敗)。**失敗にするのは、PR が変えた行の MISS、EXTRA、エラーだけ**(`-only`)。変えていない行の指摘は、`pre-existing` の注意として出し(表に件数)、失敗にしない。`body_replaced_packages` の宣言を足した、または変えたパッケージは、絞らず、すべての指摘で失敗にする(宣言したパッケージは、流れの行だけに頼るので、先にきれいにする)。増えた `miss-reviewed` は、`miss-reviewed-added ...` の行で先に出る(失敗にしない)。CI の定期と手動の層の `model-diff`(決まったパッケージの一覧)は、そのまま残る。

**コマンド(段階 1。Go と Dart)**: `make model PKG=<Go のパッケージ | Dart のパッケージ名> VERSION=<v> LANG=<go|dart>`(`bench/model-author`)は、公開 API の候補(第 1 層)を、モデルの行と `rejected:`(`docs/MODELING.md` の 4c-1 節)に照合する。どちらにもない候補があると、終了コード 70 で止まり、`.claude/work/model-author/` に一覧を書く(一覧の各項目に、行を足すか、理由(`why`)を付けて `rejected:` に足す)。`ARGS="-diff -write-verified"` を付けると、全部が決まったあとに、この節の差分テストを走らせ、通れば `verified_versions` を書く。`ARGS="-recall"` は、第 1 層の規則の再現率を出す。`ARGS="-breakdown"` は、候補を種類、規則、語、型の形で数えた表を出す。`ARGS="-expand"` は、モデルの `adopt_rules:`(一括採用の決め。`MODELING.md` の 4c-2 節)を行に展開して `*.adopt.gen.yaml` に書く(Go だけ。`-out <dir>` で別の場所に書く。`covers` が増えると終了コード 70)。決められない候補は、モデルの `deferred:`(`ref` で課題のファイルを指す。`MODELING.md` の 4c-1 節)に置く。終了コードの表は、`bench/model-author/main.go`。取得だけで、取得したコードは実行しない(Dart は `dart pub get` と、analyzer による型の読み取りだけ。`hook/build.dart` を持つパッケージは、終了コード 10 で止める。Dart の注意は `DESIGN-package-models.md` 6.11 の「段階 1(Dart)の状態」)。**対象は、上流が今もサポートしている major(公開の事実で決める。根拠は、モデルのファイルの冒頭か PR の本文に 1 行)。** 既定は最新。複数の major が並行してサポートされているパッケージは、それぞれが対象で、`VERSION` を替えて複数回実行する(`PKG` に `/v*` を書くときは、`VERSION` の major を調べる。`verified_versions` は、major ごとに具体的な module の path の記録を足す)。Dart と Rust はパスに major が入らないので行は共通で、並行サポートのバージョンがあれば、それぞれのバージョンで確認する。本体を読むパッケージ(`body_replaced_packages` にもなく、標準ライブラリでもない)では、`calls` の候補は出さない(解析が関数の値とインターフェースの呼び出しを解決する。`calls:` の行は、本体を読まない関数のため)。

## 10. この手順書を「使える」と言う条件

手順書の種類(1 節の「新規」「広げる」「例からの昇格」)ごとに、**ローカルのサブエージェントが、この手順書だけを見て、1 回通す(着手の判定から、PR の本文まで。push はしない)**。通せたら、その種類は「使える」。通せなかった所(読み違えた所、さわってはいけないファイルをさわった所、書いていなくて聞いた所)は、手順書に書き足してから、もう 1 回通す。3 種類とも、1 回ずつ通るまで、クラウドの定期の実行(`rivus-model-author`)は有効にしない。
