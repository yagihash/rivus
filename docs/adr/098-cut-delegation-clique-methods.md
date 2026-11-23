# ADR-098: 実装の多いインターフェースのメソッドを、委譲のクリーク全体をモデルにして切る基準(ADR-092 の条件 1 とは別の基準。`package:http` の `Client.send`)

- 状態: 有効
- 関連: MODELING 4a、DESIGN-deps-cost 11 節、ARCHITECTURE 5.5
- 状態の詳細: ADR-092 の条件 1 とは別の基準。`package:http` の `Client.send`
- 要旨: 公開の対象で finding が同じ + `dependency_body_unread`

**背景:** ADR-092 の条件 1(時間かメモリの 9 割)は、パッケージを丸ごと切るときの条件として残す。`package:http` の `Client.send` は別の形で、`Client` を実装する型が多く(`IOClient`、`RetryClient`、`googleapis_auth` の 7 つの認証つきクライアント)、実装どうしが同じ `send` に委譲する(委譲のクリーク)。呼び出しの候補が多く、そのたびに依存の実装の本体の要約を計算する。公開の測定(`bench/dart-gcp`、`docs/DESIGN-deps-cost.md` の 11 節): 解析 `duration_ms` 1,018 → 325(-68%)、`summary_elems` 28,543 → 22,158(-22%)。wall と RSS は、ほぼ変わらない(extractor が大半)ので、条件 1 の目安には届かない。

**決めたこと:** 実装の多いインターフェースのメソッドで、実装どうしが同じメソッドに委譲する形は、次の 2 つを満たせば、そのメソッドの行を `flows` のモデルにして、依存の実装の本体を読まない。
1. **公開の対象で、本体を読んだ場合と、finding の集合が同じ。** 照合に使った対象を、設計の文書に書く(`bench/dart-gcp`、`bench/dart-grpc`、公開のパッケージだけを使う合成の例)。
2. **読まなかった本体に、注記が付く。** `dependency_body_unread`(ADR-075)。サイレントにしない。

モデルは、そのメソッドだけにする。そのメソッドを呼ぶ薄い便利なメソッド(`get`、`post`)にはモデルを書かず、sink だけにして本体を読む(便利なメソッドを切ると、自分のコードの `send` への呼び出しのエッジが消える。ADR-092 の条件 2 の(b))。自分のコードの実装は、今までどおり読む。

**限界:** コンストラクタで渡した関数をあとで呼ぶ形(`RetryClient(when:)`)は、`calls:` で書けず、注記だけが出る。`summary` に、切ったメソッドの一覧はない(ADR-092 の 5)。Dart の `calls: [{method: ...}]` があれば、便利なメソッドも切れる。


