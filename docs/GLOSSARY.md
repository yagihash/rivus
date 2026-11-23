# プログラム解析 / taint 解析 用語集

これまでの会話で登場した用語をまとめたもの。
★ が付いているものは、自作の taint 解析を設計するうえで特に重要な概念。

> 改訂メモ: 初版からの変更点は、SARIF の項（GHAS 制約の訂正）、関数サマリ / functional approach の項、
> `ssa.NaiveForm` の項、terminal kind の追加（4種 + `attributable_to`）、および
> 「10. rivus 固有の用語」の新設。

---

## 1. 中間表現とコンパイラの基礎

**AST（Abstract Syntax Tree, 抽象構文木）**
ソースコードを構文規則に従って木構造にしたもの。パーサの出力。
「構文の形」しか持っておらず、実行順序や分岐の情報を持たない。
そのため taint 解析には不十分で、CFG への変換が必要になる。

**CFG（Control Flow Graph, 制御フローグラフ）** ★
基本ブロックをノード、制御の移り先をエッジとしたグラフ。
「この文の次に実行され得るのはどこか」を表現する。
データフロー解析はすべてこのグラフの上で行われる。

**基本ブロック（Basic Block）**
途中で分岐も合流もしない、必ず先頭から末尾まで直列に実行される命令の並び。
分岐命令（if, switch, ループの条件）とその飛び先でブロックが切れる。

**SSA（Static Single Assignment, 静的単一代入形式）** ★
各変数が「ただ一度だけ代入される」ように書き換えた中間表現。
`x = 1; x = 2` を `x1 = 1; x2 = 2` にする。
これにより「どの代入がこの参照に届いているか」が名前の一致だけで判定できるようになり、
データフロー解析が大幅に簡単になる。

**φ 関数（phi function）** ★
SSA において、制御フローの合流点で「どの経路から来たかによって値が変わる」ことを表す擬似命令。
`x3 = φ(x1, x2)`。
実行時に対応する機械語命令があるわけではなく、レジスタ割り当ての段階で
「両方の枝で同じレジスタに書く」という形に解消される。
**taint 解析における「合流点での和集合」は、この φ の扱いとして実装される。**

**IR（Intermediate Representation, 中間表現）**
ソースと機械語の間にある、解析や最適化のための表現。AST・CFG・SSA はいずれも IR の一種。

**脱糖（desugaring）**
糖衣構文を、より原始的な構文に展開すること。
`for range` をループとイテレータ呼び出しに開く、など。
脱糖済みの IR は扱うべきノード種別が減るため、解析器を書くのが楽になる。
rivus では、言語固有の脱糖は extractor の責務（ARCHITECTURE.md 参照）。

**型解決 / 名前解決（type resolution / name resolution）** ★
ソース中の識別子が「どの宣言を指し、どの型を持つか」を確定させる処理。
`db.Query(x)` の `db` が `*sql.DB` なのか自作のラッパーなのかを判定するために必須。
**sink の識別は型解決に依存しているため、taint 解析の精度を根本的に左右する。**

**element model（Dart）**
`package:analyzer` が提供する、宣言の実体を表すオブジェクト群。
Go の `go/types` に相当する。ファイルを跨いだ参照が解決済みなので、
これがあると呼び出し先の特定がファイル境界を越えて行える。

**MIR（Rust）**
rustc の中間表現のひとつ。基本ブロックと terminator からなるため、**そもそも CFG の形をしている**。
borrow checker などの解析はこの上で動く。

**Kernel AST / `.dill`（Dart）**
Dart の中間表現。whole-program でリンク・脱糖済み。`package:kernel` で読める。

**モノモーフィゼーション（monomorphization）**
ジェネリックなコードを、具体的な型引数ごとに実体化すること。
Rust の MIR はジェネリックのままなので、どの段階の MIR を解析するかで性質が変わる。

---

## 2. データフロー解析の基礎

**データフロー解析（dataflow analysis）** ★
CFG の各プログラム点で「成り立つ事実の集合」を求める解析手法の総称。
taint 解析はこの応用のひとつ。

**事実（dataflow fact）** ★
解析が追跡する情報の単位。taint 解析では「変数 v は汚染されている」がひとつの事実。

**遷移関数（transfer function）** ★
ひとつの命令が、事実の集合をどう変換するかを表す関数。
gen（追加）と kill（削除）の組で記述されることが多い。

**gen / kill** ★
- gen: その命令によって新たに成り立つようになる事実（例: `v = source()` で `v` を汚染に追加）
- kill: その命令によって成り立たなくなる事実（例: `v = sanitize(u)` で `v` を汚染から削除）

**join（合流関数）** ★
制御フローの合流点で、複数の入力集合をどうマージするかを定める演算。
**taint 解析では和集合。** 片方の経路でしか汚染されていなくても汚染扱いにする＝安全側に寄せる。

