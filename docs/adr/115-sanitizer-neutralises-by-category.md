# ADR-115: sanitizer は、安全にできる sink の種類(カテゴリ)だけ値を隠す(`neutralises`。2026-11-23)

- 状態: 有効
- 関連: ADR-031(`drops` と `adds`)、ADR-086(インターフェースのメソッドの sanitizer)、MODELING 4b-1、docs/MODELING.md 4b

**背景:** 組み込みの sanitizer 7 行(Go 3、Dart 4)は、どれも `drops: [user_input]` と `adds: <ラベル>` だった。組み込みの sink は 170 行のうち 169 行が `accepts: [user_input]` で、sink は自分のカテゴリを見ない。そのため、sanitizer を 1 回通した値は、種類を問わず全部の sink から見えなくなっていた(`adds` のラベルは、どの sink も受けないので、使われていなかった)。HTML のエスケープを通した値が、コマンド、パス、SQL の sink に届いても、報告されなかった。URL のエンコードも同じで、`..` や `$()` や `;` は変わらない。サイレントな見逃しで、報告が減る向きの誤りだった。

**決めたこと:**
1. sanitizer の行に `neutralises: [<カテゴリ>]` を足す。値が安全になる sink のカテゴリの一覧。`adds` を必須にし、`drops: ["*"]` とは併用できない。空の一覧 `[]` は「安全だと言えるカテゴリはない」。
2. `neutralises` がある sanitizer を通った値は、`adds` のラベルを持つ。sink は、そのラベルを、(a) 落としたラベルを受けたはずで、かつ (b) sink のカテゴリが `neutralises` にないとき、受ける(カテゴリが複数の sink は、全部が `neutralises` にあるときだけ隠れる)。同じラベルを複数の行が与えるときは、`neutralises` の共通部分だけを隠す(隠す主張は、全部の行が言うときだけ)。sink の行 170 行は変えない。実装は `model.Rules.AcceptsLabel` と、エンジンの `acceptedAt`。
3. `neutralises` を書かない sanitizer は、これまでの意味(全部のカテゴリの sink から隠す)のままにする。プロジェクトのルールに、そういう行があれば、rivus は標準エラーに 1 回言い、`summary` に数を出す。
4. 組み込みの行は `neutralises` を必ず書く(`TestSanitizersAreReviewed`)。空の一覧には、`testdata/sanitizers-reviewed.txt` に理由を書く。書いたカテゴリは、組み込みの sink のカテゴリでなければならない。
5. 行の内容: `html.EscapeString`、`html/template.HTMLEscapeString`、`html/template.JSEscapeString`、Dart の `HtmlEscape.convert` は `[xss]`(JSEscapeString は JavaScript の文字列の中だけで、属性や URL では足りない。分かっている言いすぎ)。Dart の `Uri.encodeComponent` と `encodeQueryComponent` は `[]`(値が来る位置で決まり、`'`、`..`、`$()` は変わらない)。`Uri.encodeFull` は sanitizer をやめ、そのまま流す行にする(`: / ? # @` も変えない)。
6. 同じ変更で、メソッドの sanitizer の既定の流れに、レシーバを足す(`text.Clean()` で、データがレシーバにあるとき、`drops` に書かないラベルまで消えていた)。

**続けて通した sanitizer(2026-11-23 の追記):** `neutralises` のある sanitizer を続けて通した値は、ラベルを `a&b` のようにまとめる(名前を辞書順に並べ、各 1 回)。
1. finding のラベルに、まとめた名前がそのまま出る(例 `cleaned_cmd&html_escaped`)。sink の `accepts` にこの名前を書くと、その名前だけに一致する(完全一致が先)。ふつうの sink(`accepts: [user_input]` か空)は、次の規則で受ける: 落としたラベルのどれかを受けるはずで、かつ、まとめた名前の各ラベルの `neutralises` の和に、sink のカテゴリがない。
2. 順序に依らない: `cleanSql(cleanCmd(x))` と `cleanCmd(cleanSql(x))` は同じ名前になる。HTML のエスケープのあとに URL のエンコード(`html_escaped&url_encoded`)も、その逆も、xss の sink には隠れ、ほかのカテゴリの sink には届く。
3. sanitizer が別の関数の中にあっても同じ(要約は、通した sanitizer の操作を覚え、呼び出し元の実際のラベルに当てる)。`maxOps`(4)を超えた操作は覚えず、その分は隠さず、報告が増える向きに働く。
4. 数の上限: まとめる名前は、sanitizer の `adds` の名前の部分集合なので、組み込みのラベル 3 種(`html_escaped`、`js_escaped`、`url_encoded`)では高々 7、k 種なら高々 `2^k - 1`。1 つの値は元のまま 1 つの要素で、要素は増えない(まとめる前の要素は、置き換わる)。実際のプロジェクトでの要素の数は PR の本文に測った値を書く。
5. `neutralises` のない sanitizer が連なりにあるときは、それが受ける値(ラベルが `user_input` でないもの)を落とさず、そのまま通す(これまでと同じ)。

**効果と限り:** sanitizer を 1 回通しても、種類の違う sink には届き続けるので、報告は増える向きに変わる。

**しないこと:** 組み込みの sink の `accepts` の変更、project のルールの `neutralises` の強制、Rust の sanitizer 行(まだない。zoo のしくみの確認は、zoo のルールの `neutralises` で行う)。
