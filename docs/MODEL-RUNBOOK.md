# 手順書: 1 パッケージ分の組み込みモデルを作る

読む人: モデルを作って PR を立て続けるセッション。ユーザーに承認を聞けない。並行のセッションが複数ある。この文書だけで作業できるようにする。設計は [DESIGN-package-models.md](DESIGN-package-models.md)、モデルの書き方は [MODELING.md](MODELING.md)、候補の一覧は [DART-MODEL-CANDIDATES.md](DART-MODEL-CANDIDATES.md)(Dart)。

道具の状態: 差分テスト、生成器、宣言(`unread_packages`)は、段階が入るまで **(未実装)**。未実装の段では、ない道具の代わりに、`rivus models coverage` と zoo のセルで進め、PR の本文に「その段は未実装のため省いた」と書く。

## 1. 次の 1 つを取る

1. 候補の一覧(Dart は `DART-MODEL-CANDIDATES.md` の 3 節の順。Go と Rust は、同じ種類の一覧)から、先頭の、着手していないものを選ぶ。
2. `git ls-remote --heads origin 'model/*'` で、`model/<言語>-<パッケージ>` がまだないことを確認する。
3. そのブランチを作り、署名つきの空のコミットを 1 つ載せて、**すぐ push する。これが着手の宣言。** push が拒否されたら(同じ名前が先に取られた)、別のパッケージを取る。
4. リポジトリのファイル(候補の一覧、`docs/ISSUES.md` など)に、着手のマークを書かない(conflict の元)。

取得してよいもの: 候補の一覧にある公開のパッケージだけ(`go mod download`、`dart pub get`、`cargo fetch` まで)。取得したコードは実行しない(`go run`、取得物の `go test`、`dart run`、ビルドスクリプトなし)。

## 2. 作る(標準の 4 段)

1. **生成(未実装)**: 型から粗いモデルを作る。`internal/model/builtin/<lang>-<package>.gen.yaml`(コミットする。先頭に生成器のバージョンと入力のパッケージとバージョン)。書き込み先の引数は、`results` とレシーバへの流れの読み取り元にも含める。
2. **照合(未実装)**: 差分テストで本体と比べる。MISS は 0。EXTRA は、意図した差だけ、手の部分のファイルの冒頭のコメントに、理由つきで書く。
3. **追加**: `<lang>-<package>.yaml`(手の部分)に、sink、source、sanitizer、`calls:`(利用者のコードを呼び返す API: コールバック、利用者が実装するインターフェース)を足す。`origin: human` か `llm`。zoo に、肯定と否定のセルを足す。`rivus models coverage` で、未モデルの呼び出しを確認する。
4. **宣言(未実装)**: [DESIGN-package-models.md](DESIGN-package-models.md) 2 節 5 の条件(差分テストで照合済み、`calls:` が書いてある、ほかのパッケージから呼ばれる扱いが決まっている)を満たしたときだけ。満たさなければ、宣言なしで PR を出す。

テスト: `go test ./internal/model/... ./cmd/rivus/ -run 'Builtin|Zoo'`、`regomark check --no-judge`(0)。

## 3. zoo と、conflict を避ける置き方

zoo のゲート(`scripts/zoo-loosened-check.sh`、#167): 新しいセル(`bench/zoo/marks.lock` にまだないファイル)の `@sink(none)`、`@miss`、`@fp` の行の追加は、承認を求めない。新しい `@miss` / `@fp` には、`@ref(#N)` が必須(課題は `docs/issues/` にある番号。新しい課題は作らず、既にある課題に当てはまるものだけ。当てはまるものがなければ、そのセルは作らず、PR の本文に書く)。`@noisy`、`@na`、`@todo`、`@manual` の新しいセルと、既存のセルへの行の追加は、承認の行が要る。**モデルの PR では作らない。**

- **いまの形**(共有のファイルの分割が入る前): 共有のファイルは、`bench/zoo/marks.lock`、`bench/parity/dead-allow.txt`、`bench/zoo-dart/cells/pubspec.yaml` と各 `pubspec.lock`、`bench/zoo-dart/exec/lib/cases.dart`、`docs/ISSUES.md`、`docs/DECISIONS.md`、`docs/DESIGN-deps-cost.md`。ISSUES、DECISIONS、DESIGN-deps-cost は、モデルの PR で触らない(測定は PR の本文とモデルのファイルの冒頭のコメントに書く)。zoo の共有のファイルは、触る必要がある(セルを足す)ので、main を取り込むたびに conflict が出うる。出たら、main 側の行を残して、自分の行を足し直す(行は並べ替えて、重複を消す)。`pubspec.lock` は、`dart pub get --offline` で作り直す。
- **段階 0 が入ったあとの形**: 1 パッケージ = 1 つのモデルのファイル + 専用の zoo のセルのディレクトリ + 専用のマークのファイル(`bench/zoo/marks.d/`)。`cases.dart` は生成、zoo-dart の `pubspec.lock` はコミットしない。共有のファイルを触らない。変更は、`internal/model/builtin/<lang>-<package>*.yaml`、`bench/zoo-<lang>/` の自分のディレクトリ、`bench/zoo/marks.d/<lang>/` の自分のファイルだけ。

## 4. PR

- ブランチは `model/<言語>-<パッケージ>`。`/pr` のスキルで作る。
- 本文は、公開の事実だけ(パッケージ名とバージョン、公開の API)。表: finding の件数、`dynamic_dispatch` の注記の数、`summary_elems`、時間(`-pass-stats`)。宣言の前後。差分テストの一致の割合、MISS、EXTRA の理由(差分テストが入ってから)。
- 触るファイルは 3 節のとおり。それ以外(コード、ゲート、ワークフロー、共有の文書)は触らない。触る必要が出たら、PR を出さず、理由を書いた下書きの PR にする。

## 5. 守ること

- 公開の事実だけ。特定の利用者・プロジェクトを書かない。
- GitHub の Issue を作らない・閉じない・編集しない(課題は `docs/issues/` で、相談役が管理する)。既知の見逃しは、`@miss` のセルの記述と、`@ref(#N)` で既にある課題に結ぶ。
- コミットは署名つき。`git cat-file commit <sha> | grep -c ^gpgsig` が 1。履歴は書き換えない。
- 自分でマージしない(CI が通れば自動で入る)。
- 既存のセルのマークを緩めない(既存の行の変更・削除は、ユーザーの承認が要るので、しない)。
- 言い回しは `.regomark/policy/wording.rego` に従う(`regomark check --no-judge` が 0)。
- ADR を書かない。方針の変更が要ると思ったら、PR の本文に書く。
