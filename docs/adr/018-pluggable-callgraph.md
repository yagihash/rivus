# ADR-018: コールグラフ構築は差し替え可能にする

- 状態: 有効
- 関連: ARCHITECTURE 3
- 状態の詳細: 訂正・追記あり(書き方を保つ)
- 要旨: CHA は VTA の入力として必須(訂正)。言語固有手法はコアに漏らさない

- **状況:** ブリーフ側で、Go のコールグラフは VTA 既定、CHA は比較用、と提案していた。
- **訂正:** `golang.org/x/tools/go/callgraph/vta.CallGraph(funcs, initial)` は初期コールグラフを引数に取る。
  標準的な使い方は CHA（または RTA）で粗いグラフを作り、VTA で絞り込む 2 段構え。つまり **CHA は「VTA の入力として必須」**。
  実物（`golang.org/x/tools` v0.52.0）で確認した。`initial == nil` なら内部で CHA を使う。
  渡す SSA 関数は `ssa.InstantiateGenerics` 付きで構築する必要がある。結果にはルートノードがない。
- **決定:** `CallGraphBuilder` インタフェースを切り、Go は CHA → VTA の実装を `callgraph/gocg` に置く。
  Dart は `pkg/vm` の type flow analysis、Rust は monomorphize collector など、言語ごとに別実装にする。**Go 固有の手法をコアに漏らさない。**
- **理由:** 言語ごとに使える手法が違う。

