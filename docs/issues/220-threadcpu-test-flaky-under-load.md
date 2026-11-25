# #220 `TestThreadCPU` が、CI の runner の負荷で、時々落ちる

- 状態: **open**
- 優先度: 低
- 種類: 道具(重さ: 低)
- 言語: 全
- ROADMAP の節: -
- 守るもの: `TestThreadCPU`
- 題: `internal/taint` の `TestThreadCPU` が、負荷で時々落ちる

## 何が起きるか

`internal/taint/cpu_thread_test.go` の `TestThreadCPU` は、`-v` の slow function の CPU 時間の列が、実際の CPU を測っていること(スレッドの CPU 時計が、仕事では進み、sleep では進まないこと)を守る。仕事の量は実時間で 30 ms、CPU 時間の下限は 10 ms。CI の runner の負荷が高いと、スレッドが実時間の間に CPU を取れず、`30 ms of work used 6.367928ms of CPU` のように落ちる。実時間と CPU 時間の比が負荷で揺れるテストなので、定期的に落ちうる。

## 影響の範囲

`internal/taint` のこのテストだけ。解析の結果と CPU 時間の列の実装には関係しない。落ちたときは CI の再実行で通る。

## 直し方の案

下限の閾値を緩める、または仕事の量を増やす。落ちた再実行の回数は、数えて残す(ガードレールの設計の、不安定なテストの再実行を数える方針に合わせる。`docs/WORKFLOW.md` などに、この方針の番号はまだない)。

## 見直す条件

次のどちらかのとき。

- このテストが再び落ちたとき(CI の再実行の回数を数える)。
- テストの測定の仕組みを変えたとき。
