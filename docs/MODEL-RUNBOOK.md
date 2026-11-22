# 手順書: 1 パッケージ分の組み込みモデルを作る

読む人: 前の記憶のない、新しいセッション。**1 パッケージ = 1 つの新しいセッション = 1 つの PR。** 起動の指示文は数行で、「この手順書を読んで、その手順で 1 パッケージ分だけ作り、PR を 1 本立てて終わる」。ユーザーに承認を聞けない。別のセッションが、同じ時間に走っていることがある。上から順に、迷わず進める。

必要なときだけ読むもの: 設計 [DESIGN-package-models.md](DESIGN-package-models.md)、モデルの書き方 [MODELING.md](MODELING.md)、候補の一覧 [DART-MODEL-CANDIDATES.md](DART-MODEL-CANDIDATES.md)(Dart。Go と Rust は、同じ種類の一覧があればそれ)。

道具の状態: 差分テスト、生成器、宣言(`unread_packages`)は、段階が入るまで **(未実装)**。未実装の段は、ない道具の代わりに `rivus models coverage` と zoo のセルで進め、PR の本文に「その段は未実装のため省いた」と書く。

## 0. 環境

- `make setup` 済み。`go`、`dart`(3.13.6)、`regomark` が PATH にある。`RIVUS_DART=dart`、`GOMODCACHE`、`PUB_CACHE` は環境変数で設定済み。
- **クラウドのセッションでは、Rust は、いまは CI に任せる(ローカルは下の表)。** rivus の Rust extractor は、読むときに `build.rs` と proc macro を実行する(隔離の下でだけ許される)。この環境には、Rust の nightly も、Linux の隔離もない(`make setup` は対象外。理由と案: [DESIGN-package-models.md](DESIGN-package-models.md) 6.5)。**隔離が動かないので、Rust のパッケージは、「モデルは書くが、extractor を使う確認は CI に任せる」形にする。取得したクレートのコードを、隔離なしで実行しない**(`cargo check`、`cargo build`、`cargo test`、`rivus-rust`、`-unsafe-host-exec` を、取得したクレートに対して使わない)。モデルの YAML は Go のテスト(`internal/model/builtin`)で確認し、zoo のセルは、自前のスタブ(`path:`、第三者のコードなし)で作って、CI の `rust` ジョブに確認させる。差分テストの Rust 版は、隔離が入るまで「未実施」と書く。CI が失敗したときは、ログを読んで修正して push し直す。手元で試せないので、Go と Dart のパッケージを先に取る。
**取得と実行の範囲(ユーザーが承認した上限。2026-11-23)**: 指示文に書かれていなくても、この範囲が上限になる。これより広いことは、先にユーザーに確認する。クラウドのセッションと、ローカルのサブエージェントは、確認が要る場面では、止まって、理由を書いて終わる。

| 行為 | 認められているか | 条件 |
| --- | --- | --- |
| `docs/MODEL-QUEUE.md` に載っている公開パッケージ(Go のモジュール、Rust の crate、pub のパッケージ)のソースの取得(`go mod download`、`cargo fetch`、`dart pub get`) | 認められている | そのつど聞かない。**取得だけ** |
| 取得したコードの実行(`dart run`、`dart test`、`go run`、`go test` を取得したパッケージに対して行う。取得したコードのビルドスクリプトを動かす) | 認められていない | 例外は次の行だけ |
| 一覧の Rust の crate に対する、rivus の Rust の extractor の実行(ビルドスクリプトとマクロの実行を含む。解析だけ。モデルのシンボルの確認のため) | 認められている | **macOS のサンドボックスの下だけ**([DESIGN-macos-sandbox.md](DESIGN-macos-sandbox.md) 8 節と同じ線) |
| クラウドのセッションでの Rust の extractor の実行 | 認められていない | 隔離(macOS のサンドボックス)がない。Rust のモデルは、当面ローカルで作る。クラウドでは、モデルは書き、extractor を使う確認は、CI かローカルに任せる |
| 一覧にないパッケージの取得、サンドボックスの外での実行、実在のプロジェクトをまるごと解析する長い実行 | 先にユーザーに確認 | 確認が取れるまで、始めない |

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

