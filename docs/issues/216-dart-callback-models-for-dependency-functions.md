# #216 Dart: 関数値を渡す依存の関数に `calls:` のモデルがなく、依存の中で呼ばれる流れを追っていない(#210 段階 1)

- 状態: **partial**
- 補足(状態): **部分的に解決**(2026-11-25)。標準ライブラリ(`dart:core`: `Map.map`、`Map.removeWhere`、`List.removeWhere`、`List.retainWhere`、`List.indexWhere`、`List.lastIndexWhere`、`Set.removeWhere`、`Set.retainWhere`、`Iterable.takeWhile`、`Iterable.skipWhile`)と、`package:collection` の `firstWhereOrNull`、`lastWhereOrNull`、`singleWhereOrNull`、`whereNot`、`none` に `calls:` を足した(`dart-core.yaml`、`dart-collection.yaml`。標準ライブラリは `TestZooCallbacksDart`、`collection` は zoo のセル `F/dart/sdk-callback-*`、`F/dart/collection-where-or-null`)。残り: `protobuf` の生成コードの登録(汚染を運ばない形)、`retry`、`json_annotation`、`collection` の添字つきの形(`forEachIndexed`、`mapIndexed`、`whereIndexed` ほか)と `forEachWhile`、`Iterable.expand` と `List.sort` の `calls:`
- 種類: サイレントな見逃し(通知は全体の一覧だけ。finding には、その呼び出しを通る経路にだけ付く)
- 言語: Dart
- ROADMAP の節: 3
- 守るもの: `cmd/rivus` の `TestDartFuncValueIntoDependency`(通知が付く形と、付かない形)

優先度: 中。通知 `func_value_into_dependency`(ADR-008 の追記)が付く呼び出しの先は、コールバックを呼ぶ API の一覧を、決め打ちでなくモデルで持つこと(ROADMAP 3.4、6.3「呼び出し」)の候補の表になる。

## 何が起きるか

#210 段階 1 は、Dart の自分のコードの関数値(`FUNC` の要素)を、通常のパスで運ぶ。依存の関数の要約は `FUNC` を出力に運ばない(運ぶと、実プロジェクト(約 2 万関数)の測定で要約の要素が 47 倍、時間が 8.5 倍になった)。そのため、自分のコードの関数値を、`calls:` のモデルのない依存の関数へ渡すと、依存の中でその関数値が呼ばれる流れを追わない。この呼び出しには、新しい kind `func_value_into_dependency`(`attributable_to: rivus`)の通知が付く。

**限界(隠さない)**: 落とした流れが finding にならないとき、finding には何も出ない。通知が見えるのは、全体の一覧(依存の関数ごとに 1 件。場所の数と関数の数は detail)と、その呼び出しの要約を適用した経路の finding だけ。測った 4 つの対象(2 万関数の Dart のサーバー、dart-pub、dart_services(途中まで)、dart-archive)では、finding に付いた通知は 0 件だった。これは `unmodeled_library_call` と同じ性質で、`calls:` の行を書くと、通知が消え、流れも追える。

## 確認の方法

`rivus analyze` の出力で、`uncertainties` の `kind` が `func_value_into_dependency` のものを数える(`symbol` が依存の関数、detail の最後の `[N places in M functions ...]` が数)。1 つの実プロジェクト(約 2 万関数)では 71 件(一覧の 0.69%)。上位の依存の関数:

| 依存の関数 | 場所 | 関数 | 種類 |
| --- | --- | --- | --- |
| `protobuf` の `BuilderInfo.aOM`、`aOS`、`new`、`pc`、`a`、`e`、`m` ほか(生成コードの登録) | 187、181、162 ほか | 75〜162 | コンストラクタ・生成関数の参照(`createEmptyInstance`、`subBuilder`) |
| `protobuf` の `GeneratedMessage.copyWith`、`$_defaultFor` | 162、162 | 162 | `updates` のクロージャ、生成関数 |
| `retry` の `RetryOptions.retry` | 17 | 16 | 再試行する処理 |
| `json_annotation` の `$checkedCreate` | 13 | 13 | 変換のクロージャ |
| `collection` の `IterableExtension.firstWhereOrNull` ほか | 10 | 8 | 条件のクロージャ |
| `dart:core` の `Map.map`、`List.removeWhere`、`Map.removeWhere`、`List.indexWhere` | 7、5、5、2 | 7、5、4、2 | 条件・変換のクロージャ |
| `package:github` の `GitHub.request`、`getJSON` ほか | 1〜5 | 1〜5 | リクエストの前後の処理 |

`symbol` のない通知(21 件)は、呼び出し先が関数として決まらない呼び出し(`$checkedConvert` に渡すクロージャなど)。

## 案

`calls:` のモデルを、上の表の上から足す(MODELING.md 4d 節)。SDK(`dart:core`、`dart:collection`)と `collection` は、コールバックを呼ぶ API の一覧を、決め打ちでなくモデルで持つ形にする。生成コードの登録(`protobuf`)は、渡すのがコンストラクタの参照で、汚染を運ばない形(戻り値は新しいインスタンス)なので、`pure` に近いモデルで足りる見込み。**この課題は、#475 には混ぜない**。
