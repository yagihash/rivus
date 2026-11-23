# ADR-079: Rust の `Fn*::call` の候補を、レシーバに流れ込む関数値の出所で絞る(XTALK、案 A)。閉じた世界は `closed_world_entry`

- 状態: 有効
- 関連: DESIGN-generics-dispatch
- 状態の詳細: 訂正が本文内。085 が拡張; 訂正・追記あり(書き方を保つ)
- 要旨: `closed_world_entry`

**背景:** Rust のクロージャの呼び出し(`f(s)`、`impl Fn` のヘルパー)は、抽出器が呼び出し先を決められず、`Fn::call` の dynamic な呼び出しになる。候補は、引数の型の合う全クロージャ(XTALK)。ヘルパーの 2 つの呼び出し元が、それぞれのクロージャを渡しても、互いのクロージャの sink に、互いの source が届いた(rust-curated の `@fp` 18、`@noisy` 152 流れ。zoo の `clos/two-callers`)。設計は docs/DESIGN-capture.md の「案 A」。

**決めたこと:**
1. レシーバの出所を、**もれなくたどれたときだけ**、候補を `出どころ ∩ 今の候補` に絞る(`internal/valuecalls/narrow.go`)。たどれないものは、全候補のまま(今の挙動)。たどる: 環境の `alloc`(`ir.ClosureIndex`: `sg.EnvClosures` と同じ規則)、`make_closure`、`copy` / `convert` / φ、`clone`、環境のフィールドの store(クロージャの名前のフィールド)、`&f` のセル、パラメータ → 呼び出し元(呼び出し元が全部分かるとき)。出所が 8 を超える、深さが 3 を超えると、全候補。
2. **呼び出し元が全部分かる**とは、ヘルパーが、クロージャでなく、トレイトの実装でなく、トレイト越しに呼ばれず、値として使われず(`make_closure` が参照しない)、かつ: (a) `Func.private`(`pub` が無い。呼び出し元は crate の中だけ)、または (b) **閉じた世界**: 起点(`main`)があり、外からシンボルで呼べる関数(`external_entry`)が無く、抽出器がマークを付ける(`caller_marks`)。(b) が、設計の `closed_world_entry`: 「起点のあるプログラムとして読んだので、外からの呼び出しは数えていない」。**ユーザーに報告する前提**: 新しい前提を 1 つ入れた(起点のあるプログラムは、読んだコードが閉じた世界)。(a) は前提でない。
3. **IR の追加(追加だけ。スキーマバージョンは上げない)**: `Func.private`、`Func.external_entry`、記録 `caller_marks`(`ir.Program.CallerMarks`)。Rust の抽出器が付ける。古い IR は、マークが無いので、閉じた世界にならない。
4. **出力の追加**: `Uncertainty.narrowed_from`(絞る前の候補数)、`Uncertainty.basis`(`closed_world_entry`。パラメータ由来で、private でないヘルパーのとき)と、`detail` の文、`summary.dispatch_narrowed`(`calls` / `candidates_before` / `candidates_after`)、`stats.dispatch_not_narrowed`(`open_callers` / `unknown_origin` / `limit`)。`dispatch_candidates` は、絞ったあとで数える。
5. **ガード(ADR-045)**: 絞った呼び出しで、1 つのパラメータだけを通ってきた候補に、「パラメータ p に渡された関数がこの候補」のガードを付ける。レシーバがセルの中でも付く(`dispatchGuards` は、レシーバがパラメータそのもののときだけだった)。呼び出し元が作ったクロージャを渡していれば、ほかの候補を通る流れが失われる。

**確かめたこと:** docs/DESIGN-capture.md の「実装」の表。本物の流れの削除 0(rust-curated 142、zoo 112 の、`@source` と `@sink` の組が全部残る。削除した finding は、97 件(rust-curated)と 1 件(zoo)で、全部 source の注記が、sink の注記と同じタグを持たない)。「絞ると見逃す」形の 4 つと、開いた世界の 2 つの crate(`rust-curated-lib`、`rust-curated-ffi`)で、全候補のまま出ることと、private なヘルパーが絞れることを固定した。

**限界:** 中継のガード、`Box<dyn Fn>` を作る呼び出しの先、`async` の本体はたどらない(全候補)。マクロが書く関数は private としない。衝突した関数の名前の確認は入れていない。Dart は別の機会(DESIGN-capture.md の末尾の段落)。


**訂正(アーキテクチャ助言者のコードレビュー。絞り込みが、本物の呼び出し先を何も報告せずに削る形):** 手書きの IR を、絞り込みあり / なしで比べて見つけた。直したのは次の 6 つで、どれも「全部たどれたときだけ絞る」の約束を破っていた(たどれていないのに、たどれたことにしていた)。
1. **呼び出し元ごとのガード**: たどった値の記録(`seen`)の鍵が (関数, 値) だけで、パラメータ 0 の下で先に届いた値は、パラメータ 1 の下でもう一度届いても飛ばされ、候補が「パラメータ 0 だけから来た」と数えられた(`home(a, b)` と `home(b, a)` で、b が両方のパラメータで届く)。鍵に、通っているパラメータを足した。
2. **別名と φ**: 別名(`&mut r1` のコピー)は copy と convert だけを伝わり、φ で合流すると失われた(`let p = if c { &mut r1 } else { &mut r2 }; *p = &evil;`)。書き込みが、値の記録にも「外に出た」マークにもならず、候補が絞られて evil の sink への流れが消えた(不確実性の注記は `narrowed_from` だけ)。引数のどれかが別名の φ を別名に入れる。
3. **モデルのある trait のメソッドの静的な呼び出し**(別の変更。絞り込みの外): Rust の単相化で決まった呼び出し(`a.write(x)`)は、シンボルが trait のメソッドで、呼び出し先が自前の impl のとき、モデルだけを適用し本体を読まなかった(ADR-075 の「モデルと本体の和」が dynamic だけだった)。静的でも、呼び出し先が `<型 as trait>::メソッド` でプロジェクト自身の本体なら、和にする。
4. **`pub trait` の既定のメソッド**: 抽出器の `private` の判定(`fn 名前` で始まり `pub` がない)が、trait の中のメソッドの全部に一致した(trait の中に `pub` は付かない)。trait の成員は、trait が `pub` でないときだけ private にする。
5. **閉じた世界の範囲**: プログラム全体で決めていたので、公開するライブラリと関係のないバイナリが同じワークスペースにあると、ライブラリの `pub fn` が全部絞られた。起点から呼び出しで届く関数のクレートだけを、閉じた世界にする。
6. **昇格した定数の中の fn 項目**: `&helper`、`&[helper as fn(..)]` は `const nil` に下ろされ、値として使われたと数えられなかった(その関数は「呼び出し元が全部分かる」になった)。抽出器が、昇格した定数の MIR から関数を拾い、`make_closure` を足す。あわせて、出所に fn 項目(クロージャでない)が見つかり、候補の一覧にないときは、一覧が不完全という証拠なので、絞らない。

テストは、手書きの IR(`internal/valuecalls/testdata`)で、絞り込みあり / なしの差を固定した。rust-curated(`narrow.rs`)と rust-curated-lib(`pub trait` の既定のメソッド)、zoo の Rust(`dyn/stdlib-writer-own-static`)にもセルを足した。

