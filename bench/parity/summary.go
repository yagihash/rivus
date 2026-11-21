package main

import (
	"fmt"
	"strings"
)

// summary is the table at the top: the four numbers, per language, with the counts.
func summary(rows []Row, dc map[string]DeadCounts, agg map[string]Demand, dartWithTests Demand, ai impactStat, zoo string) string {
	var b strings.Builder
	cnt := map[string]int{}
	pure := map[string]int{}
	for _, r := range rows {
		cnt[r.Lang]++
		if r.Pure {
			pure[r.Lang]++
		}
	}
	b.WriteString("## 要約: 4 つの数を言語ごとに(実数と割合)\n\n")
	t := &mdTable{head: []string{"", "Go", "Dart", "Rust"}}
	t.add("組み込みの行(全部)", n(cnt["go"]), n(cnt["dart"]), n(cnt["rust"]))
	t.add("**(A) 量**: 標準ライブラリの公開 API(型から列挙)のうち、組み込みの行がある", nOf(ai.apiModeled, ai.apiTotal), "未", "未")
	t.add("  うち型では「書く」(ii)関数に、書き込みを運ぶ行(ii)がある", nOf(ai.writeOK, ai.writeTotal)+"(行がある関数の中で。型の判定は上界)", "未", "未")
	shapeSites := func(d Demand, shapes ...string) (int, int) {
		s, m := 0, 0
		for _, sh := range shapes {
			s += d.ByShape[sh][0]
			m += d.ByShape[sh][1]
		}
		return s, m
	}
	cell := func(l string, f func(Demand) string) string { return f(agg[l]) }
	t.add("**(B) 需要**: 実コードの、本体のないライブラリ関数への呼び出し(箇所)のうち、組み込みの行がある",
		cell("go", func(d Demand) string { return nOf(d.ModeledSite, d.LibSites) }),
		cell("dart", func(d Demand) string {
			return nOf(d.ModeledSite, d.LibSites) + "; `--include-tests`(参考) " + nOf(dartWithTests.ModeledSite, dartWithTests.LibSites)
		}),
		cell("rust", func(d Demand) string { return nOf(d.ModeledSite, d.LibSites) }))
	t.add("  異なる呼び先(関数の種類。コードごとの重複あり)のうち、組み込みの行がある",
		cell("go", func(d Demand) string { return nOf(d.ModeledSyms, d.Syms) }),
		cell("dart", func(d Demand) string { return nOf(d.ModeledSyms, d.Syms) }),
		cell("rust", func(d Demand) string { return nOf(d.ModeledSyms, d.Syms) }))
	t.add("  うち標準ライブラリへの呼び出し(箇所)",
		cell("go", func(d Demand) string { return nOf(d.StdModeled, d.StdSites) }),
		cell("dart", func(d Demand) string {
			return nOf(d.StdModeled, d.StdSites) + "; `--include-tests`(参考) " + nOf(dartWithTests.StdModeled, dartWithTests.StdSites)
		}),
		cell("rust", func(d Demand) string { return nOf(d.StdModeled, d.StdSites) }))
	wr := func(d Demand) string {
		s, m := shapeSites(d, ShapeWrite, "ii?")
		return nOf(m, s) + "(名前・型の推定を含む)"
	}
	t.add("  形 (ii) と (ii?)(書く)の箇所のうち、組み込みの行がある", cell("go", wr), cell("dart", wr), cell("rust", wr))
	cb := func(d Demand) string {
		s, m := shapeSites(d, ShapeCall)
		return nOf(m, s) + "(名前・型の推定。行があっても中は運ばない)"
	}
	t.add("  形 (iii)(コールバック)の箇所のうち、組み込みの行がある", cell("go", cb), cell("dart", cb), cell("rust", cb))
	t.add("  汚れた値が**届いた**呼び出しのうち、組み込みの行がある", "未(コアの診断が足りない。下の 2)", "未", "未")
	t.add("**(C) 正しさ**: `pure` の行のうち、実行で確かめた", fmt.Sprintf("確かめた部分あり(%d 行が pure。下の 3)", pure["go"]), fmt.Sprintf("未(%d 行が pure)", pure["dart"]), fmt.Sprintf("未(%d 行が pure)", pure["rust"]))
	t.add("**(D) 当たっているか**: 鍵が呼び出しの行のうち、どの IR にも当たらない",
		nOf(dc["go"].Dead, dc["go"].Calls), nOf(dc["dart"].Dead, dc["dart"].Calls), nOf(dc["rust"].Dead, dc["rust"].Calls))
	t.add("  うち 綴りの疑い / API 不在", fmt.Sprintf("%d / %d", dc["go"].Suspect, dc["go"].Missing), fmt.Sprintf("%d / -", dc["dart"].Suspect), fmt.Sprintf("%d / -", dc["rust"].Suspect))
	t.add("  うち 別の型の同名メンバ(継承・Deref・trait の疑い)", n(dc["go"].Member), n(dc["dart"].Member), n(dc["rust"].Member))
	t.add("  うち 使われていない(似た名前も IR にない)", n(dc["go"].Unused), n(dc["dart"].Unused), n(dc["rust"].Unused))
	b.WriteString(t.String())
	b.WriteString("\n分母は言語ごとに違う(読んだコードが違う)ので、割合どうしを比べない。入力は 6 を見る。\n\n")
	return b.String()
}
