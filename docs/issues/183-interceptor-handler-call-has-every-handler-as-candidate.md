# #183 全: ミドルウェア・interceptor を通して呼ぶハンドラ(`handler(ctx, req)`)の候補が、登録した全部のハンドラになる

- 状態: **open**
- 種類: 構造的な過検出
- 言語: 全(Go、Dart、Rust)
- ROADMAP の節: 2
- 守るもの: -

## 何が誤って報告されるか

サーバーのライブラリは、ハンドラを、関数の値(またはインターフェース)にして、ミドルウェア・interceptor に渡す。interceptor は、それを `handler(ctx, req)` と呼ぶ。この呼び出しの呼び出し先の候補は、同じ型の関数値の全部になり、1 つのリクエストが、登録した全部のハンドラに渡されたものとして扱われる。

測定(Go、memos、2026-11-23。数だけで、場所は書かない): gRPC のサーバーで、この呼び出しの候補は 67(登録したメソッドの全部)。あるハンドラ(メモの添付ファイル)のリクエストが、復旧の interceptor を通って、別のハンドラ(添付ファイルを作る流れ)に届いた経路が、finding になる。1 つのリクエストの値が、別のメソッドのリクエストにも入ったことになり、メソッドごとの流れの区別が失われる。

## 候補が「登録した全部」になる所(コードを読んで分かる範囲。実行はしていない。Dart と Rust は、読んだ形からの推定で、数は測っていない)

- **Go(grpc-go)**: 生成されたコードのメソッドごとの `_<Service>_<Method>_Handler` が、`func(ctx, req any) (any, error)` の関数値(`grpc.UnaryHandler`)を作って、`interceptor(ctx, in, info, handler)` に渡す。interceptor は `handler(ctx, req)` と呼ぶ。関数値の型が全メソッドで同じなので、候補は、プログラムの中の、この型の関数値の全部。`req` は `any` で、型で絞れない。
- **Dart(`package:grpc`)**: サービスのメソッドは、`ServiceMethod` に、関数値のフィールド(`handler`)として登録される(`$addMethod`)。サーバーは、メソッドの名前から `ServiceMethod` を引いて、`handler` を呼ぶ。interceptor(`Interceptor` の関数)は、`ServiceCall` と `ServiceMethod` を受けるので、リクエストの値は渡らないが、`ServiceMethod.handler` の呼び出しは、1 か所で、登録した全部が候補になる(生成された `_Pre` のラッパー経由)。
- **Rust(tonic)**: 生成されたコードの、メソッドごとの `tonic::server::UnaryService` の実装を、`Grpc::unary(svc, req)` が呼ぶ。tower の層(`tower::Layer`、`Service::call`)は、トレイトのメソッドの呼び出しなので、候補は、プログラムの中の、そのトレイトの実装の全部になる。

3 つとも、「ハンドラの型が、メソッドをまたいで同じ」ことと、「登録の対応(どのメソッドがどのハンドラか)を、呼び出しが使っていない」ことが原因。

## 方向(この課題は、memos の目標のあとに始める)

1. **リクエストの型で絞る**: `handler(ctx, req)` の `req` の動的な型(型のアサーション、`info.FullMethod`)から、候補を絞る。
2. **登録の対応を追う**: 登録(`RegisterXServer`、`$addMethod`、`add_service`)のとき、メソッドの名前とハンドラを結んで、呼び出しの文脈(メソッド名)で、候補を 1 つにする。
3. **interceptor の連なりを、行(モデル)で表す**: ライブラリの中の呼び出しは読まずに、「interceptor に渡したリクエストは、そのメソッドのハンドラだけに届く」と、モデルで言う。

どれを採るかは、この課題で決める。memos の目標のあとの、最初の候補の 1 つ。

優先度: 中の上(gRPC を使う、Go、Dart、Rust のサーバーの全部で、メソッドの数だけ finding が増える)。
