# ルールの YAML で表現できないことの一覧(完了条件 D(3))

> **状態: 棚卸し(2026-11-27、main `70026385` で調べた)**。完了条件 D(3)の成果物。条件の原文(`.claude/work/QUEUE.md` の「現在のゴール」の表、領域 D の列):
>
> 「(3) 「YAML で表現できないこと」の一覧」
>
> 外部の設計者の返事の文言は、「(3) 「YAML で表現できないこと」の一覧がある」。

「表現できない」は、**ルールの書式(`internal/model/model.go` のキー)で書けず、エンジンの組み込みの振る舞いか、extractor の表(それも [HARDCODED-APIS.md](HARDCODED-APIS.md) に挙げた)でしか扱えない**こと。書式のキーの意味は [MODELING.md](MODELING.md) の 6 節にある。

集め方: (1) `docs/MODELING.md` の「Not yet」「cannot」と書いた所。(2) `docs/issues/` のうち、書式の不足と書いた課題。(3) `docs/DESIGN-*.md` の「書けない」「×」の所。(4) zoo のセルの `@miss` / `@ref` のうち、モデルでは直せない理由が文書にあるもの。(5) `internal/model/model.go` の構造体(あるキーと、ないキー)。**書式にあるが、使えない場面がある**ことと、**書式にない**ことは、分けて書いた。確認の程度は各項目に書く(「確認」はコードか文書で読んだ。「推測」は、その場で実行していない)。

## A. 書式にないもの(ルールの YAML では、書けない)

