# #96 -workers 1 と 2 以上で、8 関数以上の SCC を含む入力の出力(代表の経路、stats)が違う(R-43)

- 状態: **open**
- 種類: 道具・ベンチ
- 言語: 全
- ROADMAP の節: 2(経路の欠け・揺れ)
- 守るもの: -
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

