# rivus アーキテクチャ: 履歴と計画

**この文書は歴史的な記録で、今の設計を説明するものではない。** 以前の `docs/ARCHITECTURE.md` にあった、Phase の計画、実施記録、実測の数字、レビュー待ちの項目を、書いた当時のまま移したもの。

- 今のアーキテクチャは [ARCHITECTURE.md](../ARCHITECTURE.md)。設計思想は [PRINCIPLES.md](../PRINCIPLES.md)。
- 決めた経緯と理由は [DECISIONS.md](../DECISIONS.md)。各 ADR の今の状態は [ADR-INDEX.md](../ADR-INDEX.md)。
- 本文の数字（行数、秒、MB、検出数）は、書いた当時の実測で、今の値ではない。本文の「今」「現在」も、書いた当時を指す。
- 節番号（2.7.x、9、10、11）は、ARCHITECTURE.md の旧い番号のまま。ADR などの引用が、ここへ辿れるようにしてある。
- 本文中の `ARCHITECTURE.md` の節への参照は、旧い版のもの。

目次:

- 2.7.4 の実装メモ（旧 ARCHITECTURE.md 2.7.4 の末尾）
- 2.7.5 構成（案）
- 2.7.6 移行の戦略
- 2.7.7 の実測、問い合わせの案、未決
- 4.4 の移動分: 初版の割り当てと、見直しの予定
- 5.4 の移動分: `max_implementations` の暫定値
- 9 性能目標
- 10 Phase 1 の細分化
- 11 レビューで確認したい点

---

## 2.7（履歴）Phase 2 に向けた具体化

2.1〜2.6 は Phase 1 の前に書いた。Phase 1 で解析エンジン（約 4,800 行）が実際に `go/ssa` から何を使っているかが分かったので、
IR が運ぶべきものを実測から決め直す。**ここに書くのは案で、ユーザーのレビューで確定する（ADR-039）。**

（この導入文は、旧版の 2.7 の冒頭。今の ARCHITECTURE.md の 2.7 は、確定した契約だけを書く。）

### 2.7.4 の実装メモ（旧 ARCHITECTURE.md 2.7.4 の末尾）

**2.1 で分かったこと（実装メモ）:**
- `rivus extract [-text] [-func 部分文字列] [-all]` で、IR を JSON Lines か人間向けの文字列で出せる。`testdata`、ghmint、rivus 自身、x/tools（5,258 関数、87MB、約 2 秒）が、検証つきで通り、`unsupported` は 0 件
- go/ssa の内部用の呼び出し（`ssa:deferstack`、`ssa:wrapnilchk`）は、シンボル `builtin.ssa:...` の静的な `call` として、そのまま出る。`defer` を持つ関数の出口には `rt.rundefers` の `call` が出る
- アドレス計算（`FieldAddr` / `IndexAddr`）は、使い道が `load` / `store` だけなら畳み込む。アドレスが値として渡るときだけ `addr_of` になる。グローバルのアドレスは、引数のない `addr_of`（`global` つき）
- `call` の `callee` は、静的な呼び出しで `func`（呼び出し先の関数 ID）と `symbol`（ルールのシンボル。無名関数では空）の両方を持つ。クロージャの直接呼び出しは、`value` の呼び出しにする（束縛した値を失わないため）
- φ は、前任ブロックの末尾の `copy` に落とす。1 つの前任が複数の φ に値を渡すときは、一時変数を経由する
- **まだ入れていないもの:** `ArgPos`（2.3 でサイトを IR の上に載せるときに入れる）、`dynamic` の `candidates`（2.5）、`implementations`（`-implementations` で指定したメソッドだけ）

**2.2 で分かったこと（実装メモ）:** `internal/irssa`。
- 変数になるのは、(a) 2 回以上書かれるローカル（φ を落とした `copy` の宛先）と、(b) `load` / `store` のアドレスとしてしか使われない `alloc`（昇格。go/ssa の lifting と同じ基準）。`recover` の root があるとき、entry から届かないブロックで `load` される `alloc`（名前つきの結果）は昇格しない
- 答えは `Ref`（命令の結果、φ、昇格した変数のゼロ値、未定義）。`Form.Operand(block, instr, k)` が、命令の k 番目の引数の定義を返す。昇格した変数の `load` は、その場所で変数が持つ値（直前の `store` の値、または φ）に解決される
- 検証は、go/ssa の lifting（昇格した `alloc` の φ の位置）、ナイーブな go/ssa の φ（落とした φ の位置）、独立な到達定義解析（使用ごと）。testdata、rivus 自身、x/tools（5,256 関数、φ 4,646 個、使用 67,640 箇所）で一致

