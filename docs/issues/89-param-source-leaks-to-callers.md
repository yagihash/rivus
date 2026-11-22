# #89 param の source の汚染が、呼び元へ書き戻り、source と関係のない関数の sink に届く(構造的な誤検出)

- 状態: **closed**
- 種類: 過検出
- 言語: Go
- ROADMAP の節: 2
- 守るもの: Go の最小の再現(`kind: param` の source)
- GitHub: #89(移行前の番号は同じ。2026-11-21 に開いた。移行時の状態: CLOSED)

## 今の状態(2026-11-22 の見直し)

直した: 623b620、c9aa412(ADR-090)。確認: 2026-11-22。

## 本文(GitHub から移した写し)

## 確度
実行で確認(最小の Go の再現)。`param_type` の版は未確認。

## 場所
`internal/taint/summary.go` の `summarize`: パラメータのルートの要素を `OutParam` に出す箇所。除いているのは `ParamElem(idx, "")` だけ。

## 再現(Go。`runB` が `kind: param` の source。`runB` は何もしない)
```go
type Req struct{ Path string }
func helper(p string) { os.Create(p) }
func runA(r *Req) { helper(r.Path) }
func runB(r *Req) { _ = r }
func main() { r := &Req{Path: "x"}; runA(r); runB(r) }
```
実際: finding 1 件。source = `runB`、sink = `helper` の `os.Create`、経路 = `main: call runB > runB: source > main: call runA > runA: load > runA: call helper > sink`。
期待: `runB` は source の値を何にも書いていないので、`runA` の中の sink には届かない。

## 原因
`kind: param` の source は、パラメータが指すオブジェクトのセルに `Real` の要素を置く。`summarize` はそれを、そのパラメータの `OutParam`(オブジェクトがグローバルなら `OutGlobal` にも)に出す。呼び元が同じオブジェクトを別の関数に渡すと、source と関係のない関数の中の sink に届く。

## 影響
構造的な誤検出。rivus 自身の解析の 66 件(source 6 × sink 11)のうち、24 件は、source の関数から sink の関数へ呼び出しでは届かない(別セッションの集計)。

## 修正の方向(方針は承認済み: 書き戻さない)
source を定義したパラメータに由来する `Real` の要素を、その関数の、同じパラメータの `OutParam` とそこから届く `OutGlobal` に出さない。関数の中で source の値を別のパラメータやグローバルに書いた場合は、今までどおり出す。失う流れ(source の関数を呼んだあと、呼び元が同じオブジェクトを sink に渡す)をフィクスチャと ADR に明記する。ADR に、`param_type` で N × M が `other_sources` に隠れる形も書く。フィクスチャは `param_type` の版と、起きない形(別々のオブジェクト、同じローカル、string)も。

## 関連
CORE-B / CORE-C の issue(グローバル経由の経路)、ADR-047

別セッションのレビューの識別子: CORE-A

## コメント(GitHub から移した写し)

関連: グローバル経由の経路の欠け(#90。CORE-B / CORE-C)。`os.Args` を通る形は、この issue を直しても残る。採点側の不変条件は #92。

## コメント(GitHub から移した写し)

再開後の確認(2026-11-22、main 0bd0a00 の木で実行): この Issue の最小の Go の再現(`runB` が `kind: param` の source、`runA` の中の sink)は、finding が 0 件になった。対照として `runA` を source にすると 1 件出るので、ルールは効いている。直したコミット: 623b620、c9aa412(ADR-048、kind: param の source を呼び元へ書き戻さない)。閉じる。