**may 解析 / must 解析** ★
- **may**: 合流で和集合をとる。「〜である可能性がある」。taint 解析はこちら
- **must**: 合流で積集合をとる。「必ず〜である」。定数伝播や確定代入解析はこちら

**束（lattice）**
事実の集合に順序と合流演算を入れた数学的構造。
実装上は「有限で、遷移関数が単調」であれば不動点計算が停止する、という保証の根拠として理解すれば足りる。

**単調性（monotonicity）**
入力の集合が大きくなれば出力も小さくならない、という性質。
有限性と合わせて、不動点計算の停止性を保証する。

**不動点（fixed point）** ★
何度反復しても結果が変わらなくなった状態。データフロー解析の答えはこの不動点として定義される。

**worklist アルゴリズム** ★
不動点を求める実用的な手法。
「更新が必要なブロック」をキューに入れ、空になるまで処理を繰り返す。
更新のあったブロックの後続だけをキューに戻すので、無駄な再計算が減る。

**前向き解析 / 後ろ向き解析（forward / backward）**
情報を制御フローの向きに伝播させるか、逆向きに伝播させるか。
taint 解析は前向き、後述の由来追跡（スライシング）は後ろ向き。

**⊤（トップ） / ⊥（ボトム）**
束の最大元 / 最小元。
実務的には「⊤ = 何も分からない（全部あり得る）」の意味で使われることが多い。
由来追跡で「追跡不能」に落ちることを「⊤ に潰れる」と表現した。

---

## 3. taint 解析の用語

**taint 解析（汚染解析）** ★
攻撃者が制御し得る値（汚染された値）が、危険な処理に到達するかを判定する解析。

**source** ★
汚染の発生源。外部入力を読む箇所。
例: `r.FormValue()`, `os.Getenv()`, JSON デシリアライズ, deep link のパラメータ

**sink** ★
汚染された値が到達すると危険な箇所。
例: SQL 文字列の組み立て、`Process.run`, HTML への挿入, ファイルパス

**sanitizer** ★
汚染を除去する処理。ここを通った値は以降クリーンとして扱う。
例: `html.EscapeString()`, パラメータ化クエリへのバインド

**propagator**
汚染を伝播させる処理。文字列結合や、値を包んで返すユーティリティ関数など。
明示的に宣言しないと伝播が途切れる場合に、モデルとして書き足す。

**barrier / barrier guard（sanitizer guard）**
特定の条件分岐が汚染を遮断することを表す仕組み。
`if isValid(x) { sink(x) }` の true 側では汚染を除去する、といった記述。
限定的な形の経路感度と言える。

**明示的フロー / 暗黙的フロー（explicit / implicit flow）**
- 明示的: データが直接代入・受け渡しされる流れ。通常の taint 解析が追うのはこちら
- 暗黙的: `y = tainted ? 1 : 2` のように制御依存を通じて影響が及ぶ流れ。
  追い始めると分岐のたびに汚染が広がるため、injection 検出では通常無視する

**アクセスパス（access path）**
`o.f.g` のような、オブジェクトからフィールドをたどる経路。
フィールド単位で汚染を区別するために必要。

**k-limiting**
アクセスパスは原理的に無限に伸びるため、深さ k で打ち切って過大近似すること。
IFDS の「事実領域は有限」という制約を満たすために必須。実装では k=3〜5 程度が典型。
rivus の既定は k=3（設定で変更可能）。

**ラベル付き汚染（labeled taint）**
汚染をブール値でなく種別付きで扱うこと。
「HTML コンテキストではエスケープ済みだが JS コンテキストでは危険」を表現できる。
sanitizer は特定ラベルだけを除去し、sink は特定ラベルにのみ反応する。

**モデル（model）** ★
ソースを解析できない外部ライブラリの振る舞いを、手書きで記述したもの。
「`strings.Join` は引数の汚染を戻り値に伝播する」など。
**SAST の検出品質の大半は、エンジンの賢さではなくこのモデルの網羅性で決まる。**
rivus では「モデル」と「判定（verdict）」を厳密に区別する（10 節参照）。

---

## 4. 手続き間解析

**手続き内 / 手続き間（intra- / inter-procedural）** ★
- 手続き内: 関数ひとつの中で完結する解析
- 手続き間: 関数呼び出しを跨ぐ解析

**intra-file / inter-file**
ファイル境界を跨ぐかどうか。
Opengrep の `--taint-intrafile` は「関数は跨ぐがファイルは跨がない」という中間的な位置づけ。

**コールグラフ（call graph）** ★
「どの関数がどの関数を呼ぶか」を表すグラフ。手続き間解析の土台。
インタフェース越しの呼び出しがあると、呼び出し先の特定が近似になる。

**supergraph**
各関数の CFG を、呼び出しエッジと復帰エッジで繋いだ全体のグラフ。