| # | 書けないこと | 理由(エンジンの仕組み) | 今の振る舞い | 根拠 | 状態 |
|---|---|---|---|---|---|
| L1 | **登録した関数を、あとで別の関数が呼ぶ**形(`Router.call`、`Navigator.pushNamed`、コンストラクタに渡して `send` の中で呼ばれる `RetryClient(when:)`) | `calls:` は、その呼び出しの中で呼ばれる関数だけを表す。呼び出し点で、呼ばれる関数の要約を適用する仕組みで、レシーバの状態(登録した関数の集まり)を持たない。ルールのエントリは、グラフのエッジも作らない(エッジには呼び出し点が要る) | 依存の本体を読まないとき、`dependency_body_unread` などの uncertainty だけが出る。Dart の `no_edge` の一部の原因 | `docs/MODELING.md:117-130`(`calls:` の範囲)、`docs/DESIGN-deps-cost.md:407`、`docs/issues/196-dart-observed-edges-missing-from-graph.md:72`、`docs/issues/206-dart-callbacks-called-by-dependency-from-own-map.md:23`、`docs/issues/209-dart-registered-handlers-called-by-dispatcher.md:29` | #196、#206、#209(open)。書式の拡張は未設計 |
| L2 | **値の一部**を指定する: `recv.elem`、`result0.err`、`to: arg0.Field`(`flows` と `calls:` の `with`) | `flows` と `with` の位置は、値の全体だけを指す(射影がない)。`parts` は sink の位置だけで、`flows` にはない | 値の全体が汚染される(過検出、`field_imprecision`)。Rust のコールバックの表(H10)と Dart の表(H15)は、レシーバの要素を渡すので、この不足のために extractor にある | `docs/MODELING.md:203`、`docs/DESIGN-deps-cost.md:182`、`docs/DESIGN-field-sinks.md:23` | 課題 #245(新規)。書式の設計が先 |
| L3 | **結果が関数**の API(`sync.OnceValue(f)`) | `calls:` は呼び出しの結果を値の位置(`ret`)に置くだけで、結果が関数値(あとで呼ばれる)という宣言がない(`result_funcs` は案のまま) | 流れが追えない。書けるまでは、呼び出し点に uncertainty を出す | `docs/MODELING.md:203`、`docs/DESIGN-callouts.md:173` | 案のまま(課題なし。推測: 実コードで必要になるまで保留) |
| L4 | **引数のフィールドにあるコールバック**、`fold` の蓄積値(`feed`) | `fn` は `recv` か `argN` の位置だけで、フィールドを指せない。蓄積値は、呼び出しごとに前の結果が次の入力になる形で、`with` の 1 回の束縛では書けない | その呼び出しの本体を読まなければ、流れが落ちる | `docs/MODELING.md:203` | 案のまま(課題なし) |
| L5 | **名前つき引数**を `flows` の `from` / `to` と `sources` の `params` に書く(`named:` は sink だけ) | `flows` は、呼び出し点でなく、シンボルの宣言で要約に使う。呼び出しから位置を決める方法が使えず、宣言の仮引数の名前を IR の `Func` が持つ必要がある | 宣言の順の位置で書く。オーバーライドが名前つき引数を足すと、位置がずれる(サイレント) | `docs/issues/145-*.md`、`internal/model/model.go:205`(`Sink.Named` だけがある) | #145(open) |
| L6 | **フレームワークが呼ぶ関数の引数**を source にする残りの形: 変数が 2 つの関数を持つ形、フィールドや要素に入れた関数、登録の呼び出しがない形(dart_frog の `onRequest`) | `callback_param` は、呼び出しの場所で関数が引ける形だけを扱う。`kind: param` の `extends` は、サブクラスのメソッドだけ。関数名で決める `kind: param`(案 e1)は未実装 | `unresolved_callee` の uncertainty が登録の呼び出しに付くが、source の関数から見るとサイレントな見逃し | `docs/MODELING.md:514-562`(4d 節)、`docs/DESIGN-callback-sources.md:88`(案 e1)、`:94-115`(比較表の「×」)、`docs/issues/149-callback-params-source.md` | #149(partial) |
| L7 | **利用者がオーバーライドするメソッド**を、ライブラリが呼ぶ形(`Interceptor.onRequest`)の `calls:` と source | `calls:` の `fn` は引数かレシーバの位置で、メソッドの実装を指せない。`callback_param` は関数が呼び出しに現れる形だけ | その実装の引数が source にならない | `docs/issues/156-override-method-params-source.md:16`、`docs/issues/model-dio-open-points.md:18` | #156(open) |
| L8 | **検査の関数(ガード)**: 通らなければ先へ進まない関数 | サニタイザは「値を変える」関数のためで、「通らなければ止まる」を、値の流れとして書く場所がない。`guards:` は、設計(ADR-122)があるが、合格基準の「修正後」の 2 件で、構造だけでは止まらず、保留 | ガードの先の sink が報告される(過検出)。finding は消えない | `docs/issues/169-guard-declarations.md` | #169(open。2026-11-27 に、このゴールの条件から外した) |
| L9 | **書き込み先の型の性質**の宣言(「この型は、ほかの書き込み先を持たない」「書き込みが中身だけに入る」) | 型の性質はコードの表(H2、H3)にあり、ルールには、型のエントリがない | 表にない型は、「持つかもしれない」として、流れを残す(過検出の側) | `internal/taint/guards.go:92`、`internal/taint/calls.go:1148` | #244(新規) |
| L10 | **定数の値で変わる**条件(`exec.Command("sh", "-c", x)` の第 1 引数が `sh` のときだけ sink にする、など) | `Sink` に、型の条件(`when_arg_type`)はあるが、定数の値の条件のキーがない(`internal/model/model.go:220`)。確認: キーがないこと。実コードで必要な例は未調査 | 引数の全体を sink として扱う(過検出の側) | `internal/model/model.go:195-232` | 課題なし(推測: 必要な実例が出るまで保留) |
| L11 | **sink が、読んでいないコードにある**形(構造体を作って、フレームワークのリフレクションや別のプロセスが危険な処理をする) | sink は、呼び出しを持つ位置にしか付かない(ADR-099)。呼び出しがなければ、`parts` でも書けない | 見逃し(sink の呼び出しがない) | `docs/DESIGN-field-sinks.md:198`(10 節) | 案 A として、実コードで 1 件見つかるまで作らない、と決めてある |
| L12 | **ユーザーが決める型**の引数を入口にする(`functions.Typed(name, fn)`) | 引数の型は利用者が決め、`fn` は `any`。`param_type` で型を書けない。`callback_param`(4d 節)で書ける可能性がある(推測: `GCP-MODELS.md` の記述は `callback_param` の前) | `GCP-MODELS.md` では「見逃す」 | `docs/GCP-MODELS.md:78`(F7) | 確認していない。次に GCP のモデルを足すときに、`callback_param` で書けるか試す |
| L13 | **複数の言語で同じ API の表を 1 つ**にする(Go の `database/sql` と Dart の `sqflite` を同じ行から作る) | ルールのファイルは言語ごとで、シンボルの綴りが言語で違う。「1 枚の表から言語ごとの YAML を作る」は、設計文書の案 | 言語ごとに書く | `docs/DESIGN-model-authoring.md`(段階 1 の入力の列) | 設計のみ。完了条件には入れない |

