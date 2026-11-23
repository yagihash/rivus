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

1. **`local_recursion` の形**: 調べて、直した(下の「追記」)。
2. **依存・フレームワークが呼ぶコールバック**のエッジ(`Router.call`、`Navigator.pushNamed`、依存が返すクロージャ)は、#99 の根(関数値の第 2 歩)で閉じるかを、第 2 歩のあとに、観測の再実行で確認する。
3. `compare.py` を、`$bound` の中継とコンストラクタの名前に対応させる(DESIGN-observed-calls.md の 6 節の 2 の一般化で行う)。
4. 33 以上の候補が上限で切られる(#77)ときは、エッジが欠けることが、注記に出ているかを見る(今は、流れの `depth_limit` の注記だけ。グラフのエッジの側には出ない)。

エッジだけの欠落のセルは、すでに zoo にあり、流れは検出されている。zoo の注釈は流れの正解なので、エッジだけの欠落は、注釈で表せない。

優先度: 中(サイレントな見逃しにはならないが、`rivus graph` で経路が欠けて見える)。

## 追記(2026-11-24。結論)

**1. `local_recursion` の原因と修正。** ローカル関数が、自分の名前で自分を呼ぶとき、extractor は、その名前を「知らない変数」(`a variable that the extractor does not know`、`unsupported`)として読み、候補 0 件の値の呼び出しにしていた。クロージャを作る時点では、関数の名前はまだ宣言されていないので、キャプチャに入らないため。**流れの見逃しにもなる**: 引数を入れ替えて再帰する形(`go(y, x, n - 1)`)で、再帰を通ってだけ sink に届く流れが、見つからなかった(zoo の新しいセル `F/clos/local-recursion` の `vuln` と `decoy` で確認。見逃しの理由は `unresolved_callee,unsupported_construct`)。修正: ローカル関数の本体の中で、その名前を読むときは、関数自身を作り直す(`make_closure`。引数は、自分がキャプチャしたもの)。修正後は、セルの流れが見つかる。修正は `fix/dart-local-recursion`(`extractor/dart/lib/lower.dart`、`exprs.dart`)。限界(セルを書いて確認した。zoo の `F/clos/local-recursion` の `vuln2`、`vuln3`): **入れ子のクロージャの中から外のローカル関数を呼ぶ形**(`vuln2`)は、修正が効かず、流れが見つからない(`@miss(silent)`)。**2 つのローカル関数が互いを呼ぶ形**(`vuln3`。`late` の変数に入れた関数を経由)も、見つからない(`@miss(unresolved_callee)`)。どちらも `@ref(#196)`。Go は、`var f func(...)` に代入する書き方で書いた同じ形(`F/clos/local-recursion`)が、検出される(マークなし)。Rust の同じセルは、まだ書いていない(`@todo(PARITY)`)。

**2. `rivus graph path` の `exhaustive`(zoo の Dart のグラフで実行して確認)。** エッジだけの欠落の 4 つの形で、欠けたエッジを通るはずの問い合わせをした。**経路なしで `exhaustive: true` と答えたものはない**(重い失敗はなし)。経路なしのとき、`exhaustive` は常に `false` で、`exhaustive_of` に「関数が値として持たれたもの、リフレクション、フレームワークからの呼び出しは含まない」と出る(DESIGN-graph.md G-15)。

| 形 | 問い合わせ | 経路 | `exhaustive` |
|---|---|---|---|
| 返したクロージャをその場で呼ぶ(`clos/returned`) | `vuln` → `make1$1` | あり(参照のフェーズ。`certainty: reference`)。`--no-refs` では、なし | 参照ありで true、`--no-refs` で false |
| 変数・リストに入れたクロージャを呼ぶ(`clos/stored`) | `callA` → `vuln$1` | なし(参照のフェーズまで探しても) | false |
| tear-off(`dyn/method_value`) | `vuln` → `Printer.run` | あり(`$bound` を通る。`certainty: uncertain`) | true |
| フレームワークが呼ぶコールバック(`shelf_router` の `Router.call` → ハンドラ、`Navigator.pushNamed` → ビルダー) | `Router.call` → ハンドラ、`pushNamed` → ビルダー | なし(`vuln` → ハンドラは、参照のフェーズであり) | false |

参照で見つかる形は、「呼び出し」ではなく「参照」の経路として、`certainty: reference` で出る。エージェントは、探し尽くしていない、と分かる。

**3. フレームワークが呼ぶコールバックは、モデルの `calls:` で書けるか。** 書けない形がある(`docs/MODELING.md` を読んだ範囲。モデルは書いていない)。
- **書ける**: コールバックを**引数で渡して、その呼び出しの中で呼ばれる**関数(`fn: argN`)。登録のときに、登録の行で呼ばれたものとして扱う。`shelf_router` の `Router.get` は、今、`calls:` でなく `callback_param`(ハンドラの引数を source にする行)で足りている(流れは zoo のセルで検出)。
- **書けない**: 渡したときでなく、**あとで別の関数(`Router.call`、`Navigator.pushNamed`)が、レシーバが持つ関数を呼ぶ**形。MODELING.md の 2 つ目の基準の説明に、「コンストラクタに渡して、あとで呼ばれる関数(`RetryClient(when:)`)は、`calls:` では、まだ書けない」とある。`calls:` の行は、呼び出し先の関数を持たないので、グラフのエッジにもならない(モデルの行は、グラフのエッジを作らない)。
- つまり、書式が足りない。足すなら、「登録の行が持つ関数を、別の関数のエッジとして足す」形になる。エッジだけの欠落(流れは検出されている)なので、優先度は低い。