**2.3 で分かったこと（実装メモ）:**
- **テキストの切り出しは、コアではなく言語ごとの `ir.Texts` が行う（ADR-039 の修正）。** スニペットの範囲（複合文は見出しだけ）、式の正規化（コメントと空白に依らない）、本体の正規化は、言語の構文の問題で、`expr@1` / `body@1` はこの出力のハッシュだから。IR は位置（`File`、`Offset`、`Line`、`Col`）だけを運び、`Texts` は extractor と同じプロセスで、同じ構文木を使う。別プロセスの extractor を作るときは、`Texts` を同じ extractor が提供する形を考える
- 追加したフィールド: `Func.Symbol` / `Recv` / `Dep`、`Callee.Recv`、`Instr.AddrPos`（`load` / `store` に畳んだアドレス計算の位置。フィールドの source のサイトの位置に使う）、`Pos.Line` / `Col`
- アドレス計算を畳むのは、使い道が 1 つの `load` か `store` のときだけにした（`x.f += 1` のように 2 つあるときは `addr_of`。ソース上で触るのは 1 回なので、サイトも 1 つにするため）
- **検証:** 同じコードを go/ssa 経路と IR 経路で列挙し、サイトの ID・指紋 4 種・位置・スニペット・ラベル・`literal_only`・並び順が完全に一致することを、testdata（212 サイト、フィールド 7）、rivus 自身、ghmint、x/tools（243 サイト）で確かめた

**2.4〜2.6 で分かったこと（実装メモ）:**
- **新しいパッケージ:** `internal/irssa`（2.2）、`internal/sg`（SSA グラフ。IR ＋ `irssa` から、値・命令・ブロック・支配木を作る。昇格した変数の `load` / `store` と、φ を落とした `copy` は消え、定数・パラメータは値になる）、`internal/taint`（エンジン。`go/ssa` の値の代わりに `sg` の値を使う）
- **エンジンを動かす言語固有の操作は、シンボル付きの `call`:** `chan.send` / `chan.recv`、`iter.start` / `iter.next` / `iter.next_string`、`rt.rundefers`。`select` は、extractor が送信と受信の `call` に分ける（どれが起きるかはデータではない）。`index_load` / `index_store` の `Keyed` は、キーで引くこと（map、文字列）。`Folded` は、アドレス計算を畳んだ `load` / `store`。`convert` の `Operator` は、変換の種類
- **値の同一性:** `go/ssa` のタプル（複数戻り値）は、`call` の `Dsts` の 1 つずつが値になる。`Extract` は消える
- **コールグラフ:** VTA は NaiveForm の上だと lifted の上より粗い（`(go/types.Object).Name` の候補が増える）。そこで `gossa.Options.WithLifted` で lifted 版も作り、グラフはそちらから取って、NaiveForm の `call` へ位置と種類で対応づけて `candidates` に入れる。この対応づけで、`ssa:deferstack` のような lifting で消える呼び出しを避ける
- **関数だけの値:** 何も捕捉しない無名関数を変数に入れて呼ぶ場合、`sg` が `make_closure`（捕捉なし）から呼び出し先を静的に解く（これを落とすと、`unresolved_callee` が増えた）
- **検証（旧経路との突き合わせ。JSON を比べた）:** `testdata` の全ケース、rivus 自身（36 検出、614 不確実性）、ghmint（15 検出、176 不確実性）で完全に一致。x/tools（4 検出、10,628 不確実性）も、検出・抑制・収束は一致し、不確実性は 1 件だけ違う（generic な関数の中の range-over-func で、位置のない合成された呼び出し。`excfg.CFG.Out$1` を呼ぶ側）。許容する
- **性能（x/tools、15.6 万行）:** 107 秒・最大 4.9 GB → 97 秒・最大 2.4 GB（ADR-040）。メモリが増えた原因は、関数ごとの解析の状態を結果の収集まで持っていたことだった（`analyzeFunc` の最後に、検出・抑制・不確実性だけ残して手放す）
- **依存:** `go list -deps` で、`taint`・`sites`・`sg`・`irssa`・`ir`・`report`・`model` に `go/ssa`・`x/tools`・`gossa`・`go/types`・`go/ast` は入っていない。`taint`（ガードの定数評価）と `report` には `go/token` と `go/constant` が入る（標準ライブラリの、言語に依らない部分。いずれ自前にできる）。`gossa` は、Go の extractor と、`rules.Expand`（実装列挙。Go のパッケージを引く）からだけ使われる

