# ADR-073: フレームワークが呼ぶメソッド(依存の型のメソッドの上書き)を、入口の集合に足す。`reachability` に `from_framework_callback` を足す

- 状態: 有効
- 関連: README
- 要旨: `from_framework_callback`

**背景:** Flutter のアプリは、`build`、`initState`、`dispose`、`didChangeDependencies`、`onPressed` のクロージャなど、**フレームワークが呼ぶメソッド**でできている。Dart の入口は、プロジェクトの top-level `main` だけで、`runApp(MyApp())` の先(フレームワーク)は、読まないか(`--no-deps`)、読んでも `build` までたどれない(読んだ `form_app` でも、届く関数は 18 のまま)。結果、Flutter のアプリの finding は、すべて `not_from_entry` になる(出力からは落ちない)。**入口が立っていないと、ほかの改善(`setState` → `build` の流れ、コールバック)が測れない。**

**調べたこと(Flutter 3.50.1、`flutter/samples` の `a5bccd247163b1017d2eb9320cf3b94a95ccfb5e`。`flutter pub get` だけ。`--no-deps`):** 関数(本体のある、プロジェクトの関数。クロージャを含む)のうち、入口から届く(`Reachable`)ものの割合は 1 割前後。

| アプリ | 関数 | 入口から届く | フレームワークが呼ぶメソッドからも届く | 入口になったメソッド |
| --- | --- | --- | --- | --- |
| `form_app` | 76 | 18 (24%) | 74 (97%) | 13 |
| `navigation_and_routing` | 115 | 13 (11%) | 113 (98%) | 25 |
| `platform_channels` | 74 | 9 (12%) | 74 (100%) | 14 |
| `date_planner` | 87 | 15 (17%) | 83 (95%) | 14 |
| `dynamic_theme` | 31 | 5 (16%) | 31 (100%) | 6 |
| `testing_app` | 28 | 5 (18%) | 28 (100%) | 5 |
| 6 つの合計 | 411 | 65 (16%) | 403 (98%) | 77 |
| LocalSend `app/`(`221f40a9`。`pubspec.lock` では解決できない: Flutter 3.24.5 を前提に `path`、`intl` を固定し、`dev_dependencies` に `test` があって、`flutter_test` と合わない。`path`、`intl` の固定を緩め、`dev_dependencies` を外して解決した) | 20,522 | 1,546 (7.5%) | 19,750 (96%) | 388 |

入口になったメソッドの上書き元は、`StatelessWidget.build`、`State.build`、`StatefulWidget.createState`、`State.initState`、`State.dispose`、`State.didChangeDependencies`、`ModalRoute.buildPage`、`ReduxAction.reduce` など。`onPressed`、`routes`、`onGenerateRoute` のクロージャは、`build` の中で作られるので、`build` が入口になれば、`Reachable` の既存のエッジ(`make_closure`)で届く(クロージャを入口にする必要はない)。`runApp` の引数の Widget は、コンストラクタが `main` から届くが、`build` は届かない(読んでも届かない)。入口に足したあとに届かないまま残るのは、(`navigation_and_routing` と LocalSend で見た範囲では)`==`、`hashCode`(`Object` の上書き。下の「決めたこと」の 2)と、誰も作らないコンストラクタ。

**IR に `overrides` はあったか:** なかった。`TypeInfo.Supers`(型の直接の親)はあるが、メソッドごとの「どの親のメソッドを上書きしているか」はなかった。Dart の extractor は、解析器の要素モデルから、メソッドごとに判定できる(`InterfaceElement.allSupertypes` の各型に、同名のメソッド・getter・setter があるか)。小さい変更(`extractor/dart/lib/extract.dart` の `_overrides`、`irjson.dart` の出力)。