- **新規**: そのパッケージのモデルのファイルが、まだ無い。2 節の標準の 4 段で作る。
- **広げる**: 今ある行を残して、足りない API を足す作業。差は次のとおり。
  - 既存の行を変えない。変えるときは、理由と、差分テストでの確認を PR の本文に書く。
  - 既存のセルのマークを緩めない(3 節の zoo のゲートと同じ)。
  - 既存のモデルのファイルの冒頭の注釈を読んで、重複や矛盾を避ける(同じ API を、別のファイルに二重に書かない)。
  - 広げる対象のファイルは、もともと main にあるので、1 節の `ls internal/model/builtin/` の判定は使わない。判定は PR の題だけで行う。
  - PR の題は `feat(model): <言語> <パッケージ> のモデルを広げる`。1 節の判定も、この題の形で `gh pr list` を検索する。

### 例からの昇格

`examples/rules/*.yaml` と `bench/*/rules.yaml` の例を、組み込みに昇格する作業(一覧の「例から昇格」の行)。

- 例のまま移さない。ほかのモデルと同じ確認を通す: 差分テスト(入っていれば)、`result` / `results` の点検、zoo のセル、すでに組み込みにある行との重複の確認。
- 昇格したら、**例のファイルは、リポジトリの中に、ファイルを名指しで使う参照が残る場合は、中身が空のルール(「組み込みになった。`internal/model/builtin/<ファイル>`」のコメントだけ)として残し、参照がなければ消す**([ADR-102](DECISIONS.md))。決め方: `grep -rn` で、ファイル名と `examples/rules` を、リポジトリ全体(`bench/`、`cmd/`、`internal/`、`extractor/`、`docs/`、`.github/`、`Makefile`)で探し、結果を PR の本文に書く。参照の側を直す必要があるとき(例: `cmd/rivus/notbuiltin.go` とそのテスト)は、その PR でさわってよい。`examples/rules/README.md` の、そのファイルの行は、「組み込みになった(`internal/model/builtin/<ファイル>`)」の 1 行にする(ファイル名で探す利用者のため)。`bench/*/rules.yaml` の例も同じ。リポジトリの直下の README は、昇格の PR ではさわらない(例の一覧は `examples/rules/README.md` を見る、という形にしてある。README に、昇格するファイルを名指しした記述が残っていたら、書き直さず、PR の本文の「手順のとおりにできなかった所」に書く)。
- 例の位置づけ(ADR-063)の上書きは、**[ADR-101](DECISIONS.md) で済んでいる**。昇格の PR は ADR を書かない(ADR-101 を参照するだけ)。
- PR の題は `feat(model): <言語> <パッケージ> の組み込みモデル(例から昇格)`。判定は、この題の形で行う(1 節)。昇格のあと、例のファイルは無いか短くなるので、`ls internal/model/builtin/` の判定は、`<言語>-<パッケージ>*.yaml` で使える。

## 2. 作る(標準の 4 段)

