# 観測した呼び出しとの照合(試行)

設計は [../../../docs/DESIGN-observed-calls.md](../../../docs/DESIGN-observed-calls.md)。**CI では実行しない**。zoo の Dart を 1 回通し、実際に起きた呼び出しのうち、rivus のグラフにないエッジを出す。

| ファイル | 役目 |
|---|---|
| `compare.py` | 記録した stack と、`rivus index` が作ったグラフ(SQLite)を比べ、エッジごとに `ok` / `no_edge` / `no_callsite` / `callee_unresolved` を出す |
| `instrument.dart` | 方法 B。cells の**コピー**の、全部の関数の先頭に `__rv.rvEnter()` を差す(構文だけ。型は見ない) |
| `test_compare.py` | `compare.py` の単体テスト(`python3 -m unittest discover -s bench/zoo-dart/observed`。CI では実行しない) |
| `vs_miss.py` | `@miss` の行ごとに、同じファイルに「グラフにない観測のエッジ」があるかを数える。`--others` で、`@miss` のないファイルのエッジも出す |

記録の側は `bench/zoo-dart/exec/zoo_mark/lib/zoo_mark.dart`(`ZOO_STACKS=1` のときだけ stack を持つ。ふだんの出力は変わらない)と、`exec/test/zoo_test.dart`(`ZOO_STACKS_OUT` に書く)にある。

## 手順(リポジトリの根から。`$T` は作業用のディレクトリ)

```sh
# 0. 準備: rivus を作り、extractor と cells の pub get
go build -o $T/rivus ./cmd/rivus
(cd extractor/dart && dart pub get) && (cd bench/zoo-dart/cells && dart pub get)

# 1. グラフ(元の cells から)
(cd bench/zoo-dart/cells && RIVUS_DART_EXTRACTOR="dart run $PWD/../../../extractor/dart/bin/rivus_dart.dart" \
  $T/rivus index -C . -graph $T/zoo-dart.db ./...)

# 2. 方法 A: 今ある harness で、sink を呼んだ所の stack を取る
(cd bench/zoo-dart/exec && dart pub get && ZOO_STACKS=1 ZOO_STACKS_OUT=$T/stacks.json dart test test/zoo_test.dart)
python3 bench/zoo-dart/observed/compare.py $T/stacks.json $T/zoo-dart.db --json $T/zoo-A.json
python3 bench/zoo-dart/observed/vs_miss.py $T/zoo-A.json bench/zoo-dart/cells/lib package:zoo_cells/ --others

# 3. 方法 B: cells と exec をコピーして、コピーの関数の先頭に印を差し、全部の呼び出しの stack を取る
mkdir $T/zoo2 && cp -R bench/zoo-dart/cells bench/zoo-dart/exec $T/zoo2/
dart --packages=extractor/dart/.dart_tool/package_config.json bench/zoo-dart/observed/instrument.dart $T/zoo2/cells/lib
# 骨組みのパッケージの中の関数も取るには、zoo_mark に依存するパッケージの lib も、同じように 1 つずつ渡す(例: $T/zoo2/cells/packages/googleapis_auth/lib)
(cd $T/zoo2/exec && dart pub get && ZOO_STACKS=1 ZOO_STACKS_OUT=$T/stacks-enter.json dart test test/zoo_test.dart)
python3 bench/zoo-dart/observed/compare.py $T/stacks-enter.json $T/zoo-dart.db --line-only --json $T/zoo-B.json
python3 bench/zoo-dart/observed/vs_miss.py $T/zoo-B.json bench/zoo-dart/cells/lib package:zoo_cells/ --others
```

方法 B は差した分だけ同じ行の桁がずれるので、`--line-only`(同じ行の呼び出し点のどれかが、その呼び出し先へのエッジを持てば `ok`)で比べる。グラフは、差す前の元の cells から作る(行番号が同じ)。

## Dart の `no_edge` の一覧と分類(2026-11-26、rivus d31c657c)

台帳(`bench/maturity-ledger.tsv`)の Dart の `no_edge` は、**方法 A**(上の手順の 2。`bench/maturity_ledger.py` の `collect_dart_edges`)の `compare.py`(`--line-only` なし)が出す `call no_edge` の件数である。範囲は zoo-dart の cells(`bench/zoo-dart/cells`、`cells/packages` を含む)だけで、`bench/dart-curated` は入らない。方法 B ではない(`clos/returned` などの方法 B の 91 件の数え方とは別)。

### 一覧と作り方

