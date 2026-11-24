# #209 Dart: 文字列で引く登録表(`Router.call`、`Navigator.pushNamed`)が呼ぶ関数が、グラフのエッジにならない(調査と設計。実装なし)

- 状態: **open**(設計のみ。実装は #99 の根 D の続きで行う)
- 種類: 道具・ベンチ(グラフのエッジ)。登録した関数の中の流れは、今も見つかっている
- 言語: Dart
- ROADMAP の節: 1
- 守るもの: zoo `F/dart/shelf-router-params`、`F/dart/shelf-router-extra-param`、`F/dart/flutter-route-args`(今ある。エッジの検査は観測との照合、`bench/zoo-dart/observed`)
- 題: Dart: 登録表から引いて呼ぶ関数(`no_edge` の根 E、6 件)の仕組みと設計

関連: #99(値として持たれた関数の呼び出し。根 D)、#196(観測した呼び出しのうち、グラフにないエッジの分類。根 E はその 2)、#81、ADR-050、ADR-106、ADR-118。

## 調査: 登録と呼び出しの形

**どちらも、登録の側は静的な呼び出しで、呼ぶ側は「フィールドに入れた入れ物から取り出した関数値」の呼び出し**になる。呼ぶ側の呼び出し点は、グラフでは `kind: value`、候補 0 件(uncertainty は付かない)。登録した関数は、呼ぶ側のエッジの先にならない。

| | 登録 | 引いて呼ぶ所 | IR の形(zoo の骨組みで確認) |
|---|---|---|---|
| `Navigator.pushNamed`(zoo の `zoo_flutter`) | `nav.routes['/detail'] = (settings) {...}` | `routes[name]!(RouteSettings(...))` | 登録: `field_load routes` → `index_store`(キー付き)。呼び出し: `field_load routes` → `index_load` → `call(value)` |
| `Router.call`(zoo の `shelf_router` の骨組み) | `router.get('/item/<id>', (Request r) {...})` → `add` → `_routes.add((route, handler))` | `for (final (route, handler) in _routes) ... handler(req)` | 登録: `field_load _routes` → `alloc record` + `field_store $1/$2` → `List.add`。呼び出し: `field_load _routes` → `iter.start/next` → `field_load $2` → `call(value)` |
| `Router.call`(実物の shelf_router 1.1.5。`~/.pub-cache` のバージョンで、`rivus index` を実行して確認) | `Router.get` → `add` → `Trie.add` → `RouterEntry(...)` 工場 → `RouterEntry._`(`this._handler` の初期化つき引数)。入れ物は `Trie` の節の `routes` リストと、エントリのフィールド `_handler` | `Router.call` → `route.invoke(...)` → `RouterEntry.invoke` の中の `_handler(request)`(`router_entry.dart` の 136 行と 146 行)。どちらも `kind: value`、候補 0 件 | ユーザーの関数から、`_handler` への書き込みまで、**引数の中継が 5 つ**(`get` → `add` → `Trie.add` → 工場 → `_`)。`maxDepth`(3)を超える |

実物の shelf_router は、`bench/dart-corpus` と `bench/dart-services` の依存にある。ここで実行して確かめたのは、zoo と、実物の 1.1.5 だけを置いた小さい試し(`router.get('/a/<id>', h1)` と、関数リテラルの登録)。dart-services の本体は、重い実行になるので、実行していない。`@Route.get` の生成コードは、`router.add('GET', r'/x', service.method)` の形(インスタンスメソッドの tear-off を渡す)で、登録の側が変わるだけ。呼ぶ側は同じ。

`Navigator.pushNamed` の実物(Flutter)は、`MaterialApp(routes: {...})` → `WidgetsApp.routes` → `onGenerateRoute` の中の `routes[name]` と、入れ物が**コンストラクタの引数を通って、別のクラスのフィールドへ渡る**。zoo の骨組みより遠く、Flutter のソースを読む実行で確かめる必要がある(ここでは確かめていない)。

## 設計の選択肢

**A. モデルで言う(新しい行の種類。登録の呼び出しの関数を、呼ぶ側の関数のエッジの先にする)**
- 今の書式では書けない(#196 の追記 3、`MODELING.md`)。`calls:` は、その呼び出しの中で呼ばれる関数だけ。モデルの行はグラフのエッジを作らず、エッジには呼び出し点がいる。
- 足すなら、行の種類(`registers:` と呼ぶ側の関数)、Go / Rust / Dart の読み込み、ADR、`bench/model-diff` の扱い、が要る。フレームワークごとに手で書く行になる。
- 依存の**本体は既定で読んでいる**ので、入れ物と呼び出しはコードに見えている。モデルで言うと、読める情報を、手で二重に言うことになる。実物の shelf_router では、呼ぶ点は `Router.call` でなく `RouterEntry.invoke` の中なので、行がフレームワークの内部の名前を持つことになり、バージョンが変わると壊れる(`verified_versions` の範囲を広く取れない)。
- 向く所: 本体を読まない依存(`-no-deps`、`body_replaced_packages`)。ここは、今は `unmodeled_library_call` が付く。
- 結論: **主な道にはしない**。

**B. `internal/valuecalls` で、フィールドと入れ物の中身をたどる(#99 の根 D と同じ仕組み。推薦)**
- 呼ぶ側の値の元が `field_load F`(のあとの `index_load`、イテレーションの要素、レコードの `$2`)のとき、**そのフィールドの入れ物に書き込まれた関数値**(`index_store`、`List.add`、`field_store`、コンストラクタの初期化つき引数)を、候補にする。書き込む値は、今の `funcsOf` で、引数をたどって、`make_closure` か関数まで行く。
- `-resolve-func-values` を消した今の仕組み(関数値の候補を `ir.Callee.ValueCands` に持ち、グラフと taint が同じ候補を見る、`valuecalls.go` の冒頭)の中で自然。候補が 33 以上なら、今の `candidate_limit` で切れて、uncertainty が付く。
- **偽の edge の扱い(ADR-118)**: フィールドは、インスタンスを区別しないので、同じクラスの別のインスタンスに登録した関数も候補に入る。**過大近似なので、候補の呼び出し点には必ず uncertainty を付ける**(新しい原因 `field_stores`: 「フィールドや入れ物の書き込み全部を候補にした。キーとインスタンスは区別していない。呼ばれない関数が入りうる」。`ValueWhy` に足す。edge の `certainty` は tear-off の `$bound` と同じ `uncertain`)。取り出しの元が分からない(別のクラスの入れ物から渡された、など)ときは、今と同じ(候補 0 件のまま)で、`untraced_value` の `partial`。
- **精度の問題(zoo で確かめる必要がある、最大の危険)**: zoo の `vuln` / `fixed` / `decoy` は、**それぞれが別の `Router()` を作る**。インスタンスを区別しない候補では、`vuln` の `router(request)` が、`decoy` のハンドラも呼びうることになり、`typed()` の値が、別のセルの sink に届く偽の finding が出る見込み(実装して確かめていない。出れば zoo のゲート違反。`decoy` はこの種の誤りを見るためにある)。このため、**候補に「入れ物の持ち主」を付ける**: 呼び出し点のレシーバを、`Router.new`(`alloc`)まで(呼び出し元の引数をたどって)たどれて、書き込みの側のレシーバと同じ alloc に着くものだけを、その呼び出しの候補にする(今の `ValueCand.Param` が「どの引数を通ってきたか」を持つのと同じ形で、「どの持ち主か」を持たせ、呼び出し元が、自分が渡した持ち主だけを残す)。持ち主が決まらないときは、全部の書き込みを候補にして、uncertainty を付ける。
- 実物の shelf_router では、呼び出し元をたどる深さが足りない(上の表の 5 つ)。**書き込みの側の追跡だけ、別の深さ(6 ほど)にする**か、`RouterEntry._handler` のような「コンストラクタの引数が、そのまま 1 つのフィールドに入る」形を、1 つの中継として数えない。深さを上げるコストは、dart-services で測る。上げないと、今の `caller_depth_limit` の uncertainty が付く(見逃しではない)。
- `Navigator.pushNamed` の実物は、入れ物のコンストラクタ引数を通った受け渡し(`MaterialApp.routes` → `WidgetsApp.routes`)を、**入れ物の alloc までさかのぼってたどる**必要がある(リテラル `{...}` の `index_store`)。段階 2。

**C. extractor で、`Router.get` など、名前で決める**
- 却下: 特定のライブラリ名を、解析の中に持ち込む。モデルの仕事(A)か、一般の仕組み(B)のどちらか。

## 推薦と大きさ

**B を、#99 の根 D の実装の一部として行う。E だけを先に切り出さない**(登録表の形は、D の `clos/stored`、`dyn/func_map`、`dyn/func_field` と同じ仕組みで、E だけの規則はない)。段階:

1. **フィールド + 入れ物の書き込みの索引**(IR を 1 回読む。書き込む関数を読むので、extractor が「入れ物のフィールドに関数値を書く関数」にマークを付けるか、`ir.Walk` で全部読むか、は、メモリのコストで決める。Rust の `indexFields` と同じ位置づけ)と、呼び出し点の持ち主の追跡。`internal/valuecalls` に約 400〜600 行とテスト。zoo の骨組みの `Router.call` 3 件と `pushNamed` 3 件、ほか D の約 20 件が対象。
2. 実物の shelf_router 用に、書き込みの側の深さ。
3. Flutter の `routes` の受け渡し(入れ物の alloc までさかのぼる)。

大きさ: 段階 1 は**大**(`valuecalls` の新しい解析。taint が同じ候補を使うので、zoo の期待値、curated、dart-services の合格ライン D の測り直しが要る)。E の 6 件の `no_edge` が減る数(65 → 59 の見込み)は小さいが、`Router.call` は Dart のサーバのコードの入口の呼び出しなので、**実物では、`rivus graph path` の「入口 → ハンドラ」が `exhaustive: false` のまま「参照のフェーズ」でしか届かない**という欠けが、これで閉じる。

**注意**: 持ち主を区別しない候補は、zoo で偽の finding を出す(上)。区別せずに入れる案は取らない。区別して uncertainty を付ける形だけが、uncertainty のない見逃しにも、偽の経路にもならない。

## この課題でしたこと

調査と設計のみ。コード、セル、ベンチの数(`no_edge` 65 / `ok` 749)は変えていない。