1. **生成(未実装)**: 型から粗いモデルを作る。`internal/model/builtin/<lang>-<package>.gen.yaml`(コミットする。先頭に生成器のバージョンと入力のパッケージとバージョン)。書き込み先の引数は、`results` とレシーバへの流れの読み取り元にも含める。
2. **照合(未実装)**: 差分テストで本体と比べる。MISS は 0。EXTRA は、意図した差だけ、手の部分のファイルの冒頭のコメントに、理由つきで書く。
3. **確認したバージョンを書く**: モデルのファイルの冒頭に `verified_versions`(形式と検証: [DESIGN-package-models.md](DESIGN-package-models.md) 6.7)。パッケージ(Go: モジュールのパス、Dart: パッケージ名、Rust: クレート名)、確認したバージョンの範囲、実際に確認したバージョン(`checked_with`)、日付(`checked`)。手で書いた行だけのファイルにも書く。生成物があれば、先頭の入力のバージョンと同じにする。検証が入る前も、書く。
4. **追加**: `<lang>-<package>.yaml`(手の部分)に、sink、source、sanitizer、`calls:`(利用者のコードを呼び返す API: コールバック、利用者が実装するインターフェース。パッケージの行には各行に `checked_by: cell` を書く。実行の確認の道具は標準ライブラリだけを呼ぶ。代わりに、その API を呼び、`@sink` か `@miss` のマークがあるセルが要る。[MODELING.md](MODELING.md) 3 節、ADR-107)を足す。パッケージの行を足したら、**その行を空にして(または消して)セルが落ちること**を確認する。セルが行を使っていることの確認で、実行の確認より弱いのを補う)。`origin: human` か `llm`。zoo に、肯定と否定のセルを足す。`rivus models coverage` で、未モデルの呼び出しを確認する。
5. **宣言(未実装)**: [DESIGN-package-models.md](DESIGN-package-models.md) 2 節 5 の条件(差分テストで照合済み、`calls:` が書いてある、ほかのパッケージから呼ばれる扱いが決まっている)を満たしたときだけ。満たさなければ、宣言なしで PR を出す。

**source の選び方**: サービスのクライアントは、「その値を誰が書いたか」で選ぶ([ADR-108](DECISIONS.md))。迷う値は入れず、モデルのファイルのコメントに、入れなかった理由を書く。

**骨組みは、公開のシグネチャと、記録するだけの本体にする**: rivus は、骨組みの本体も読む(依存として)。interceptor を呼ぶ、検査をするなど、凝った本体にすると、本物と違う流れになり、モデルの確認が、骨組みの形に引きずられる(実例: Dart の `dio` の最初の骨組み)。本体は、引数を記録する(`zoo_mark` の `recordAny` など)か、空にする。骨組みの公開のコンストラクタは、互いを `this(...)` で呼ばない。非公開の 1 つを通す(互いを呼ぶと、その呼び出しが sink 行のサイトになり、zoo に想定外の流れが出る。実例: shelf の `Response`)。

**開発用の依存とぶつかるパッケージの骨組み(Dart)**: `bench/zoo-dart/exec` の `test` が持ってくる本物のパッケージ(`shelf`、`shelf_static`、`shelf_web_socket`、`web_socket_channel`、`async`、`collection`、`http_parser`、`stream_channel`、`path` など)と同じ名前の骨組みは、`bench/zoo-dart/cells/packages/<名前>`(path の依存。本物の依存は書かない)に置き、骨組みの `pubspec.yaml` の末尾に `zoo_exec_override: <本物の範囲>` の 1 行を書く(`exec/pubspec.yaml` の `dependency_overrides` は、これから生成される。`exec/pubspec.yaml` と `cells/pubspec.yaml` は生成物で、コミットしない。`make zoo-dart-gen`)。解析(cells)は骨組みを読み、実行(exec)は、`test` がすでに動かしている本物で動く。cells の `pubspec.yaml` の依存に本物の名前を hosted で書かない(zoo のプログラムに本物の依存が入り、依存の関数を検査するテストが失敗する)。骨組みは、本物の公開のシグネチャに合わせる(セルのコードが、両方で型が合うこと)。

**骨組みの置き場(Go と Rust)**: 外部のパッケージを使うセルは、パッケージごとに独立したモジュールに置く([DESIGN-package-models.md](DESIGN-package-models.md) 6.8 の表)。Go は `bench/zoo-go-pkg/<パッケージ>/`(`go.mod`、`skel/`、`cells/`、`rules.yaml`)、Rust は `bench/zoo-rust-pkg/<パッケージ>/`(`Cargo.toml`、`Cargo.lock`、`skel/`、`cells/src/`、`rules.yaml`)。**テストは、そのディレクトリを探す(一覧はない)ので、ディレクトリを足すだけで、実行、真の注釈の照合、rivus の照合が働く**(合成の `zoo_synth` が、見本)。セルの名前は `F/pkg/<パッケージ(`_` は `-`)>/<セル>`、マークの固定は `bench/zoo/marks.d/<lang>/pkg/<パッケージ>/`。Rust の `Cargo.toml` は、zoo-mark を `path = "../../zoo-rust/zoo-mark"` で指す(テストが、サンドボックスのために、対象に写して書き換える)。

