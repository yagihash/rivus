# #245 `calls:` に、レシーバの要素をコールバックの引数にする書式がなく、Rust と Dart の extractor が、標準ライブラリのコールバックの表(25 件と 14 件)を持つ

- 状態: **open**
- 種類: 道具(重さ: 中)
- 言語: Rust / Dart
- ROADMAP の節: -
- 守るもの: zoo の `clos/to_lib` 系のセル
- 進める完了条件: D(1)、D(3)

## 何が起きるか

`Iterator::map`、`Option::map`、`Result::map_err`、`Iterable.forEach`、`Future.then` のように、レシーバが持つ値(要素、`Some.0`、`Ok.0`、`Err.0`)をコールバックの引数にして呼ぶ API は、`calls:` の `with` では書けない(`with` は値の全体の位置だけ。`docs/MODELING.md` の 3 節の「Not yet」)。そのため、extractor が表を持ち、呼び出しを IR に書く。Rust は `extractor/rust/src/lower.rs` の `call_back`(25 件)、Dart は `extractor/dart/lib/exprs.dart` の `_callbackMode`(14 件)。表にない API は、コールバックの呼び出しが落ちる(#80)。

## 直す方向の案

`with` に、レシーバの要素(例 `recv.elem`、`recv.Some.0`)を書ける射影を足す(設計が先。[YAML-LIMITS.md](../YAML-LIMITS.md) の L2)。書式ができたら、表を `rust-std.yaml` と `dart-core.yaml` に移す。#221 の「見直す条件」の 1 つ目と同じ。一覧は [HARDCODED-APIS.md](../HARDCODED-APIS.md) の H10 と H15。
