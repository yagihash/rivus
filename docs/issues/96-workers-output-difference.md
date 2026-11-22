# #96 -workers 1 と 2 以上で、8 関数以上の SCC を含む入力の出力(代表の経路、stats)が違う(R-43)

- 状態: **closed**(決まった形の差は直した。まれな失敗は再現せず)
- 種類: 道具・ベンチ
- 言語: 全
- ROADMAP の節: 2(経路の欠け・揺れ)
- 守るもの: `cmd/rivus/determinism_test.go` の `TestDeterministicLargeSCC`(`-workers` 1・2・16・既定の出力の完全一致。Skip はなし)、`sameEveryTime` の `-workers 1`(例外なし)
- GitHub: #96(移行前の番号は同じ。2026-11-21 に開いた。移行時の状態: OPEN)

## 今の状態(2026-11-22 の見直し)

R-43 の件。

## 本文(GitHub から移した写し)

## 現象
`rivus analyze -workers 1` と `-workers 2` 以上で、8 関数以上の SCC を含む入力の出力が違う(ヘルプと `Options` のコメントは「結果は workers に依らない」)。

アーキテクチャ助言者の全体レビュー R-43(2026-11-21)。自分で再現した。

## 再現
`testdata/scc`(`python3 gen.py 10 14` で作った、互いに呼び合う 14 関数。SCC が 8 関数以上):

```
cd testdata/scc
rivus analyze -workers 1  -rules rules.yaml ./... > w1.json
rivus analyze -workers 2  -rules rules.yaml ./... > w2.json
rivus analyze -workers 16 -rules rules.yaml ./... > w16.json
```
- `-workers 2` と `-workers 16` は同じ(時間を除いて)。
- `-workers 1` は違う: `stats.passes`、`stats.reanalyses`、`stats.summary_steps` が違い、finding の `fingerprint["path@1"]`(代表の経路)が違う(同じ入力の seed 6 と 10 で確認。finding の集合は同じ)。

## 原因の見立て
`internal/taint/engine.go:472`: `workers > 1 && len(scc) >= parallelSCC`(8)のときだけ、SCC の関数を「前の回の要約に対して」まとめて解析して順に commit する。`workers == 1` は、SCC の中を順に、直前の更新を見ながら解析する。到達する不動点は同じでも、途中の代表の経路や steps が違う。CPU 1 個の CI やコンテナは既定が workers=1 になる。

## 直す方向
回し方を 1 つにする(常に「前の回の要約に対して」)。これはコア(`internal/taint`)の変更で、finding の経路が変わりうるので、compare.sh で確かめる。

## テスト
`cmd/rivus/determinism_test.go` の `TestDeterministicLargeSCC` を足した。今は不一致を検出して `t.Skip`(この issue の番号つき)する。直したら、Skip が消えて PASS になる。`sameEveryTime` の workers に `1` を足した(今までは「既定(CPU 数)・2・16」で、1 を一度も比べていなかった。既存の入力には 8 関数以上の SCC がなく、1 を足しても通る)。

## コメント(GitHub から移した写し)

追記: 既存の入力の rust-curated(bench/rust-curated)にも 8 関数以上の SCC があり、-workers 1 だけ代表の経路が違う(TestDeterministicCuratedRust)。sameEveryTime は -workers 1 の差を、この issue の番号つきで Skip にした。上の「既存の入力には 8 関数以上の SCC がない」は、Go の入力についてのみ正しい。


## 追記(2026-11-22、開発者)

`TestDeterministicParamshare` が、手元で 3 回中 1 回、`GOMAXPROCS="1" -workers="" と -workers="2" の出力が違う` で落ちた(残りの 2 回は通る)。`GOMAXPROCS=1` で `-workers` 未指定は、`-workers 1` と同じ並びになり、この課題(`-workers 1` と 2 以上の差)が現れる。`sameEveryTime` が Skip にするのは、`-workers="1"` の差だけで、`-workers=""`(GOMAXPROCS が 1 のとき)の差は、Skip にならずに落ちる。CI でも、まれに赤くなりうる(今日の変更とは関係しない)。直すなら、`sameEveryTime` の Skip の条件を、実際の worker 数で決める(`-workers=""` と GOMAXPROCS=1 の組を、`-workers=1` と同じに扱う)。

## 経緯(直し)

- 2026-11-22: `internal/taint/engine.go` の `workers > 1 &&` を外し、8 関数以上の SCC は、`-workers` によらず「回ごと」(前の回の要約に対して解析し、順に commit)の道を通す。既定(workers = CPU 数 > 1)の出力は変わらず、変わるのは `-workers 1` だけ。相談役の判断で、この形にした。
- 確かめたこと: `-workers 1` と既定の出力の完全一致を、`testdata/scc`(試験に固定)、go-curated、dart-curated、rust-curated、動物園の Dart と Rust で確かめた。動物園の Go は、`stats.functions` などが、`-workers` を変えなくても同じ入力で 579〜586 に揺れる(#84。go/ssa のラッパー由来)ので、完全一致の比較には向かない(`-workers` の差ではない)。
- 「まれに TestDeterministic* が落ちる」は、ローカルの `-count=8` と、直近の CI 25 回で再現しなかった。次に出たら、実行の番号つきで開き直す。
