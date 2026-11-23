### 6.1 conflict しない置き方

1 パッケージ = 1 つのモデルのファイル + 専用の zoo のセルのディレクトリ + 専用の期待値。モデルの PR が、共有のファイルをさわらずに済む形にする。

**今の conflict の実例**(`git show` で確認): #160 のブランチに、main(#161)を取り込んだマージ(`38b8cdc`)で、`docs/ISSUES.md` が、両側の変更でコンフリクトした。続けて、#160 の取り込み(`6afd10a`)で `docs/DECISIONS.md`(ADR の番号が両側で 098 を取った)。#162 のブランチに main(#160)を取り込んだマージ(`7c12a31`)では、`bench/parity/dead-allow.txt`、`bench/zoo-dart/cells/pubspec.lock`、`bench/zoo-dart/cells/pubspec.yaml`、`bench/zoo-dart/exec/lib/cases.dart`、`bench/zoo-dart/exec/pubspec.lock`、`bench/zoo/marks.lock`、`docs/DESIGN-deps-cost.md`、`docs/ISSUES.md`、`docs/AGENT-GUIDE.md` などが、両側で変わっていた。

| 共有のファイル | なぜ共有か | 分ける案 | 規模 |
| --- | --- | --- | --- |
| `docs/ISSUES.md` | 課題の索引の 1 つの表。各 PR が行を足す | モデルの PR は、課題を作らない。既知の見逃しは、zoo のセルの `@miss` の記述(と、モデルのファイルの冒頭のコメント)だけに書く。索引は、`docs/issues/*.md` の先頭の項目から生成する(`ISSUES.md` は生成物。CI が一致を確認) | 小(生成スクリプト 1 つ + CI の確認)。課題を作る PR(モデル以外)の conflict は、生成で消える |
| `docs/adr/` | ADR は 1 件 1 ファイル(以前は `docs/DECISIONS.md` に追記で、番号の取り合いで conflict した)。番号の重なりは `TestAdrFiles` が止める | モデルの PR は、ADR を書かない(モデルの方針は、既存の ADR で決まっている)。測定と判断は、PR の本文と、モデルのファイルの冒頭のコメントに書く。新しい方針が要るときは、相談役に依頼する | なし(運用のルール) |
| `docs/DESIGN-deps-cost.md`(11、12 節が、モデルごとの測定の追記先になった) | モデルごとに追記した | 追記しない。測定は、PR の本文と、モデルのファイルの冒頭のコメント。まとめは、相談役が、あとで文書にする | なし(運用のルール) |
| `bench/zoo/marks.lock`(574 行、5 列) | 全セルの「緩いマーク」を、1 つのソート済みのファイルに持つ(ADR の zoo の保証) | セルのディレクトリごと(または、言語とセルごと)のファイルに分ける(`bench/zoo/marks.d/<lang>/<cell>.lock`)。テストと `scripts/zoo-loosened-check.sh` は、ディレクトリから集める。新しいセルは、新しいファイルを足すだけで、他の PR とぶつからない | 中(分割の 1 回の変更 + テストとゲートの読み込み。ゲートの意味は変えない) |
| `bench/parity/dead-allow.txt`(1,306 行) | 「zoo の行が当たらない」組み込みの行の許可。全モデル共有の 1 つ | モデルのファイルごと(`bench/parity/dead-allow.d/<model>.txt`)にする。または、`TestBuiltinRowsAreHitByTheZoo` を、モデルのファイルの冒頭のコメント(`# dead-allow: <symbol> | <理由>`)から読む。新しいモデルは、当たらない行がなければ、何も足さない | 小〜中(テストの読み込みの変更) |
| `bench/zoo-dart/cells/pubspec.yaml`、`pubspec.lock`、`exec/pubspec.lock` | 全セルが、1 つの pubspec のパッケージに入り、`path:` の依存の一覧が 1 つ。ロックは、その一覧のソート済みの写し | (1) セルの依存が、`path:` のスタブだけなら、ロックはコミットしない(`.gitignore`)。CI は、もう `dart pub get` を走らせている(`ci.yml` の「Resolve the packages」)。(2) `pubspec.yaml` の依存の一覧は、モデルごとの小さい pubspec(`bench/zoo-dart/models/<pkg>/pubspec.yaml`)に分け、全体の `pubspec.yaml` は、ディレクトリから生成する(コミットしない。または、Dart の pub workspace) | 中 |
| `bench/zoo-dart/exec/lib/cases.dart`(149 行の import と `register()` の一覧) | 全セルのケースの登録 | ディレクトリ(`cells/lib/**/*.dart`)から生成する(コミットしない。`dart run tool/gen_cases.dart` を、テストの前に走らせる) | 小〜中 |
| `README.md`、`docs/AGENT-GUIDE.md`(フラグの表など) | 機能の追加で変わる | モデルの PR では変えない(モデルの追加は、フラグも機能も変えない) | なし |

分けたあとの、1 つのモデルの PR が触るファイル: `internal/model/builtin/<lang>-<pkg>.yaml`(と `.gen.yaml`)、`bench/zoo-<lang>/.../<pkg>/`(セルとスタブ)、`bench/zoo/marks.d/<lang>/<pkg>*.lock`。他のモデルの PR と、共有しない。`docs/DART-MODEL-CANDIDATES.md` の状態の更新も、PR ではしない(着手のマークは、ブランチ名。6.4)。**この分割は、モデルの PR を並べて出す前に、1 つの準備の PR として、先に入れる。**

