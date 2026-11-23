# #179 Go: 埋め込み構造体の昇格を通すと、フィールドの区別が失われる(`f.Name` を source にすると `f.Comment` も報告される)

- 状態: **open**
- 種類: 過検出
- 言語: Go
- ROADMAP の節: -
- 守るもの: -

## 何が誤って報告されるか

`kind: field` の source(`archive/zip.FileHeader.Name`)があるとき、`*zip.File` の値 `f`(`FileHeader` を埋め込む)で `f.Name` を読むと、同じ `f` の `f.Comment` を sink に渡す行も finding になる。`FileHeader` を取り出して読む形(`h := f.FileHeader; h.Name` と `h.Comment`)では、`Comment` は静かで、正しい。`archive/tar.Header`(埋め込みなし)も正しい。つまり、昇格(埋め込みを通した読み出し)でだけ、フィールドが区別されない。

構造の誤検出で、`kind: field` の source が増えるほど(埋め込みを持つ型の source が増えるほど)、ほかのフィールドへ広がる。

## 再現

`bench/zoo-go-pkg/archive/cells/entry/entry.go` の decoy。`f := zipReader(...).File[0]` のあとで、`name := f.Name` を source に、`f.Comment` を別の sink に渡すと、後者が報告される。今の decoy は、`h := f.FileHeader` を通す形にしてあり、この課題を避けている(直したら、`f.Comment` の形に戻して、セルに足す)。

## 原因(推測。確認が要る)

昇格のフィールド読み出しの経路(`f.Name` は IR で `FileHeader` を通る)が、場所のパスに `FileHeader.Name` でなく、ベースの値全体を使っている可能性がある。`internal/taint/transfer.go` の `fieldSources` は `pathJoin(l.Path, field)` で場所を足すので、`l.Path` と `field` の付け方を、昇格の読み出しで確かめる。

## 優先度

中の上。組み込みの `kind: field` の source が増える(アーカイブ、GitHub のクライアントなど)ほど、過検出が出る場所が増える。