**文脈感度（context sensitivity）** ★
呼び出し元ごとに解析結果を区別すること。
区別しないと、呼び出し元 A で渡した汚染が呼び出し元 B の結果に漏れ、誤検知の主因になる。

**call-string / k-CFA**
呼び出しスタックの直近 k 段を文脈のラベルとして使う方式。
k を上げると精度が上がるが、組み合わせが爆発する。

**関数サマリ（function summary）** ★
関数の入出力関係を一度だけ計算し、呼び出し側で展開する方式。
「引数 0 が汚染なら戻り値も汚染」「引数 1 が汚染なら sink に到達」といった記述。
関数の解析回数が呼び出し箇所数に依存しないため、スケールする。
**IFDS の summary edge に相当するものを、functional approach で計算する。**
実用的な inter-procedural 解析はほぼこれ。

**functional approach（関数的アプローチ）** ★
Sharir & Pnueli (1981) による手続き間解析の方式のひとつ。
各関数の効果を「入力の事実集合 → 出力の事実集合」という関数（サマリ）として先に求め、
呼び出し側ではそのサマリを適用する。呼び出し先から順に（bottom-up で）計算する。
IFDS の tabulation（呼び出し元からのオンデマンド探索）とは別物。
**rivus はこちらを採用する。** 全関数の完全なサマリが先に揃うので、
「seed なしで sink 側から source を引く」ようなクエリが素直に実現できる。
文献を探すときは「functional approach」「summary-based analysis」で探す。

