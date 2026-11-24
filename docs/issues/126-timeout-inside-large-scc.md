# 126: `-timeout` の超過: 巨大な SCC の中で期限を見ない

- 状態: **closed**
- 種類: 利用者を止める(直した)
- 言語: 全
- ROADMAP の節: 0
- 守るもの: `cmd/rivus/passstats_top_test.go`(`TestTimeoutInsideALargeSCC`)
- 題: `-timeout` の超過: 巨大な SCC の中で期限を見ない(並列ラウンドの中は直した)

`-timeout` は、関数ごとに期限を見る。8 関数以上の SCC を `-workers` が 2 以上で解く道(並列ラウンド)は、1 ラウンドの全関数の計算が終わるまで、期限を見なかった。

公開の測定: `dart_services` で、`-timeout 540s` に対し 585 秒で止まった(45 秒の超過)。最大の SCC は 12,130 関数。

## 直したこと

並列ラウンドの各関数の計算の前に期限を見る。切れていたら、その関数は計算せず、ラウンドの後の commit で飛ばす。`stats.timed_out` は、関数ごとの道と同じに立つ。期限がなければ(`-timeout 0`)結果は変わらない。

- テスト: `cmd/rivus/passstats_top_test.go`(`TestTimeoutInsideALargeSCC`: 60 関数の SCC を合成し、`-timeout 50ms` で全体が 1.5 秒以内に終わる。全部を解く時間は約 3.4 秒)。

## 残り

直っていない超過は、課題 #127 に分けた。

状態: closed(並列ラウンドの中)。

関連: 型が合う関数値を候補に補う案(今は入れない)は #207。