**骨組みは、sink の行の関数が呼ばれたら、記録する**(zoo のセルで sink の行を確認するため。記録しない骨組みでは、実行の確認が、sink に届かない)。Go の骨組みは `zoo/mark` を、Rust の骨組みは `zoo-mark` を呼ぶ。記録の場所は、セルの中の、骨組みの関数の呼び出しの行になる(Go は、スタックの中の最初の `/cells/` のフレーム。Rust は、骨組みの関数に `#[track_caller]` を付ける)。最小の例(見本は、`bench/zoo-go-pkg/zoo_synth`、`bench/zoo-rust-pkg/zoo_synth` の `Run` と `run`):
- Go(骨組みの `go.mod` に `require zoo v0.0.0` と `replace zoo => ../../../zoo-go`): `func Run(cmd string) { mark.SinkCmd(cmd) }`
- Rust(骨組みの `Cargo.toml` に `zoo-mark = { path = "../../../zoo-rust/zoo-mark" }`): `#[track_caller] pub fn run(cmd: &str) { zoo_mark::sink_cmd(cmd); }`
sink の行は、パッケージの `rules.yaml` の `sinks:` に、骨組みの関数の記号で書く(`zoo/mark` の sink の記号は、zoo の `rules.yaml` が持つ)。Rust のテストは、パッケージの crate と骨組みの crate の `Cargo.toml` の zoo-mark の path 依存を、写した zoo-mark に書き換える。

テスト: `go test ./internal/model/... ./cmd/rivus/ -run 'Builtin|Zoo'`、`regomark check --no-judge`(0)。Dart のセルは、Dart のテスト(`dart analyze` を含む)も通す。

## 3. 既知の見逃しと誤検出は、セルと課題のファイルで残す

PR の本文は、マージのあと誰も見ない。**見逃し(`@miss`)と誤検出(`@fp`)は、セルとして残す。**

1. セルを、新しいセルとして作る(既存のセルのマークは変えない)。
2. 課題を、**番号を取らないファイル名** `docs/issues/model-<パッケージ>-<名前>.md` で作る(GitHub の Issue は作らない)。
3. セルから `@ref(model-<パッケージ>-<名前>)` で結ぶ(新しい `@miss` / `@fp` には `@ref` が必須。ゲートが確認する)。
4. 課題のファイルは、決まったテンプレート(何が見逃されるか / 誤検出されるか、原因、再現するセル)を満たす。テンプレートは `docs/issues/` の README を見る(ゲートが確認する)。段階 0 の PR が入ったあとに、この節に具体的な手順を書く(いまは、README がなければ、既存の `docs/issues/*.md` の 1 つを、形の見本にする)。
5. `docs/ISSUES.md` の一覧への反映は、モデルの PR では要らない。

