# ADR-090: ポインタを包んだインターフェースへの書き込みは、要約をまたぐとき、そのポインタの指す先にも適用する(R-65、ADR-060 / ADR-082 の続きの見逃し)

- 状態: 有効
- 関連: -
- 状態の詳細: 060/082 の続き

**背景:** `func fill(w io.Writer, s string) { fmt.Fprint(w, s) }` を `fill(&b, src)`(`b` は `bytes.Buffer`、`strings.Builder`、自前のライター)で呼び、呼び出し元で `b.String()` / `w.data` を sink に渡す形、`var s Setter = b; s.Set(src); sink(b.v)` の形が、finding も不確実性も出さなかった(ADR-082 の末尾の「インターフェース型のパラメータで受けたヘルパー」の未着手分、ADR-060 の限界「インターフェース経由で Set して元のポインタで読む」)。原因(確かめた): `make_interface` は、ポインタの指す先の中身をインターフェースの値の `/*` の下に**写す**(`convert`)。要約の `OutParam`(`fill` の `w` への書き込み)は、呼び出し元で、実引数(インターフェースの値)のセルにだけ適用され、元のポインタ(`&b`)の指す先に届かなかった。`fill` の要約は、書き込みを正しく持っていた。

**決めたこと:** `applySummary` の `OutParam` で、実引数が、その場で `make_interface`(ポインタの)、コピー、`change_interface`、φ で作られたインターフェースの値のとき(`ifacePointers`、`internal/taint/ifaceptr.go`)、そのポインタの指す先のセルにも、同じ書き込みを適用する。経路の先頭の `/*`(インターフェースの値がポインタの指す先を置く段)は除く(`ifaceTargets`)。今までの適用先は残す(置き換えでなく加える)。`make_interface` は Go のみなので、他の言語は変わらない。パラメータで受けたインターフェース(出所がここで分からない)は、その関数自身の要約が `OutParam` に持つので、要約の連鎖で届く。

**確かめたこと:** `testdata/go/cases/writer` に、関数に渡して書く形を 9 つ(`bytes.Buffer`、`strings.Builder`、自前のライターの `Fprint` / `Fprintf`、`io.WriteString`、フィールドのポインタで持つライター、`Setter` 経由の `Set` 直接と関数越し)と、デコイ(別のバッファ、別の box、別のフィールド、同じ構造体の別のポインタ)を足した。直す前は、出るはずの 8 つが全部落ちた。zoo `dyn/iface-write-passed`(脆弱 / 直した / デコイ)を足し、`dyn/iface-ptr-recv-write` の `@miss(silent)` を外した(直った)。`bench/compare.sh --light`(dc811ca から): **削除 0**、go-curated / dart-curated / rust-curated / go-synth-S / go-scale / parseable / dart-pub / gophish / gitea-cut は finding も経路も同じ(収束も同じ)。追加は、zoo-go の 4(新しいセル 2 つ、脆弱とデコイの sink 側。期待どおり)、gogs 2 つで各 6、casdoor で 1。実コードの追加 13 件は 1 件ずつ見た: **新しい sink の場所は 0**(どれも、前から別の source で報告されている sink に、別の source からの経路が 1 本増えたもの)。経路は 79 〜 105 歩で、xorm / beego などの依存の本体と `dynamic_dispatch` の候補の合流(`container_element_imprecision` を含む)を通る。本物の流れとは判断できない(**誤検出として分類**。ただし、直した経路(`io.Copy` / `Write` の先のインターフェース)は、書き込みの先を正しく運んでおり、ノイズの根は、前からある依存の合流点のほうにある。その sink には、前から同じ種類の別の source の finding がある)。

**数字(前 → 後):** 時間は同じ(gogs 16 → 16 秒、29 → 30 秒、gin-vue-admin 30 → 30 秒、casdoor 10 → 10 秒)。再解析は +2 〜 4%(gogs 18939 → 19599、23916 → 24738、gin-vue-admin 29998 → 30710、casdoor 14803 → 14856)、最大 RSS は gogs 3173 → 3444 MB、3488 → 3632 MB、gin-vue-admin 5271 → 5901 MB(+12%)、go-test-bench 1175 → 1197 MB、casdoor 2700 → 2773 MB。未収束は、base と head で変わらない(zoo 3 つの「NOT CONVERGED」は、base もそうで、同じ)。go-scale の期待値は変わらない(54 件、4833 回の再解析、同じ)。

**限界:** 書き込みの先が、パラメータで受けたインターフェースで出所が見えないとき(その関数自身の要約が連鎖して運ぶ)と、インターフェースの値をフィールドや map に入れて取り出したときは、出所をたどらない(`ifacePointers` は、その場の `make_interface` / コピー / φ だけ)。読む側(callee が iface のパラメータからポインタの指す先のフィールドを読む)は、`convert` が置く写しを読む(ADR-060 のまま)。

