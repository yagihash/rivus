# #88 Dart: 渡しただけのクロージャ(Timer.run、scheduleMicrotask、Future.delayed、Button(onPressed:) など)の中の sink に、キャプチャした値が届かない(サイレントな見逃し)

- 状態: **closed**
- 種類: サイレントな見逃し
- 言語: Dart
- ROADMAP の節: 1
- 守るもの: zoo `F/dart/closure-to-sdk`(`Timer.run`、`scheduleMicrotask`、`Future.microtask`、`Future.delayed`、`Button(onPressed:)`)
- 題: Dart: 渡しただけのクロージャ(Timer.run、scheduleMicrotask、Future.delayed、Button(…
- GitHub: #88(移行前の番号は同じ。2026-11-21 に開いた。移行時の状態: CLOSED)

## 今の状態(2026-11-22 の見直し)

直っていた(再開後の確認、2026-11-22。GitHub 側のコメント参照)。

## 本文(GitHub から移した写し)

## 確度
実行で確認(origin/main 1a429f0 で、dart-curated のコピーに足して確認)。

## 場所
クロージャは、rivus が「呼ぶところ」を見つけたときだけ、キャプチャの値とつながる(`internal/valuecalls/valuecalls.go` の `funcsOf`、`internal/sg/build.go` の `resolveValueCalls`、`internal/taint/summary.go` の `applySummary`)。プラットフォームの API やフィールドに渡しただけのクロージャは、呼ぶところが見えないので、中の sink にキャプチャの値が届かない。

## 再現(Dart。5 つの形、全部見逃し)
```dart
final t = source();
Timer.run(() => sink(t));                                   // モデルなし。unmodeled_library_call が出るだけ
scheduleMicrotask(() => sink(t));                           // 何も出ない
Future.microtask(() => sink(t));                            // モデルあり(args → result)。何も出ない
Future.delayed(const Duration(seconds: 1), () => sink(t));  // 同上
Button(onPressed: () => sink(t));                           // コンストラクタ → フィールド。何も出ない
```
対照: `final f = () => sink(t); f();` は見つかる。

## 期待と実際
期待: キャプチャした `t` の source が、クロージャの中の `sink(t)` に届く(または、届かないことが不確実性に出る)。実際: 5 つとも出ない(サイレントな見逃し)。

## 影響
Flutter 風のコード(`onPressed: () => ...`)やマイクロタスクで、キャプチャした値の流れが失われる。#76(CAP)、#79(STORE)と同じ根。

## 修正の方向
`make_closure` の命令で、クロージャの要約を、キャプチャの値(`make_closure` の引数)で具体化する(作った場所で適用する)。コールバックの引数に呼ぶ側が入れる値は、案 2(ADR-047)とモデルの「引数 n を呼ぶ」で。呼ぶところが見えないクロージャには、その旨の不確実性を付ける。ユーザーの判断待ち(バックログ C-7)。
フィクスチャ: 上の 5 つ、否定の形(キャプチャしていない値は届かない、2 つのクロージャのキャプチャが混ざらない)、Go と Rust の同じ形。

## 関連
#76、#79、#81、ADR-047

別セッションのレビューの識別子: DART-16

## コメント(GitHub から移した写し)

再開後の確認(2026-11-22、main の zoo で実行): この Issue の 5 つの形はすべて、今の main で見つかる。`Timer.run`、`scheduleMicrotask`、`Future.microtask`、`Future.delayed`、`Button(onPressed:)`(クラスのフィールドに持たせ、呼ぶ形)を、zoo の `F/dart/closure-to-sdk`(`bench/zoo-dart/cells/lib/dart/closure_to_sdk.dart`。実行で裏を取る)に入れた。直したのは assumed_call(ADR-059、C-7)。閉じる。再現が残る形があれば再び開ける。

