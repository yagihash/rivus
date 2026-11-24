package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

func writeJSONFile(path string, v any) error {
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

type mdTable struct {
	head []string
	rows [][]string
}

func (t *mdTable) add(cells ...string) { t.rows = append(t.rows, cells) }

func (t *mdTable) String() string {
	var b strings.Builder
	b.WriteString("| " + strings.Join(t.head, " | ") + " |\n")
	b.WriteString("|" + strings.Repeat(" --- |", len(t.head)) + "\n")
	for _, r := range t.rows {
		b.WriteString("| " + strings.Join(r, " | ") + " |\n")
	}
	return b.String()
}

func n(v int) string { return fmt.Sprintf("%d", v) }

func nOf(a, b int) string { return fmt.Sprintf("%d / %d (%s)", a, b, pct(a, b)) }

var shapeLabel = []struct{ id, label string }{
	{ShapeValue, "(i) 値を返すだけ"},
	{ShapeWrite, "(ii) 引数・受け手に書く"},
	{"ii?", "(ii?) 書き先になりうる型(スライス・マップ)だけ。上界"},
	{ShapeCall, "(iii) コールバックを呼ぶ"},
	{ShapeRule, "(iv) source / sink / sanitizer / boundary"},
	{ShapePure, "pure(流れを切る)"},
	{ShapeNone, "運ばない型(結果も書き先もない)"},
	{ShapeUnknown, "形が分からない(モデルがなく、型も引けない)"},
}

type impactStat struct{ total, writeTotal, writeOK, writePure, apiTotal, apiModeled int }

func render(tablePath, deadPath string, rows []Row, mi *ModelIndex, corpora []*Corpus, digests map[string]*Digest, gu *GoUniverse, zoo string) error {
	by := map[string][]*Corpus{}
	for _, c := range corpora {
		by[c.Lang] = append(by[c.Lang], c)
	}
	var aImpact impactStat
	aggAll := map[string]Demand{}
	aggSuppDart := Demand{ByShape: map[string][2]int{}}
	var head, secRead, secShapes, secDead, secA, secB, secC, secNote strings.Builder
	var b *strings.Builder
	w := func(f string, a ...any) { fmt.Fprintf(b, f, a...) }
	b = &head

	w("# 言語間の差の表(段階 1)\n\n")
	w("`go run ./bench/parity report` が作る(コミットしない)。手順は `bench/parity/README.md`。**割合だけでなく実数を並べる**(分母が言語ごとに違う)。\n")
	w("「モデルがない」は見逃しではない(CORE-K のあと、モデルのない関数は「入ったものは出る」+ `unmodeled_library_call` で運ばれる)。\n")
	w("**既定の扱い**(`internal/taint/calls.go` の `unmodeled`): 受け手と引数のすべての中身が、結果のすべてに流れる(bool は除く)。**受け手や引数への書き込みは含まない**。コールバックは呼ばれない(CB-LIST)。\n")
	w("よって効くのは (ii) と (iii)、(iv) と pure(流れを切る)。(i) は、モデルがなくても流れは運ばれる(モデルは不確実性を減らすだけ)。\n\n")

	// ---- the corpora
	b = &secRead
	w("## 6. 読んだもの\n\n")
	t := &mdTable{head: []string{"言語", "入力(IR)", "実コード", "関数", "呼び先の名前(種類)", "uncertainty"}}
	for _, l := range langs {
		for _, c := range by[l] {
			real := "いいえ(fixture)"
			if c.Real {
				real = "はい"
			}
			t.add(langName[l], c.Name, real, n(c.Funcs), n(len(c.Calls)), c.Note)
		}
	}
	w("%s\n", t)

	// ---- model rows by shape (3)
	b = &secShapes
	w("## 5. 組み込みの行数を、形で分けた数(実数)\n\n")
	w("形は行の flows から決める。(ii) = いずれかの flow の行き先が結果でない(`recv` / `argN`)。(iii) = **名前による推定**(モデルの書式はコールバックを表せない。CB-LIST)。(iv) = source / sink / sanitizer / boundary / config_read。pure = `pure: true`。「呼び出しでない鍵」は、型・フィールド・変数・marker(IR の呼び先では当たらない)。\n\n")
	t = &mdTable{head: []string{"行の種類", "Go", "Dart", "Rust"}}
	cnt := map[string]map[string]int{}
	for _, l := range langs {
		cnt[l] = map[string]int{}
	}
	for _, r := range rows {
		cnt[r.Lang]["all"]++
		if !r.Call {
			cnt[r.Lang]["nocall"]++
			continue
		}
		cnt[r.Lang]["shape:"+r.Shape]++
		cnt[r.Lang]["role:"+r.Role]++
	}
	t.add("**組み込みの行(全部)**", n(cnt["go"]["all"]), n(cnt["dart"]["all"]), n(cnt["rust"]["all"]))
	for _, s := range []struct{ id, label string }{{ShapeValue, "(i) propagator: 値を返すだけ"}, {ShapeWrite, "(ii) propagator: 引数・受け手に書く"}, {ShapeCall, "(iii) propagator: コールバック(名前の推定)"}, {ShapePure, "pure"}, {ShapeRule, "(iv) source / sink / sanitizer / boundary / config_read"}} {
		t.add(s.label, n(cnt["go"]["shape:"+s.id]), n(cnt["dart"]["shape:"+s.id]), n(cnt["rust"]["shape:"+s.id]))
	}
	t.add("呼び出しでない鍵(型・フィールド・変数・marker)", n(cnt["go"]["nocall"]), n(cnt["dart"]["nocall"]), n(cnt["rust"]["nocall"]))
	for _, role := range []string{"source", "sink", "sanitizer", "propagator", "config_read"} {
		t.add("  うち "+role, n(cnt["go"]["role:"+role]), n(cnt["dart"]["role:"+role]), n(cnt["rust"]["role:"+role]))
	}
	w("%s\n", t)

	// ---- dead rows (D)
	b = &secDead
	w("## 4. (D) 当たっているか: どの IR にも当たらなかった行\n\n")
	w("行の鍵(`symbol`)が、読んだ IR の呼び先の名前(`callee.symbol`)か、関数の名前(`kind: param` の行)と**文字列で一致**するか。**IR に出ないだけ(使われていないだけ)の行と、綴りの誤りで当たらない行は区別できない**ので、次に分ける: **API 不在**(Go のみ: `go/types` にその名前がない = 綴りの誤りか、消えた API)、**綴りの疑い**(同じ API の別の綴りが IR にある: 末尾が同じで経路・レシーバの `*` などが違う)、**別の型の同名メンバ**(Dart と Rust のみ: 同じライブラリ(crate)の別の型に、同じメンバ名の呼び出しが IR にある。`List.join` に対する `Iterable.join` のように、宣言した型が違う: 継承・Deref・trait の疑い。`new`・`get` などの一般的な名前は除く)、**使われていない**(IR に似た名前もない)。一致は `match: implementations` の展開と、`ValueCands`(コアが作る関数値の候補)を見ない。\n\n")
	results := map[string][]RowResult{}
	dc := map[string]DeadCounts{}
	for _, l := range langs {
		results[l] = CheckRows(l, rows, by[l], gu)
		dc[l] = CountDead(results[l])
	}
	t = &mdTable{head: []string{"", "Go", "Dart", "Rust"}}
	row := func(label string, f func(DeadCounts) string) {
		t.add(label, f(dc["go"]), f(dc["dart"]), f(dc["rust"]))
	}
	row("組み込みの行", func(d DeadCounts) string { return n(d.Rows) })
	row("鍵が呼び出しの行(分母)", func(d DeadCounts) string { return n(d.Calls) })
	row("**当たった行**(どれかの IR)", func(d DeadCounts) string { return nOf(d.Hit, d.Calls) })
	row("  うち実コードの IR に当たった", func(d DeadCounts) string { return nOf(d.HitReal, d.Calls) })
	row("**当たらなかった行**", func(d DeadCounts) string { return nOf(d.Dead, d.Calls) })
	row("  API 不在(go/types にない。Go のみ)", func(d DeadCounts) string { return n(d.Missing) })
	row("  **綴りの疑い**(別の綴りが IR にある)", func(d DeadCounts) string { return n(d.Suspect) })
	row("  **別の型の同名メンバ**(同じライブラリの、継承・Deref・trait の疑い)", func(d DeadCounts) string { return n(d.Member) })
	row("  使われていない(似た名前も IR にない)", func(d DeadCounts) string { return n(d.Unused) })
	w("%s\n", t)
	w("形ごと(行 / 当たった / 当たらず・綴りの疑い・API 不在・別の型の同名メンバ / 当たらず・使われていない):\n\n")
	t = &mdTable{head: []string{"形", "Go", "Dart", "Rust"}}
	for _, s := range []string{ShapeValue, ShapeWrite, ShapeCall, ShapePure, ShapeRule} {
		cell := func(l string) string {
			a := dc[l].ByShape[s]
			return fmt.Sprintf("%d / %d / %d / %d", a[0], a[1], a[2], a[3])
		}
		t.add(s, cell("go"), cell("dart"), cell("rust"))
	}
	w("%s\n", t)
	w("綴りの疑いと別の型の同名メンバの例(モデルの鍵 -> IR にある別の綴り。全部は DEAD.md):\n\n")
	for _, l := range langs {
		w("- %s\n", langName[l])
		k := 0
		for _, r := range results[l] {
			if (r.Dead == DeadSuspect || r.Dead == DeadMember) && k < 10 {
				w("  - `%s` -> `%s`\n", r.Symbol, r.Spellings[0])
				k++
			}
		}
		if k == 0 {
			w("  - (なし)\n")
		}
	}
	w("\n")
	if err := writeDead(deadPath, results); err != nil {
		return err
	}

	// ---- (A) quantity (Go only)
	b = &secA
	w("## 1. (A) 量: 標準ライブラリの公開 API(型から列挙。分母を手で作らない)\n\n")
	if gu != nil {
		apis := gu.Enumerate()
		type acc struct{ total, modeled int }
		sh := map[string]*acc{}
		for _, a := range apis {
			k := a.Shape
			if a.Weak {
				k = "ii?"
			}
			x := sh[k]
			if x == nil {
				x = &acc{}
				sh[k] = x
			}
			x.total++
			if mi.HasModel("go", a.Symbol) {
				x.modeled++
			}
		}
		w("**Go のみ(段階 1)**。`go/types` で、標準ライブラリ(%d パッケージ)の公開の関数とメソッドのうち、受け手か引数に、文字列・バイト列・コレクション・Reader / Writer・空のインターフェース・型パラメータ・それらを持つ構造体の類がある関数を数える(%d 個)。形は型から: (iii) = 関数型の引数がある、(ii) = ポインタの受け手・ポインタの引数・io.Writer の類・空のインターフェースがある(書ける。**上界**: `Len` のように読むだけのポインタ受け手のメソッドも入る)、(ii?) = スライスかマップの引数だけ(書き先かもしれない。上界)、(i) = 結果があり、書き先がない。モデルの有無は、組み込みの行に同じ名前があるか(pure・source・sink を含む)。**ジェネリック型のメソッドは数えない**。公開 API でも、使う人が稀な関数が大半なので、割合は「需要」(下)と合わせて読む。\n\n", len(gu.Std), len(apis))
		t = &mdTable{head: []string{"形", "標準ライブラリの API", "組み込みにある", "割合"}}
		tot, mod := 0, 0
		for _, s := range shapeLabel {
			if x := sh[s.id]; x != nil && s.id != ShapeRule && s.id != ShapePure && s.id != ShapeUnknown {
				t.add(s.label, n(x.total), n(x.modeled), pct(x.modeled, x.total))
				tot += x.total
				mod += x.modeled
			}
		}
		t.add("**計**", n(tot), n(mod), pct(mod, tot))
		aImpact.apiTotal, aImpact.apiModeled = tot, mod
		w("%s\n", t)
		// what the model says about the functions that the types say write or call back
		impact := map[string]map[string]int{}
		for _, a := range apis {
			if !mi.HasModel("go", a.Symbol) {
				continue
			}
			k := a.Shape
			if a.Weak {
				k = "ii?"
			}
			if k != ShapeWrite && k != ShapeCall && k != "ii?" {
				continue
			}
			if impact[k] == nil {
				impact[k] = map[string]int{}
			}
			impact[k][mi.ShapeOf("go", a.Symbol)]++
			aImpact.total++
			if k == ShapeWrite {
				aImpact.writeTotal++
				if ms := mi.ShapeOf("go", a.Symbol); ms == ShapeWrite {
					aImpact.writeOK++
				} else if ms == ShapePure {
					aImpact.writePure++
				}
			}
		}
		w("**型では書く・コールバックを呼ぶ関数に、組み込みの行があるとき、その行は何と言っているか**(行の形。(ii) の行だけが書き込みを運ぶ。(i) は、書き込みを落とす。pure は、書き込みも切る。型の判定は上界なので、(i) と pure の中には、読むだけの関数が入る):\n\n")
		t = &mdTable{head: []string{"型から見た形", "モデルがある", "行の形 (ii)", "(i)", "pure", "(iii)", "(iv)"}}
		for _, k := range []string{ShapeWrite, "ii?", ShapeCall} {
			m := impact[k]
			tot := 0
			for _, v := range m {
				tot += v
			}
			t.add(k, n(tot), n(m[ShapeWrite]), n(m[ShapeValue]), n(m[ShapePure]), n(m[ShapeCall]), n(m[ShapeRule]))
		}
		w("%s\n", t)
		// rows of the model that name something outside the standard library
		stdRows, otherRows := 0, 0
		for _, r := range rows {
			if r.Lang == "go" && r.Call {
				if p := goPkgOf(r.Symbol); strings.Contains(strings.SplitN(p, "/", 2)[0], ".") {
					otherRows++
				} else {
					stdRows++
				}
			}
		}
		w("Go の呼び出しの行 %d のうち、標準ライブラリの名前 %d、それ以外(grpc・protobuf・x/* など)%d。\n\n", stdRows+otherRows, stdRows, otherRows)
		w("**Dart と Rust は未**(形が決まってから。Dart の `dart:*` は型のライブラリ URI から、Rust の `std` は rustdoc の JSON から、同じ基準で列挙できる見込み)。\n\n")
	} else {
		w("(`-no-gotypes` で省いた)\n\n")
	}

	// ---- (B) demand
	b = &secB
	w("## 2. (B) 需要: 読んだコードから、本体のないライブラリ関数への呼び出し\n\n")
	w("**Go の注意**: 解析は、標準ライブラリ以外の依存のソースを読んで、その関数の本体を解析する(`Deps`)。モデルのない依存の関数への呼び出しは「本体のない呼び出し」ではないので数えない(表の「本体を読む依存」の列)。Dart と Rust は `--no-deps` で抽出しているので、依存と SDK への呼び出しは、本体がない側に数える(bench の設定と同じ)。\n\n")
	w("**呼び出しの箇所**(IR の `call` 命令)のうち、呼び先が IR に本体のないもの(静的な呼び出しで、呼び先の関数が IR にない。動的な呼び出しで、候補が IR にない)を「ライブラリへの呼び出し」とし、呼び先の名前に**組み込みの行があるか**を数える。IR だけから数える(解析は要らない)ので、**汚れた値が届いたかは見ていない**(下の「届いた」を参照)。形は、行があれば行の形、なければ型(Go の標準ライブラリ)か名前(コールバック・書き込みの類)から。形の記号: `-` = 型では運ぶものがない(`reflect.Value` のように、型に出ない書き込みを含む)、`?` = 分からない、`ii?` = 書き先になりうる(型がスライス・マップで、データを返さない関数。または名前が push / add / write の類)。\n\n")
	for _, l := range langs {
		realC := []*Corpus{}
		var suppC []*Corpus
		for _, c := range by[l] {
			if c.Supp {
				suppC = append(suppC, c)
			} else if c.Real {
				realC = append(realC, c)
			}
		}
		if len(realC) == 0 {
			w("### %s\n\n(実コードの入力なし)\n\n", langName[l])
			continue
		}
		w("### %s\n\n", langName[l])
		t = &mdTable{head: []string{"入力", "ライブラリへの呼び出し(箇所)", "モデルがある箇所", "うち標準ライブラリへ(箇所 / モデルあり)", "異なる呼び先", "モデルがある呼び先", "(Go)本体を読む依存への呼び出し(数えない)", "解析の `unmodeled_library_call`", "`unmodeled_top`: 汚れが届いた(上位 20 の合計)"}}
		agg := Demand{ByShape: map[string][2]int{}}
		for _, c := range realC {
			d := CountDemand(c, mi, gu, false)
			dg := digests[c.Name]
			ul, reach := "-", "-"
			if dg != nil {
				u := 0
				for _, k := range dg.Uncertainties {
					_ = k
				}
				u = dg.Uncertainties["unmodeled_library_call"]
				ul = n(u)
				r := 0
				for _, x := range dg.Top {
					r += x.Reached
				}
				reach = fmt.Sprintf("%d (%d 個)", r, len(dg.Top))
			}
			t.add(c.Name, n(d.LibSites), nOf(d.ModeledSite, d.LibSites), nOf(d.StdModeled, d.StdSites), n(d.Syms), nOf(d.ModeledSyms, d.Syms), depCell(l, d.DepBody), ul, reach)
			addDemand(&agg, d)
		}
		for _, c := range suppC {
			d := CountDemand(c, mi, gu, false)
			t.add(c.Name+"(参考。合計に入れない)", n(d.LibSites), nOf(d.ModeledSite, d.LibSites), nOf(d.StdModeled, d.StdSites), n(d.Syms), nOf(d.ModeledSyms, d.Syms), "-", "-", "-")
			if l == "dart" {
				addDemand(&aggSuppDart, d)
			}
		}
		w("%s\n", t)
		if l == "dart" {
			w("%s", "**Dart の入口(ADR-073。この表の入力の数ではなく、Flutter のアプリの測定の記録)**: フレームワークが呼ぶメソッド(依存の型のメソッドの上書き。`build`、`initState`、`dispose` など)を入口に足した。`flutter/samples`(`a5bccd24`)の 6 つのアプリ(`--no-deps`)で、入口から届く関数は、411 のうち 65(16%)から 403(98%)に。LocalSend の `app/` は 20,522 のうち 1,546(7.5%)から 19,750(96%)に。finding の `reachability` は `not_from_entry` から `from_framework_callback` に変わる(finding の集合は同じ)。dart-pub(`main` のある CLI。フレームワークの上書きがない)は変わらない。\n\n")
		}
		aggAll[l] = agg
		show := agg
		w("形ごと(実コードの合計。参考の入力は入れない):\n\n")
		t = &mdTable{head: []string{"形", "ライブラリへの呼び出し(箇所)", "モデルがある箇所", "割合"}}
		w("形は、モデルがある箇所は行の形、ない箇所は型(Go の標準ライブラリ)か名前(コールバック・書き込みの類)からの推定。**Dart / Rust と Go の標準ライブラリ以外では、モデルのない箇所の形の大半が「分からない」**ので、(ii)(iii) の「割合」は、分かった分の中の割合で、実際より高い。\n\n")
		for _, s := range shapeLabel {
			a := show.ByShape[s.id]
			if a[0] == 0 {
				continue
			}
			t.add(s.label, n(a[0]), n(a[1]), pct(a[1], a[0]))
		}
		w("%s\n", t)
		// the reached side, from the analysis
		for _, c := range realC {
			dg := digests[c.Name]
			if dg == nil || len(dg.Top) == 0 {
				continue
			}
			w("**%s**: 汚れた値が届いた、モデルのない呼び先の上位(`reached` = 届いた回数、`on_path` = finding の経路に載った数):\n\n", c.Name)
			t = &mdTable{head: []string{"呼び先", "reached", "on_path", "形"}}
			for i, x := range dg.Top {
				if i >= 8 {
					break
				}
				s := mi.ShapeOf(l, x.Symbol)
				if s == "" {
					s = shapeOfUnmodeled(l, x.Symbol, gu)
				}
				t.add("`"+x.Symbol+"`", n(x.Reached), n(x.OnPath), s)
			}
			w("%s\n", t)
		}
	}
	w("**届いた割合の限界**: `-pass-stats` の `unmodeled_top` は**モデルのない関数だけの上位 20**で、モデルがある関数に汚れが届いた回数と、`reached` の総数は出ない。このため「汚れが届いた呼び出しのうちモデルが当たった割合」は、まだ出せない(上の「`unmodeled_top`」の列は、モデルのない側の上位 20 の合計で、分母ではない)。出すには、コアの診断に、モデルがある呼び出しの `reached` の合計と、モデルのない呼び出しの `reached` の合計(20 件で切らない)を足す必要がある(コアは触っていない。助言者への依頼事項)。\n\n")

	// ---- (C) correctness
	b = &secC
	w("## 3. (C) 正しさの現状\n\n")
	pure := map[string]int{}
	pureCall := map[string]int{}
	for _, r := range rows {
		if r.Pure {
			pure[r.Lang]++
			if r.Call {
				pureCall[r.Lang]++
			}
		}
	}
	t = &mdTable{head: []string{"", "Go", "Dart", "Rust"}}
	t.add("`pure` の行(実数。行 = 呼び出しの名前ごと)", n(pure["go"]), n(pure["dart"]), n(pure["rust"]))
	pp := pureProbe(zoo)
	t.add("`pure` の実行の検査(`TestZooPure*`): 呼んだ結果の部分 / うち誤り", pp, "未(検査がない)", "未(検査がない)")
	t.add("表 S(組み込みの全行を、行ごとに実行で確かめる)", "`pure` の一部だけ(上)。propagator / sanitizer / source / sink の行は未", "未", "未")
	t.add("zoo(共通の形のマス): マス / ケース / 出る / `@miss`", zooCell(zoo, "go"), zooCell(zoo, "dart"), zooCell(zoo, "rust"))
	t.add("zoo: 相手の言語に**本当に無い**形(`@na`)の数(Go の列は、Dart だけにあるマス)", zooMarksCell(zoo, "go", false), zooMarksCell(zoo, "dart", false), zooMarksCell(zoo, "rust", false))
	t.add("zoo: 書ける形で**まだ無い**(未実装、`@todo`。印のないものも数える)の数", zooMarksCell(zoo, "go", true), zooMarksCell(zoo, "dart", true), zooMarksCell(zoo, "rust", true))
	w("%s\n", t)
	if list := zooMarksList(zoo); list != "" {
		w("`@na` と `@todo` の中身(マスの名前。Dart の `@todo` は、印のない「段階 3 でまだ」も含む):\n\n%s\n", list)
	}
	if zoo != "" {
		w("zoo のテストの出力(`go test ./cmd/rivus -run 'TestZooPureGo|TestZooGo|TestZooDart' -v` の記録の行):\n\n```\n%s```\n\n", zoo)
	} else {
		w("(zoo のテストの出力は `-zoo` で渡す)\n\n")
	}

	b = &secNote
	w("## 7. 読み方の注意\n\n")
	w("- 「当たった」は、**読んだ IR にその名前の呼び出しがある**こと。当たらなかったことは、行が誤りであることではない(使われていないだけかもしれない)。実コードの入力を増やすと、「使われていない」は減る。\n")
	w("- (iii) の形は名前による推定。モデルの書式がコールバックを表せない(CB-LIST)ので、(iii) の行は、モデルがあっても、コールバックの中を運ばない。\n")
	w("- Rust は、自前のコード(rust-curated)と、すでに IR のある parseable だけを読んだ(C-19)。\n")

	b = &head
	w("\n")
	w("%s", summary(rows, dc, aggAll, aggSuppDart, aImpact, zoo))
	out := head.String() + secA.String() + secB.String() + secC.String() + secDead.String() + secShapes.String() + secRead.String() + secNote.String()
	return os.WriteFile(tablePath, []byte(out), 0o644)
}

func addDemand(a *Demand, d Demand) {
	a.LibSites += d.LibSites
	a.ModeledSite += d.ModeledSite
	a.Syms += d.Syms
	a.ModeledSyms += d.ModeledSyms
	a.StdSites += d.StdSites
	a.StdModeled += d.StdModeled
	a.DepBody += d.DepBody
	for k, v := range d.ByShape {
		x := a.ByShape[k]
		x[0] += v[0]
		x[1] += v[1]
		a.ByShape[k] = x
	}
}

func writeDead(path string, results map[string][]RowResult) error {
	var b strings.Builder
	b.WriteString("# どの IR にも当たらなかった組み込みの行\n\n`go run ./bench/parity report` が作る。分け方は、同じ道具が作る TABLE.md の 1。「別の綴り」は、IR にある、末尾が同じ名前(最大 6)。\n\n")
	for _, l := range langs {
		rs := results[l]
		b.WriteString("## " + langName[l] + "\n\n")
		for _, kind := range []string{DeadMissing, DeadSuspect, DeadMember, DeadUnused} {
			var sel []RowResult
			for _, r := range rs {
				if r.Dead == kind && r.Call {
					sel = append(sel, r)
				}
			}
			if len(sel) == 0 {
				continue
			}
			title := map[string]string{DeadMissing: "API 不在(go/types にない)", DeadSuspect: "綴りの疑い", DeadMember: "別の型の同名メンバ", DeadUnused: "使われていない"}[kind]
			fmt.Fprintf(&b, "### %s: %d\n\n", title, len(sel))
			for _, r := range sel {
				fmt.Fprintf(&b, "- `%s` (%s, %s, %s)", r.Symbol, r.File, r.Role, r.Shape)
				if len(r.Spellings) > 0 {
					b.WriteString(" -> " + strings.Join(quoteAll(r.Spellings), ", "))
				}
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func quoteAll(xs []string) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = "`" + x + "`"
	}
	sort.Strings(out)
	return out
}

var pureProbeRe = regexp.MustCompile(`zoo-go pure: (\d+) parts called.*, (\d+) wrong`)
var zooCellRe = regexp.MustCompile(`zoo-(go|dart|rust): cells (\d+) / cases (\d+) \| found (\d+) / (\d+) \| @miss (\d+)`)

func pureProbe(zoo string) string {
	if m := pureProbeRe.FindStringSubmatch(zoo); m != nil {
		return m[1] + " / " + m[2]
	}
	return "(`-zoo` なし)"
}

func zooCell(zoo, lang string) string {
	for _, m := range zooCellRe.FindAllStringSubmatch(zoo, -1) {
		if m[1] == lang {
			return fmt.Sprintf("%s / %s / %s of %s / %s", m[2], m[3], m[4], m[5], m[6])
		}
	}
	return "(`-zoo` なし)"
}

var zooMarksRe = regexp.MustCompile(`zoo-(go|dart|rust): parity marks: @na (\d+) \[([^\]]*)\] \| @todo (\d+) \[([^\]]*)\]`)

// zooMarks reads the "parity marks" line of a language: the count and the names of the cells that the language
// lacks, in two kinds. @na: the form does not exist in the language. @todo: it can be written, and is not yet.
func zooMarks(zoo, lang string) (naN, naNames, todoN, todoNames string, ok bool) {
	for _, m := range zooMarksRe.FindAllStringSubmatch(zoo, -1) {
		if m[1] == lang {
			return m[2], m[3], m[4], m[5], true
		}
	}
	return "", "", "", "", false
}

func zooMarksCell(zoo, lang string, todo bool) string {
	naN, _, todoN, _, ok := zooMarks(zoo, lang)
	switch {
	case !ok:
		return "(`-zoo` なし)"
	case todo:
		return todoN
	}
	return naN
}

// zooMarksList is the names of the cells, per language, under the table.
func zooMarksList(zoo string) string {
	var sb strings.Builder
	for _, l := range []struct{ key, name string }{{"go", "Go"}, {"dart", "Dart"}, {"rust", "Rust"}} {
		naN, naNames, todoN, todoNames, ok := zooMarks(zoo, l.key)
		if !ok {
			continue
		}
		fmt.Fprintf(&sb, "- %s: `@na` %s: %s\n", l.name, naN, nameList(naNames))
		fmt.Fprintf(&sb, "- %s: `@todo`(未実装) %s: %s\n", l.name, todoN, nameList(todoNames))
	}
	return sb.String()
}

func nameList(names string) string {
	if strings.TrimSpace(names) == "" {
		return "なし"
	}
	return "`" + strings.Join(strings.Fields(names), "`, `") + "`"
}

func depCell(lang string, n0 int) string {
	if lang != "go" {
		return "-"
	}
	return n(n0)
}
