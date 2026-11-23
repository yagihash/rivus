# 180: finding が (source, sink) ごとに代表の経路を 1 本しか持たず、同じ sink に届く別の呼び出しの位置が見えない

- 状態: **open**
- 種類: 出力が実際より狭く見える(誤った注記に近い)(重さ: 中の上)
- 言語: 全
- ROADMAP の節: 2
- 守るもの: -

1 つの (source, sink) の finding は、代表の経路を 1 本だけ持つ(エンジンが、入口から届く、不確実性が少ない、短い、の順で選ぶ)。同じ sink に、別の呼び出しの位置を通って届く経路があっても、出力に出ない。利用者は「この 1 か所を直せば済む」と読む。公開の例では、`bench/dart-pub` の zip slip は、1 つの脆弱性が 9 か所の呼び出しで起きるが、finding の経路が通るのはそのうちの一部になる(位置の単位の数は 4/9。ベンチの `through_tp`)。

今の出力が持つ、ほかの経路の情報(`internal/taint/result.go`):
- `other_sources` と `other_sources_count`: 同じ sink に同じラベルで届く、**別の source**(同じ種類の引数など)。別の呼び出しの位置を通る、同じ source の経路は含まない。
- `dispatch_candidates`(finding の注記と `summary`): 動的な呼び出しで、候補の数。経路の数ではない。
- `path_steps`: 代表の経路の長さだけ。

直す方向: finding に、同じ (source, sink) に届く、ほかの経路の数と、それが通る呼び出しの位置の一覧を出す(一覧には上限を付け、超えたら、超えたと言う)。ROADMAP の 5 節(エージェントが問い合わせるインターフェース)の `graph` の答えにも、同じ形(`complete` と合わせて、[#171](171-graph-answer-complete.md))で出す。