**決めたこと:**
1. **IR に `Func.overrides` を足した**(足すだけ。`schema_version` は同じ。ほかの言語の extractor は出さない): このメソッドが上書き・実装している、親の型のメソッドの symbol の一覧(`dart:*` のもの、`_` で始まる名前は除く)。
2. **入口の基準(`sg.Program.FrameworkCallbacks`。事実に基づく。名前では決めない):** 依存でない関数で、`overrides` のどれかが、**読んでいるコードの(依存でない)どの関数の symbol でもない**もの。つまり、上書き元を宣言しているのが、読んでいるコードの外(依存の型。読んでいない依存も、読んで `dep` の関数になったものも)。読んでいるコードの別のクラスのメソッドを上書きしているだけのものは入らない(その呼び出しは、`Reachable` の候補のエッジがたどる)。`dart:*`(`Object.toString`、`==`、`hashCode`、`Iterator.moveNext` など)は、`overrides` に入れない: 誰でも呼ぶので、「フレームワークが呼ぶ」と言えない。そのため、`==` と `hashCode` の上書きは、入口にならない。
3. **`Reachable` の定義は変えない。** `ReachableWithCallbacks`(入口 + `FrameworkCallbacks` から届く関数)を足した。`Reachable` は、それの部分集合のまま(`graph path --from-entry` も同じ)。
4. **`reachability` の値を足した:** `from_entry`(今までどおり)、**`from_framework_callback`**(経路に、入口から届かない関数があり、届かないものは、すべて、`FrameworkCallbacks` から届く)、`not_from_entry`(今までどおり。それでも届かない関数がある)。`reachability_note` は、`from_framework_callback` には `taint.FrameworkCallbackNote`(「入口からは届かないが、依存が宣言するメソッドを上書きしている関数から届く。フレームワークが呼ぶ」)、`not_from_entry` には `NotFromEntryNote`(文面を直した: 「フレームワークに値として登録されたハンドラ」、読まなかった依存のコード)を付ける。
5. **結果は変えない:** finding の集合、経路、fingerprint は同じ。**代表の経路の選び方も同じ**: 同じ source と sink の finding の中での並び(`engine.go` の `ra, rb`)は、`from_framework_callback` を、`not_from_entry` と同じ側(届かない側)に数える。ここで「届く」側に数えると、並びが変わりうるので、マークだけの変更でなくなる。
6. 言語: Dart のみ(`overrides` を出すのが Dart の extractor だけ)。Go と Rust は `overrides` が空なので、`from_framework_callback` は出ない。

**確かめたこと:**
- 上の表の 6 つのアプリと LocalSend: 同じ IR から `overrides` を除いたもの(これまでの挙動)と、あるもの(この変更)で、`rivus analyze`(測定のための仮のルール。`TextEditingController.text`、`FormFieldState.value`、`State.widget` を source に、`print`、`debugPrint`、`Uri.parse`、`Text.new`、`pushNamed` を sink に。セキュリティのルールではない)を比べた: finding の集合は同一(id、fingerprint、経路、確度、不確実性が一致。`reachability` と `reachability_note` だけが違う)。`not_from_entry` → `from_framework_callback`: `form_app` 1 → 1、`date_planner` 3 → 3、`dynamic_theme` 1 → 1、LocalSend 10 → 10(`not_from_entry` は 10 → 0)。残りの 3 つのアプリは、仮のルールで finding がなかった。
- zoo の Flutter の骨組みの、アプリの形(`bench/zoo-dart/app`。`main` が `runApp(RootApp())`。骨組み `zoo_flutter` に `StatelessWidget`、`TextButton`(`onPressed`)、`runApp` を足した): `StatelessWidget.build` の sink、`build` で作った `onPressed` のクロージャの sink、`State.initState`、`State.dispose` の sink は `from_framework_callback`、`main` と、`main` から呼ぶ関数は `from_entry`、読んでいるコードの別のクラスのメソッドを上書きしたものと、誰も呼ばない関数は `not_from_entry`。`TestDartFrameworkCallbacks` が固定する(ソースの行のコメントと照合)。`sg` の単体テスト `TestFrameworkCallbacks`(依存の型を上書き → 入口、読んでいるコードの親を上書き → 入口でない、依存の関数 → 入口でない)。
- zoo のセル(`bench/zoo-dart/cells`)は、`main` のない library なので、公開の関数が入口で、全部 `from_entry`: これまでと同じ。

**限界:**
- 入口になるのは、**上書き**だけ。フレームワークに**値として**登録される関数(`MaterialApp(routes: {...})` のコールバックは `build` の中で作るので届く。`main` でも `build` でもない場所から、別のオブジェクトに渡されるハンドラ、`Timer`、`Stream.listen` のコールバックなど)は、作った場所から届かなければ届かない。
- `setState` → `build`、`initState` → `dispose` のように、フレームワークが呼ぶ順に、フィールドを通る流れは、見つけない(入口が立っただけ。zoo の `receiver-field-across-entries` は、呼ぶ順を、セルの中で書いている)。入口が立ったので、次に測れる。
- `--no-deps` でなく、Flutter を読んでも、`build` には、フレームワークの呼び出しのエッジでは届かない(`form_app` の `Reachable` は、読んでも 18 のまま。どこでエッジが切れるかは、調べていない)。
- `graph path --from-entry` は、これまでの入口のまま(`from_framework_callback` の入口を足していない)。足すなら、`graph` の `--from-entry` の入口の定義を足す、別の変更。
- LocalSend は、固定した依存のバージョンが、今の Flutter と合わないので、緩めて解決したバージョンで測った。数は参考。

