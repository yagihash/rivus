# #81 Dart: クロージャが、パラメータで受けた関数を呼ぶ形(g(a)。コールバックのコールバック)で、流れが失われる

- 状態: **partial**
- 種類: サイレントな見逃し(一部)
- 言語: Dart
- ROADMAP の節: 2
- 守るもの: zoo `F/clos/callback-of-callback`(`@miss(depth_limit)`)
- GitHub: #81(移行前の番号は同じ。2026-11-20 に開いた。移行時の状態: OPEN)

## 今の状態(2026-11-22 の見直し)

**残っている形: `g(a)`(クロージャが、パラメータで受けた関数を呼ぶ形)だけ**。関数値の第 2 歩で閉じる見込み。

## 本文(GitHub から移した写し)

## 現象
クロージャが**関数の引数**を呼ぶ(コールバックのコールバック)、またはクロージャが**キャプチャした関数**を呼ぶ(ラッパー、中継)と、その呼び出しが未解決で、呼び出し先の中の sink に届かない。ラッパー(`retry(() => work())` のような形)ではよく起きる。不確実性は、ヘルパーの中のクロージャに `unresolved_callee` が出るだけで、source / sink の関数には出ない。

## 再現(Dart)
```dart
void run(void Function() f) => f();
void relay(void Function() f) { run(() => f()); }

void a() {
  final a = source();
  relay(() => sink(a));      // 期待: source → sink   実際: 出ない(relay$1 の f() が unresolved_callee)
}

String viaInner(String s, String Function(String Function(String)) outer) => outer((x) => x + s);
void b() {
  final a = source();
  sink(viaInner('const', (g) => g(a)));   // 実際: 出ない(g(a) が unresolved_callee)
}
```
## `@miss`
`bench/dart-curated`: `relay-c-a` `relay-c-b`(source / sink の関数には uncertainty なし)、`takes-a` `takes-b`。`bench/MISSES.md` の型 `PARAM-CALL`(4 件)。

## 落ちる場所の推定
`internal/valuecalls/valuecalls.go`: `indexCallers`(値の呼び出し経由で呼ばれる関数を callers に入れない。クロージャの引数 `g` の呼び出し元が見つからない)と、`classify` の `refParam`(宣言した引数だけ。キャプチャした関数は `refOther`)。ADR-047(案 2)は、中継を最初のバージョンでは扱わない(条件 3)ので、これも別に直す必要がある可能性が高い。

## コメント(GitHub から移した写し)

再開後の確認(2026-11-22): **残っているのは `g(a)` の形**(クロージャが、自分のパラメータで受けた関数を呼ぶ。zoo の `F/clos/callback-of-callback`、`@miss(depth_limit) @ref(#81)`)。**relay の形(`run(() => f())` キャプチャした関数を呼ぶ)は、今は見つかる**(zoo の `F/clos/relay`)。uncertainty の kind は assumed_call / depth_limit / recursion_approximation で、「再帰のない形で recursion_approximation が出る」疑いは #77 と同じ目で追う。

