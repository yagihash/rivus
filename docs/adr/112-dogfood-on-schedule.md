# ADR-112: TestDogfood は schedule の実行で、前回から差分があるときだけ回す。capture_unbound の上限は記録だけにする(2026-11-23)

- 状態: 有効
- 関連: ADR-037、ADR-053、`.github/workflows/ci.yml`

**背景:** `TestDogfood`(`cmd/rivus/dogfood_test.go`)は、rivus が自分のコードを、linux と darwin の 2 つの見方で解析する。CI では 160〜230 秒かかり、PR の CI の下限(1 つのシャードの時間)を決めていた。期待のファイル(`testdata/dogfood-flows.txt` と `dogfood-flows-darwin.txt`)の履歴は 9 回の変更で、どれも rivus 自身のコードやモデルの変更に合わせた更新だった。エンジンの退行を見つけた記録はない。`capture_unbound` の上限(37)は、新しいコードで 1 晩に 3 回、CI を落とした。

**決めたこと:**
1. `TestDogfood` は、環境変数 `RIVUS_DOGFOOD` が空のとき自分で飛ばす(理由はログの `SKIP` に出る)。PR の実行、main への push、`make check` では回らない。
2. 回すのは、(a) CI の週 1 回の schedule の実行で、前回の成功した schedule の実行の head から今の HEAD まで、コードとモデルに差分があるとき(`changes` ジョブの `dogfood` の step。範囲は paths-filter の `code` と同じで、ワークフローは除く。前回の実行がない、前回の head が履歴にない、のときは回す)。差分がなければ、飛ばしたことをログに出す。(b) 手動の実行(`workflow_dispatch`)は常に。(c) 手元の `RIVUS_DOGFOOD=1 go test ./cmd/rivus -run TestDogfood -count=1`(約 60〜90 秒)。
3. schedule の周期は週 1 回のまま。schedule の実行は、`-short` なしの全部のテストと `model-diff` も回すので、毎日にすると、差分のない日にも、その全部を回すことになる。差分の判定は前回の成功した実行からなので、実行が失敗した週の次の実行は、同じ範囲をもう一度見る(直すまで回り続ける)。
4. schedule の実行は、専用の concurrency のグループにした。main への push が、回っている schedule の実行を取り消さない。
5. `capture_unbound` の数は、失敗にしない。`capture_unbound: N (last recorded M)` をログに出すだけ。定数(`wantCaptureUnbound`)は「最後に記録した値」として残す。flows のファイルと findings の期待は、今までどおり失敗にする。

**理由:** 下限が 160〜230 秒のテストを、PR や push のたびに回す価値が、上の履歴では見合わない。上限は、新しいコードの正しさではなく、rivus 自身のコードの形に結びついていて、監視として働く前に CI を止めていた。

**弱くなる所:**
- rivus 自身のコードの変更で期待がずれても、PR と main への push では分からない。次の schedule の実行まで失敗が出ない(最大で 1 周期、1 週間)。そのときは、次の PR で期待を直す。急ぐときは、手動の実行か、手元で回す。
- 気づく方法は、schedule の実行の失敗(`ci-ok` が失敗する)。通知の仕組みは足さない。
- `capture_unbound` が増えても落ちない。増減はログで見る。