### 2.7.5 構成（案）

- `internal/ir`: 型（`Func`、`Block`、`Instr`）、JSON Lines の符号化・復号、検証（全ブロックに終端がある、`Dst` が宣言済み、など）
- `internal/frontend/goir`: `go/packages` ＋ `go/ssa`（NaiveForm）→ IR。`gossa` の下の層（ロード、型、コールグラフ、実装列挙）を使う
- `internal/ir/ssa`（名前は未定）: IR ＋ `ssabuild` → 値の名前つきの SSA 形
- `internal/taint` / `internal/sites`: **`go/ssa` を import しない。** IR の SSA 形だけを見る。`gossa` への依存も取り除く
- **Go では、同じプロセスの中で IR の型を直接渡す。** JSON Lines は `rivus extract` の出力と、Rust / Dart extractor（別プロセス）の入力のためだけに使う。ただし、直接渡す経路と JSON を経由する経路の結果が一致することをテストする

### 2.7.6 移行の戦略（実施済み）

エンジンの移植は大きい（`taint` 約 4,800 行）。壊さないために、**旧経路（`go/ssa` 直）と新経路（IR 経由）を、移行が終わるまで両方動かし、`Report` を突き合わせる。**
対象は `testdata` の全ケース、ghmint、rivus 自身、`x/tools`。結果が一致するまでは旧経路を消さない。
意図した差（IR で表せなくなったもの、位置の表現）が出たら、ADR に書いてから許容する。

### 2.7.7 の実測、問い合わせの案、未決（旧 ARCHITECTURE.md 2.7.7 から）

実測（コアの `ir.StreamTexts` の説明にあった数字も、ここに移した: x/tools で、命令の位置 28 万個に対して、異なるスニペットは 1.6 MB 分）:

- **実測（x/tools、依存を含む 8,096 関数）:** IR 120 MB に対し、テキストの分は 27 MB（+23%）。位置ごとにスニペットをそのまま書くと 53 MB になるところを、グループ化で 1.6 MB 分の文字列に絞れた。問い合わせの対話は要らない

問い合わせの案（実装は、これと違う形になった。今の契約は ARCHITECTURE.md 2.7.7）:

**問い合わせ:** extractor を常駐させ、コアと標準入出力で対話する形にする。最初は、**1 回の実行で必要な問い合わせが決まる**ものに絞る。
1. 実行の前に、コアが規則から、列挙してほしい**インタフェースのメソッドのシンボル**の一覧を extractor に渡す（`--implementations <file>`）
2. extractor は IR を出す。テキストは、`rivus sites` が必要とする分（サイトの位置）と、`analyze` が必要とする分（ガードの条件、呼び出しの引数）を、**最初から全部**出す（サイトに限らず、ガードに使いうる `branch` と、引数のある `call` も）。対話は後回し
3. 大きさが問題になったら、位置の範囲による絞り込み（`--text-for <positions>`）を足す

未決だった項目:

**未決（レビューで決めたい）:**
1. 実装列挙を、事前の一覧で渡す案は、規則の `match: implementations` がモデルのシンボルだけで決まる限りは足りる。呼び出しの先でシンボルが決まる場合は、対話が要る

---

## 4.4 の移動分

`mixed` の割り当ては暫定で、Phase 1 のステップ 1.12（ドッグフーディング）で実測してから見直す（`dynamic_dispatch` の候補 0 件を `rivus` にしたのはステップ 1.9 の実測による。ADR-032）。

（旧版の kind 表の見出しは「kind ごとの `attributable_to`（初版の割り当て）」だった。）

## 5.4 の移動分

上限の既定値は**暫定 100**。ステップ 1.9 で参照コーパスを使って実測してから確定する（項目 5 と同じ進め方）。ルールごとに `max_implementations` で上書きできる。

