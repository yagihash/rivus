# ADR-051: `Reachable` が、捕まえる値のあるクロージャの関数もたどる（CORE-E）

- 状態: 有効
- 関連: -
- 状態の詳細: Rust は 058

**背景:** `sg.Program.Reachable`（finding の `reachability` と、graph の `path --from-entry` の元）は、呼び出し、候補、関数値（`KFunc`）をたどる。`make_closure` のうち、**捕まえる値のないもの**（`Args` が空）は `KFunc`（関数値）になり、呼び出しの引数に出るのでたどられる。**捕まえる値のあるものは `KFunc` にならず、たどられなかった**（graph の `refs` は、捕まえる値のある `make_closure` を、`closure` 種別の参照にしていた。x/tools で 454 関数、約 8 %）。実際には届く関数が、finding の `reachability` で `not_from_entry` になりうる、誤った注記だった。

**決めたこと:** `Reachable` は、届いた関数の中の `make_closure` 命令の、クロージャの関数（`IR.Func`）もたどる。捕まえる値の有無に依らない（graph の `refs` の `closure` 種別と、`KFunc` の参照を合わせると、同じ集合になる）。`TestReferencesCoverReachable` の例外（`Lit`: 「graph は届くが、`Reachable` は届かない」）をなくした。**graph のたどる集合と `Reachable` は、呼び出し + 参照で一致する**ことを、そのテストが固定する（graph が届く関数は、すべて `Reachable` にある。逆も同じ）。

**finding の集合は変わらない**（`Reachable` は finding を消さない）。変わるのは、`reachability` の値（`not_from_entry` → `from_entry`）と、代表の経路の選び方（「到達可能」が、選び方の最初の基準）。

**検証:** go-curated、go-synth S / M、paramshare、dogfood、Dart / Rust のサンプルと curated で、finding の追加・削除 0、`reachability` の変化 0、経路の変化 0（これらのプログラムでは、`Reachable` が足りなかったことで `not_from_entry` になった finding がない）。x/tools などの実プロジェクトのベンチは、このマシンのスワップが逼迫していた（94 %）ので、**実行していない**。454 関数が対象の x/tools での件数（`not_from_entry` → `from_entry`、代表の経路が変わる件数）は、未確認。

