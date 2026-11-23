# ADR-083: Go の型の別名は、ルールの側で同義に扱う。IR は使った名前のまま、別名の表(`type_aliases`)を足す

- 状態: 有効
- 関連: GCP-MODELS
- 状態の詳細: 訂正が本文内; 訂正・追記あり(書き方を保つ)

**背景:** Go の IR は、型を使った名前で出す(別名は解決しない)。Pub/Sub の v1 / v2 の `Message` は、ルートのモジュールの内部の型の別名で、`cloudevents/sdk-go/v2.Event` は `event.Event` の別名。利用者が実体の名前(正しい型の名前)でルールを書くと、コードが別名で書いた引数に一致せず、何も報告しない(docs/GCP-MODELS.md)。

**決めたこと:**
1. 抽出器は、読んだ全部のパッケージの、型の別名の宣言(`type A = B`。型引数なしのもの)を、`type_alias` の記録(`name` と、別名をほどいた `target`)で、IR の末尾に足す(Program の `type_aliases`)。足すだけで、古い IR はそのまま読める(表が空)。型の名前そのものは変えない。
2. 照合は、ルールの型と IR の型の**両方を、別名をほどいた名前**にして比べる(`ir.Program.CanonType`。`*` と `[]` の接頭辞は残す)。だから、別名の名前で書いたルールも、実体の名前で書いたルールも、別名で書いたコードにも実体で書いたコードにも一致する。対象: `param_type`、`when_arg_type` の型、`field`(`pkg.Type.field`、`pkg.Type.*`)。
3. 照合の側で同義にするので、結果が変わるのは、今まで何も報告していなかった所が検出される方向だけ。

**限界:** メソッドのシンボル(`pkg.Type.Method`)は、メソッドが実体の型に宣言されるので、IR のシンボルは常に実体の側。ルールを別名の名前のレシーバで書くと一致しない(実体の側で書く。docs/GCP-MODELS.md の `DataAs` の行)。入れ子の中の別名(`map[string]Alias`、`Foo[Alias]`、`[]*Alias`、関数の型の中)も、`CanonType` が型の文字列の名前ごとにほどく(追記: 以前は先頭の `*` と `[]` の接頭辞だけだった。`testdata/go/cases/typealias` の e / f / g / h)。Dart の `typedef`: 抽出器の型は `element` のクラスで、型の側はすでに実体(別名の名前で書いたルールが一致しない抜けは、同じ表を足せば塞げる。未着手)。Rust: 型の別名は rustc が展開し、`use … as` は定義のパスに影響しないので、同じ抜けはない(未確認の範囲は、`impl` の対象の別名)。

**訂正(関数の中で定義した同名の型):** 別名の表は名前を鍵にするので、関数の中で定義した型が、そのパッケージの別名と同じ名前(`type Shadow = real.S` と、関数の中の `type Shadow struct{…}`)だと、別名の先と同じ型として扱われ、別名の先のルールが一致した(誤検出)。修正: 抽出器は、パッケージの別名と同じ名前の、関数の中の型を、`pkg.Name·位置` の名前で出す(`typeString`。`*`、`[]`、配列、`map`、`chan`、型引数の下も)。別名の表は、パッケージ直下の宣言だけなので、この型には一致しない。ほかの型の名前は変わらない。テスト: `testdata/go/cases/typealias` の `SShadowDecoy`(前は検出された、今は検出されない)。限界: 関数の型、構造体の型の中の、同名の関数内の型は、名前のまま(`typeString` が下りない)。

**確かめ直した(別名の別名、ポインタ、スライス、マップの別名):** `type PT = *T`、`type LT = []T`、`type M = map[string]PT`、`type PCT = *CT` 越しのフィールドは、`CanonType` が型の文字列の名前ごとに、別名の先を再帰的にほどく(目標も別名を含みうる。深さ 8 まで)ので、置き換えたあとの `*` / `[]` / `map` / `chan` / 関数の型の接頭辞は、実体の型として並ぶ(別名のまま残らない)。`map[string]api.T` も一致する(上の「限界」の入れ子)。テスト: `testdata/go/cases/typealias` の `PtrAliasSpelling`、`ListAliasSpelling`、`MapAliasSpelling`、`FieldThroughPtrAlias`、`internal/ir/canon_test.go`。接頭辞を外し直す処理は不要だった(文字列の名前ごとの置き換えが、固定点まで進む)。