（実装の既定値は 300 になった。`internal/model/expand.go`、ADR-030 の追記。）

---

## 9. 性能目標

叩き台（現実的でないと分かったらレビューで見直す）は 2 軸で置く。

1. **一括解析**: 20 万行規模のプロジェクトを、ノート PC で 5 分以内。CI の PR チェックで回せる上限として置く
2. **問い合わせ**: サマリが揃った後の 1 回のクエリ（`rivus query --site`）の応答時間。
   エージェントが対話的に何度も呼ぶため、数秒以内を目標にする（数値はステップ 1.8 の実測後に決める）。
   `rivus sites`（L0 / L1）はそれより速く、ほぼ即時を目標にする

問い合わせ型の使い方では、重いサマリ算出と軽いクエリ応答を分け、**サマリを保存して再利用できる**形にする必要がある。

- 増分解析は Phase 1〜4 のスコープ外。ただしサマリの関数単位の独立性は壊さない
- メモリに全部載せてよい前提で始める。ノート PC を想定し、上限の目安は 8GB（これも仮説で、実測で見直す）

### 数字の扱い

上の数字は**実測前の仮説**であり、ステップ 1.8 で測ってから確定する（ADR-027）。

- **当面の対象は Go のみ。** Dart は `package:analyzer` のコールド解析、Rust は rustc のビルド時間が下限になり得るため、
  Dart スパイクと Phase 3 の最初の調査の後に別の数字を置く
- **規模の階層:**

  | 階層 | 規模 | 扱い |
  |---|---|---|
  | 小 | 〜 5 千行 | 開発中の高速な確認用 |
  | 中 | 〜 5 万行 | 日常の利用 |
  | 大 | 〜 20 万行 | 目標の上限（プライベートの実プロジェクトと、著名 OSS を想定） |
  | 超大 | 20 万行超 | 約束しない。動かなくなるのではなく、上限に達したら打ち切って `depth_limit` 等で記録する（段階的に劣化する） |

- **参照コーパス:** 各階層の実在の Go プロジェクトを数本選び、行数（`tokei` 等で数える）と計測値を `docs/` に記録する。
  著名 OSS を含める。具体的な選定はステップ 1.8 の直前に行う
- **頑健性（著名 OSS で回すための要件）:** 実在のプロジェクトは、cgo、ビルドタグ、vendor、一部パッケージのロード失敗などを含む。
  ロードエラーがあっても**全体を失敗させず**、解析できた範囲で続行し、見えなかった箇所を `missing_source` として記録する
- **問い合わせ応答:** クエリ時にソースのロードをやり直さずに答えられるよう、サマリ・コールグラフの辺・位置情報を保存する。
  サマリは `go/ssa` の型に依存させず `FuncID` と位置で表す（4.3 の定義はそうなっている）。常駐サーバ化は、この方式で足りなかった場合に検討する

---

## 10. Phase 1 の細分化

方針: 1 ステップごとに区切り、**各ステップの終わりで `git commit` ＋ ユーザーの確認**を挟む。
Phase 1 では中立 IR を経由せず、`go/ssa` から直接 taint 解析まで作る。`go/ssa` に触れるコードは `internal/frontend/gossa` に隔離する。

