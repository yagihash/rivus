# ADR-060: インターフェース越しに呼ぶ、ポインタレシーバのメソッドで、レシーバの中身を `/*` の下から読み書きする(CORE-J)

- 状態: 有効
- 関連: -
- 状態の詳細: 090 が続き

**背景:** `var i I = &P{f: src}; sink(i.Get())`(`func (t *P) Get() string { return t.f }`)が、finding も、流れを除去した不確実性も出さなかった(出るのは `dynamic_dispatch`、候補 1 だけ)。値レシーバと、具象のポインタ(`p.Get()`)は出る。原因(確かめた): `make_interface` は、ポインタの指す先の中身を、インターフェースの値の `/*` の下に置く(`transfer.go` の `convert`)。一方、ポインタレシーバのメソッドの要約は、レシーバのパラメータ(ポインタ)の `Param0 "/f"` と、その指す先を直接示す。dynamic な呼び出しで、実引数がインターフェースのとき、`expandParam`(読む側)と `OutParam`(書く側)が、`locs(iface) + "/f"` を読み、中身が実際にある `locs(iface) + "/*/f"` を見ていなかった。

**決めたこと:** `applySummary` が、呼び出しが dynamic(`CalleeDynamic`)で、実引数のレシーバがポインタでなく(インターフェース)、候補のレシーバのパラメータがポインタのとき、`recvBoxed` を立てる。`Param0` の展開と、`OutParam` の書き込みで、**今までの場所と、`/*` を足した場所の両方**を使う(置き換えない)。理由: Go 以外の言語(Rust の dyn)は、`/*` の規約が違い、置き換えると、rust-curated で finding 6 件が消えた(`compare.sh` で見つけた。和集合にして、消えない)。

**検証:** `testdata/go/cases/dispatch` に 5 つの形を足した(ポインタレシーバ、値レシーバ(回帰なし)、sink がメソッドの中、メソッド値、グローバルの interface。直す前は、値レシーバ以外の 4 つが落ちた)。`bench/compare.sh`(go-curated、dart-curated、rust-curated、go-synth-S、gogs 2 つ、gin-vue-admin、go-test-bench、parseable、dart-pub)で、finding の追加 0、削除 0、すべて収束、パス数は同じ。**実コードでは、この形の finding は増えなかった**(今ある基準には、interface 越しにレシーバのフィールドを読む形が、finding になるところにはない。Gitea では、この変更の単独の測定は取っていない)。`other_sources` の変更が go-test-bench に 16(順序の入れ替わりで、集合は同じ)。限界: 「インターフェース経由で `Set` して、元のポインタで読む」形(別名の扱い)は、別の問題で、これでは直らない。
