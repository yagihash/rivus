# #92 ベンチの採点が、見逃しが何も報告していないかと、経路に source の関数が出ることを見ない

- 状態: **closed**
- 種類: 道具・ベンチ
- 言語: 全
- ROADMAP の節: 1
- 守るもの: `cmd/rivus/bench_curated_test.go`、`cmd/rivus/offsource_test.go`
- GitHub: #92(移行前の番号は同じ。2026-11-21 に開いた。移行時の状態: OPEN)

## 本文(GitHub から移した写し)

## 確度
コード読解(`cmd/rivus/bench_curated_test.go` の `checkCuratedReport`。見るのは、finding の (source の位置, sink の位置) の組、`LoadErrors`、`Converged` だけ)。CORE-B が採点で見つからなかったことは実行で確認。

## BENCH-1: 採点が、見逃しの「何も報告していないか」を見ない
`@miss` は、uncertainty つきでも何も報告していなくても同じ 1 件。判断の基準では重さが違う(サイレントな見逃しは重い、uncertainty つきは軽い)。#76 の型のサイレントな見逃し(dart-curated で 13 件前後)が、ほかの見逃しと区別できない。
- 方向: 注釈を 2 つに分ける。`@miss(silent)`: 何も報告していない(直すべき不具合)。`@miss(<kind>)`: その kind の不確実性が source か sink の関数に出ている。テストは、その kind の不確実性が実際にあることを確かめる。サイレントな見逃しに uncertainty が付いたらテストが失敗して、注釈を書き換える(片方向の歯止め)

## BENCH-2: 採点が、経路の証拠を見ない
グローバルを通る流れの経路から source の関数が消える不具合(CORE-B の issue)は、どのベンチ・サンプルのテストでも落ちず、rivus 自身の解析を人が読んで見つかった。
- 方向: すべての finding について、言語に依らない不変条件を 1 つ足す: 経路のどこかに、source のサイトがある関数が出る。3 つの curated、サンプルのテスト(`checkSampleFlows`)、`TestDogfood` に入れる。今落ちるもの(グローバル経由)は、数を記録して CORE-B の受け入れにする

## 関連
CORE-B の issue、#76

別セッションのレビューの識別子: BENCH-1、BENCH-2

## 直した内容

038b2c0e(BENCH-1 / BENCH-2)。`@miss` を `@miss(silent)`(何も報告していない)と `@miss(<kind>)`(その kind の不確実性が出ている)に分け、テストが両側から確かめる(uncertainty が付いたら、改善として落ちる)。経路に source の関数が出ない finding の数は、`offSourcePath`(`offsource_test.go`)で数え、`TestDogfood`、3 つの curated、`paramshare` のサンプルで上限として固定した。e6317bfb が、サイレントな見逃しの割合を curated のテストのログに出す。`other_sources` は経路のステップを持たないので、BENCH-2 の対象にしていない。
