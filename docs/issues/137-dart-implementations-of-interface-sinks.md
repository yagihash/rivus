# 137: Dart で、インターフェースのメソッドの sink が、パッケージの外の実装の型の変数からの直接の呼び出しに届かない

- 状態: **open**
- 種類: 検出の見逃し(重さ: 低)
- 言語: Dart
- ROADMAP の節: 0
- 守るもの: -
- 題: Dart で、インターフェースのメソッドの sink が、パッケージの外の実装の型の変数からの直接の呼び出しに届かない
- 見つけた経緯: `package:http` の `Client` の組み込みモデル(`dart-http.yaml`)を書いたとき。

`match: implementations` は Dart で使えない(展開は Go だけ。`docs/ARCHITECTURE.md` の実装列挙)。そのため、インターフェースのメソッド(`Client.send`)の sink は、そのメソッドを通る呼び出し(動的な呼び出しとその候補)には届くが、**実装の型の変数からの直接の呼び出し**(`IOClient().send(...)`)は、その型の `send` の記号になり、行がないと届かない。`dart-http.yaml` は、`package:http` の中の具体的な型(`IOClient`、`BrowserClient`、`RetryClient`、`MockClient`、`BaseClient`)の行を足して塞いだ。**パッケージの外**で `Client` を実装した型(自前のクラスや他のパッケージのクラス)が、自分の `send` を持ち、その型の変数から直接呼ばれる形は、塞がっていない(`BaseClient` を継承して `send` を上書かない型は `BaseClient.*` の行で届く)。

対応の案: Dart の `match: implementations`(型の `extends` / `implements` の `type` レコードで部分型を探して、実装のメソッドに行を広げる)。zoo のセルは、zoo の依存が自分のコードとして読まれるため(依存の本体を読まない形が作れない)、作れていない。