- 一覧: [`no-edge-2026-11-26-d31c657c.tsv`](no-edge-2026-11-26-d31c657c.tsv)(64 行 + 見出し。9 KiB 未満)。列は `root`(下の表の根)、`from`(呼び出し元と位置)、`to`(呼び出された関数)、`site_kind`、`site_ncand`(グラフの呼び出し点の種類と候補の数)、`cases`(観測したセル)。
- 作り方: `compare.py $T/stacks.json $T/zoo-dart.db --json $T/zoo-A.json` のうち `kind == "call"` かつ `status == "no_edge"` の 64 件を取り、`root` 列は `to` のファイルから、下の表の対応で手で付けた(根を出す道具はない)。元の JSON(186 KiB)は保存しない。上の手順の 2 を、`origin/main`(d31c657c)で、そのまま実行した。実行の前に `go test ./cmd/rivus -run '^TestZooDartGenerate$' -update` で、cells の生成物を作る(`maturity_ledger.py` と同じ)。
- 件数(事実): `call no_edge 64`、`call ok 434`、`call no_callsite 28`、`sdk ok 76`、`sdk callee_unresolved 3`。台帳の最新の行(2026-11-25、411ff404)の `no_edge` 64、`calls_observed` 526(= 64 + 434 + 28)と同じ。

### 根の定義

根の記号は、`docs/DESIGN-callouts.md` 5.6、#99、#196、#206、#209、#210 に出る。**A、B、H は、どの文書、課題、コミットにも定義が見つからなかった**(`git log --all -S` と `grep` で確認した)。使われている記号は C、D、E、F、G、I だけで、この一覧の 64 件に出るのは C、D、E だけである。F、G、I は直っている(#465、#466、b2d5afc8)。

