# #213 Dart: 依存を読むと、`dart:io` の `FileSystemEntity.path` を通る流れが、uncertainty なしで消える

- 状態: **open**(原因 (a) は `fix/dart-fsentity-path-model` で直す。原因 (b) は設計メモのみ)
- 優先度: 1 位(uncertainty のない見逃し)
- 種類: サイレントな見逃し
- 言語: Dart(依存を読む実行。`-no-deps` では出る)
- ROADMAP の節: -
- 守るもの: `bench/dart-curated/lib/fs_entity_path.dart`(`fs_entity_dir`、`fs_entity_link`)。依存の取り込みを外すと見つかる、取り込むと(モデルがなければ)見つからない、の両方を確かめた
- 題: SDK の呼び出しが、依存の候補だけに置き換わって、SDK 本体の分が落ちる

## 1. 事実

`dart_services` で、`-no-deps` だけで出る path_traversal が 2 件ある(`_copyPath` の `Directory.new` と `Link.new`)。依存を読むと消える。

差は `file.path`(`dart:io#FileSystemEntity.path`)。

- `-no-deps`: モデルがなく(unmodeled)、レシーバが結果に流れる。
- 依存を読む: IR の呼び出しは `dynamic`、symbol は `dart:io#FileSystemEntity.path`、`candidates` は `package:file` の実装(`ForwardingFileSystemEntity.path`)だけになる。呼び出しはその候補の要約で置き換わり、SDK 側の分を足す処理がない。
- 候補の `ForwardingFileSystemEntity.path`(`=> delegate.path`)の中の `delegate.path` も、同じ symbol の dynamic で、候補はその関数自身だけ。つまり、再帰していて、底がない。`delegate` も抽象の getter(候補なし)。要約はレシーバに由来する出力を持たない。
- 落ちた場所に uncertainty が付かない。`dart_services` の 2 件は、uncertainty のない見逃し。

## 2. 再現のセル

`bench/dart-curated/lib/fs_entity_path.dart`(`fs_entity_dir`、`fs_entity_link`)と、依存のスケルトン `bench/dart-curated/packages/file`(`ForwardingFileSystemEntity.path => delegate.path` と同じ形)。`pubspec.yaml` と `pubspec.lock` に path の依存を足した(新しい取得はしていない。`bench/dart-curated/pubspec.lock` に `package:file` はなかったので、スケルトン)。

- モデルを足す前: 2 件とも `@miss(silent)`(`TestCuratedDart` が、見つからず、source と sink の関数に uncertainty がないことを確かめた)。
- 対照: `package:file` の依存を pubspec から外すと、同じコードで 2 件とも見つかる(モデルなしでも)。
- source は `File.fromUri(Uri.file(source()))`。`File.new` は組み込みの sink なので、source の行に sink が重なり、`@miss` が付けられない。

## 3. 原因の分類

