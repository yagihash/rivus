package taint

import "strings"

// KindInfo is one row of the table of the kinds of uncertainty.
type KindInfo struct {
	Kind string
	// Attributable is the attributable_to of the kind (AttributableTarget, AttributableRivus or AttributableMixed). The one
	// kind that has two values says so in Doc: a dynamic_dispatch that has no candidate is the rivus' (calls.go).
	Attributable string
	// Doc is one sentence, in plain Japanese, of what the kind says. The longer notes are in docs/ARCHITECTURE.md 4.4.
	Doc string
}

// Kinds is the one table of the kinds of uncertainty: the value of attributable_to of each kind (attributable reads it) and
// what the kind says. docs/ARCHITECTURE.md 4.4 holds the table that KindsTable makes from it (`make docs-gen WHAT=kinds`;
// TestDocsSync holds that the two are the same), and TestKindsTableCoversConstants holds that every Kind constant has a row.
// To add a kind: the constant (result.go), a row here, `rivus.result/N` (ADR-127), and the table of the document.
var Kinds = []KindInfo{
	{KindConfigRead, AttributableTarget, "値が設定から読まれていて、その元は読んだコードの外にある。"},
	{KindExportedAPIParam, AttributableTarget, "ライブラリの公開関数の引数で、値の元は呼び出し側(読んだコードの外)にある。"},
	{KindGlobalVar, AttributableTarget, "値がパッケージの変数から来ていて、書き込み箇所を列挙してある。"},
	{KindReflection, AttributableTarget, "リフレクションで、実行時に何が起きるかが決まる。"},
	{KindFFIBoundary, AttributableTarget, "呼び出しが、その言語の外(cgo や他言語の関数)に出る。"},
	{KindStoredDataBoundary, AttributableTarget, "データストアへの出し入れで、読み戻した値の元は追わない。"},
	{KindMissingSource, AttributableTarget, "対象のコードの一部がない(生成コードが未生成、パッケージが読み込めないなど)。"},
	{KindDynamicDispatch, AttributableMixed, "呼び出し先が実行時に決まる(候補が 0 件のときは rivus の限界として rivus になる)。"},
	{KindAssumedCall, AttributableMixed, "呼ばれる所の見えないクロージャを、作った所で呼ばれたものとして扱った。"},
	{KindPointerAlias, AttributableRivus, "ポインタ越しの書き込みや読み出しを、別名で近似した所を経路が通る。"},
	{KindFuncValueIntoDependency, AttributableRivus, "自分のコードの関数値が、モデルのない依存の関数に渡され、依存の中でそれが呼ばれる流れは追わない。"},
	{KindUnresolvedCallee, AttributableMixed, "関数値の呼び出し先が実行時に決まり、候補を絞り込めなかった。"},
	{KindConcurrency, AttributableMixed, "実行順序に依存する所(goroutine、channel、コールバック)で、順序は扱っていない。"},
	{KindUnmodeledLibraryCall, AttributableRivus, "ライブラリの呼び出しにモデルがなく、汚染がどう通るかを知らない。"},
	{KindDependencyBodyUnread, AttributableRivus, "モデルのあるインターフェースの呼び出しの候補に、本体を読んでいない依存の関数があった。"},
	{KindProjectBodyUnread, AttributableRivus, "プロジェクトの関数の本体を、extractor が読めなかった。"},
	{KindModelApproximation, AttributableRivus, "流れのモデルが、自分で近似だと宣言している。"},
	{KindNumericStop, AttributableRivus, "source のデータが数値や bool になり、そこから先は追わない(既定)。"},
	{KindUnsupported, AttributableRivus, "extractor が下げられなかった構文で、そこで起きることは解析に入っていない。"},
	{KindContainerImprecision, AttributableRivus, "コンテナの要素を区別していない(結論に影響した所だけ記録する)。"},
	{KindDepthLimit, AttributableRivus, "上限や時間で解析を打ち切った(理由は detail に書く)。"},
	{KindAccessPathTruncated, AttributableRivus, "経路が、深さの上限で切ったアクセスパスを通り、切った所より下を丸ごと汚染として扱った。"},
	{KindCharTransform, AttributableRivus, "文字列から 1 文字を読んだ値(かその派生)が、数や bool の場所に入った(`-char-taint` のとき)。"},
	{KindRecursionApproximation, AttributableRivus, "再帰のある関数群が固定点に届かず、近似で止めた。"},
	{KindFieldImprecision, AttributableRivus, "フィールドを指定した sink に、そのフィールドにあるとは言えない汚染が届いた。"},
	{KindFieldUnresolved, AttributableRivus, "sink が指定したフィールドが、プログラムにない(綴りの誤りなど)ので、sink を全体として扱った。"},
	{KindNamedUnresolved, AttributableRivus, "sink が `named:` で指定した引数の名前が、プログラムのどの呼び出しにもないので、呼び出し全体として扱った。"},
	{KindGlobalHoldsNothing, AttributableRivus, "何も保持しないとルールが宣言した変数に、汚染を書いた関数がある(変数ごとに 1 件)。"},
}

// KindsTable is the table of Kinds (kind, attributable_to, the sentence) as docs/ARCHITECTURE.md 4.4 holds it. `make docs-gen
// WHAT=kinds` prints it.
func KindsTable() string {
	var b strings.Builder
	b.WriteString("| kind | attributable_to | 説明 |\n|---|---|---|\n")
	for _, k := range Kinds {
		b.WriteString("| `" + k.Kind + "` | " + k.Attributable + " | " + k.Doc + " |\n")
	}
	return b.String()
}