| ステップ | 内容 | 完了の目安 |
|---|---|---|
| 1.0 | 足場: `go.mod`（Go 1.27、`golang.org/x/tools` 最新）、CI（GitHub Actions）、LICENSE（Apache-2.0）、`testdata/` ハーネス | `go test ./...` が通る CI |
| 1.1 | ローダ: `go/packages` → `go/ssa`（`InstantiateGenerics` 付き）、`FuncID` 付与、ソース位置とスニペット切り出し | 小さな Go プロジェクトの全関数と位置を列挙できる |
| 1.2 | モデル: YAML 読み込み、シンボル照合（`exact` のみ先に）、組み込みの最小セット | source/sink/sanitizer を YAML で書いて照合できる |
| 1.2b | サイト列挙（`rivus sites`）: L0 の列挙、L1 のリテラルマーク、サイト ID、JSON 出力 | 型解決つきで sink の呼び出し箇所を列挙でき、grep との差（別名インポートなど）がテストで示せる |
| 1.3 | 手続き内 taint: ラベル、worklist、φ 相当の合流、sanitizer の `Drop` | 1 関数内の `testdata` ケース（検出する / しない）が通る |
| 1.4 | 出力 v0: 構造化 JSON（経路ノードとスニペット） | 1.3 の結果が JSON で読める |
| 1.5 | コールグラフ: `CallGraphBuilder` インタフェース、Go 実装（CHA → VTA）、SCC 縮約、トポロジカル順 | 呼び出し先から順に関数を列挙できる |
| 1.6 | 関数サマリ: 算出、展開、SCC 内の不動点。ファイル・パッケージ跨ぎ | 複数ファイル・複数パッケージにまたがる `testdata` が通る |
| 1.7 | ヒープ: フィールド感度（k=3）、クロージャ、グローバル、コンテナの扱い | 構造体・クロージャ経由のケースが通る |
| ★ Dart スパイク | **（実施済み。ADR-037。コードは削除した。git の履歴にある）** 1.5〜1.7 のあいだ、ただし Phase 2 の前に。2 日のタイムボックス（下記） | 可否の判断 ＋ IR 設計に反映すべき難所のリスト |
| 1.8 | 一方向クエリ: sink から source を引く / source から sink を引く。サイト ID を seed に取れる | seed なしのクエリと、サイト起点のクエリが動く。サマリの保存・再利用と、応答時間の初回計測 |
| 1.8b | **（実装済み。ADR-033）** ブックマークの解決（`rivus resolve`）: 指紋による対応付け（内容 → 構造）、状態の返却（`unchanged` / `changed` / `rematched` / `ambiguous` / `unresolved`）、検出結果の `flow` と `path@1` | ブックマークをした後でコードを変更（行の挿入、引数の書き換え、関数の削除、sanitizer の追加）しても、期待どおりの状態が返る |
| 1.9 | 不確実性の記録: terminal kind、`attributable_to`、`dynamic_dispatch` の候補列挙、`match: implementations`（展開上限と中止時の記録を含む） | すべての打ち切りが分類されて出る。参照コーパスで展開数を実測し、`max_implementations` の既定値を確定する。**実装済み（ADR-030 の追記、ADR-032）。既定値と `mixed` の割り当ての確定はレビュー待ち** |
| 1.10 | **（実装済み。ADR-035）** 値依存: 引数由来のガード抽出（taint ソルバ流用）、`conditional_nodes`、provenance、ガード上限 | ブリーフの `render` の例が期待どおりに出る |
| 1.11 | **（実装済み、複数経路を除く。ADR-034）** 運用機能: モデルカバレッジレポート、`origin` と `--deterministic-only`、結果のグループ化・安定 ID・経路数上限、`entrypoints` | `rivus models coverage` が動く |
| 1.11b | **（実装済み。ADR-036）** 自作 SSA 化（`ssabuild`）と differential test: `go/ssa` の NaiveForm 出力から SSA 化し、lifted `go/ssa` と φ の配置・到達定義を突き合わせる。Phase 1 の `testdata` を回帰テストにする | differential test が全 `testdata` で一致し、解析結果が変わらない |
| 1.12 | **（実施済み。ADR-037）** ドッグフーディング: rivus 自身を解析。`depth_limit` と `attributable_to: rivus` の比率を計測 | 計測値を README に記録 |

SARIF 出力は Phase 1 のスコープ外。JSON が固まってから。

### Dart スパイク（`spike/dart-cfg/`。削除済み、履歴にある）

タイムボックスは 2 日。コードは捨ててよい。成果物は README に書く「可否の判断」と「IR 設計に反映すべき難所のリスト」。

1. **CFG が組めるか。** 手書きの Dart スニペットで試し、CFG を DOT で出して目視確認する
   - `async` / `await`
   - `try` / `catch` / `finally`（特に `finally` が複数経路から到達する形）
   - `for-in` と `await for`（`Stream`）
   - ラベル付き `break` / `continue`
   - `switch` のパターンマッチング
2. **パッケージ解決が効くか。** melos などで分割されたモノレポで、`AnalysisContextCollection` がパッケージ境界を越えて参照を解決できるか
3. **生成コードが見えるか。** `build_runner` 実行後、`*.g.dart` / `*.freezed.dart` が解析対象として普通に扱えるか

