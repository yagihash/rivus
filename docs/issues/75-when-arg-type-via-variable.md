# #75 when_arg_type の型の条件が、変数を経由した値で成り立たず、sink が見逃される

- 状態: **open**
- 種類: サイレントな見逃し
- 言語: Go
- ROADMAP の節: 3
- 守るもの: -
- GitHub: #75(移行前の番号は同じ。2026-11-20 に開いた。移行時の状態: OPEN)

## 本文(GitHub から移した写し)

## 問題

`when_arg_type`(引数やレシーバの静的な型を条件にする sink の条件)は、値が**変数を一度経由する**と、元の型をたどれず、条件が成り立たない。条件が成り立たないと、sink は何も報告せずに消える(見逃し)。

```go
var o Out = Web{}
o.Put(x)            // o を経由: 型 Web がたどれず、sink にならない
Out(Web{}).Put(x)   // 直接: sink になる
```

型は、引数の局所変数の型と、そこに直接入る `convert` の元の型(最大 4 段)だけを見る(`internal/sites/enumerate.go` の `argTypeHolds`)。上の `o` は、`alloc` した変数を `load` したものなので、`convert` につながらない。

## 影響

- 組み込みの `when_arg_type` は、`fmt.Fprint(w, …)` の `w`(`net/http.ResponseWriter`)など、直接渡す形が中心で、普通は困らない。
- ただし、`w` を変数に入れてから `io.Writer` として渡す書き方では、XSS の sink が見逃される。#56 で足した `recv: true`(レシーバの型の条件)も、同じ限界を持つ。

## 方向性(案。エンジンの型の扱いに触れるので、案を決めてから)

- `argTypeHolds` が、`load` の元になっている `alloc` に、`store` された値(1 つだけなら、その値。複数なら、すべての値)の型をたどる。複数のうち 1 つでも条件に合えば、条件が成り立つ(見逃しを避ける向き)。
- または、SSA 形式(`irssa`)の上で、φ を含めてたどる。

## 関連

#56(`when_arg_type` の数え方、`recv: true`)、ADR の「`when_arg_type` の数え方を `args` に揃え」の追記

