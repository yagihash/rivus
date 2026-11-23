# ADR-086: sanitizer のあるインターフェースのメソッドは、プロジェクトの実装の本体を読み、結果への流れだけ捨てる。組み込みの sanitizer は、読んだ種類を一覧に持つ

- 状態: 有効
- 関連: -
- 状態の詳細: 075 を一部置換
- 要旨: 組み込みの sanitizer の影響は 0 行

**背景:** ADR-075 は、sanitizer のあるシンボルを、本体を読まない対象から外していた(本体を読むと「結果は無害」を取り戻すため)。その結果、`type Cleaner interface{ Clean(string) string }` に `Clean` の sanitizer を書くと、プロジェクトの実装の本体の中の sink(`Clean` が自分で `exec` に渡す、グローバルに書く)が見えなかった。

**決めたこと:** (1) 呼び出し先のモデルが sanitizer で、動的な呼び出しのとき、プロジェクト自身の実装の本体も読み、**結果への流れだけ捨てる**(`callCtx.noResult`、`applySummary` の `OutResult`)。本体の中の sink への到達、引数・レシーバ・グローバルへの書き込みは採る。`read_impls` のある行は、今までどおり結果も採る。依存の本体は読まない(ADR-075 のまま)。(2) 組み込みの sanitizer の全行を `internal/model/builtin/testdata/sanitizers-reviewed.txt` に、種類(関数、static、上書きされうるクラスのメソッド、インターフェース / trait のメソッド)つきで持ち、行が増えて一覧にないと `go test` が失敗する(`TestSanitizersAreReviewed`)。

**数えたこと:** 3 言語の組み込みモデルの sanitizer 7 行のうち、インターフェース / trait のメソッドは **0 行**(Go の関数 3、Dart の static 3、Dart の上書きされうるクラスのメソッド 1)。したがって組み込みの結果は変わらない。この実装は、利用者のルールの sanitizer(自前のインターフェースへの行)のためのもの。

**確かめたこと:** `testdata/go/cases/sanitizeown`(結果は無害のまま、本体の sink、グローバルへの書き込みは見える、本体が洗えば検出されない)、zoo の `dyn/sanitizer-own-impl`(脆弱 / 直した / デコイ)。

