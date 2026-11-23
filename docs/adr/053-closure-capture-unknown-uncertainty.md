# ADR-053: クロージャのキャプチャした値が、呼ぶところで分からないとき、何も報告せずに埋めることはしない（DART-15 の不確実性の注記）

- 状態: 有効
- 関連: DESIGN-capture
- 状態の詳細: 059 が作った場所で適用する経路を足す。`capture_unbound` は残る
- 要旨: `depth_limit`/`capture_unbound`

**背景:** ヘルパーがクロージャを受け取って呼ぶ形（`run(() => sink(a))`、Go の `tx_capture_plain`）で、`applySummary` は、呼び出し先のクロージャの引数のうち、足りないもの（キャプチャした値。`make_closure` の引数が付かない `FreeVars`）を、`KUndef` で**何も報告せずに**埋めていた（`summary.go`）。キャプチャした値を通る流れ（`a` が sink に届く）は、見つからないのに、何も残らなかった（`@miss(silent)`）。

**決めたこと:**
1. 足りない引数が**キャプチャの分**のとき（呼び出し先に `FreeVars` があり、呼び出し元が束縛〔`bindings`〕を渡さない）、かつ**クロージャの要約が、キャプチャした値に依る**とき（出力か sink の要素が、宣言された引数より後のパラメータを指す、またはキャプチャした変数に書く。`usesCaptures`）、不確実性を出す。キャプチャした値を使わないクロージャには出さない（失うものが無い）。
2. kind は `depth_limit`（`attributable_to: rivus`）、理由は `capture_unbound`。**`unresolved_callee` にしなかった理由:** 呼び出し先の関数は分かっている（「関数が分からない」のではない）。足りないのは、rivus のヘルパーの要約が、渡されたクロージャのキャプチャした値を運ばないこと（解析の側の上限。ADR-050 の `candidate_limit` と同じ考え方）。`depth_limit` が、既存の「rivus の上限で切った」の系列（`guard_limit`、`path_limit`、`candidate_limit`）に合う。
3. **場所:** クロージャを**作った関数**（キャプチャした値を持つ関数）の、`make_closure` の位置。作った関数は、読む人が、見逃した流れ（source と sink）を見に行く場所で、`rivus resolve` の `absent_now` の定義（source と sink の関数にある不確実性）にも一致する。呼ぶヘルパーの側ではない。文面: 「このクロージャは、値を n 個キャプチャしていて、渡された先の関数が、それを知らずに呼ぶ。通る流れは追わない」。クロージャごとに 1 件（重複は 1 件にまとめる）。
4. 変わらないもの: finding の集合（`KUndef` で埋めることは同じ。不確実性の注記を足しただけ）。

**検証:** go-curated（`@miss(silent)` 9 件 → `@miss(depth_limit)`）、dart-curated（6 件）。go-synth S / M、paramshare、dogfood、Dart / Rust のサンプル、rust-curated で、finding の追加・削除は 0。不確実性は増える（dogfood の `depth_limit` が 4 → 39: rivus 自身のクロージャのうち、キャプチャした値に依るもの。go-curated は 2 → 40、dart-curated は 4 → 11。dogfood の depth_limit の割合は 5 % を超えた（7.8 %。`TestDogfood` の上限を、一律に 10 % に上げたが、あとで分離した。下の「`TestDogfood` の上限の分離」）。増えた分が、不確実性の注記が付いたもの）。サイレントな見逃しの割合: 66 / 78（85 %）→ 51 / 78（65 %）。

**限界:**
- Rust は、同じ抜けが、この経路には無い（rust-curated の不確実性は同じ）。Rust の閉包は、キャプチャした値を引数として運び、`FreeVars` を使わないため、見逃し（15 件。`@miss(silent)`）の原因は別（型が合う全クロージャが候補になる過検出と、ヘルパーの要約）。**まだ不確実性の注記が付かない**。
- Dart で、9 個を超えるクロージャを渡すヘルパー（`run` など。fixture が増えた）は、ADR-050 の `candidate_limit` で、そもそも呼び出し先を解決しない。この不確実性の注記は、ヘルパーの側で、source と sink の関数にはない（`@miss(silent)` のまま）。
- キャプチャした値は、クロージャを作った関数が呼ぶとき（束縛がある）は、これまでどおり正しく扱う。ヘルパーの要約に、キャプチャした値の出力を持たせるのは、C-7 / 案 2 の領分。

**`TestDogfood` の上限の分離:** 最初は、`depth_limit` の割合の上限を、5 % から 10 % に一律に上げた（承認なし）。一律に上げると、ほかの理由（`path_limit`、`budget`、`guard_limit` など）の悪化も通してしまうので、直した。`cmd/rivus/dogfood_test.go` は、`depth_limit` のうち detail の先頭の語が `capture_unbound` のものを数えて、**別の上限（今の件数 35、ちょうど 35 であること）**にする。件数が増えたら失敗し（新しい場所でキャプチャした値を何も報告せずに失っている）、**減っても失敗する**（上限を下げる歯止め。C-7 で下がるはず）。それ以外の `depth_limit` は、不確実性全体の 5 % に戻した（今は 4 件、0.8 %）。

