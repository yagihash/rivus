# #211 `bench/compare.sh --graph-check` を Dart に広げる

- 状態: **open**(実装は `feat/graph-check-dart`。PR がマージされたら closed)
- 優先度: 中
- 種類: 道具・ベンチ
- 言語: Dart(Rust は対象外)
- ROADMAP の節: -
- 守るもの: `python3 -m unittest bench.test_graph_check`
- 題: グラフ表現の比較(IR、エッジ、要約、finding、uncertainty、順序)を Dart の対象でも出す

関連: #472(順序の 2 行)、docs/WORKFLOW.md(メモリとコストの変更は、グラフの表現が前後で同じことを示す)、#203、#209、#210。

## 1. 背景

これから Dart の結果を変えうる変更と、変えない変更が続く(根 C、D、E、粗くする安全網の有効化、割り当て削減)。変えない変更は、IR、エッジ、要約、finding、uncertainty が前後で同じことを示す必要がある(WORKFLOW)。Go にはその道具(`--graph-check`)があるが、Dart は手で比べるしかなかった(`rivus index` のエッジ、`rivus analyze` の finding と uncertainty の順序つき SHA-256、`RIVUS_INTERNAL_SUMMARY_DUMP`)。毎回の手作業は抜けやすいので、道具にする。

## 2. 今の道具の制約

- `bench/compare.sh` の `--graph-check` の枝: `spec` が返した `KIND` が `dart` か `rust` のとき「not a Go benchmark」で終了コード 2。
- `bench/graph_check.py`: `-target` は `lang != "go"` のとき弾く。`-dir` の経路は Go の形だけ(`rivus debug extract -all`、パッケージ指定 `./...` を足す)。
- `rivus debug extract` は `-lang go|rust` で、Dart の IR を出せない。Dart の IR は Dart の extractor(`extractor/dart/bin/rivus_dart.dart`)の出力で、`rivus index` と `rivus analyze` は `RIVUS_DART_EXTRACTOR`(または `-dart-extractor`)のコマンドを実行して受ける。`rivus index` に `-ir` はない。

## 3. 設計

Go と同じ 5 行 + 順序の 2 行が、同じ形で出せる。

| 行 | Go | Dart |
|---|---|---|
| IR | `rivus debug extract -all` | そのバージョンの extractor の出力(`compare.sh` の既存の Dart の経路と同じ呼び方)。比較は同じ `graphcheck ir`(`accept.Compare`) |
| edges | `rivus index` のグラフ | `rivus index -lang dart`(`RIVUS_DART_EXTRACTOR` は IR と同じコマンド)。同じ `graphcheck edges` |
| summaries | `RIVUS_INTERNAL_SUMMARY_DUMP` | 同じ(`rivus analyze -lang dart`)。同じ `graphcheck summaries` |
| findings / uncertainties | 集合の比較 | 同じ |
| findings-order / uncertainties-order | 順序つき SHA-256 | 同じ |

- extractor はバージョンの一部なので、base と head は**それぞれ自分のコミットの extractor**で、**head のコード(対象のファイル)**を読む。IR の行は、extractor の変更も見える(変えない変更の PR で IR が違えば、それが指摘になる)。`dart pub get`(対象と extractor)、zoo-dart の生成ファイルは `compare.sh` の `setup`(既存の Dart の経路)に任せる。
- 対象は `dart-curated` と `zoo-dart`(`bench/compare.sh` の `spec`)。ルールは `rules.yaml`、`-no-deps` は付けない(`compare.sh` と同じ)。
- **収束しない対象(zoo-dart は `rivus analyze` が終了コード 4)**: TestZooDart のように再帰のセルを分けて解析する案と、4 のときは比較しない案がある。選んだのは第 3 の形で、**比較はして(止まった時点の状態を同じ道具で比べる)、`convergence: NOT CONVERGED` の行を足して終了コード 3(同じだが全部は比較していない)にする。合格にしない**。理由: (1) 分ける案は `zooSplit` と `zooDartCopy`(Go のテストの中)と同じ作業を bash か Python に写すことになり、写しがずれると「分けた対象」が本体と別物になる。(2) 4 のときに何も比べない案は、zoo-dart で道具が使えない。パス上限で止まる動きは決まった回数(再現する)ので、差が出れば本物の差の可能性が高く、見る価値がある。(3) 終了コード 3 は合格ではないので、「収束したと誤読して合格にする」危険はない。収束した状態を示したいときは、分ける形(5 節)か `dart-curated` を使う。この行は Go にも効く(タイムアウトで止まった実行を、これまで終了コード 0 の `same` にしていた)。
- Rust は対象外(`rivus setup rust` をしない方針)。`compare.sh` は Rust の対象を従来どおり弾く(メッセージは「Go or Dart」)。
- 重い実行のロック(`bench/heavylock.py`)は、`graph_check.py` が各実行の前に取る(既存どおり)。

## 4. 確認

- `python3 -m unittest bench.test_graph_check`(収束の行、Dart のコマンドの組み立てと終了コード 4)。
- `bench/compare.sh --graph-check dart-curated <同じコミット> <同じコミット>` が全行 `same`(終了コード 0)。
- `zoo-dart` は同じ入力で、7 行が `same` で `convergence` の行が出て、終了コード 3 になる見込み(**未確認**: 実行がロックの待ちに当たった。確認したら、ここを書き換える)。

## 5. 残り

- zoo-dart の再帰のセルを分けて、収束した部分を合格にできる形にする(必要になったとき)。
- `-workers` の自己確認(`--graph-workers 1,8`)を Dart でも通す(同じ道具で動く。実行は未確認)。
