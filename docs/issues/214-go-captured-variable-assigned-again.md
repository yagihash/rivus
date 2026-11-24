# #214 Go: クロージャが捕まえた変数に別の値を入れ直すと、前の値にも書き込みが流れる(過検出)

- 状態: **open**
- 種類: 過検出
- 言語: Go
- ROADMAP の節: -
- 守るもの: `testdata/go/cases/heap` の `CapturedReassigned`(見つかるべき流れ `r.out` だけを書いている。過検出の `h.out` は、テストに書いていない)

優先度: 低い(まれな形。見逃しより過検出の側で、既存の方針どおり)。

## 何が起きるか

クロージャが引数(レシーバ)`h` を捕まえると、SSA は `h` をセルに入れる。コミット e143fb3d の修正(`internal/taint/ptrcells.go`。ローカルのセルに入れたポインタの先を、Go でも ptrAlias で追う)のあと、同じセルに `h = r` と別のポインタを入れ直すと、セルは両方のポインタを指すと扱われる(順序は追わない)。`h.out = s` の書き込みは、`r.out` にも、最初の `h.out` にも流れる。コードの上では、最初の `h` には書かれない。

## 確認の方法

`testdata/go/cases/heap/heap.go` の `setThenReplace` と `CapturedReassigned`。`lib.Sink(h.out)` を足すと、`TestHeap` が「想定外の流れ」で落ちる。

## 案

セルへの Store が 1 つだけのときに限る(引数の初期値だけ)。別の値を入れる変数は従来どおり追わないが、そうすると `r.out` が見逃される(サイレント)。過検出のほうを残すのが、精度優先の方針(`docs/PRINCIPLES.md`)に合う。
