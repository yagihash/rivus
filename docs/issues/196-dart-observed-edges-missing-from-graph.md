# 実行して観測した呼び出しのうち、グラフにないエッジ(Dart。zoo と curated の最初の分類)

- 状態: **open**
- 種類: 道具・ベンチ(グラフのエッジ)。一部は流れの見逃しにもなる(既存の課題と `@miss` で追っている)
- 言語: Dart
- ROADMAP の節: 1
- 守るもの: -(エッジだけの欠落は、zoo の注釈(流れの正解)では表せない。観測との照合の再実行で見る)
- 題: Dart: 実行して観測した呼び出しのうち、グラフにないエッジの分類(zoo 91 件、curated 68 件)

[DESIGN-observed-calls.md](../DESIGN-observed-calls.md) の方法 B(全部の入口で stack を取る)を、2026-11-24 の main で、zoo の Dart(415 ケース)と `bench/dart-curated`(全部の引数なしの関数を 1 回ずつ呼ぶ、作業用の runner)に実行し、`rivus index` のグラフと比べた。`no_edge`(グラフにないエッジ)を 1 件ずつ見て、そのセル・ファイルの `@miss`(流れの見逃しの注釈)の有無で、流れの見逃しにもなるかを分けた。

## 結果

| | zoo | curated |
|---|---|---|
| 観測した別々のエッジ | 718 | 515 |
| グラフにない(`no_edge`) | 91 | 68 |
| うち、**流れの見逃しにもなる**(同じセルに `@miss` がある。既知) | 56 | 48 |
| うち、**エッジだけの欠落**(流れはセルで検出されている。`rivus graph` だけに効く) | 25 | 17 |
| うち、グラフの形の違い(`m$bound` という中継のノードを通る。経路はある) | 10 | 3 |
| 今の main で解消済み(2026-11-22 の試行で名前の出た 5 セルは、どれも今も出る) | 0 | - |
| 観測の側の誤り(`callee_unresolved`。`no_edge` の外) | 13 | 0 |
| `toString` の暗黙の呼び出し(`sdk no_ref`。`no_edge` の外) | 6 | - |

数は、2026-11-22 の試行(zoo 39)より増えている。zoo が 187 から 415 ケースになり、骨組みのパッケージ(`cells/packages`)も対象に入ったため。同じ形が、`vuln` / `fixed` / `decoy` で別々に数えられている。

## 分類

流れの見逃しにもなる(既知。追っている課題と `@miss` がある):

- `clos/helper-overflow-many` 36 件、curated の `limits` 34 件: 同じヘルパーに渡すクロージャが 33 以上で、候補が上限で切られる(#77)。エッジも、33 本目から欠ける。
- `clos/callback-of-callback` 4 件、curated の `viaInner` 2 件(#81)。
- `dyn/func_map` 4 件、`dyn/func_field` 4 件、`clos/partial-callers` 3 件(候補が 1 つで、もう 1 つのクロージャが欠ける)、`dart/tearoff-instance` の `Button.press` 1 件、`dart/super-mixin-impl` 4 件(mixin のメソッドへの interface 呼び出し。候補 0)、curated の `Emitter.emit`、`Lazy.value`、`Hub.emit`(共有パッケージ)、`holder`、`curried`、`captured_fn`、`dynamic` のレシーバ(`dynamic_arg`、`dynamic_body`。#87)。

エッジだけの欠落(流れは、セルで検出されている。グラフを読むエージェントには、エッジが欠けて見える。#99 の「候補 0 件の値の呼び出し」と同じ根):

- zoo 25 件: 返したクロージャ `clos/returned` 3、フィールドやリストに入れたクロージャ `clos/stored` 6、`clos/capture-write-by-ref` 1、`dart/closure-to-sdk` 1(#99)。**依存や骨組みのフレームワークが、渡されたコールバックを呼ぶ**もの: `shelf_router` の `Router.call` 3、`Navigator.pushNamed` 3、依存が返したクロージャ(`wrapFn`)の呼びと、その中からの呼び 8(`clos/wrapped-by-dependency`)。
- curated 17 件: `relayCapture` / `relayBoth` 4、`compose` / `composed_*` 6、`Runner.go` 2、`field_function_*` 2、`table_lookup` 1、`dynamic_drop` 1、**`local_recursion`**(ローカル関数が自分を呼ぶ `go(x, n - 1)` が、値の呼び出し・候補 0 件になる。名前で直接呼んでいるので、本来は静的なエッジのはず。推測)1。

形の違い(欠落ではない): メソッドの tear-off の呼び出しは、`m$bound` という中継のノードを通る(`呼び出し点 → Printer.run$bound → Printer.run`)。観測は `呼び出し元 → Printer.run` と見えるので、`no_edge` に出る(zoo 10、curated 3)。比べる側で、中継を 1 段たどって合わせるべき。

観測の誤り(欠落ではない): `callee_unresolved` 13 件は、コンストラクタ(`new ServiceAccountCredentials`、`new RepositorySlug.fromJson`)の名前を、観測の側(`bench/zoo-dart/observed/compare.py`)が、グラフの `X.new` / `X.fromJson` に対応させていない。グラフには、呼び出し元からの静的なエッジがある(確認した)。

## すること

1. **`local_recursion` の形**(ローカル関数の再帰呼び出しが、候補 0 の値の呼び出しになる)を、zoo のセルにして、原因を調べる。流れが落ちる形(再帰の中で値を運ぶ)があるかも調べる。
2. **依存・フレームワークが呼ぶコールバック**のエッジ(`Router.call`、`Navigator.pushNamed`、依存が返すクロージャ)は、#99 の根(関数値の第 2 歩)で閉じるかを、第 2 歩のあとに、観測の再実行で確認する。
3. `compare.py` を、`$bound` の中継とコンストラクタの名前に対応させる(DESIGN-observed-calls.md の 6 節の 2 の一般化で行う)。
4. 33 以上の候補が上限で切られる(#77)ときは、エッジが欠けることが、注記に出ているかを見る(今は、流れの `depth_limit` の注記だけ。グラフのエッジの側には出ない)。

新しいセルは足していない: 上の「エッジだけの欠落」のセルは、すでに zoo にあり、流れは検出されている。zoo の注釈は流れの正解なので、エッジだけの欠落は、注釈で表せない。1 の形だけ、調べたあとにセルにする。

優先度: 中(サイレントな見逃しにはならないが、`rivus graph` で経路が欠けて見える)。
