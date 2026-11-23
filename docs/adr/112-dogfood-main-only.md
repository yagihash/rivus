# ADR-112: TestDogfood は main の実行だけにして、capture_unbound の上限は記録だけにする(2026-11-23)

- 状態: 有効
- 関連: ADR-037、ADR-053、`.github/workflows/ci.yml`

**背景:** `TestDogfood`(`cmd/rivus/dogfood_test.go`)は、rivus が自分のコードを、linux と darwin の 2 つの見方で解析する。CI では 160〜230 秒かかり、PR の CI の下限(1 つのシャードの時間)を決めていた。期待のファイル(`testdata/dogfood-flows.txt` と `dogfood-flows-darwin.txt`)の履歴は 9 回の変更で、どれも rivus 自身のコードやモデルの変更に合わせた更新だった。エンジンの退行を見つけた記録はない。`capture_unbound` の上限(37)は、新しいコードで 1 晩に 3 回、CI を落とした。

**決めたこと:**
1. `TestDogfood` は `testing.Short()` のときに自分で飛ばす。PR の CI は `go test -short`、main は `-short` なしなので(`ci.yml` の `SHORT`)、新しい仕組みは足さない。main への push、週 1 回の実行、手動の実行では回る。`make check` も `-short` なので回らない。飛ばしたことは、`-v` のログに `SKIP` と理由で出る。
2. `capture_unbound` の数は、失敗にしない。`capture_unbound: N (last recorded M)` の 1 行をログに出すだけで、増えても減っても落ちない。定数(`wantCaptureUnbound`)は「最後に記録した値」として残す。flows のファイルと findings の期待(`checkDogfood` の確認)は、今までどおり失敗にする。

**理由:** 下限が 160〜230 秒のテストを、PR のたびに回す価値が、上の履歴では見合わない。上限は、新しいコードの正しさではなく、rivus 自身のコードの形に結びついていて、監視として働く前に CI を止めていた。

**弱くなる所:**
- rivus 自身のコードの変更で期待がずれると、PR では分からず、マージのあとで main の CI が失敗する。次の PR で期待を直す。
- 気づく方法は、main の CI の失敗。PR の実行と `make check` では回らない。
- `capture_unbound` が増えても落ちない。増減はログで見る。

**手元で回す:** `go test ./cmd/rivus -run TestDogfood -count=1`(`-short` なし。約 60〜90 秒)。期待のファイルを直すときは `-update` を付ける。
