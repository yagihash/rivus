# #228 `-timeout` と `-func-timeout` が、`buildPtrAlias` の中で効かない

- 状態: **open**
- 優先度: 高
- 種類: 利用者を止める(約束を破る)
- 言語: 全
- ROADMAP の節: -
- 守るもの: `-timeout` と `-func-timeout` の約束(ADR-120: 完全でない解析は、結果を出したうえで終了コード 4)
- 進める完了条件: -

## 何が起きるか

入力によっては、解析が `buildPtrAlias`(`internal/taint/ptrcells.go`)の中で止まり、`-max-passes 1` でも終わらず、出力が 0 バイトのまま、`-timeout` と `-func-timeout` が効かない。利用者への約束(期限に届いたら、結果を出したうえで終了コード 4)を破る。

## 現状(コードで確かめたこと)

- `buildPtrAlias` の最後のループ(`ptrcells.go:165-194`)は、`valStores` と `ptrAlias` を数え直す 3 重のループで、時間の確認も上限もない。
- `prepareLocs`(`locs.go:70`)が呼ぶ。`prepareLocs` の期限の確認は、`buildPtrAlias` が返ったあとの、値ごとのループにしかない。
- 他のループ(`analyze` の命令ごとのループ、`prepareLocs` の値ごとのループ)は、`limit`(`-func-timeout` と `-timeout` の早いほう)を見る。

## 直す方向

`prepareLocs` の `limit` を `buildPtrAlias` に渡し、ループの内側で、一定の回数ごとに期限を確認する(既存の期限の確認の仕組みに合わせる)。期限に届いたときは、他の期限の扱いと同じ(`func_timeout` か `timeout` で、その関数を打ち切る)。期限に届かない入力では、結果を変えない。

## 小さな PR で直せる見込み

見込みあり。`ptrcells.go` と `locs.go` だけ。