- (a) `dart:io` の `FileSystemEntity.path` にモデルがない(`File.path`、`Directory.path` にはある)。モデルの欠け。
- (b) 一般の問題: **呼び出しの symbol が `dart:`(SDK。IR に本体がない)でモデルもなく、`dynamic` で、候補が依存の実装のとき、候補の要約だけが使われ、SDK 本体の分(unmodeled の既定の流れと uncertainty)が落ちる**。コード上の場所: `internal/taint/calls.go` の `callBody` 周辺。モデルがある呼び出しは、モデルと候補を足す(ADR-075: `modelled` の分岐)。モデルがない呼び出しには、その足し合わせがない(`external` は、候補が 0 のときだけ)。候補の中で同じ symbol を呼ぶ(再帰)と、底が空になる。
  - 依存の関数の中の uncertainty は、出力に出ない(`collect` が `fn.Dep` を飛ばす。#212)。そのため、候補の側(依存の関数)に uncertainty を付けても見えない。付けるなら、呼び出し側の関数(own のコード)に付ける。

## 4. 修正の方向

### (a) 直接の修正(実装済み)

`internal/model/builtin/dart-io.yaml` に `dart:io#FileSystemEntity.path`(`File.path` と同じ: recv→result、args→result)。セルの `@miss(silent)` を外し、見つかる(`@vuln` が通る)。

### (b) 一般の修正(設計メモ。実装していない)

呼び出しが次を全部満たすとき、候補の要約に加えて、unmodeled の扱いをする: (1) `dynamic`、(2) symbol が `dart:` で、`rules.HasModel` が偽、(3) 候補が 1 つ以上。

- 流れ: 既定の unmodeled の流れ(レシーバと引数が結果へ)を足す。これは、`-no-deps` と同じ結果に戻す(依存を読んで、`-no-deps` の finding が消える、をなくす)。
- uncertainty: `KindUnmodeledLibraryCall`(`externalKind` の既定: "the function has no body and no model")。`dependency_body_unread` は、`skippedBody`(extractor が本体を読めなかった)か、モデルがある呼び出しの依存の候補(`noteDepBodies`)の種類で、ここでは当たらない。**新しい種類は足さない**。呼び出し側(own のコード)の関数に付く。
- 影響: 結果が変わる(finding が増える方向。`-no-deps` との差が縮む)。uncertainty も増える。#212 のとおり uncertainty の数が増えるので、依存の関数の中の呼び出しには付けない(出力に出ないため、作らない)案が自然。
- 大きさの見込み: `callBody` の分岐 1 か所 + `unmodeled` の呼び出し + zoo / curated のセル。小さい(数十行)。測定は、下の「5」。

## 5. 同じ形の数

測定: IR(extractor の出力)で、`dynamic` の呼び出しで、symbol が `dart:`、組み込みのモデルになく、候補が 1 つ以上のもの(上の (b) の条件)。

測定したもの(2026-11-25、組み込みのモデルを足したあとの tree。`dart:io#FileSystemEntity.path` はモデルがあるので数えない):

| 対象 | 数(own のコードの呼び出し / 依存の関数の呼び出し) | 代表 |
| --- | --- | --- |
| dart-curated | 0 / 0 | (モデルを足す前は、own 2 + 依存 1 = このセルの 3 か所) |
| zoo-dart(`cells`、依存のスケルトンつき) | 8 / 0 | `dart:core#Sink.add` 4、`dart:core#Sink.close` 4(`dyn/stdlib_writer_own.dart`。候補は、セルの自前の実装) |
| dart-gcp、cocoon、`dart_services` | **未測定** | 重い実行のロックを別のセッションが保持していて(`dart_services` の割り当ての測定)、IR の抽出を待たせられなかった。`dart_services` と cocoon は、`~/.cache/rivus-bench` のソースを、コピー(`--offline` の `pub get`、Flutter の workspace 員を外す)で抽出できることは確かめた。dart-gcp も同じ |

IR の測定は、要約の測定ではない。依頼の「依存の関数の要約が空 + 本体の底が外部」の数は、実行したあとの要約のダンプ(elems=0)が要る。IR の数は、その上限の見積もり(同じ形の呼び出しの数)。本体の底が SDK で、依存の関数で、要約が空になる関数の数は、未測定。

Go と Rust(コードを読んだだけで、実行していない):

- Go: 本体のない関数の呼び出し(`external`)は、候補がない場合に `unmodeled_library_call` と既定の流れ(引数→結果)が付く。候補が 1 つ以上あり、本体のない関数もある場合も `external` が立つ。Go のローダは標準ライブラリの本体も読むので、interface の呼び出しの候補に標準ライブラリの実装が入る。本体のない宣言(アセンブリ、`//go:linkname`)は、`external` になり、既定の流れで流れは落ちない。依存の関数の中の uncertainty は、出力に出ない(#212)。Dart と違い、流れが落ちる形は、確認していない。
- Rust: 標準ライブラリの実装は IR にない(trait の呼び出しの候補は、読んだ crate の実装だけ)ので、Dart と同じ形(候補が依存の実装だけ + SDK 本体の分が落ちる)がありうる。`bench/rust-curated/src/modelled.rs`(RUST-14)が近い形。IR の数は、未測定。

uncertainty の増え方(推定): (b) の条件に当たる呼び出しの数だけ、own のコードの関数に `unmodeled_library_call` が付く。zoo-dart では最大 8 件(測定した範囲)。dart-gcp、cocoon、`dart_services` は未測定。多すぎる場合は、「要約が空の関数が finding の経路に入るときだけ付ける」に絞る案(`applySummary` が、候補の要約が空のとき、呼び出し側に note を付ける)。それは、(b) の条件に「候補の要約が、レシーバ由来の出力を持たない」を足すのと同じで、実行したあとでないと判定できない。

zoo に形を足す件(依頼 (1)): `bench/zoo-dart/cells` のセルは、exec の実行(`cases.dart`)、Go と Rust の対応するセル、`@na` / `@todo`(ユーザーの承認が要る)がそろう必要がある。このブランチでは足していない。curated のセルが、同じ形を守る。

## 6. 結果の確認(実装 (a))

- `bench/dart-curated`: `TestCuratedDart` が通る。見つかる flow は 2 件(モデルなしでは見つからない。セルの `@miss` で確かめた): `fs_entity_path.dart:16 -> :17`(`Directory.new`)、`:21 -> :22`(`Link.new`)。どちらも、セルの `@source` / `@sink` と一致する。
- `TestZooDart`、`Builtin`、`Model` のテストが通る(zoo の finding は変わらない)。
- `--graph-check dart-curated,zoo-dart` と `dart_services` は、実行していない(ロックの待ちと、uncertainty の変化を見るための実行は、相談役が PR を読んだあとにする)。
