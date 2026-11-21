// Package global_chain is a cell of the zoo.
//
// @cell(F/state/global-chain)
// @shape: a value goes through a chain of package variables, one function per stage (3, 13 and 30 stages)
package global_chain

import "zoo/mark"

func init() {
	mark.Case("F/state/global-chain/vuln", vuln)
	mark.Case("F/state/global-chain/vuln13", vuln13)
	mark.Case("F/state/global-chain/vuln30", vuln30)
	mark.Case("F/state/global-chain/fixed", fixed)
	mark.Case("F/state/global-chain/decoy", decoy)
}

var c3g0 string
var c3g1 string
var c3g2 string
var c3g3 string

func c3s1() { c3g1 = c3g0 }
func c3s2() { c3g2 = c3g1 }
func c3s3() { c3g3 = c3g2 }
func vuln() { // @vuln
	c3g0 = mark.Src("state-global-chain-vuln") // @source(state-global-chain-vuln)
	c3s1()
	c3s2()
	c3s3()
	mark.SinkCmd(c3g3) // @sink(state-global-chain-vuln)
}

var c13g0 string
var c13g1 string
var c13g2 string
var c13g3 string
var c13g4 string
var c13g5 string
var c13g6 string
var c13g7 string
var c13g8 string
var c13g9 string
var c13g10 string
var c13g11 string
var c13g12 string
var c13g13 string

func c13s1()  { c13g1 = c13g0 }
func c13s2()  { c13g2 = c13g1 }
func c13s3()  { c13g3 = c13g2 }
func c13s4()  { c13g4 = c13g3 }
func c13s5()  { c13g5 = c13g4 }
func c13s6()  { c13g6 = c13g5 }
func c13s7()  { c13g7 = c13g6 }
func c13s8()  { c13g8 = c13g7 }
func c13s9()  { c13g9 = c13g8 }
func c13s10() { c13g10 = c13g9 }
func c13s11() { c13g11 = c13g10 }
func c13s12() { c13g12 = c13g11 }
func c13s13() { c13g13 = c13g12 }
func vuln13() { // @vuln
	c13g0 = mark.Src("state-global-chain-vuln13") // @source(state-global-chain-vuln13) @miss(recursion_approximation) @ref(R-44)
	c13s1()
	c13s2()
	c13s3()
	c13s4()
	c13s5()
	c13s6()
	c13s7()
	c13s8()
	c13s9()
	c13s10()
	c13s11()
	c13s12()
	c13s13()
	mark.SinkCmd(c13g13) // @sink(state-global-chain-vuln13)
}

var c30g0 string
var c30g1 string
var c30g2 string
var c30g3 string
var c30g4 string
var c30g5 string
var c30g6 string
var c30g7 string
var c30g8 string
var c30g9 string
var c30g10 string
var c30g11 string
var c30g12 string
var c30g13 string
var c30g14 string
var c30g15 string
var c30g16 string
var c30g17 string
var c30g18 string
var c30g19 string
var c30g20 string
var c30g21 string
var c30g22 string
var c30g23 string
var c30g24 string
var c30g25 string
var c30g26 string
var c30g27 string
var c30g28 string
var c30g29 string
var c30g30 string

func c30s1()  { c30g1 = c30g0 }
func c30s2()  { c30g2 = c30g1 }
func c30s3()  { c30g3 = c30g2 }
func c30s4()  { c30g4 = c30g3 }
func c30s5()  { c30g5 = c30g4 }
func c30s6()  { c30g6 = c30g5 }
func c30s7()  { c30g7 = c30g6 }
func c30s8()  { c30g8 = c30g7 }
func c30s9()  { c30g9 = c30g8 }
func c30s10() { c30g10 = c30g9 }
func c30s11() { c30g11 = c30g10 }
func c30s12() { c30g12 = c30g11 }
func c30s13() { c30g13 = c30g12 }
func c30s14() { c30g14 = c30g13 }
func c30s15() { c30g15 = c30g14 }
func c30s16() { c30g16 = c30g15 }
func c30s17() { c30g17 = c30g16 }
func c30s18() { c30g18 = c30g17 }
func c30s19() { c30g19 = c30g18 }
func c30s20() { c30g20 = c30g19 }
func c30s21() { c30g21 = c30g20 }
func c30s22() { c30g22 = c30g21 }
func c30s23() { c30g23 = c30g22 }
func c30s24() { c30g24 = c30g23 }
func c30s25() { c30g25 = c30g24 }
func c30s26() { c30g26 = c30g25 }
func c30s27() { c30g27 = c30g26 }
func c30s28() { c30g28 = c30g27 }
func c30s29() { c30g29 = c30g28 }
func c30s30() { c30g30 = c30g29 }
func vuln30() { // @vuln
	c30g0 = mark.Src("state-global-chain-vuln30") // @source(state-global-chain-vuln30) @miss(recursion_approximation) @ref(R-44)
	c30s1()
	c30s2()
	c30s3()
	c30s4()
	c30s5()
	c30s6()
	c30s7()
	c30s8()
	c30s9()
	c30s10()
	c30s11()
	c30s12()
	c30s13()
	c30s14()
	c30s15()
	c30s16()
	c30s17()
	c30s18()
	c30s19()
	c30s20()
	c30s21()
	c30s22()
	c30s23()
	c30s24()
	c30s25()
	c30s26()
	c30s27()
	c30s28()
	c30s29()
	c30s30()
	mark.SinkCmd(c30g30) // @sink(state-global-chain-vuln30)
}

var f3g0 string
var f3g1 string
var f3g2 string
var f3g3 string

func f3s1() { f3g1 = f3g0 }
func f3s2() { f3g2 = f3g1 }
func f3s3() { f3g3 = f3g2 }
func fixed() { // @fixed
	f3g0 = mark.CleanCmd(mark.Src("state-global-chain-fixed"))
	f3s1()
	f3s2()
	f3s3()
	mark.SinkCmd(f3g3) // @sink(none)
}

var d3g0 string
var d3g1 string
var d3g2 string
var d3g3 string

func d3s1() { d3g1 = d3g0 }
func d3s2() { d3g2 = d3g1 }
func d3s3() { d3g3 = d3g2 }

var e3g0 string
var e3g1 string
var e3g2 string
var e3g3 string

func e3s1() { e3g1 = e3g0 }
func e3s2() { e3g2 = e3g1 }
func e3s3() { e3g3 = e3g2 }
func decoy() { // @decoy
	d3g0 = mark.Src("state-global-chain-decoy") // @source(state-global-chain-decoy)
	e3g0 = "constant"
	d3s1()
	d3s2()
	d3s3()
	e3s1()
	e3s2()
	e3s3()
	mark.SinkCmd(e3g3) // @sink(none) @nosink(state-global-chain-decoy)
	mark.SinkSQL(d3g3) // @sink(state-global-chain-decoy)
}