| 根 | 定義(出所) |
|---|---|
| C | 同じヘルパーに渡すクロージャが 33 以上で、候補が上限(32)で切られる(#77、#196。C という名前は #196 の分類と #210 の数からの読み。5.6 に定義の行はない) |
| D | フィールド・map・リスト・グローバル・コンストラクタ引数に入れた関数値を、あとで呼ぶ(候補 0)(5.6、#99) |
| E | 依存・骨組みが、渡されたコールバックを自分の map / リストから呼ぶ(`Router.call`、`Navigator.pushNamed`)(5.6、#206、#209) |
| F、G、I | 返したクロージャ、パラメータで受けた関数、代入し直した変数。どれも直った(5.6) |
| 比べる側の列 | 根ではない。この一覧で見つけた、`compare.py` の列の合わせの問題(下の 62 と 64 の差) |

### 分類

| 根 | 件数 | 例 | 直す / 直さない(理由) | 関連課題 |
|---|---|---|---|---|
| C | 36 | `clos/helper-overflow-many`(`transform` の `f(s)` が 33 本目以降のクロージャを呼ぶ。候補 0) | 直さない(上限は意図した切り方、ADR-118。流れの見逃しは `@miss(depth_limit)` で追っている)。残す作業は、エッジの欠けを uncertainty に出すこと(#196 のすること 4) | #77、#196 |
| D | 20 | `clos/stored` 6、`dyn/func-field` 4、`dyn/func-map` 4、`clos/partial-callers` 3、`clos/capture-write-by-ref` 1、`dart/tearoff-instance` 1、`dart/closure-to-sdk` 1 | 直す(`closure-to-sdk` 以外 19 件は #210 の要素の持ち主つきの運び方で。実装は段階 2 以降で、まだない)。`closure-to-sdk` 1 件は別で、モデルの行で言う。どちらも手間は大 | #99、#79、#210、#196 |
| E | 6 | `Router.call`(`dart/shelf-router-params`)3、`Navigator.pushNamed`(`dart/flutter-route-args`)3 | 直す(D のあと。#210 と同じ仕組み)。`calls:` の書式では書けない(MODELING.md)。流れは zoo で検出済みなので、優先度は低い | #206、#209、#210 |
| 比べる側の列 | 2 | `dart/operator-index-split` の `b[0] = b[0]` の `[]=`(vuln と fixed) | 直す(道具の側。グラフにはエッジがある。下) | -(課題なし。この PR で書く) |

D と E の件数は、#99、#210 の 20 と 6 と同じ。`site_ncand` が 0 のものが大半で、`clos/partial-callers`(候補 1。もう 1 つのクロージャが欠ける)だけ 1 である。

### 62 と 64 の差(一覧で確かめた)

文書(5.6、#99、#210)の「C 36 + D 20 + E 6 = 62」と、台帳の 64 の差は 2 件で、**`dart/operator-index-split` の `[]=` 2 件**(`Box.[]=` と `BoxFixed.[]=`。2026-11-24 の 037887a4 で足したセル)である。これはグラフのエッジの欠けではない。グラフには `b[0] = b[0]` の行に、`[]=`(列 3)と `[]`(列 10)の呼び出し点があり、`[]=` の呼び出し点は `Box.[]=` へのエッジを持つ。VM が報告する列は 4(`[` の位置)で、`compare.py` は「フレームの列以降で一番左の呼び出し点」を選ぶので、列 3 の `[]=` を飛ばして列 10 の `[]` を選び、`no_edge` になる。**`--line-only` で同じ stack を比べると `no_edge` は 62 で、差の 2 件だけが消え、増えるものはない**(確認した)。

つまり、台帳の 64 は、62 の根つきの欠け + 比べる側の列の合わせの 2 件である。前の担当が想定した「62 = 方法 B」「別の根が 2 件ある」ではない。

### 推測(確かめていない)

- `[]=` の列の合わせは、`compare.py` で、代入の形(`a[i] = v`、`a.x = v` などの setter)では、`=` の左の式の始まりも候補に入れれば直ると読める。`dart/setter_forms` などの `no_callsite`(28 件のうち、演算子とセッターの形が目に付く)も同じ原因の可能性がある。
- 台帳の数を 62 にそろえるには、`compare.py` の既定を `--line-only` にするか、列の合わせを直す。どちらも、台帳の過去の行との比較を変える(その日から別の列の意味になる)ので、行を足すときに注記が要る。
- A、B、H は、最初の分類(2026-11-24 の 91 件)で、のちに名前が消えた根(直った、または統合した)の可能性がある。出所は見つからない。

### まとめ

62 件の根つきの欠けは、直す方向が決まっている(C は直さない、D と E は #210)。**この PR は直さない**。分類と、一覧の保存だけ。

## 照合の誤りを直したあと(2026-11-26、433ca373)

`compare.py` の列の合わせを直した(上の「62 と 64 の差」の原因)。呼び出し先が演算子かセッター(`[]`、`[]=`、`x=`、`==` など)のとき、同じ行で、フレームの列より左にあり、エッジがその呼び出し先に届く呼び出し点を探す。呼び出し先の名前で見分けるので、別の呼び出しを取り違えない(単体テストは `test_compare.py` の `Operators`)。`--line-only` を既定にする案は採らなかった(本物の列の違いを見落とす)。

- `no_edge`: 64 → **62**(C 36、D 20、E 6。上の分類の「比べる側の列」2 件は消えた)。`ok` 434 → 449、`no_callsite` 28 → 15。`calls_observed` は 526 のまま。台帳の行: 2026-11-26、433ca373。古い行は書き換えていない。
- 一覧: [`no-edge-2026-11-26-433ca373.tsv`](no-edge-2026-11-26-433ca373.tsv)(62 行)。古い一覧([`no-edge-2026-11-26-d31c657c.tsv`](no-edge-2026-11-26-d31c657c.tsv))は履歴として残す。入力(stack とグラフ)は d31c657c の作りで、433ca373 との差は文書だけ。

### `no_callsite` 28 件の分類(一覧から)

`no_callsite` は、呼び出し元の行に、グラフの呼び出し点がない(または、列の合わせで選べない)。

| 原因 | 件数(直す前 → 後) | 例 | 見逃しか | 扱い |
|---|---|---|---|---|
| 列の合わせ(演算子・セッター・`super` の演算子。VM は演算子の列、グラフはレシーバの始まり) | 13 → 0 | `dart/operator-forms` の `[]`、`[]=`、`setter-forms` の `x=`(`a.x += 'v'`)、`super-operator-static` | 見逃しではない(グラフにエッジがある)。照合の誤り | 直した(上) |
| 依存 `package:collection` の中の呼び出し(`firstWhereOrNull`、`lastWhereOrNull`、`singleWhereOrNull`、`none`、`whereNot` が、渡されたクロージャを呼ぶ) | 15 → 15 | `dart/collection-where-or-null` の vuln、fixed、decoy の 3 件ずつ | 流れの見逃しではない(流れは zoo のセルで検出)。グラフの `package:collection` の関数は 5 件で、位置は行 5、9 など(本物の本体の行 256、283 などではない)、`iterable_extensions.dart` の呼び出し点は 0(**事実**)。モデルが本体の代わりに立っていると読める(推測。モデルの行はグラフのエッジを作らない: #196 の追記 3) | 直さない。推測のとおりなら、モデルの設計で、`calls:` の書式に足りない形(#196 の追記 3)の側 |

残りの 15 件に、見逃しの根(C、D、E)に当てはまるものはない。`sdk callee_unresolved` 3 件(`WhereIterator.moveNext` から入る `whereNot` の内側のクロージャ)も、同じ依存の形である。
