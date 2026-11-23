# #197 Go: `for … range f` が呼ぶイテレータの呼び出し点に、行がない(位置が 0)

- 状態: **open**
- 種類: 道具・グラフ(呼び出し点の位置)
- 言語: Go
- ROADMAP の節: 1
- 守るもの: -(流れの結果は変わらない。グラフを位置で引く道具に効く)
- 題: Go: 関数を `range` で回す文が呼ぶ呼び出し点の行が 0 になる

## 何が起きるか

`for v := range seq`(`seq` が `func(yield func(T) bool)`)は、`seq(body)` の呼び出しに書き換えて読まれる。この呼び出し点は、グラフの `callsites` に **line 0 の `value` の呼び出し**として入り、元の `for` の行を持たない。

見つけた方法: Go の zoo(`bench/zoo-go`)を実行して観測した呼び出し([DESIGN-observed-calls.md](../DESIGN-observed-calls.md)、`bench/observed-edges`)をグラフと比べたとき、`clos/range_func` の 3 つの関数(vuln / fixed / decoy)の `range` の行で、観測した呼び出しに合う呼び出し点がなかった。同じ関数に、line 0 の呼び出し点がある。zoo 全体の観測 663 本のうち、これが 3 本。

## 影響

- エッジはある(呼び出し先も正しい)。流れの結果にも影響がない(zoo の `clos/range_func` は検出できている)。
- 呼び出しを**位置で引く**道具(`rivus graph` の位置指定、観測との照合)で、`range` の行から呼び出し点に着けない。

## すること

1. どこで line 0 になるかを調べる(SSA で `range` が作る呼び出しの位置が、`for` の文の位置を持たない見込み。`internal/frontend/goir` の位置の付け方)。
2. 呼び出し点に、`for` 文の行を付ける。
3. 照合の道具(`bench/observed-edges`)は、直るまで、この形を「照合の外(`no_position`)」に分けている。直ったら、その分け方を外す。

優先度: 低(結果は変わらない。位置で引く道具だけ)。