**IFDS** ★
Interprocedural, Finite, Distributive, Subset problems。
Reps, Horwitz, Sagiv (POPL '95) による枠組み。
**データフロー解析をグラフ到達可能性問題に還元する。**
以下の3条件を満たす問題に適用できる。
- Finite: 事実の領域が有限
- Distributive: 遷移関数が合流演算に対して分配的（`f(a ∪ b) = f(a) ∪ f(b)`）
- Subset: 束が冪集合、合流が和集合または積集合

taint 解析はこの3条件を満たす。定数伝播は分配的でないため満たさない。

**分配性（distributivity）** ★
`f(a ∪ b) = f(a) ∪ f(b)` が成り立つこと。
これが成り立つと **事実を集合単位でなく1個ずつ独立に追える**ため、
状態数が 2^|D| から |D| に落ちる。IFDS の核心。

**0 事実（zero fact / tautological fact）**
常に真である特殊な事実。何もないところから事実を生成（gen）するために使う。
`0 → d` のエッジが「無条件に d が発生する」を表す。source の表現に必要。

**exploded supergraph**
supergraph の各ノードを事実の数だけ複製したグラフ。
ノード `⟨n, d⟩` は「プログラム点 n で事実 d が成り立つ」を意味する。
**事実 d が n で成り立つ ⟺ ⟨n, d⟩ が ⟨entry, 0⟩ から到達可能。**

**valid path / realizable path**
呼び出しと復帰が括弧として対応している経路。
これを守らないと「A から入って B に戻る」あり得ない経路をたどってしまう。
文脈自由言語による到達可能性（CFL-reachability）の問題になる。

**tabulation アルゴリズム**
IFDS を解くワークリストアルゴリズム。
path edge を伝播させ、手続きの出口に達したら summary edge として記録・再利用する。
計算量は O(E·D³)（E は supergraph のエッジ数、D は事実領域の大きさ）。

**path edge / summary edge**
- path edge: 手続き内で「入口に d1 で入ると n で d2 が成り立つ」
- summary edge: 呼び出し点から復帰点への「d1 が入ると d2 が出る」。これが関数サマリの実体

**IDE**
Interprocedural Distributive Environment problems。IFDS の拡張。
各事実が **別の束の値** を持てるようになり、エッジには値を変換する micro-function が乗る。
IFDS は「値の束が真偽値だけ」の IDE の特殊ケース。
ラベル付き汚染やアクセスパスを、事実を増やさずに値側で扱いたいときに使う。

**SCC（Strongly Connected Component, 強連結成分）**
互いに到達可能なノードの集まり。コールグラフでは相互再帰する関数群。
SCC を1点に縮約するとグラフが DAG になり、トポロジカル順（呼び出し先から先）に処理できる。
SCC の内部だけは不動点まで反復して解く。

---

## 5. 精度の軸（〜感度）

**フロー感度（flow sensitivity）**
文の実行順序を考慮するか。しないと `x = sanitize(x)` の前後を区別できない。

**経路感度（path sensitivity）** ★
分岐条件の内容を考慮するか。
`func render(s string, escape bool)` のように引数の値で挙動が変わるケースは、
経路感度がないと正しく扱えない。
**IFDS は path-insensitive。** 合流で和集合をとる前提が枠組みに組み込まれているため。

**フィールド感度（field sensitivity）** ★
`o.x` は汚染だが `o.y` はクリーン、を区別できるか。
区別しないと構造体を一箇所触った時点で全体が汚染になり、実用に耐えない。
一方コレクション（配列・map）は要素を区別しないのが普通。

**オブジェクト感度（object sensitivity）**
同じクラスの異なるインスタンスを区別するか。主に OOP 言語のポインタ解析で使われる軸。
rivus は採用しない（コストに対して injection 検出での見返りが薄い）。

**エイリアス解析 / ポインタ解析（alias analysis / points-to analysis）** ★
「2つの参照が同じ実体を指しているか」を判定する解析。
`b := a; b.Name = tainted` としたとき `a.Name` も汚染されている、を追うために必要。
**taint 解析本体より重くなることすらある。**
taint とエイリアスを同時に IFDS で解こうとすると分配性が壊れるため、
FlowDroid はオンデマンドで後ろ向き IFDS を起動する方式をとっている。
rivus は完全なエイリアス解析を作らない（必要箇所の列挙に留める）。

**動的ディスパッチ（dynamic dispatch）** ★
インタフェース越しなど、呼び出し先が静的に確定しない呼び出し。
全実装を候補にすると精度が落ち、絞りすぎると見逃す。

**devirtualization（脱仮想化）**
動的ディスパッチの呼び出し先を静的に一意に特定すること。

**type flow analysis**
Dart SDK の AOT コンパイラが持つ whole-program の型フロー解析。
tree-shaking のために devirtualization を行う。
**精度の高いコールグラフが既製品として得られるため、Dart で自作する際の有力な土台。**

---

## 6. 健全性と精度

**健全（sound）** ★
見逃しがないこと。実際に起こり得るフローをすべて含む＝過大近似。

**完全（complete）** ★
誤検知がないこと。

**soundy**
「原理的には健全を目指すが、リフレクションや eval など特定の言語機能は意図的に無視する」
という実用的な立場。実在するツールはほぼこれ。自作でも最初にこの立場を宣言しておくとよい。

**過大近似 / 過小近似（over- / under-approximation）**
- 過大近似: 実際より広く見積もる。見逃しは減るが誤検知が増える
- 過小近似: 実際より狭く見積もる。誤検知は減るが見逃しが増える

**FP / FN（false positive / false negative）**
誤検知 / 見逃し。

**プログラムスライシング（program slicing）**
ある値に影響を与える（または与えられる）文だけを抽出すること。
- 後ろ向きスライス: その値の**由来**をたどる。引数の provenance 追跡はこれ
- 前向きスライス: その値が**影響する先**をたどる。taint 解析はこれに近い

---

## 7. ツールと製品

**CodeQL**
GitHub の semantic code analysis エンジン。
コードをリレーショナル DB に変換し、QL というクエリ言語で問い合わせる。

**QL**
CodeQL のクエリ言語。宣言的なオブジェクト指向ロジック言語で、
内部的には Datalog 系のエンジンでボトムアップ評価される。再帰が素直に書ける。

**Datalog**
論理プログラミング言語の一種。再帰的なクエリを効率的に評価できる。
プログラム解析の分野で広く使われる。

**extractor**
ソースコードを解析用の DB / 中間表現に変換するコンポーネント。
CodeQL ではコンパイル言語に対してビルドをトレースし、
コンパイラの起動引数からファイル集合とフラグを取得する。
rivus では「ソース → 解決済み AST → CFG → 中立 IR」までを担当する。

**build-mode**
CodeQL の DB 構築モード。`autobuild` / `manual` / `none`。
`none`（ビルドレス抽出）は C/C++、C#、Java、Rust で利用可能。**Go は非対応。**

**variant analysis**
1件の脆弱性を見つけたら、同じパターン（variant）をクエリ化して横展開する手法。
CodeQL の設計思想の中核。

**MRVA（Multi-Repository Variant Analysis）**
VS Code の CodeQL 拡張の機能。自作クエリを多数のリポジトリに一括で投げる。

**SARIF（Static Analysis Results Interchange Format）**
静的解析結果の標準交換フォーマット。
**GitHub の Security タブ（code scanning）へのアップロードは、
パブリックリポジトリか、GitHub Code Security を有効にした組織のリポジトリに限られる。
プライベートリポジトリで Code Security なしには使えない。**
（出典: GitHub Docs "Uploading a SARIF file to GitHub"）
rivus では副次的な出力形式であり、本体は構造化 JSON。SARIF の用途は
パブリックリポジトリでの利用、VS Code の SARIF Viewer 等のローカルビューワ、
将来 Code Security を契約した場合や他の SARIF 対応基盤での互換性。

**GHAS（GitHub Advanced Security）**
2025年4月に **GitHub Code Security**（$30/active committer/月、CodeQL を含む）と
**GitHub Secret Protection**（$19/同）に分割された。パブリックリポジトリは無料。

**active committer**
GHAS の課金単位。有効化されたリポジトリに過去90日以内にコミットが push されたユーザー。

**Semgrep / Opengrep**
パターンベースの静的解析ツールと、その LGPL-2.1 フォーク。
Opengrep は Semgrep が商用側に移した機能（cross-function taint など）を無償で維持している。
`--taint-intrafile` で関数跨ぎ（ただし同一ファイル内）の taint 解析が可能。

**Joern / CPG（Code Property Graph）**
Apache 2.0 のコード解析基盤。ソースを CPG（AST・CFG・データ依存を統合したグラフ）に変換し、
Scala の DSL で `reachableByFlows` などを問い合わせる。
**無償でファイル跨ぎの手続き間解析ができる数少ない選択肢。** Dart・Rust には非対応。

**FlowDroid**
Android 向けの taint 解析ツール。IFDS ソルバ Heros と Soot の上に構築されている。
論文と実装が揃っており、この分野の教材として最良。

**Heros / Phasar**
IFDS/IDE ソルバ。Heros は Java、Phasar は LLVM IR（C/C++）向け。

**Flowistry**
Rust の MIR 上で information flow 解析とスライシングを実装したライブラリ。

**MIRAI**
Rust の MIR 上で抽象解釈を行う静的解析器。tag/taint の伝播をサポートする。

**Pysa**
Meta の Python 向け taint 解析エンジン（Pyre の一部）。

---

## 8. 言語別のパッケージ

### Go

| パッケージ | 役割 |
|---|---|
| `go/ast` | 構文木 |
| `go/types` | 型解決 |
| `golang.org/x/tools/go/packages` | パッケージのロードと依存解決。**ビルドせずに型解決済み AST が得られる** |
| `golang.org/x/tools/go/ssa` | SSA と CFG |
| `golang.org/x/tools/go/ssa/ssautil` | SSA 構築のヘルパー |
| `golang.org/x/tools/go/callgraph` | コールグラフ（cha / rta / vta） |
| `golang.org/x/tools/go/analysis` | 解析器のドライバ・フレームワーク |

**`ssa.NaiveForm`**
SSA ビルダーのオプション。指定するとローカル変数のレジスタ昇格を行わず、
`Alloc` / `Store` / `UnOp(*)` が残った素朴な形になる（φ も出ない）。
指定しない場合と比較すると「SSA 化とは何をする変換か」が視覚的に分かる。
**rivus の Go extractor はこの形で IR を出す。** 指定しない lifted 版は捨てず、
コア側の SSA 化の正しさを検証する differential test の oracle（正解役）に使う。

**`ssa.InstantiateGenerics`**
ジェネリクスを具体的な型引数ごとに実体化して SSA を作るオプション。
`vta.CallGraph` に渡す SSA 関数はこのフラグ付きで構築されている必要がある。

**`Referrers()`**
SSA 値がどこで使われているかを返す。def-use 鎖が実質タダで手に入る。

### Dart

| パッケージ | 役割 |
|---|---|
| `package:analyzer` | パース + 型解決 + element model。pub.dev で公開 |
| `package:kernel` | Kernel AST（`.dill`）の読み書き |
| `pkg/vm`（SDK 内） | type flow analysis。pub.dev には未公開 |
| `custom_lint` | lint プラグインのドライバ |

**CFG・SSA・データフロー枠組みは公開 API に存在しない。** 自作が必要。

### Rust

| クレート / モジュール | 役割 |
|---|---|
| `syn` | パースのみ。型解決なし |
| `rustc_middle::mir` | MIR。CFG の形をしている。`rustc_private` 経由 |
| `rustc_mir_dataflow` | データフロー解析フレームワーク。gen/kill と不動点ソルバが標準搭載 |
| `rustc_public`（旧 `stable_mir`） | 外部ツール向けの MIR 公開 API。nightly のみ、まだ不安定 |
| `dylint` | lint ドライバ |

---

## 9. この会話で出てきた設計上の論点

**値依存のサニタイズ（value-dependent sanitization）**
`func render(s string, escape bool)` のように、引数の値によって sanitize するかが変わる関数。
単一のサマリでは表現できず、path sensitivity が必要になる。
対策は、モデルによる上書き → barrier guard → **サマリの特殊化（引数の定数値ごとに別サマリ）**
→ 述語付きサマリ → シンボリック実行 + SMT、の順にコストが上がる。
実務的には「サマリの特殊化」までで大半が片付く。
rivus は最初の2つ（モデル上書き、値依存ノードとしての記録）を中核とし、
余裕があればサマリの特殊化まで。述語付きサマリ以降は実装しない。

**terminal kind（由来追跡の打ち切り理由）**
後ろ向きの由来追跡が完全にはできない以上、
**どこでたどれなくなったかを分類して記録する**ことが重要になる。
LLM に判定を委譲する設計では、由来そのものより「不確実性の所在」が価値を持つ。
列挙は閉じた集合とする方針で始めた(ADR-008)。実際には、kind を足しても出力のスキーマ `rivus.result/1` のバージョンは変えていない(ADR-043 の追記)。
一覧の正本は `internal/taint/result.go` の定数(2026-11-22、9c26a39 時点で下の表のとおり)。

| kind | 意味 |
|---|---|
| `config_read` | 設定ファイル / 環境変数 |
| `exported_api_param` | 公開 API の引数。由来はリポジトリ外 |
| `global_var` | パッケージ変数。書き込み箇所は列挙済み |
| `dynamic_dispatch` | インタフェース。候補 N 件を列挙 |
| `unmodeled_library_call` | 外部ライブラリでモデル未定義 |
| `dependency_body_unread` | モデルのあるインターフェースの呼び出しの候補に、本体のある依存の関数があったが、読まなかった(ADR-075) |
| `project_body_unread` | プロジェクトの関数の本体を、extractor が読めなかった(Rust の async closure など。ADR-109)。依存の関数は `dependency_body_unread` |
| `reflection` | reflect / DI。追跡不能 |
| `concurrency` | channel / goroutine 境界 |
| `container_element_imprecision` | コレクション要素を区別していない |
| `depth_limit` | 探索上限に到達 |
| `ffi_boundary` | cgo、`dart:ffi`、Rust の `extern` / unsafe（追加） |
| `unresolved_callee` | 関数値・クロージャ経由の呼び出しで呼び出し先が確定しない（追加） |
| `missing_source` | 生成コード未生成、proc-macro 展開結果、パッケージ解決失敗などで見えない。サブ理由を文字列で持つ（追加） |
| `recursion_approximation` | SCC を近似で閉じた（追加） |
| `stored_data_boundary` | 保存されたデータ(DB など)の境界で、そこから先は追えない |
| `model_approximation` | モデルの流れが「近似」と書いたもの。通った finding に付く（ADR-067） |
| `numeric_stop` | source のデータが数値や真偽値になり、そこから先は追わない（`-numeric-taint` で追える） |
| `unsupported_construct` | extractor が表せなかった構文。その中の動作は解析に入っていない |
| `assumed_call` | 呼び出し先が見えない所で呼ばれるクロージャを、作った場所で、呼ばれたものとして適用した（ADR-059）。モデルも本体もない関数に渡した関数値は、呼ばれて(ほかの引数がどの引数にも届きうる)、返り値が結果に届くと見なした（ADR-116） |
| `access_path_truncated` | 経路の段で、値がアクセスパスを深さの上限で切った所を通った。この先の流れは、切ったために出ているだけかもしれない |
| `pointer_alias` | ポインタ越しの書き込みを別名で近似した所を通った。流れが実在しないことがある |
| `field_imprecision` | sink が引数のフィールドを指定するのに、汚染がそのフィールドにあると言えない（ADR-074） |
| `field_unresolved` | sink が指定するフィールドが、プログラムに無い（綴りの誤りなど）。sink は値全体として扱う |
| `global_holds_nothing` | `holds: nothing` のモデルが、グローバルへの流れを止めた（ADR-056） |

リテラル定数に着いた由来追跡は、kind ではなく `resolved` として出る（terminal kind は「たどれなかった理由」のための分類）。

**`attributable_to`**
各 kind に付ける属性。`target`、`rivus`、`mixed` のいずれか。
- `target`: 対象（解析される側のコード）の性質に由来し、rivus を改善しても解決しない
- `rivus`: rivus 側の都合であり、改善すれば減らせる
- `mixed`: 言語機能に由来するが、rivus の精度向上でも減らせる（`dynamic_dispatch` など）。集計では `rivus` と別に出す

`attributable_to` の割り当ては、`assumed_call` が `mixed`、`stored_data_boundary` が `target`、上の表の新しい kind の残りが `rivus`。

`attributable_to: rivus` の比率がそのまま「rivus の未熟さの指標」になり、投資先を数字で示す。
kind ごとの割り当ては ARCHITECTURE.md を参照。

---

## 10. rivus 固有の用語

**モデル（model）と判定（verdict）**
- モデル: 関数 1 つの振る舞いの記述。再利用する。YAML のモデル設定ファイルに保存する
- 判定: 検出結果 1 件（経路全体）についての「無害 / 有害」の評価。再利用しない。検出結果の注釈にのみ残す

LLM の出力はこの 2 種類に分け、混ぜない。

**`origin`**
モデルの出所。`builtin` / `human` / `llm`。`llm` の場合は `decided_at`、`rationale`、
`model_version`（判定した LLM）を必須とする。

**`--deterministic-only`**
`origin: llm` のモデルをすべて無視して実行されるフラグ。
これにより「rivus 単体は決定論的」と断定できる。

**ガード（guard）**
サマリの各エントリに付ける、引数由来の分岐条件。
「`escape == true` のとき sanitize される」のような対応関係を記録するためのもので、
条件を評価して判定することはしない(引数が定数のときだけは例外。下の「ガードを評価する例外」を参照)。1 サマリあたり 4 個程度を上限とする。

**条件ガードの判定対象**
関数内ローカルの分岐（ループカウンタの比較など）はガードに含めない。
サマリの外から見て意味があるのは、呼び出し側が制御できる引数に由来する条件だけだから。

**differential test**
同じ入力を 2 つの独立した実装に通し、結果を照合するテスト。
rivus では、コアの SSA 化と lifted `go/ssa` の出力を照合する。

**functional approach / tabulation の使い分け**
rivus の「sink だけ指定して到達し得る source を列挙する」クエリは、
seed となる source を与えずに投げる形なので、全関数のサマリを先に作る
functional approach の方が素直に実現できる。

**完全でない解析（incomplete analysis）**
全体の時間切れ(`-timeout`)、未収束(`-max-passes` の上限)、関数ごとの打ち切り(1 関数の時計 `-func-timeout`、作業量の上限)のどれかで終わった解析。見つけた分の結果は全部出したうえで、終了コード 4 で終わる。`summary.incomplete`(bool)と `summary.incomplete_reasons`(`timeout`、`not_converged`、`func_timeout`、`budget`)に出る。関数ごとの打ち切りを数えるのは、基準が「流れを落としたか」だからで、その関数の要約は流れを欠くことがある(ADR-120 の追記)。打ち切った関数は、`depth_limit` の注記(`func`、`detail` の先頭が `func_timeout:` か `budget:`)に、関数ごとに出る。

**エントリポイント（entry point）**
コールグラフの根。ライブラリには `main` がないため、
「どこから先が外部入力か」を言語・用途ごとに既定と追加指定で決める。

**サイト（site）**
ルールの source / sink にマッチする、コード上の箇所。`rivus sites` が列挙する単位で、安定した ID を持つ。
L2 の詳細分析（`rivus query --site <ID>`）の seed になる。

**L0 / L1 / L2**
rivus の絞り込みの 3 段階。
- L0: サイトの列挙（ローダ ＋ シンボル照合のみ）
- L1: L0 に加えて、sink の引数がリテラル定数だけのものにマークを付ける（落とさない）
- L2: 詳細分析（サマリ、ラベル、由来追跡、不確実性の記録）

L0 / L1 は判定ではなく列挙。grep と違って型解決つきなので、別名インポートなども拾える。

**エージェント（agent）**
LLM を使って自律的にタスクを進めるプログラム。rivus の主要な利用者の 1 つで、
脆弱性の有無を判定するときのコード分析の道具として rivus を呼び出す。

**指紋（fingerprint）**
サイトや検出結果が持つ、複数の種類のハッシュ値の集まり。名前は役割で付け、末尾の `@N` がアルゴリズムのバージョン。
- 同一性のキー（`structural@1`、`content@1`）: 旧新の対応付けに使う
- 変化検知のキー（`expr@1`、`body@1`）: 対応付けた後で、中身が変わったかを判定する
- 類似度のキー（将来 `sim@1`）: simhash。対応付けで残ったものに候補と距離を示す

**simhash**
似た入力に対して似たハッシュ値を返す（近いほどビットの差が小さい）ハッシュ。通常のハッシュは 1 文字違うだけで全く別の値になるが、simhash は距離で類似度を測れる。
rivus では ID ではなく、リネームや移動をまたぐ対応付けの補助として使う予定（Phase 1 のスコープ外）。

**ブックマーク（bookmark）**
エージェントが、前回の探索で見た箇所を保存した記録。ID、指紋（全部）、見たときのスニペットを含む。
保管はエージェント側の責務で、rivus はステートレスのまま。再訪のときに `rivus resolve` に渡す。

**`rivus resolve`**
ブックマークの指紋を現在のコードに適用して、対応する箇所を答えるコマンド。
状態は `unchanged` / `changed` / `rematched` / `ambiguous` / `unresolved`。
一致の根拠（どのキーで一致したか）を事実として示す。同一かどうか・安全かどうかは判定しない。

**`flow`（resolve の結果）**
検出結果のブックマークについて、「今も届くか」を表す。`present` / `absent_now` / `not_evaluated`。
`absent_now` は「現在のコードとモデルの範囲では届く経路が見つからない」という事実で、安全の判定ではない。

**pure（モデル）**
その関数を通って汚染が流れない、というモデル（`pure: true`）。ロック、時計、ログ、述語など。
モデルのない関数に対する「引数が戻り値に流れる」という保守的な既定と、それに伴う不確実性を、取り除く。

**モデルカバレッジ（model coverage）**
モデルがなくて rivus が推測した関数やインタフェースメソッドの一覧。検出結果の経路に載っている数の順に並べ、
どの関数にモデルを書くと効くかを示す（`rivus models coverage`）。

**suppression（抑制の記録）**
`origin: llm` の sanitizer が、実際に汚染を除去した場所。モデルがなければ検出結果になっていたものを、検出結果としては出せないので、別に残す。

**entry point（`kind: param` の source）**
フレームワークが汚染を渡す関数の引数を、関数のシンボルと位置で宣言するルール。

**値依存ノード（conditional node）**
検出結果の経路で、「結果が呼び出し側の渡す値で変わる」場所。関数の両側（条件が真のとき・偽のとき）に違う結果があるときだけ出す。
どちらが起こるかは判定せず、どの値のときにどの結果かを書き出す（`conditional_nodes`）。

**由来追跡（provenance）**
値依存ノードの条件が依存する引数の値を、呼び出し側から後ろ向きにたどること。
定数に着いたら `resolved`、たどれなくなったら、その理由を terminal kind で分類して `unresolved` に出す。

**ガードを評価する例外**
流れが起こる条件(ガード)のうち、引数に由来するもの。評価はせず、どの側に入ったかを記録する（ADR-006、ADR-035）。
引数が定数で、条件が定数の比較だけでできているときに限って、条件を評価して、起こりえない流れを捨てる。

**セル（cell）**
taint 解析が汚染を持つ場所。`Root`（SSA の値、複数の戻り値のうちの 1 つ、または解析中の関数の戻り値）と、アクセスパスの組。汚染の状態は、セルから汚染の要素の集合への写像（ARCHITECTURE.md 4.2）。

**parts（sink のフィールド）**
sink が、位置の引数のうち、どのフィールドが危険かを指定する書き方（ADR-074。`parts: [{arg: n, field: "A.B"}]`）。フィールドにある汚染は確かな検出で、フィールドにあるとは言えない汚染は `field_imprecision` を付けて出す。書き方は MODELING.md、設計の理由は DESIGN-field-sinks.md。

**valuecalls（関数値の候補）**
関数値(クロージャ、関数ポインタ、`dyn Fn`)の呼び出しの呼び出し先の候補を、IR の上で 1 か所で決める層（`internal/valuecalls`）。候補を絞ってよいのは、出所をもれなくたどれたときだけで、たどれなければ全候補を残す。絞ったときは、絞る前の数（`narrowed_from`）と、根拠（`basis`）を出力に残す（ADR-045、079）。

**閉じた世界（closed world）**
「起点があり、外からシンボルで呼べる関数がなく、呼び出し元が全部読まれている」とみなすプログラムの読み方。この前提を入れて絞ったときは、不確実性に `basis: closed_world_entry` を付けて示す（ADR-079）。

**構造の層 / taint の層（グラフの層）**
`rivus index` が保存するファイルの、層。構造の層は、関数、呼び出し点、エッジ、サイト。taint の層は、findings と不確実性で、保存したファイルにはない（JSON の index に出す）。どの層があるかは、ファイルの `meta` に記録する。層がないことを「findings がない」と読み違えないため。

**source hash（グラフの鍵）**
保存したグラフが、今のソースと合っているかを調べる鍵。設定（フラグ、ルール、rivus のバージョン、環境、ツールチェイン）、読んだファイルの内容、ファイルの一覧から作る。疑わしいときは、いつも「古い」として扱う（`rivus graph info -C` が `fresh` を返す）。

**ハブ（hub）と フロンティア（frontier）**
`rivus graph` の検索で、候補が多い動的な呼び出し。候補が 3 以下なら展開し、それより多いものは、境界（フロンティア）として見せて止める。20 を超えるものがハブで、`path` の探索には使わない（`hub_skipped`）。

**隔離（isolation）**
信頼できない Rust を、Docker の中で読むこと。`rivus analyze -lang rust` の既定。ネットワークなし、非 root、読み取り専用などの制限がある。Go と Dart には、まだ無い（docs/rust-isolation.md、ADR-064）。

**メモリの上限（memory limit）**
`-max-memory`(Go のランタイムの `debug.SetMemoryLimit`、環境変数 `GOMEMLIMIT` と同じ)。やわらかい上限で、プロセスのメモリの合計が近づくと GC が強くなる。守る保証はなく、生きている量より低いと守れず、GC が回り続けて CPU が増える。結果は変えない。指定がなければ、`GOMEMLIMIT`、それもなければ物理メモリ(cgroup があれば小さいほう)の 1/4。`summary.memory_limit_bytes` / `memory_limit_source` / `memory_limit_note` に出る(ADR-121)。