## B. エンジンの組み込みの振る舞い(言語の意味で、YAML の対象ではない)

ルールの YAML は、**関数のシンボル**に付く。次のものは、関数のシンボルがなく、言語の構文やメモリの意味なので、YAML では書けない(書く対象でもない)。zoo の `@miss` / `@fp` の多くは、この種類。`@ref` の数は、マークの数(セルの数ではない)。

| # | 振る舞い | エンジンのどこか | zoo の `@ref` と課題 |
|---|---|---|---|
| E1 | 組み込み関数(`append`、`copy`、`min`、`max` ほか 22)と、`recover()`(パニックの値を追わない) | `internal/taint/calls.go:805-860` | [HARDCODED-APIS.md](HARDCODED-APIS.md) の H5 |
| E2 | 文字を 1 つずつ読む形で、数や bool に止まる(`numeric_stop`) | `internal/taint/state.go:403-470` | #185(`@ref` 46) |
| E3 | 関数値を、入れ物(フィールド、map、リスト、変数)に入れてから呼ぶ | `internal/valuecalls`、`internal/taint` の関数値の候補 | `FUNC-VALUE-MEMORY`(5)、`CLOSURE-IN-MEMORY`(4)、`XTALK`(12)、`PARTIAL-CALLERS`(3)、#77〜#81 |
| E4 | インターフェースや trait の呼び出しが、実装の全部に届く | `internal/model/expand.go`、`internal/taint` の候補 | `DYN-IMPLS`(22)、#235、#236 |
| E5 | 入れ物の要素とキーを区別する(スライスの要素、map の鍵) | `internal/taint` のアクセスパス | `KEYED-MAP`(7)、#173、#194 |
| E6 | `Drop::drop`(スコープの終わりで呼ばれる)、`thread::spawn`、`mpsc::channel`、`async` / `.await` | `extractor/rust/src/lower.rs`、`internal/taint` | `R-17`(10)、`CAP`、`STD-MODEL`、`ASYNC`(各 2) |
| E7 | `reflect` を通る書き込み、`any` の型 switch、map 型の変換を通る書き込み | 標準ライブラリの本体を読む側(`internal/taint`) | #152、#153、#154 |
| E8 | 周回の上限(`PASS-LIMIT`)、widening、作業量の上限 | `internal/taint/engine.go` | `PASS-LIMIT`(2)、#187 |

E3、E4 は、**YAML で補える部分がある**(`calls:` で標準ライブラリの関数が呼ぶ関数を書く。H1、H10、H15)。それ以外は、エンジンの修正だけが直す手段で、「モデルの拡充」では直らない。

## C. 書式にあるが、書けない場面があるもの(参考)

次は、書式があるので、A には入れない。書く前に、制約を知っておく項目。

- `calls:` は、**本体を読むパッケージには、候補を作らない**(関数値かインターフェースの呼び出しは解析が解決する)。読まないパッケージ(`body_replaced_packages`、標準ライブラリ)にだけ効く(`docs/MODELING.md:471`)。
- 同じラベルを `adds` で足す 2 つのサニタイザで、`neutralises` が違うと、両方にある種類だけが残る(`docs/MODELING.md:438`)。
- sink の `parts` は、位置が全体で危険なとき、`parts` で狭められない(`docs/MODELING.md:238`)。
- `named:` は Dart の名前つき引数だけで、sink の位置にだけ使える(L5 と対)。
- `rivus models coverage -yaml` が作るテンプレートは、型が分からないとき、広い書き込み(`to: args`)を書く(`docs/MODELING.md:26-40`)。

## 件数

| 区分 | 件数 | 内訳 |
|---|---|---|
| A. 書式にないもの | 13 | 課題がある 7(既存 5: L1、L5、L6、L7、L8。新規 2: L2 の #245、L9 の #244)。設計文書に記録がある、課題はない 4(L3、L11、L12、L13)。文書の記述だけ 2(L4、L10) |
| B. エンジンの組み込み | 8 | 課題か zoo の `@ref` がある 6(E2、E3、E4、E5、E7、E8)。ない 2(E1、E6。E6 は zoo の `@ref` のみ) |

「課題がある」は、直す予定がある、という意味ではない。`guards:`(L8)は、2026-11-27 に、このゴールの条件から外した。表の「根拠」の行番号は、main `70026385` の時点。
