# #99 Dart: 値として持たれた関数・クロージャ(フィールド、map、返したクロージャ、変数)の呼び出しが、候補 0 件のグラフのエッジになる

- 状態: **open**
- 種類: 道具・ベンチ(グラフのエッジ)
- 言語: Dart
- ROADMAP の節: 1
- 守るもの: zoo `F/clos/*`、`TestZeroCandidateCalls`
- GitHub: #99(移行前の番号は同じ。2026-11-21 に開いた。移行時の状態: OPEN)

## 今の状態(2026-11-22 の見直し)

関数値の要素(段階 4)の第 2 歩とキャプチャセルで閉じる見込み。第 1 歩(診断 `-func-values`)は 88c4cf2 で作って、消した(下の追記)。

## 追記(2026-11-24)

**キャプチャした変数を呼ぶ形**(関数の中で宣言したローカル関数を、同じ関数のクロージャから呼ぶ。Dart で最も普通の書き方の 1 つ)は、これまで候補 0 件で、ローカル関数の中の sink に流れが届かなかった(注記のない見逃し。zoo のセル `F/clos/local-fn-from-closure`)。直した(`fix/dart-captured-var-calls`): `internal/valuecalls` が、取り込んだ変数の値を、クロージャを作った `make_closure` の引数からたどる。候補 0 件は、zoo-dart で 49 から 41、curated で 20 から 15。あとで別の関数を代入した変数(セルに入るもの)は、まだ候補が付かない(`vuln2` の `@miss`)。

## 本文(GitHub から移した写し)

## 現象
`h.a!()`(フィールドに入れたクロージャを呼ぶ)、map から取り出した関数の呼び出し、返したクロージャをその場で呼ぶ形、変数に入れたクロージャの呼び出しは、グラフ(`rivus index`)の呼び出し点が `kind: value` で、**候補が 0 件**になり、実際に呼ばれる関数へのエッジがない。taint の側は、多くの形で流れを見つけている(zoo の `clos/stored`、`clos/returned`、`dyn/func_field`、`dyn/func_map` ほか)。

## 再現(`bench/zoo-dart/cells/lib/clos/stored.dart`)
```dart
class Holder { void Function()? a; }
void callA(Holder h) => h.a!();
void vuln() {
  final v = src('t');
  final h = Holder();
  h.a = () { sinkCmd(v); };
  callA(h);
}
```
`callA` の `h.a!()` の呼び出し点は候補 0 件。実行すると `vuln` のクロージャが呼ばれる。

## どう見つけたか
zoo の Dart を実行して観測した呼び出しと、グラフのエッジの照合(`docs/DESIGN-observed-calls.md`)。エッジがない 39 件のうち、この形が約 14 件(stored、partial_callers、func_field、func_map、returned、tearoff の一部)。ほかに super / mixin(`super_mixin_impl` 4 件)と `Navigator.pushNamed` のルート(3 件)が、同じくエッジがない。

## 関係
#77、#81、#97(呼び出しが未解決になる形)と関連する。参照のフェーズ(`rivus graph path` の 4 つ目のフェーズ)では届くものと届かないものがある。

## 追記(2026-11-24。`-resolve-func-values` を消した。根 D と I)

`-resolve-func-values`(候補 0 件の値の呼び出しに、関数値のパスで候補を補う実験)は、コスト(合格基準の 4 対象が全部 FAIL。エンジンを余分に 1 回走らせる)のために消した([DESIGN-callouts.md](../DESIGN-callouts.md) 5.6)。それが補っていた Dart の `no_edge` は、観測した呼び出しとの照合(方法 B。`bench/zoo-dart/observed/README.md`)で、そのまま `no_edge` として見え続ける。**偽の edge は足していない**。**注記のない見逃しは増えない**: 候補 0 件の値の呼び出しには、通常のパスの `unresolved_callee` が付く。

件数の区別: 根 D の 20 件のうち、フラグで消えたのは 17 件(残りの `clos/partial-callers` 3 件は、フラグでも消えなかった)。だから、フラグが補っていた `no_edge` は 37 件、根の課題に載る件数の合計は 40 件。

**根 D**(zoo-dart の `no_edge` 20 件。候補 0): フィールド・map・リスト・グローバル・コンストラクタ引数に入れた関数値を、あとで呼ぶ。例のセル: `clos/stored` 6、`dyn/func_field` 4、`dyn/func_map` 4、`clos/partial-callers` 3、`clos/capture-write-by-ref` 1、`dart/closure-to-sdk` 1、`dart/tearoff-instance` の `Button.press` 1。直す所: `internal/valuecalls/narrow.go`(origins、field、cell。フィールド・map・リストの書き込みから、それを読む側をたどる)。taint 側の関数値の記憶と同じ根(#79)。大きさ: 大。**偽の edge**: 同じ field や map への書き込みは、どれも候補になるので、キーや field を区別しないと過大(`func-map` の decoy。キーが定数の map の要素は、今の要素の置き場で 1 つの `[]` にまとまる)。区別すれば足さない。

**根 I**(3 件): 先に別の値を入れ、あとで別の値を代入した変数(`var f = first; f = second`、`late` の変数をあとで代入する相互再帰)の関数値。例のセル: `clos/local-fn-from-closure` の `vuln2`、`clos/local-recursion` の `mutual`(`@miss(unresolved_callee)`)。直す所: `internal/valuecalls` の、取り込んだ変数に複数の代入をたどる所(上の追記の「セルに入るもの」)。大きさ: 中。**偽の edge**: 代入が全部候補になるので過大(代入の順序を見ない。クロージャがいつ呼ばれるかが分からないと、安全に絞れない。ROADMAP のキャプチャセルの作業と同じ根)。

フラグで補ったときに見つかった過検出の形(map のキーを区別しない、変数の代入の順序を見ない)は、修正の材料として、上の 2 つに書いた。