zoo のゲート(`scripts/zoo-loosened-check.sh`、#167): 新しいセル(`bench/zoo/marks.lock` にまだないファイル)の `@sink(none)`、`@miss`、`@fp` の行の追加は、承認を求めない。`@noisy`、`@na`、`@todo`、`@manual` の新しいセルと、既存のセルへの行の追加は、承認の行が要る。**モデルの PR では作らない。**

## 4. conflict を避ける置き方

1 パッケージ = 1 つのモデルのファイル + 専用の zoo のセルのディレクトリ + 専用のマークのファイル(`bench/zoo/marks.d/`)。`cases.dart` と、zoo-dart の `cells/pubspec.yaml`、`exec/pubspec.yaml` は生成(コミットしない。`make zoo-dart-gen`)、zoo-dart の `pubspec.lock` もコミットしない。共有のファイルをさわらない。**さわってよいファイルは、次のものだけ**:
- `internal/model/builtin/<lang>-<package>*.yaml`(`verified_versions` も、このファイルに書く)と、そのモデルの `.dead-allow`(`internal/model/builtin/<lang>-<package>*.dead-allow`。**さわるのは、「新しい行がどこにも一致しない」という失敗が出たとき(その行を、理由つきで足す)だけ。「zoo から一致するようになったので外せ」の指摘は、無視する**(#187 のあと、通常は記録だけになる。掃除は定期の別の作業で、モデルの PR は、ほかのモデルの `.dead-allow` をさわらない))。**`.dead-allow` の書式は、1 行 1 シンボルの `symbol | 理由`(同じシンボルを 2 回は書けない。ワイルドカードや、まとめの行はない)。zoo のセルが当たらないメソッドで、同じクラスの同じ形のもの(例: 同じ形のオーバーロード、`get`、`post`、`put` のような同じ形の HTTP のメソッド)は、1 行ずつ、同じ理由の文(例: `debt | no cell of the zoo calls it yet`、または `not called: same shape as <当たっているメソッド>`)で書いてよい(書式の拡張は要らない)。1 行のグループにまとめる書式は、いまはない。**
- `bench/zoo-<lang>/` の自分のディレクトリ(セル、骨組み)、外部のパッケージのセルは、`bench/zoo-go-pkg/<パッケージ>/`、`bench/zoo-rust-pkg/<パッケージ>/`(ディレクトリを足すだけでよい。テストも `ci.yml` も足さない)、`bench/zoo/marks.d/<lang>/` の自分のファイル、`docs/issues/model-<パッケージ>-*.md`。
- **広げる PR**(1 節): もともと main にある、そのパッケージのモデルのファイル(と、その `.dead-allow`)。
- **例からの昇格の PR**(1 節): 昇格する `examples/rules/<その 1 つ>`(消すか、空のルールにする。1 節)と、参照の側(洗い出して必要なものだけ)、`examples/rules/README.md` のその行(「組み込みになった」の 1 行に)、`bench/*/rules.yaml` の例(あれば)。
- **例からの昇格の、さわってよい範囲の事実(洗い出し)**: 例のファイルを使っているのは、`bench/compare.sh`(gin: 145、147 行。`gin-vue-admin` と `go-test-bench` のルール)、`bench/parity/collect.sh`(103 行。gin)、`bench/README.md`(gin: 8、158〜160、258 行)、`cmd/rivus/notbuiltin.go` と `notbuiltin_test.go`(actix-web のファイルを名指し。テストが読む)、`bench/parseable/rules.yaml` と `bench/parseable/README.md`(actix-web の写し)、`internal/model/builtin/rust-web.yaml` の 3 行目のコメント、`extractor/rust/README.md` の 108 行。ファイルを消すと、`compare.sh` の gin の 2 つのベンチは、ルールのファイルがなく飛ばされる。**gin は、例のファイルを消さず、中身を空のルール(「組み込みになった」のコメントつき)にして残す案が、使っている所を壊さない**(`compare.sh` は、比べる 2 つのバージョンが、それぞれの木のファイルを読むので、古いバージョンは昔の gin のルール、新しいバージョンは組み込み、になり、昇格の前後の比較の意味が保たれる)。**actix-web は、`notbuiltin.go` の表の行と、そのテストを直す必要がある**(昇格すると、`actix_web::` が組み込みのルールにあるので、知らせは出ない)。この 2 ファイルは、actix-web の昇格の PR で、さわってよい。決める(相談役。ADR-101 の「消す」との関係)まで、gin は、消さずに残す。

- ほかのファイル(`docs/DECISIONS.md`、`docs/ISSUES.md`、直下の README、コード、ゲート、ワークフロー)は、さわらない。conflict が出たら、main 側の行を残して、自分の行を足し直す。

## 5. PR(完成)

- 題: `feat(model): <言語> <パッケージ> の組み込みモデル`(1 節の題と同じ)。`/pr` のスキルで作る(1 節のドラフトを更新する)。
- 本文は、公開の事実だけ(パッケージ名とバージョン、公開の API)。**次の項目を、必ず、この順で書く**(ないものは「なし」、できなかったものは「未実施」と理由を書く。空にしない):
  1. **モデルの行の数と種類**: sink / source / sanitizer / flows / pure / calls の、それぞれの数。生成した行と手で書いた行の内訳(生成器が入ってから)。
  2. **差分テストの結果**: MISS / EXTRA / 意図した差(理由つき)。差分テストが main にないうちは、「未実施(差分テストは未実装)」と書く。
  3. **zoo のセル**: vuln / fixed / decoy の数。
  4. **既知の見逃しと課題**: `@miss` / `@fp` のセルと、`docs/issues/model-<パッケージ>-<名前>.md` の一覧(なければ「なし」)。
  5. **公開のベンチでの前後の数字**: finding(件数と ID の集合が同じか)、`dynamic_dispatch` の注記の数、`summary_elems`、時間(`-pass-stats`)。中を読んだ場合(モデルなし)との照合と、違いの理由。公開のベンチに、そのパッケージを使うものがなければ、「該当するベンチなし」と書く。
  6. **確認したバージョン**: `verified_versions` に書いた、パッケージ、範囲、確認したバージョン。
  7. **手順のとおりにできなかった所**: 手順書が曖昧だった所、道具がなかった所、自分で判断した所。なければ「なし」。手順書の改善の入力になる。**`docs/MODEL-QUEUE.md` の案(境界の案)と違う判断をしたとき(source にしない、sink を足す・外すなど)は、ここに、理由を書く。**
- **最初の 3 本(言語ごとに 1 本)は、相談役が、中身を確認する。** そのために、上の 7 に加えて、**手順の改善案**(次のセッションの手順書に足したいこと)を、本文の最後に書く。
- 触るファイルは 4 節のとおり(広げる PR と、例からの昇格の PR の分も、4 節に書いてある)。それ以外(コード、ゲート、ワークフロー、共有の文書)は触らない。触る必要が出たら、完成にせず、ドラフトのまま、理由を本文に書いて終わる。
- push のあと、コミットの署名(0 節)と、CI の結果を確認する。CI が失敗したら、直して push し直す(自分で `gh pr merge` はしない)。

## 6. 守ること

- 公開の事実だけ。特定の利用者・プロジェクトを書かない。
- GitHub の Issue を作らない・閉じない・編集しない(課題は `docs/issues/` のファイル。3 節)。
- 自分でマージしない(CI が通れば自動で入る)。履歴は書き換えない。
- 既存のセルのマークを緩めない(既存の行の変更・削除は、ユーザーの承認が要るので、しない)。
- 言い回しは `.regomark/policy/wording.rego` に従う(`regomark check --no-judge` が 0)。
- ADR を書かない(昇格は、[ADR-101](DECISIONS.md) で済んでいる)。方針の変更が要ると思ったら、PR の本文に書く。
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
- 生成した行の `origin` は `generated`(生成器のバージョンつき)。手で書いた行は `human` か `llm`(設計 4.3a の f)。


## 9. 差分テスト(`bench/model-diff`。実装済み。2 節の 2 の道具)

モデルの行を、依存の本体と照合する。**Go と Dart。Rust は対象外**(クレートを読むには、`build.rs` と proc macro を動かすので、隔離が要る。上の 0 節)。

```sh
make model-diff PKG=strings                                   # Go の標準ライブラリ。行は、組み込みのモデルのすべてのファイルから取る
make model-diff PKG=package:http MODEL=internal/model/builtin/dart-http.yaml   # Dart。MODEL は、その 1 ファイルの行だけ
go run ./bench/model-diff -pkg google.golang.org/grpc -model mine.yaml -version v1.80.0   # -rules で渡すファイル、バージョンの指定
```

- **やること**: モデルの行にある関数を、source の値で、位置(`recv`、`arg0`、...)ごとに 1 つずつ呼ぶコードを作り、出てくるもの(戻り値ごと、書き込まれうる引数、レシーバ)を、**関数と場所ごとに別の sink** に渡す。同じコードを、(A) そのパッケージの行を外した組み込みモデルで、本体を読んで解析し、(B) モデルで解析して、比べる。出力は、表と、JSON(`-json`。既定は `<out>/report.json`)。
- **結果の種類**: `agree`(一致。`agree_no_flow` は、A も B も流れがなく、その位置について何も言えなかった数)、**`MISS`**(A にあって B にない流れ。モデルが落とした)、**`EXTRA`**(B にあって A にない流れ。モデルが広い)、`EXTRA-intended`(意図した差。下の注釈)、`unverified`(未検証。理由つき)。
- **終了コード**: MISS があれば 1。EXTRA は、注釈のないものがあれば 1。注釈に理由がない、sink の行が存在しない位置を宣言している、生成したコードがビルドできない、も 1。**MISS は、原則、注釈でも許さない**(落ちた流れは、モデルを直す)。例外は、本体を読んで、データの流れではないと確認した 1 つの位置だけ(下の `miss-reviewed`)。
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
- **sink の行は、本体ではなく宣言と照合する**(依存の中の sink は finding にならない。ADR-099)。位置ごとに 1 つだけ source にした呼び出しで、宣言した位置では finding が出て、宣言していない位置と、定数だけの対照では出ないことを確認する。`parts` / `field` は、1 段のフィールドだけ。
- **未検証として出し、何も報告せずに飛ばさないもの**: ジェネリクスの関数、コールバックを取る関数、作れない型の引数、実装のないインターフェースのメソッド、source を入れられないレシーバ(`new` で作れる値と `New...` の関数で作れない構造体のメソッドなど。Dart は `value` の注釈がないクラス)、`named` / `when_arg_type` / `match` のある sink の行。PR の本文の「差分テストの結果」に、MISS / EXTRA / 未検証の数を書く(5 節)。
- **Go の標準ライブラリ**: rivus は標準ライブラリの本体を読まないので、A は、パッケージと、それが import する `internal/...` を、一時のモジュールの中に写して読む(アセンブリは写さない。本体のない関数は、本体のない関数として読まれる)。写せないパッケージの flows の行は、未検証になる(sink の行は照合できる)。
- **バージョン**: `-version` がなければ、Go は、このリポジトリの `go.mod` にあるバージョン(なければ最新)、Dart は、`bench/` などの `pubspec.lock` にあるバージョン(なければ `any`)。標準ライブラリは、いまの Go のバージョン。`verified_versions` への記録は、別の作業。
- **取得と実行**: 依存の取得(`go get`、`dart pub get`)だけで、取得したコードは実行しない(Go は型検査と解析まで、Dart は analyzer まで)。ライブラリごとに分けるので、1 回は数秒〜十数秒、依存込みで 1 GB 前後。
- **PR で動かす対象の求め方**: `go run ./bench/model-diff -changed origin/main` が、`internal/model/builtin/` の行(flows と sinks)が変わったパッケージだけを、`go <pkg>` / `dart <pkg>` の形で出す。CI の定期と手動の層に、`model-diff` のジョブがある(PR には入れていない。設計 6.3)。

## 10. この手順書を「使える」と言う条件

手順書の種類(1 節の「新規」「広げる」「例からの昇格」)ごとに、**ローカルのサブエージェントが、この手順書だけを見て、1 回通す(着手の判定から、PR の本文まで。push はしない)**。通せたら、その種類は「使える」。通せなかった所(読み違えた所、さわってはいけないファイルをさわった所、書いていなくて聞いた所)は、手順書に書き足してから、もう 1 回通す。3 種類とも、1 回ずつ通るまで、クラウドの定期の実行(`rivus-model-author`)は有効にしない。
