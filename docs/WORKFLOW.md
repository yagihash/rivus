# 進め方のルール

設計の判断の基準は [PRINCIPLES.md](PRINCIPLES.md)。ここには、PR、測定、記録、マージのルールだけを書く。
- **PR のタイトルと本文、コミットのメッセージにも、用語チェックのルールを適用する。** PR を立てる前に、タイトルと本文を一時の `.md` に書いて `regomark check` に通す(用語チェックのフックは `.md` を書いたときだけ効くため)。
- **作業の順**は `.claude/work/QUEUE.md`(追跡されない。相談役が書く)。コストか精度を上げる作業は、始める前に、課題の「修正の方向」に「これで動くはずの数字」を 1 行書く。マージのあとに台帳の行と比べ、外れたら課題にそう書く。
- **memos の前後の測定**(`bench/compare.sh --cost-check memos`。交互に 2 回ずつ)が必要なのは、(i) 「結果は同一」と言う PR(速さとメモリ)、(ii) 何度も通る関数に触る PR だけ。関数は `internal/taint` の `add`、`copyLoc`、`copyLocs`、`copyLocsNoted`、`copyLocsGuarded`、`copyLocsAlias`、`applySummary`(呼び出しの数をコードで確かめた)。それ以外の解析の PR は、PR の CI(`TestScaleSynth`、zoo、curated)で足りる。
- **メモリとコストの変更は、グラフの表現(IR、エッジと候補、関数ごとの要約)が前後で同じことを示す。違うなら、それは精度の変更として扱う。** 示すのは `bench/compare.sh --graph-check <対象の名前、カンマ区切り> <base> <head>`(zoo、curated、go-synth-S、memos など。Dart は `dart-curated` と `zoo-dart`(Rust は対象外)。zoo-dart はパス上限で止まるセルがあるので、`convergence: NOT CONVERGED` の行が出て終了コードは 3(止まった時点の状態を比べたもの。合格にしない)。同じコードを前後のバイナリで解析して、IR、エッジ、要約のハッシュ、finding、uncertainty を比べる。finding と uncertainty は集合としての比較に加えて、出力の順序のままの比較(`findings-order`、`uncertainties-order`)もする。同一なら 1 行ずつ `same`)。finding の件数が変わらないことは、これの代わりにならない。PR の本文に、同一だった対象を書く。
- **言語共通のコード(`internal/valuecalls`、`internal/taint`、`internal/sg`、`internal/ir`、`internal/callgraph` など、特定の言語の extractor でないもの)を変える PR は、Dart や Rust 向けの修正でも、Go の `bench/compare.sh --graph-check`(go-std10 と go-curated は必須、memos はロックが空いていれば)の結果を本文に付ける。** 言語共通なので、Dart 向けの `valuecalls` の修正(#465、#466、#471)が Go の memos の edges に 1 本足した(#471 のあとの確認で見つけた)。結果が変わる修正では `--graph-check` が DIFFERENT になるので、差が「見逃しが減った向きだけ」であることを本文で分類する。
- **測定から結論を出すとき**: `bench/compare.sh` は、1 回だけの測定を「single run: no conclusion」と出し、結論にしない。手で測るときは、交互に 2 回以上(A、B、A、B)。ロードアベレージ、方法(主なプロセスか木の合計か)、コミットを付け、「疑い」と「確認した」を分ける。時間を比べる実行は、測定の道具(反復ごとのダンプ `RIVUS_INTERNAL_SUMMARY_DUMP_EACH_ITER`、プロファイル、`RIVUS_TMP_BUDGET_LOG` など、解析の速さに効くもの)の有無を揃え、台帳(`bench/heavy-ledger.tsv`)の `settings` の列に書く(反復ごとのダンプつきの実行は、ダンプなしの実行より、後半のラウンドが数倍遅く、「収束しない」と見えたことがある。`dart_services`、2026-11-24)。
- **重い測定の記録**は台帳(`bench/heavy-ledger.tsv`、`bench/heavy_ledger.py`)だけ。数字は本文に写さず、行を日付とコミットで指す。道具が書いた行は、その日のうちに、1 つの PR で main に入れる(main に無い行は、記録として扱わない)。重い実行は、ロックの下で 1 本ずつ(`bench/heavylock.py`)。
- **時間の判定**: 合格基準 D(と時間の余裕の議論)の判定は、静かな状態で 3 回の中央値(`bench/deps_bench.py -repeat 3`)。開始と終わりの負荷(`uptime` の 1 分、5 分、15 分)と、同時に動いていた重いプロセス(CPU 20% 以上の上位 5 件のコマンド名)は、道具が台帳の `settings` に書く。ロックはロックを取らない実行(ビルド、ほかのセッション)を止めないので、ロックだけでは静かと言えない(`dart_services` で、静かと報告された 527 秒の実行は、wall − 解析が 200 秒、同じ設定の別の実行は 44 秒だった)。
- **調査の途中の記録**は、追跡されない作業メモ(`.claude/work/`。課題ごとに 1 ファイル)に置き、課題には、結論が出たときに 1 回だけ書く。公開の場所には、外部の設計者の返事のファイル名ではなく、何を、なぜ決めたかを書く。課題と ADR への追記は、作業の PR に含める。作業の PR に入れられない文書の変更は、まとめて 1 日 1 本。
- **道具が守るもの**:
  - 1 日の報告の最後に、`make daily-stats SINCE=<日付>` の 1 行(解析・モデル・extractor の PR の数と、それ以外の数)を付ける。
  - マージ済み(または閉じた)PR のブランチへの push は、`.githooks/pre-push` が止める(有効にするのは `make setup`。意図した push は `RIVUS_ALLOW_PUSH_TO_MERGED=1`)。`daily-stats` の `orphan remote branches: N` が 0 でなければ、取り込むか消す。
  - `make merge PR=<n>`(`scripts/merge-pr.sh`)は、main の最新の `ci.yml`(push)が失敗しているとマージしない。main の CI を直す PR だけ `RIVUS_MERGE_FIXES_MAIN=1`。PR は `-short` なので、重いテストは main でしか実行されない。限界: 失敗するコミットの CI が実行中の間は、その前の成功を見て通る。
  - 既定で off の機能の一覧は、コードとテストが持つ。規則は([PRINCIPLES.md](PRINCIPLES.md) 7 節の「既定で off の機能(フラグ)の持ち方」、`cmd/rivus/flagsides.go`)。
- **相談役が読む PR**は、(i) 解析の結果を変える PR、(ii) 出力の項目・uncertainty と不確実性の種類・`summary`・終了コードを変える PR だけ。結果が同一であることを数字で示した PR、道具、文書は読まない。マージは開発者が行う(`ci-ok` のあと、マージコミット)。