2 と 3 は CFG とは別のリスクだが、駄目なら Phase 4 の前提が崩れるため同時に潰す。

### Phase 2 以降の見通し

- **Phase 2:** 中立 IR の切り出し。Go extractor（NaiveForm）が最初の実装例。`ssabuild` と differential test は Phase 1 のステップ 1.11b で導入済みなので、ここでは `gossa` を extractor に置き換えて IR を挟むことだけに集中する。Phase 1 の `testdata` がそのまま回帰テストになる

Phase 2 のステップ（案。2.7 のレビュー後に確定する）:

| ステップ | 内容 | 完了の目安 |
|---|---|---|
| 2.0 | **（実装済み）** `internal/ir`: 型、JSON Lines の符号化・復号、検証、人間が読める整形 | 手書きの IR が往復でき、不正な IR が検証で落ちる |
| 2.1 | **（実装済み）** Go extractor（`goir`）: NaiveForm → IR。`rivus extract` | `testdata` の全関数が IR になり、検証を通る。目視用の出力が読める。SQL への補間（`+`、`Sprintf`、`strings.Builder`）のケースで、`const` の中身と補間の形が IR に残ることを確かめる |
| 2.2 | **（実装済み）** IR → SSA 形（`ssabuild` の接続）。`testdata` と `x/tools` で、旧経路の SSA の値の使用関係と一致 | differential test が IR 経由でも通る |
| 2.3 | **（実装済み）** `sites` と指紋を IR の上へ。**ID と指紋が、旧経路と全ケースで一致** | `rivus sites` / `resolve` の出力が変わらない |
| 2.4 | **（実装済み）** `taint` の移植（手続き内 → 手続き間 → ヒープ → ガード）。旧経路との `Report` の突き合わせ | `testdata`・ghmint・dogfood で一致 |
| 2.5 | **（実装済み。2.4 の中でやった）** コールグラフを extractor へ。`call` の `candidates` として IR に入れ、コアは IR だけから SCC を作る | `x/tools` で一致し、時間が Phase 1 の ±20% 以内（実測: 107 秒 → 100 秒） |
| 2.6 | **（実装済み）** 旧経路の除去。`taint` / `sites` / `report` から `go/ssa` と `gossa` の import を消す | `go list -deps` で確認（`taint` に `go/token`・`go/constant` が残る。下記） |
- **Phase 3:** Rust。最初のタスクは「`stable-mir-json` 等の既存ツールで、実在のクレートがどれくらい通るかの確認」。成果物は「固定した nightly でビルドが通る Rust プロジェクトを解析できるツール」。想定より通らなければ、Rust の優先度を下げる判断もあり得る
- **Phase 4:** Dart。解決済み AST → CFG → IR。**最初の版を実装済み**（`extractor/dart`、ADR-041）。残りは、パターン、関数値のコールグラフ、`Stream`、依存パッケージのモデル

---

## 11. レビューで確認したい点

1. **【決定済み】** 4.4 の `dynamic_dispatch` / `unresolved_callee` / `concurrency` は `attributable_to: mixed`（3 値化）。
   Phase 1 のステップ 1.12 の実測後に見直す
2. **【決定済み】** 自作 SSA 化と differential test は **Phase 1 の終盤（ステップ 1.11b）**で導入する（ADR-026）。
   Phase 1 の間は lifted の `go/ssa` を使い、`testdata` が揃ってから差し込む
3. **【決定済み】** `global_load/store` は独立のまま、`slice` は `copy` の `view` フラグに畳む。命令数は 26（2.3）
4. **【決定済み】** `finally` は、extractor が出口ごとにブロックを複製する（ADR-037）。複製されたサイトは 1 つとして数える。投げうる呼び出しは、`try` の範囲の中ではブロックの終端にする
5. **【決定済み】** 性能目標は仮説として置き、ステップ 1.8 の実測で確定する（9 節、ADR-027）。20 万行は目標の上限。Go のみが当面の対象
6. **【決定済み】** ID と指紋の設計は 4.6 と ADR-028。検出結果の単位は (source サイト, sink サイト) の組
7. **【決定済み】** 上限を超えたら展開を中止して記録する。既定値は暫定 100 で、ステップ 1.9 の実測で確定する。sanitizer への展開は既定で禁止（5.4、ADR-030）
