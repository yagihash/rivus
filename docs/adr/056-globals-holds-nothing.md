# ADR-056: 書き込まれても、読む側に何も返さないグローバル（`globals:` の `holds: nothing`）（採用: ユーザーが承認した、C-14、2026-11-21）

- 状態: 有効
- 関連: MODELING に記載なし
- 状態の詳細: 採用、ユーザー承認
- 要旨: 流れを止めたことを `global_holds_nothing` で記録

**背景:** Gitea の解析で、`certmagic`（ACME の依存）のパッケージ変数 15 個に、380 個の `*net/http.Request` 型の source が、1 個あたり 4 つのパスで載り、要約の最大が 15,200 要素になった。追跡（`RIVUS_TRACE_ELEM`）で、入口は、`log.FallbackErrorf` の `fmt.Fprintf(os.Stderr, ...)`（引数から書き込み先へ流れる、という既定）だった。`os.Stderr` の指す先が汚染され、`log.New(os.Stderr, ...)`（引数から結果へ）で、すべてのライブラリのロガーが、汚染を運び、ロガーを持つパッケージ変数へ広がった。`os.Stderr` に書いたものは、プログラムの外へ出るだけで、同じプロセスの誰も読み戻さない。

**決めたこと:**
1. ルールの書式に `globals:` を足す（`symbol`: パッケージ付きの変数名、`holds: nothing | everything`）。`nothing`: そのグローバル（と、その指す先）への書き込みは、(a) 関数の要約の出力（`OutGlobal`）に出さない、(b) グローバルの格納に書かない、(c) 読む関数として登録しない。つまり、他の関数から見て、読んでも汚染は返らない。**書き込みの呼び出し自体（`fmt.Fprintf(os.Stderr, x)`）は、今までどおり呼び出しとして見える**（sink のルールが名前で適用できる。ログへの書き込みを sink にしたい利用者は、呼び出しで捕まえる）。
2. **関数の中では、変数は、ふつうの変数のまま**（同じ関数の中で `os.Stderr` に書いて、すぐ読む形は、今までどおり流れる。フィクスチャ `globalshold` の `WriteAndRead`）。流れなくなるのは、関数の外へ、変数を通って出る流れだけ。
3. 組み込みのモデル（`go-pure.yaml`）に、`os.Stderr` と `os.Stdout` を書く。プロジェクトのルールで、`holds: everything` と書くと、外れる（精度の限界を、利用者が外せる）。`Merge` は、どの集合でも `everything` とされた変数を、`nothing` から外す（集合の順序によらない）。
4. エンジンのコードに、`os.Stderr` とは書かない（`Rules.HoldsNothing(name)` を引くだけ）。

**測定（Gitea、`reflect` のモデルあり、callouts なし）:** `summary_elems` 839,401 → 439,321、`summary_steps` 35.1M → 16.5M、最大の要約 15,200 → 935、`reanalyses` 134,075 → 120,899、`depth_limit` 1,431 → 1,217、収束は 8 パスのまま、finding 37 → 38（source の入れ替わり: 出ていく 6 組、入る 7 組）。最大 RSS（13.5 GB）とパスの数は変わらない。rivus 自身（dogfood、同じ木 = main の 2dd5022 を、main のバイナリと `holds` のバイナリで解析）: `reanalyses` 10,163 → 7,656（−24.7 %）、`summary_elems` 31,653 → 27,426、finding は 39 件で同一（`(source, sink)` の組 39、sink 11 か所。数え方は、finding の件数と、組の数の両方）、`depth_limit` 39 → 38（`capture_unbound`）。gogs: 変わらない。（C-1 の前のバージョンで測った数字は、finding 88 だった。測り直した。）

**限界:**
- **見逃す形**: `os.Stdout` / `os.Stderr` を差し替えて、読み戻すコード（`os.Pipe` と、`os.Stdout = w`、別の端から読む）は、追わない。`holds: everything` で戻せる。
- サイレントな見逃しにしない: 書き込みがあった変数ごとに、不確実性を **1 件**出す（`global_holds_nothing`、`attributable_to: rivus`。書いた関数（上位 10 件）を `candidates` に、最初の 1 つを `func` に）。理由: `numeric_stop` と同じく、モデルの判断で、利用者がルール（`holds: everything`）で取り戻せるので、rivus の側の限界として扱う。新しい種類の値は足さず、既存の `rivus` を使う（`kind` は新しい）。`stats.dropped_globals` に、変数ごとの、書いた関数の数を出す。件数は、変数の数（組み込みでは最大 2）。
- `holds` の値は、今は 2 つ。将来の 3 つ目（「メタデータだけ持つ」）を足せる名前にしてある。
- `sync.Pool`（`sync.allPools`、`sync.oldPools`）のような、型のモデルで示す形は、別。

