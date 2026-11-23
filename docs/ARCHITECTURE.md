# rivus アーキテクチャ

状態: 今のアーキテクチャの説明（確認: 2026-11-22、`aa79d0e`）。コードと照合して書いた。古くなる前提で読むこと。

- 設計思想: [PRINCIPLES.md](PRINCIPLES.md)
- 経緯と理由: [adr/](adr/README.md)(1 件 1 ファイル。各 ADR の今の状態は、ファイルのヘッダ。一覧は `make docs-gen WHAT=adr`)
- 個別の設計: `DESIGN-*.md`（[DESIGN-graph.md](DESIGN-graph.md)、[DESIGN-memory.md](DESIGN-memory.md)、[DESIGN-capture.md](DESIGN-capture.md)、[DESIGN-field-sinks.md](DESIGN-field-sinks.md) など）
- 歴史・計画: [history/](history/)。この文書から移した Phase の計画と実測は [history/ARCHITECTURE-plans.md](history/ARCHITECTURE-plans.md)
- 用語: [GLOSSARY.md](GLOSSARY.md)

節番号は、ADR、コードのコメント、extractor の README が引用している。1〜8 節の番号は変えない。履歴・計画へ移した節には、同じ番号に案内を残してある。9〜11 節は、全部移した。新しい内容は 12 節以降に足した。

---

## 1. 全体構成

rivus は「脆弱性を報告する SAST」ではなく、**到達可能性に関する事実を問い合わせるクエリエンジン**である。
安全か危険かの判定は行わない。決定論的に確定できる事実と、確定できなかった理由を構造化して出す。

```
                 言語ごとの extractor（言語固有の知識が要る部分）
 ┌──────────────────────────────────────────────────────────────┐
 │ Go   extractor (Go 製・同梱・同じプロセス) go/packages → go/ssa(NaiveForm)│
 │ Rust extractor (Rust 製・外部・macOS は OS のサンドボックス)  rustc_public → MIR     │──┐
 │ Dart extractor (Dart 製・外部)  package:analyzer → 自作 CFG               │  │ サブプロセス起動
 └──────────────────────────────────────────────────────────────┘  │ （Go は IR を直接渡す）
                                                                    ▼ 中立 IR (JSON Lines, schema_version 付き)
 ┌──────────────────────────────────────────────────────────────────────────┐
 │ コアエンジン（Go 製・言語非依存）                                          │
 │                                                                          │
 │  IR 読み込み・検証                                                         │
 │     ↓                                                                    │
 │  関数値の呼び出しの候補を足す（valuecalls。13 節）                           │
 │     ↓                                                                    │
 │  SSA 化（dominator → φ 挿入）                                              │
 │     ↓                                                                    │
 │  コールグラフ（IR の call の candidates。SCC 縮約 → トポロジカル順）         │
 │     ↓                       ┌──────────────┐                             │
 │  関数サマリ算出（bottom-up） ←─ モデル(YAML)  │ source/sink/sanitizer/      │
 │     ↓                       └──────────────┘ propagator。組み込みも同形式 │
 │  クエリ（前向き / 後ろ向き / source-sink 組）                              │
 │     ↓                                                                    │
 │  由来追跡 (provenance) ・ 条件ガード ・ 不確実性の記録                      │
 │     ↓                                                                    │
 │  出力: 構造化 JSON（SARIF は出さない。ADR-009）                             │
 └──────────────────────────────────────────────────────────────────────────┘
```

コールグラフだけを保存して検索する経路（`rivus index` / `rivus graph`）は、このパイプラインの横にある（12 節）。

責務の境界は「**言語固有の知識が必要か**」で引く。CFG 構築まで（言語固有の脱糖を含む）は extractor、SSA 化から先はコア。

### 想定する使われ方（エージェントの道具）

rivus の主な利用者の 1 つは、脆弱性の有無を判定する **LLM エージェント**である。
エージェントは「疑わしい箇所を軽く洗い出し → 気になる箇所だけ詳細に分析する」という 2 段構えで使う。

| レベル | 内容 | 必要な処理 |
|---|---|---|
| L0 | ルールの source / sink にマッチする**サイト**（箇所）を列挙する | ローダ ＋ シンボル照合のみ（ほぼ即時） |
| L1 | L0 に加えて、sink の引数が**リテラル定数だけ**のものにマークを付ける。落とさず、事実としてマークを付けるだけ | 手続き内の簡単な確認 |
| L2 | 詳細分析（サマリ、ラベル、由来追跡、不確実性の記録） | 全部 |

```
rivus sites ./...                   # L0 / L1: 疑わしい箇所（サイト）の列挙
rivus query --site <ID> ./...       # L2: そのサイトの詳細分析
rivus resolve --bookmarks b.json ./...  # 前回見た箇所が今どこにあるかを答える（4.7）
```

- L0 / L1 は判定ではなく**列挙**であり、ADR-001（事実を出し、判定はしない）と矛盾しない
- サイトには**安定した ID**と**指紋（fingerprint）**を持たせ（4.6、ADR-028）、ID を L2 のクエリの seed として受け取れるようにする
- サイトの列挙は IR の `call`（呼び出し先 `FuncID`）で行うので、言語中立
  - 関数値の呼び出し（`value`）は、候補（`internal/valuecalls`。コピー・φ・パラメータを通って届く関数、または extractor のコールグラフ）のシンボルで照合し、候補ごとにサイトを作る（ADR-045）。同じ候補を、列挙と解析のグラフが見る
- 呼び出し口は、まず **CLI ＋ JSON 出力**。`query` 層は差し替え可能に保ち、必要になれば常駐サーバ（MCP など）を足す
- エージェントのコンテキストを圧迫しないよう、出力の詳細度を選べるようにする（7 節）

### 設計上の要点

- **IR は CFG の高さで切る。φ なしの CFG ＋ 書き換え可能なローカル変数。** SSA 化はコアに一本化する
- **bottom-up の関数サマリ（functional approach）で手続き間解析を行う。** 全関数の完全なサマリが先に揃うので、
  seed なしで sink 側から source を引くクエリが素直に実現できる
- **汚染はラベル集合で持つ。** 初期設計から入れる（後から入れられない）
- **soundy。** 扱わなかった箇所は必ず出力に記録する。記録は terminal kind で分類する
- **LLM の出力は「モデル」と「判定」に分ける。** 再利用するのはモデルだけ

---


## 2. 中立 IR

### 2.1 基本方針

- 関数 1 つが「基本ブロックの列」。各ブロックは「命令の列 ＋ 終端命令 1 つ」
- 値は関数ローカルの**名前付きローカル変数**。同じローカルへの複数回の代入を許す（φ はない）
- メモリ操作（`alloc` / `load` / `store`）は、Go の NaiveForm や Rust の MIR と同じ形で表す
- 例外・`async`/`await`・`defer`・`go` は**新しい命令にせず、エッジの種別と `call` のフラグで表す**
- **言語固有の脱糖は extractor の責務。** イテレータ（`for-in`、`range`、`Stream`）は
  「イテレータ取得の `call` ＋ `next` の `call` ＋ 後方エッジ」に展開して出す
- 分岐条件は**式言語を作らず、条件を計算した値への参照**として持つ。人間可読な式は、ソース範囲から元のテキストを切り出して提示する

### 2.2 識別子と位置

| 名前 | 内容 |
|---|---|
| `FuncID` | 関数の一意な ID（完全修飾名）。例 `example.com/m/pkg.(*T).Method` |
| `TypeID` | 型の同一性を判定できる ID。構造は持たない |
| `FieldID` | 型 + フィールド名。フィールド感度のための名前（IR では型ではなく、`field_load` などの `field` の文字列） |
| `Pos` | ファイル、オフセット、長さ。読む人のために行・桁も持つ |
| `LocalID` | 関数内ローカルの番号 |
| `BlockID` | 関数内ブロックの番号 |

### 2.3 命令セット v0（26 種）

命令は次の 26 種（`internal/ir/ir.go` の `Op` と `TermKind`）。

**値の生成・変換**

| 命令 | 内容 |
|---|---|
| `const` | リテラル定数。値と型を持つ |
| `param` | 関数引数の受け取り。位置番号を持つ（レシーバは 0 番） |
| `copy` | 単純代入 `dst = src`。`view` フラグを立てると「元と同じメモリを共有する部分参照」を表す（Go の `base[lo:hi]` など。添字の値は伝播しない） |
| `unop` | 単項演算 |
| `binop` | 二項演算。文字列結合もここ |
| `convert` | 型変換（数値変換、型アサーション成功時の値など） |
| `type_assert` | 型アサーション。成否の分岐は `branch` で表す |

**メモリ**

| 命令 | 内容 |
|---|---|
| `alloc` | ヒープ / スタックオブジェクトの確保。型を持つ |
| `load` | ポインタの参照外し `dst = *p` |
| `store` | `*p = v` |
| `addr_of` | ローカルやフィールドのアドレス取得 |
| `field_load` | `dst = base.f` |
| `field_store` | `base.f = v` |
| `index_load` | `dst = base[i]`（配列・スライス・map・コレクション） |
| `index_store` | `base[i] = v` |
| `global_load` | パッケージ変数・static の読み出し |
| `global_store` | パッケージ変数・static への書き込み |

**関数**

| 命令 | 内容 |
|---|---|
| `make_closure` | クロージャ生成。呼び出し先 `FuncID` とキャプチャした値のリスト |
| `call` | 関数呼び出し（2.4 参照） |

**例外・不明**

| 命令 | 内容 |
|---|---|
| `exception_value` | 例外ハンドラブロックの先頭で、キャプチャした値を受ける |
| `unsupported` | extractor が脱糖できなかった構文。理由の文字列を持ち、不確実性 `unsupported_construct` の起点になる（4.4）。値を作る構文は、元の値を引数、結果を `dsts` に持てる。コアは、引数の位置から結果を作り、note を流れの途中に付ける（ADR-091） |

**終端命令（各ブロックに 1 つ）**

| 命令 | 内容 |
|---|---|
| `jump` | 無条件分岐 |
| `branch` | 条件分岐。`cond=<ローカル> true=<ブロック> false=<ブロック>` と、条件式のソース範囲 `Pos` |
| `return` | 戻り（複数戻り値は値のリスト） |
| `throw` | 例外送出 |
| `unreachable` | 到達不能（`panic` 後など） |

合計: 値 7 ＋ メモリ 10 ＋ 関数 2 ＋ 例外・不明 2 ＋ 終端 5 ＝ 26。

**`slice` を独立命令にしなかった理由:** 構文として存在するのは主に Go で、Rust は範囲インデックス（MIR では `Index::index` の呼び出し）、
Dart の `sublist` / `substring` はただのメソッド呼び出しになる。汚染の意味は `copy` と同じ（`base` の汚染を引き継ぐだけ）なので、`copy` の `view` フラグで表す。
スライスは元の配列とメモリを共有するため、`s2 := s[1:]; s2[0] = tainted` のように**ビュー経由の書き込みが元に及ぶ**ケースは、
エイリアス解析を作らない方針のため追えない。結論に影響した場合は `container_element_imprecision` として記録する。

**`global_load` / `global_store` を独立命令にした理由:** 関数サマリの境界には引数・戻り値に加えてグローバル変数が含まれ、
`global_store` が独立していれば「この関数は `G` に書く」が構文的に分かる。
terminal kind `global_var` の「書き込み箇所は列挙済み」も、全関数から `global_store` を拾うだけで実現できる。

`switch` と `match` は extractor が `branch` の連鎖に落とす。パターンマッチは `type_assert` ＋ `branch` に展開する。

### 2.4 `call`

```
call dst? callee args flags
  callee: static(FuncID)               呼び先が静的に確定
        | dynamic(candidates []FuncID, receiver) 仮想呼び出し。extractor が分かる範囲の候補を添える
        | value(local)                 関数値の呼び出し（呼び先不明）
  flags:  go | defer | await | none    実行順序・非同期の性質
```

- `go` は実行順序の保証がないことを表す。`defer` は呼び出しを出口側へ移す。`await` は後続ブロックへのエッジ
- 呼び出し点には**例外エッジ**を付けられる（ハンドラブロックへ）

### 2.5 エッジの種別

`normal` / `true` / `false` / `exception` / `defer` / `back`（ループの後方エッジ。必須ではなく補助情報）。
`finally` が複数経路から到達されるときは、extractor が経路ごとにブロックを複製する（Dart スパイクで決定。ADR-037）。
複製されたコードは同じソース位置を持つので、サイトの列挙は 1 つとして数える。
保護された範囲（`try`）の中では、投げうる呼び出しはブロックの終端で、例外のエッジと正常な継続のエッジを 1 本ずつ持つ。

### 2.6 ストリーム形式

JSON Lines。**先頭行は必ず `{"schema_version": "rivus.ir/0"}`**。以降は 1 行 1 レコードで、`kind` が種類を示す。種類は `func`（関数）、`implementations`、`global`、`type`、`type_alias`、`text`、`call_text`、`func_text`、`load_error`、`tests_skipped`、`caller_marks`、`body_not_read`（`internal/ir/stream.go`）。
`rivus debug extract` で、Go の IR を単独で出せる。`-text` で人間向けの形、`-func` で関数を絞り、`-all` で依存も出す。`-implementations`、`-texts`、`-check` もある（`cmd/rivus/extract.go`）。
`-lang rust` は、Rust の extractor が出すストリームをそのまま出す（macOS では既定で OS のサンドボックスの下で動かす。15 節）。`-lang` は `go` か `rust` で、Dart の extractor は `rivus debug extract` からは呼べない（`rivus-dart` を直接動かす）。
人間が読んで検証できることを一級の要件とする。

### 2.7 IR の具体化（エンジンの要求と、extractor の契約）

2.1〜2.6 の方針を、実装が決めた形に具体化したもの（ADR-039、ADR-040）。ここに書くのは今の契約。
Phase 2 の実施記録、実装メモ、移行の戦略、実測は、[history/ARCHITECTURE-plans.md](history/ARCHITECTURE-plans.md) に移した。

#### 2.7.1 エンジンが IR に要求するもの

「今の使い方」の列は、IR を切り出す前（go/ssa を直接使っていた Phase 1）のエンジンでの使い方。要求の由来を示すために残す。今のエンジンは `sg` の値の上で動く。

| 要求 | 今の使い方 | IR での持たせ方 |
|---|---|---|
| 値の同一性 | セルの根 `Root{ssa.Value, Idx}` | コアが SSA 化した値（`ValueID`）。`ssabuild` の結果から作る |
| 呼び出し | `CalleeSymbol`、`CallArgs`、静的 / インタフェース / 関数値の別 | `call`（2.4）。`callee` はシンボル文字列を持つ。複数の戻り値のため `dsts` はリスト（下記の変更点） |
| ポインタ様の値の位置 | ポインタ型なら位置は指す先 | `alloc` / `load` / `store` / `addr_of` と、**値の型の分類**（2.7.3） |
| フィールド・添字経路 | `FieldAddr` / `IndexAddr` / `Field` / `Index`、k=3 の切り詰め | `field_load/store`、`index_load/store`。アドレスが値として渡る場合は `addr_of`（base と経路を持つ） |
| グローバル | `ssa.Global` | `global_load` / `global_store`（2.3） |
| クロージャ | `MakeClosure`、`FreeVar` | `make_closure`（呼び出し先とキャプチャ値）。呼び出し先の関数はキャプチャ値を `param` の続きとして受ける |
| ガード | `If` の条件と、`BinOp` の木 | `branch` の `cond` ＋ `CondPos`。条件の値は SSA 上の `binop` / `unop` / `call` の木としてたどれる |
| 定数 | `Const`、`IsConstant` | `const`（値と型） |
| スニペット・式テキスト | AST から切り出す（`text.go`） | 命令の `Pos`、呼び出しの引数ごとの `Pos`、`branch` の `CondPos`。**言語ごとの `ir.Texts` が位置から切り出す**（IR 自体はテキストを運ばない。2.3 で修正。下の実装メモ） |
| 指紋 | シンボル、関数、出現順、式テキスト、本体テキスト | すべて `Pos` とシンボルから作れる。コアに置く |
| 実装列挙 | `Implementations(symbol)`（`match: implementations`） | Go では `gossa` が答える。ストリームには、インタフェースメソッドごとの `implementations` レコードの形がある（2.7.7） |
| コールグラフ | `callgraph.BuildGo`（CHA → VTA） | **extractor 側**。結果は `call` の `dynamic(candidates)` として IR に埋める。コアは IR だけから SCC を作る。関数値の呼び出しの候補は、コアの `internal/valuecalls` が IR の上で足す（13 節） |
| `defer` / `recover` | `ssabuild` の `Roots`（recover 後のブロック） | `defer` フラグのエッジと、`roots` を持つ関数（2.5） |


#### 2.7.2 2.3 からの変更点

1. **`call` の戻り値はリスト**（`dsts []LocalID`）。Go の複数戻り値と `Extract` を、IR の側で「`call` の n 番目の戻り値」として持つ。タプル型の値は作らない
2. **`addr_of` は base と経路を持つ**（`&x.f[i]`）。経路は `field` と `index` の列で、`taint` のセルの経路表現と同じ
3. **`param` はクロージャのキャプチャ値も受ける。** 位置番号の後ろに、キャプチャ値の番号を並べる
4. **`call` / `branch` / `return` に、引数ごとのソース範囲**（`ArgPos`）を持たせる。テキストの切り出しとガードの対応づけに必要
5. 命令数は 26 のまま。上の 1〜4 はどれもフィールドの追加で、命令は増やさない


#### 2.7.2b 文字列とテキストの扱い

IR からテキストを落としても、「SQL 文に変数が直接展開されているか」のような判定は、値の流れで行える。

- `const` は、文字列リテラルの中身を省略せずに持つ。長さの上限を設ける場合は、超えたことを `truncated` として明示する（何も報告せずに切ることはしない）
- 文字列補間は、各言語の extractor が `binop`（結合）か、補間専用のシンボルの `call` に展開する。断片は `const`、式は値の参照として残す（Go の `+` と `fmt.Sprintf`、Dart の `'$x'`、Rust の `format!`）
- extractor が定数の畳み込みをしてもよいが、`const` の中身を捨ててはいけない
- テキストの切り出し（スニペット、式、本体）は、表示と指紋のためだけに使う。判定には使わない

#### 2.7.3 型の扱い

IR の型は `TypeID`（同一性だけ）に加えて、**エンジンが問う分類**を 1 つ持つ。構造は持たない。

`TypeClass`: `pointer`（指す先が位置になる）/ `interface`（動的ディスパッチ）/ `func` / `container`（スライス・map・チャネル・リスト。値が、別の場所にある要素を指す）/ `struct`（値として持つ集合体。構造体と配列）/ `bool` / `string` / `number` / `other`（2.4 で `scalar` を `bool` / `string` / `number` に分け、配列を `container` から `struct` に移した。エンジンが問うのは「指す型か」「真偽値か」と、ゼロ値の綴り）

Phase 1 のエンジンが型について実際に問うのは、この分類（と型の同一性）だけだった。`ParamLabel` などの表示用の名前は、extractor がシンボル文字列にして渡す。

#### 2.7.4 Go extractor の対応表（NaiveForm → IR）

| go/ssa | IR |
|---|---|
| `Alloc` / `Load` / `Store` | `alloc` / `load` / `store` |
| `FieldAddr`＋`Load/Store` | `field_load` / `field_store`（融合）。アドレスが逃げるときは `addr_of` |
| `IndexAddr` / `Lookup` / `MapUpdate` | `index_load` / `index_store` |
| `Field` / `Index` | `field_load` / `index_load` |
| `Slice` | `copy`（`view`） |
| `Convert` / `ChangeType` / `ChangeInterface` / `MakeInterface` / `SliceToArrayPointer` | `convert` |
| `TypeAssert` | `type_assert` ＋ `branch`（`comma-ok` の場合） |
| `BinOp` / `UnOp` | `binop` / `unop` |
| `Call`（静的 / インタフェース / 関数値） | `call`（`static` / `dynamic` / `value`）。`Builtin` はシンボル付きの静的な `call` |
| `Go` / `Defer` | `call` の `go` / `defer` フラグ |
| `MakeClosure` / `FreeVar` | `make_closure` / `param`（キャプチャ値） |
| `Range` / `Next` | イテレータ取得の `call` ＋ `next` の `call` ＋ 後方エッジ |
| `Select` / `Send` / `Recv` | `call`（シンボル `chan.send` など）。`concurrency` の不確実性は、コアがこのシンボルから出す |
| `Phi` | **出さない**（NaiveForm には出ない。φ はコアが作る） |
| `Panic` / `Return` / `If` / `Jump` | `throw`（または `unreachable`）/ `return` / `branch` / `jump` |
| `DebugRef` | 捨てる（位置は命令の `Pos` に持つ） |

NaiveForm のローカルは、`Alloc` の結果がそのままローカル変数になる。コアの `ssabuild` が、ローカルの代入 ＋ 使用から φ と到達定義を作る（Phase 1 で検証済み。ADR-036）。

**Go extractor の決めごと**（`internal/frontend/goir/`、`internal/ir/ir.go`）:
- `call` の `callee` は、静的な呼び出しで `func`（呼び出し先の関数 ID）と `symbol`（ルールのシンボル。無名関数では空）の両方を持つ。組み込み関数は、`builtin.<名前>` のシンボルの静的な `call`
- アドレス計算（`FieldAddr` / `IndexAddr`）は、使い道が 1 つの `load` / `store` だけなら畳む（`Folded`。位置は `AddrPos`）。アドレスが値として渡るときだけ `addr_of` になる。グローバルのアドレスは、引数のない `addr_of`（`global` つき）
- φ は、前任ブロックの末尾の `copy` に落とす（NaiveForm には φ がない。コアが `ssabuild` で作る）
- エンジンを動かす言語固有の操作は、シンボル付きの `call` で表す: `chan.send` / `chan.recv`、`iter.start` / `iter.next` / `iter.next_string`、`rt.rundefers`。`select` は、extractor が送信と受信の `call` に分ける。`index_load` / `index_store` の `Keyed` は、キーで引くこと（map、文字列）
- 何もキャプチャしない無名関数を変数に入れて呼ぶ場合は、`make_closure`（キャプチャなし）から呼び出し先を、コアの `sg` が静的に解く
- 型の別名（Go の `type A = B`）は、`type_alias` レコードで渡す（ADR-083）

2.7.4 の末尾にあった「で分かったこと（実装メモ）」（検証の結果と、その時点の実測）は、履歴・計画は [history/ARCHITECTURE-plans.md](history/ARCHITECTURE-plans.md) の 2.7.4 節（実装メモ）にある。

#### 2.7.5 構成

履歴・計画は [history/ARCHITECTURE-plans.md](history/ARCHITECTURE-plans.md) の 2.7.5 節にある。今の構成は 3 節。要点だけ: Go では同じプロセスの中で IR の型を直接渡し、JSON Lines は `rivus debug extract` の出力と、Dart / Rust の extractor（別プロセス）の入力に使う。直接渡す経路と JSON を経由する経路の結果が一致することは、`TestTheStreamGivesTheSameResultAsTheDirectIR`（`internal/taint/helper_test.go`）で確かめている。

#### 2.7.6 移行の戦略

履歴・計画は [history/ARCHITECTURE-plans.md](history/ARCHITECTURE-plans.md) の 2.7.6 節にある（旧経路と新経路を照合した移行。実施済み）。

#### 2.7.7 別プロセスの extractor の契約

Go の extractor は、コアと同じプロセスで、`ir.Program` と `ir.Texts` を直接渡す。Dart と Rust の extractor は別プロセス（Rust は macOS では既定でサンドボックスの下。15 節）なので、コアが Go の中で頼っていたものを、ストリームに載せて渡す。

| コアが必要なもの | Go（同じプロセス） | 別プロセスの extractor |
|---|---|---|
| 関数と命令 | `ir.Program` | JSON Lines（2.6）。`rivus debug extract` と同じ形式 |
| コールグラフ | `call` の `candidates` | 同じ。extractor が、その言語の方法で解き、`candidates` に入れる |
| 実装列挙（`match: implementations`） | `rules.Expand` が `gossa` を引く（`internal/frontend/gossa/implementations.go`） | ストリームに `implementations` レコードの形はあるが、今、Dart / Rust の extractor は出さず、コアも展開には使わない（`cmd/rivus/common.go` の `rules.Expand` は `gossa` だけ）。Dart / Rust の継承の問い合わせは `type` レコード（`extends:`、`marker:`）で行う |
| テキスト（スニペット、式、本体、条件、引数） | `ir.Texts` | **ストリームに載せる**（下の「テキスト」） |
| 型の分類 | `Local.Class` | 同じ |
| 読み込みの失敗 | `LoadError` | `load_error` レコード（パッケージ名とメッセージ） |

**起動:** コアは extractor を `<command> [--no-deps] [--include-tests] [--allow-deps-under DIR]... <絶対パス>` で起動し、標準出力の JSON Lines を読む（`cmd/rivus/lang.go` の `extractRaw`）。`--allow-deps-under` は Dart だけ。`--include-tests` は、`-tests` のとき。
コマンドは、フラグ（`-dart-extractor`、`-rust-extractor`）、環境変数（`RIVUS_DART_EXTRACTOR`、`RIVUS_RUST_EXTRACTOR`）、既定の名前（`rivus-dart`、`rivus-rust`）の順で決める。見つからなければ、入れ方を添えて止める（`extractorCommand`）。Dart は、`rivus setup dart` がコンパイルした実行ファイルがあり、鍵（extractor のソースの hash、Dart SDK のバージョン、OS / アーキテクチャ）が今と合っていれば、フラグも環境変数も指していないときに、PATH の `rivus-dart` より先にそれを使う（合わなければ使わず、そう言う）。差し替えのためのインタフェースは置いていない。

**テキスト:** 別プロセスでは、`Texts` を呼び返せない。そこで extractor は、**必要なテキストを、あらかじめストリームに載せる**。コアが必要なのは、(a) 命令の位置ごとのスニペット（検出の経路の 1 歩は、どの命令でもありうる）、(b) サイトの位置の、正規化した式、(c) 分岐の後の命令の位置ごとの、条件のテキスト、(d) 呼び出しごとの、全体と引数のテキスト、(e) 関数ごとの、正規化した本体と、表示用のソース（80 行まで）、(f) グローバル変数の宣言位置のスニペット。
- 形式: `text`（種類、ファイル、テキスト、**その位置のオフセットの一覧**）、`call_text`、`func_text`、`load_error` のレコード。同じ文に属する位置は多いので、テキストを 1 回だけ書き、オフセットを並べる
- コア側は `ir.StreamTexts`（`internal/ir/streamtexts.go`）が `ir.Texts` を満たす。**Go の extractor も `goir.AddTexts` で同じレコードを出せ、ストリームを通した結果（テキストも含む）が、直接渡した結果と一致する**ことをテストしている（`TestTheStreamGivesTheSameResultAsTheDirectIR`）。他言語の extractor は、`AddTexts` が何を出すかを、契約の見本にする
- テキストは、`rivus sites` が必要な分（サイトの位置）と、`analyze` が必要な分（ガードの条件、呼び出しの引数）を、**最初から全部**出す。コアと extractor の間に、問い合わせの対話はない
- Go の `rivus debug extract` は、`-texts` を付けたときだけテキストを出す。`-implementations <シンボル>` は、そのインタフェースメソッドの `implementations` レコードを出す

**モジュール境界:** 複数パッケージ（Dart の melos、Rust の workspace）を 1 回の実行で出し、`Func.Dep` で、解析の対象と依存を分ける。依存は、ソースがあるものだけを出す。
**バージョン:** ストリームの先頭の `schema_version`（今は `rivus.ir/0`）が違えば、読まずに止める（`internal/ir/stream.go`）。Dart の extractor は、自分のバージョンを `--version` で返す（`rivus_dart rivus.ir/0`）。コアは、グラフの鍵に、extractor の実行ファイルのハッシュと、Dart ではこのバージョンを入れる（`cmd/rivus/graphkey.go`）。
**失敗:** 読めなかったもの（解決できないパッケージ、生成コードがない）は、`load_error` として出す。コアは `missing_source` の不確実性にする。
**読まなかったもの:** ビルドが出荷しないもの（テスト、example、開発用の依存）を読まなかったときは、`tests_skipped` レコードで示す（ADR-062）。呼び出し元が全部読まれているかは、`caller_marks` と `Func.Private` / `Func.ExternalEntry` で示す（13 節）。

2.7.7 のレビュー待ちだった項目と、問い合わせの案、実測は、履歴・計画は [history/ARCHITECTURE-plans.md](history/ARCHITECTURE-plans.md) の 2.7.7 節にある。

---

## 3. コアエンジンのパッケージ構成

Go モジュール `github.com/yagihash/rivus`。依存は標準ライブラリ ＋ `golang.org/x/tools` ＋ YAML・TOML のパーサ ＋ `golang.org/x/sys` ＋ SQLite（純 Go の `modernc.org/sqlite`）（`go.mod`）。

```
cmd/rivus/                CLI（sites / analyze / query / index / graph / resolve / extract / models）
internal/
  ir/                     IR の型定義、JSON Lines の読み書き、検証、ダンプ、ir.Texts
  irssa/                  IR の SSA 形（ssabuild をつなぐ。昇格、φ、到達定義）
  sg/                     SSA グラフ（値・命令・ブロック・支配木）。エンジンが走る面
  ssabuild/               CFG → SSA（dominator tree、dominance frontier、φ 挿入）。言語非依存
  ssacheck/               ssabuild の検証（go/ssa の lifting、独立な到達定義との比較）。テストのみ
  frontend/gossa/         go/packages → go/ssa。go/ssa に触れるコードはここと goir だけ
  frontend/goir/          Go extractor（NaiveForm → IR）と、Go の ir.Texts
  callgraph/              Go 用: CHA → VTA（Go の extractor が使う）
  valuecalls/             関数値の呼び出しの候補を、IR の上で決める（13 節）
  model/                  YAML ルールの読み込み、シンボル照合（exact / implementations）、実装の展開
    builtin/              組み込みルール（YAML。ユーザー定義と同形式）
  taint/                  エンジン（手続き内、サマリ、ガード、由来追跡、不確実性）
  sites/                  サイト（source / sink の候補箇所）の列挙と ID 付け。L0 / L1
  fingerprint/            指紋の鍵とハッシュ
  report/                 結果のグループ化、クエリ、resolve、モデルカバレッジ、JSON 出力
  graphdb/                コールグラフを SQLite に保存して読む（12 節）
  graphq/                 保存したグラフへの問い合わせ（callers / callees / path / impls / dispatchers）
  vcs/                    解析したコードのリビジョン
  testutil/               testdata ハーネス
extractor/                Dart と Rust の extractor（dart/、rust/。それぞれ別言語のプロジェクト）
testdata/                 マイクロベンチマーク（go/ ほか）
bench/                    ベンチ（zoo、curated、wild など。8 節）
docs/
```

### 依存の向き

```
cmd → report → taint → sg → irssa → ssabuild
                 ↘ sites ↗     ↘ ir
        model ──→ (taint, sites から参照)
        valuecalls → (sg、sites から呼ばれる。ir、irssa を読む)
cmd → frontend/goir → frontend/gossa (go/ssa), callgraph → ir
cmd → graphdb ← graphq（graphdb は sg のグラフから書く）
```

**`go/ssa` や `go/callgraph` を import してよいのは `frontend/gossa`、`frontend/goir`、`callgraph` と、そのテストのみ。**
コア（`ir`、`irssa`、`sg`、`ssabuild`、`sites`、`taint`、`model`、`report`、`valuecalls`、`graphdb`、`graphq`）は IR だけを見る。
Rust と Dart の extractor は、同じ `ir` を出せば、コアを変えずにつなげられる。

### 差し替えを前提にする境界

1. **Extractor の在り処** — インタフェースではなく、`cmd/rivus/lang.go` の `extractorCommand` が、フラグ、環境変数、既定の名前の順で決める（2.7.7）。`go:embed` で埋め込む形は取っていない
2. **コールグラフ** — extractor が `call` の `candidates` として IR に入れる。Go は CHA → VTA（`internal/callgraph`）、Dart と Rust は extractor が自分の方法で解く。関数値の呼び出しは、コアの `valuecalls` が IR の上で候補を足す。コアに Go 固有の手法を漏らさない（`call` を解く部分は IR だけを見る）
3. **エントリポイント解決** — extractor が `Func.Entry` を付ける ＋ ルールの `kind: param` の source（6 節）

### Go のコールグラフに関する実装メモ

`vta.CallGraph(funcs map[*ssa.Function]bool, initial *callgraph.Graph)` を実物で確認済み（`golang.org/x/tools` v0.52.0）。

- `initial` に CHA（または RTA）の結果を渡して絞り込むのが標準的な使い方。`initial == nil` なら内部で CHA を使う
- 渡す SSA 関数は **`ssa.InstantiateGenerics` 付き**で構築しておく必要がある
- 結果にはルートノードがない
- 結果は reflection と unsafe を除いて健全。これらは `reflection` / `ffi_boundary` として記録する側の責務
- 結果は reflection と unsafe を除いて健全。これらは `reflection` / `ffi_boundary` として記録する側の責務

---

## 4. 主要なデータ構造

実際の型は、各節に書いたファイルにある。この節は、型の役割と、出力に出る名前を説明する。擬似コードでなく、コードが正本。

### 4.1 IR

型は `internal/ir/ir.go`。主な形:

- `Program`: `Funcs`、`Impls`、`Types`（上位の型）、`Globals`、`TypeAliases`、テキスト（`Texts`、`CallTexts`、`FuncTexts`）、`LoadErrors`、`CallerMarks`、`TestsSkipped`
- `Func`: `ID`、`Params`（`Class`、`Captured`）、`Results`、`Locals`、`Blocks`（先頭が entry）、`Roots`、`Pos`、`File`。マークとして `Exported`、`Entry`、`Init`、`Symbol`、`Recv`、`Dep`、`Private`、`ExternalEntry`、`Overrides`
- `Block`: `Instrs`、`Term`（`Kind`、`Cond`、`CondPos`、`Values`）、`Succs`（種別つきのエッジ）
- `Instr`: `Op`、`Dsts`（呼び出しは戻り値の数だけ）、`Args`、`Const`、`Operator`、`Field`、`Path`、`Global`、`Func`、`View`、`Folded`、`Keyed`、`Callee`、`Go` / `Defer` / `Await`、`Reason`、`Pos`、`AddrPos`、`ArgPos`、`ArgNames`（名前つき引数の名前。`Args` と並ぶ。Dart だけが付ける。足しただけで、`schema_version` は `rivus.ir/0` のまま。読む側は、ないものとして扱う。`named:` の sink が使う）
- `Callee`: `Kind`（`static` / `dynamic` / `value`）、`Func`、`Symbol`、`Recv`、`Candidates`、`FFI`。ストリームには載らず、コアが埋めるもの: `ValueCands`、`ValueLimit` など（13 節）
- `Pos` は `File`、`Offset`、`Length`、`Line`、`Col`

指紋（4.6）に使う本体のハッシュは、IR の型ではなく、`ir.Texts` の出力から作る（2.7.7）。

### 4.2 汚染

型は `internal/taint/elem.go`、`state.go`。

- 汚染の単位は `Elem`（小さく、比較できる）。`Real`（source のサイトから来た具体的なラベル。ラベル名と、元の source サイトの ID を持つ）と、`Param`（「呼び出し側が引数 i に渡すもの、のこの経路の下」という記号的な汚染。サマリになる）の 2 種
- 汚染の状態は、**セル**（`Root` ＋ アクセスパス）から `Elem` の集合への写像。`Root` は、SSA の値、複数の戻り値のうちの 1 つ、または解析中の関数の戻り値
- アクセスパスの深さは k（既定 3。`-k`）で打ち切る。打ち切りは記録する
- コンテナ要素は区別しない

### 4.3 関数サマリ

型は `internal/taint/summary.go`、`guards.go`。

- `Summary`: 関数 ID、`Outputs`、`Sinks`、`Opaque`（読まれなかった関数が触ったかもしれない引数）
- `Output`: 汚染がどこに出るか。種類は戻り値、引数が指す先、グローバル変数、例外（`OutResult`、`OutParam`、`OutGlobal`、`OutThrow`）。中身は `OutElem` の並びで、各要素に経路（`Steps`）と、その要素が届く条件（`Guards`）が付く
- `SinkReach`: 引数が、関数の中（またはその呼び出し先）の sink に届くこと
- `Guard`: 条件が依存する引数（`Params`）、条件式のソース（`Cond`）、通った側（`Polarity`）、呼び出し側から見たときの呼び出し点（`Call`）

**ガードの制約:** 記録するのは引数由来の条件のみ。「その条件値が引数からのデータフローで到達可能か」で判定し、
この判定には taint ソルバを流用する（source = 引数、sink = 分岐条件）。条件式は提示のために切り出すだけで、評価して判定することはしない。
1 フローあたり上限 4 個（`maxGuards`。`internal/taint/guards.go`）。超えたらガードなしの過大近似（全経路の和集合）に畳み、
`attributable_to: rivus` の不確実性（`depth_limit`、detail が `guard_limit: ...`）として記録する。

### 4.4 不確実性

型は `internal/taint/result.go`（`Uncertainty`）。`kind` の定数と `attributable_to` の割り当て（`attributable()`）も、そこにある。
`Uncertainty` の主なフィールド: `kind`、`attributable_to`、`detail`、`symbol`、`candidates`（`dynamic_dispatch` の候補）、`narrowed_from` と `basis`（13 節）、`func`、`position`、`snippet`。

`kind` は、`rivus.result/1` のまま足されてきた（ADR-008 の「閉じた集合。追加はバージョンを上げる」は運用で逆。ADR-043 の追記、[ADR-008](adr/008-uncertainty-closed-kinds.md) のヘッダを参照）。この表が、コードの定数と合う一覧。

#### kind ごとの `attributable_to`

| kind | attributable_to | 備考 |
|---|---|---|
| `config_read` | target | |
| `exported_api_param` | target | ライブラリの公開関数の引数が、外から来る値 |
| `global_var` | target | 書き込み箇所は列挙する |
| `reflection` | target | |
| `ffi_boundary` | target | |
| `stored_data_boundary` | target | データストアに出し入れする呼び出し。その先は追わない |
| `missing_source` | target | ワークフロー前提（`go generate` / `build_runner` 未実行）に由来 |
| `dynamic_dispatch` | 候補 1 件以上: mixed／候補 0 件: rivus | 言語機能として呼び出し先が決まらないが、候補の絞り込み精度（VTA など）は rivus の改善余地がある。候補が 0 件なのは、実装が読み込んだコードの外にあるときで、モデルを書けば減る（rivus）。detail が `expansion_limit` のものは、`match: implementations` の展開を上限で止めた記録（5.4） |
| `assumed_call` | mixed | 呼ばれるところが見えないクロージャ（渡す、保存する、返す）を、作った場所で、呼ばれるものとして適用した（キャプチャした値を、クロージャの要約に適用する。ADR-059）。付くのは、要約がキャプチャした値に依る効果を持つクロージャだけ。throw と結果は適用せず、そのとき detail に書く。1 度も呼ばれないクロージャの流れも、finding になりうる（過検出） |
| `unresolved_callee` | mixed | 関数値は実行時に決まるが、生成箇所を追う型フロー解析で候補を絞れる |
| `concurrency` | mixed | 実行順序に依存するが、channel を単純なキューとしてモデル化すれば追える範囲が広がる |
| `unmodeled_library_call` | rivus | モデルを書けば減る |
| `dependency_body_unread` | rivus | モデルのあるインターフェースのメソッドの動的な呼び出しで、候補に本体のある依存の関数（プロジェクトの外）があったが、読まなかった（ADR-075）。依存が値を包んで渡した先の、自前の実装の中の sink は、たどっていない。`dynamic_dispatch` と並べて付き、候補に、読まなかった依存の関数を並べる |
| `project_body_unread` | rivus | プロジェクトの関数の呼び出しで、extractor がその関数の本体を読めなかった（Rust: `rustc_public` が async closure を変換できず、関数を飛ばした。`fn_skipped`、`summary.functions_skipped`。ADR-109）。関数の中で値がどうなるかは、たどっていない。依存の関数は、`dependency_body_unread`（`detail` に理由）。モデルの行があれば、行が使われ、注記は付かない |
| `model_approximation` | rivus | 流れのモデルが、自分で「近似」と宣言している（`approximate` の文）。モデルの限界（ADR-067） |
| `numeric_stop` | rivus | source のデータが数値や bool になり、それ以降は追わない（既定。`-numeric-taint` で追う）。source の種類（シンボルとラベル）ごとに 1 件で、最初の場所と、起きた関数の一覧（10 件まで）を持つ。サマリを通って止まる分（呼び出し先の中の、引数の記号的なデータの止まり）は、記録されない |
| `unsupported_construct` | rivus | extractor が下げられなかった構文（IR の `unsupported`）。その場所で何が起きるかは、解析に入っていない。`missing_source` ではない（対象のコードが足りないのではなく、rivus が読めなかった） |
| `container_element_imprecision` | rivus | 結論に影響した場合のみ記録（ノイズ防止） |
| `depth_limit` | rivus | 打ち切り。detail が理由を示す（`guard_limit`、`candidate_limit`、`caller_depth_limit`、`func_timeout` など）。例外: detail が `path_steps_omitted:` のものは、**解析の打ち切りではなく、表示の省略**（流れは見た。200 段を超える経路の、真ん中の段を見せていない。省いたステップの不確実性・モデル・到達性は finding に入っている）。ただし、source までさかのぼる途中の打ち切り（`the path was followed back for ...`）は、その先の段を見ていない |
| `access_path_truncated` | rivus | **finding の経路**にある関数の中で、アクセスパスを深さの上限(`-k`、依存の関数は `-dep-k`)で切った。切った所より下は「まるごと汚染」として扱うので、この流れは、切ったために出ているだけかもしれない(多めに出る側。見逃しにはならない)。`candidates` は、切った関数(10 件まで)。`summary.access_path_depth`(`{project, dependency}`)は、`-dep-k` でプロジェクトと依存の深さが違うときだけ出る。プログラム全体の記録は、`depth_limit` の `path_limit:`(関数の一覧つき) |
| `recursion_approximation` | rivus | |
| `field_imprecision` | rivus | フィールドを指定する sink に、そのフィールドにあるとは言えない汚染が届いた（14 節）。detail は `whole_value`、`path_limit`、`element`、`opaque_callee` |
| `field_unresolved` | rivus | sink が指定したフィールドが、プログラムにない（綴りの誤りなど）。sink を全体として扱った（14 節） |
| `named_unresolved` | rivus | sink が `named:` で指定した引数の名前が、プログラムのその関数の呼び出しのどれにもない（綴りの誤りなど）。sink を呼び出し全体として扱った（`docs/MODELING.md` 4a-1 節） |
| `global_holds_nothing` | rivus | 読み戻す人がいないとルールが宣言する変数（ADR-056）に、汚染を書いた関数を、変数ごとに 1 件記録する（`internal/taint/heldnothing.go`） |

`mixed` は「言語機能由来で、かつ rivus の精度向上でも減らせる」もの。
集計では `target` / `rivus` / `mixed` を**別々に**出し、`mixed` を `rivus` に含めない（投資で減らせる量を過大に見積もらないため）。
`mixed` の割り当てが暫定だった時期の見直し予定は、[history/ARCHITECTURE-plans.md](history/ARCHITECTURE-plans.md) の 4.4 の移動分にある。

`literal` は kind ではない。値が定数だと分かったことは、由来追跡（5.6）の確度（`certainty`。`internal/taint/provenance.go`）で示す。

### 4.5 出力

型は `internal/taint/result.go`（`Finding`、`Path`、`Step`、`ModelRef`、`Suppression`）と `internal/report/report.go`（`Report`）。結果のバージョンは `rivus.result/1`。

`Finding` の主なフィールド（JSON の名前）:

- `id`、`fingerprint`: (source サイト, sink サイト) の組から生成（4.6）
- `source`、`sink`（`SiteRef`）、`category`、`categories`、`labels`。`categories` は、同じ位置を読む別カテゴリの行がある sink の、全部のカテゴリ（先頭が `category`。2 つ以上のときだけ出る。項目を足しただけなので `schema_version` は変えない）
- `paths`: 経路。今は**常に 1 本**（`internal/taint/engine.go`。7 節）。各ステップは `steps`（`func`、`position`、`snippet`、`desc`、`notes`、`model`）
- `other_sources`、`other_sources_count`: 同じ sink に届く、同じ種類の別の source。経路は繰り返さない
- `uncertainties`: 経路上で解決できなかったこと
- `path_steps`: 表示する経路のステップ数（順位付けのための事実）
- `reachability`（`from_entry` / `from_framework_callback` / `not_from_entry`）と `reachability_note`: 経路上の関数が、プログラムの起点から届くか
- `dispatch_widened`、`dispatch_candidates`: インタフェースや関数値の呼び出しが、複数の実装すべてに届くとして進めた数（順位付けのための事実）
- `confidence`: `no_uncertainty_on_path`（経路に未解決がない）か `requires_review`。判定ではなく、確定度の分類
- `models`、`llm_influenced`: 経路を決めたモデルと、LLM が決めたものが影響したか
- `conditional_nodes`: 結果が呼び出し側の値に依る所（5.6）
- `shared_state`: パッケージ変数を通る流れの根拠（書き込み側の数など）。出力だけで、結果を変えない

`Report` は、`findings` のほか、`sites`、`uncertainties`（全体）、`suppressions`、`load_errors`、`expansions`、`project_rules`、`stats`、`summary` を持つ（`internal/report/report.go`）。出力の読み方は [README.md](../README.md)。

### 4.6 サイトと指紋（fingerprint）

サイトは「関数の中の、ルールにマッチした 1 箇所」。検出結果は (source サイト, sink サイト) の組。型は `internal/sites/sites.go` の `Site`（`id`、`role`、`kind`、`func`、`symbol`、`category`、`categories`、`labels`、`position`、`snippet`、`literal_only`、`fingerprint`）。
`id` は公開用で、`"s1:"` ＋ `structural@1` の値から作る。`literal_only` は L1 のマーク（sink の引数がリテラル定数だけ）。

指紋は**入れ物 1 つの中に、複数の種類を並べる**。種類は役割で名付け、バージョンは `@N` で表す（`internal/fingerprint`）。

| キー | 役割 | 材料 |
|---|---|---|
| `structural@1` | 同一性 | 関数 ID ＋ ルールのシンボル ＋ 同じ関数内での出現順（序数） |
| `content@1` | 同一性 | 関数 ID ＋ ルールのシンボル ＋ 呼び出し式の正規化テキストのハッシュ（序数は同一テキストの中だけ） |
| `expr@1` | 変化検知 | 呼び出し式の正規化テキストのハッシュ |
| `body@1` | 変化検知 | 関数本体のハッシュ |
| `path@1`（検出結果のみ） | 変化検知 | 経路の呼び出しの連鎖のハッシュ。経路が変わったかを検知する |
| `sim@1`（将来。未実装） | 類似度 | 関数本体の正規化トークン列の simhash |

- **同一性のキーは、2 つのバージョン（旧と新）のサイトを対応付けるために使う。** 優先順位順に適用する。内容キー → 構造キー → （将来）類似度の候補提示。
  内容キーを先に適用するのは、同じシンボルの呼び出しを前に挿入したとき、構造キー（出現順）だけだと誤った相手に結び付くため
- **変化検知のキーは、対応付けた後で「中身が変わったか」を判定する。** 注釈は (ID, 変化検知キー) の組に紐づけ、変化検知キーが変わったら再確認が必要として扱う
- 指紋のキーの種類を増やしても、マッチャーを書き換えずに済む。マッチャーは、各キーの役割と優先順位だけを見る
- 検出結果の指紋は、source サイトと sink サイトの**同じキー名ごとに合成**する:
  `Finding.Fingerprint[k] = hash(source.Fingerprint[k], sink.Fingerprint[k])`。`Finding.ID` は `structural@1` の合成値から作る
- ID の材料にコールグラフの位置は使わない。`rivus sites`（L0）がコールグラフなしで即時に返せなくなり、rivus の改善（VTA の精度向上など）で ID が揺れるため
- バージョンをまたぐ同一性は、ID だけで保証するものではなく、対応付けで成立する。ブックマークによる対応付けが `rivus resolve`（4.7）。旧新の結果どうしの比較（`rivus diff`）と、類似度の指紋（`sim@1`）は、作っていない

### 4.7 ブックマークと `rivus resolve`

エージェントは、前回の探索で見た箇所を**ブックマーク**として保存し、あとで再び見る。
コードが変わると ID が引けなくなることがあるので、**保存した指紋を今のコードに適用して、対応する箇所を答える**コマンドを持つ。

- **ブックマーク**は、エージェント側が保存する。rivus が出力したサイトや検出結果の JSON をそのまま使える（ADR-033）。中身は `id`、指紋（全部）、見たときのスニペット。検出結果をブックマークする場合は、source / sink の両サイトの情報と、検出結果の指紋（`path@1` を含む）。
  **rivus はステートレスのまま**で、保存場所を持たない
- `rivus resolve` は複数のブックマークをまとめて受け取り（ソースのロードは 1 回）、ブックマークごとに状態を返す。前回の解析結果（ベースライン）は不要

型は `internal/report/resolve.go`（`Resolution`、`SiteResult`、`Flow`、`ResolveResult`）。`SiteResult` の主なフィールド: `status`、`current`（見つかった今のサイト。新しい ID を含む）、`match_basis`（どのキーで一致したか）、`changed_keys`、`previous_snippet`、`candidates`、`reason`、`evidence`。`Resolution` は、そのほかに `kind`（`site` / `finding`）、`from_revision`、`revision_change`、検出結果のときの `source` / `sink` / `flow` を持つ。

| 状態 | 意味 | エージェントの動き |
|---|---|---|
| `unchanged` | ID が見つかり、変化検知のキー（`expr`、`body`）も同じ | そのまま再利用できる |
| `changed` | ID は見つかったが、`expr` か `body` が違う（どちらかも返す） | 注釈は無効。再確認する |
| `rematched` | ID は見つからなかったが、別のキーで対応するサイトが見つかった。新しい ID と一致の根拠を返す | 新しい ID に付け替えて記録する |
| `ambiguous` | 対応する候補が複数ある。候補の一覧を返す | 候補のスニペットを見て選ぶ |
| `unresolved` | 見つからない | 理由を見て対応する |

`unresolved` の理由は 2 つに分けて返す。
- `not_found`: 解析できているが、該当する箇所がない（コードが消えた、または大きく変わった）
- `not_analyzed`: そのパッケージが読み込めなかった（`missing_source` と同じ）。「消えた」とは別の事実

**付け替えの手順:** 内容キー（`content@1`）→ 構造キー（`structural@1`）→（将来）類似度（`sim@1`）の順に適用する。
構造キーだけで一致した場合は、同じ関数内で同じシンボルの呼び出しが前に挿入され、別のサイトがたまたま同じ出現順になった可能性が残る。
この場合は `rematched` とし、`MatchBasis = "structural@1"` と明記して、旧スニペットと新スニペットを並べて見せられるようにする。同じ関数内のサイト数が変わっていれば `Evidence` に添える。
確実とは断定できない一致を確実のように見せず、**一致の根拠を事実として示す**。

**検出結果のブックマーク（`flow`）:** 2 つのサイトの対応付けに加えて、**今も届くか**を返す。

| `Flow` | 意味 |
|---|---|
| `present` | 今も届く。経路が変わっていれば（`path@1` が違えば）、変わったことも示す |
| `absent_now` | 現在のコードとモデルの範囲では、届く経路が見つからない。**これは「安全」の判定ではない。** 見つけられなかった理由（不確実性）も一緒に返す |
| `not_evaluated` | サイトが見つからなかったので評価していない |

---

## 5. 解析の仕組み

### 5.1 手続き内

- SSA 化されたローカルの上で worklist により不動点を求める
- 合流（φ）は和集合（may 解析）
- ラベル付き: sanitizer は特定ラベルだけを `Drop` し、sink は特定ラベルにのみ反応する
- 明示的フローのみ追う。暗黙的フロー（制御依存）は追わない（ただし条件ガードの抽出では分岐条件を見る）

### 5.2 手続き間（bottom-up の関数サマリ）

1. コールグラフを構築し、SCC に縮約してトポロジカル順（呼び出し先から）に処理する
2. 各関数の手続き内 taint を解き、サマリを算出する
3. 呼び出し側でサマリを展開する。**SCC の内部だけ**不動点まで反復する。グローバル変数への書き込みなどでサマリが変わったときは、関数を解析し直す。プログラム全体のパスの上限は `-max-passes`（既定 12）。収束したかは `stats.converged` に出る

サマリの独立性（関数単位で閉じていること）を保つ。これが将来の増分解析の土台になる。増分解析自体は、今のスコープ外（ADR-021）。

### 5.3 ヒープの扱い

- **フィールド感度あり（フィールド名ベース）、アクセスパス深さ k=3**。k は設定で変更可能
- アロケーションサイトによるオブジェクト感度は入れない
- コンテナ（配列・map など）は要素を区別しない。`container_element_imprecision` は、**結論に影響した場合にだけ**記録する
- エイリアス解析は作らない。`b := a; b.Name = tainted` のような必要箇所は、列挙して出力に残す範囲に留める


### 5.4 モデル（YAML）と LLM の扱い

#### ルール形式

- 宣言的 YAML のみ。**クエリ言語は作らない**
- 名前空間とバージョンを持つ: `rivus/go/stdlib@1`、`myorg/internal@3`
- 組み込みルールもユーザー定義と同じ形式（ドッグフーディング）
- **シンボル指定** は完全修飾名。次の 2 種類を明示的に区別する

```yaml
# 具体的な関数だけにマッチ
- symbol: "(*database/sql.DB).Query"
  match: exact

# インタフェースのメソッドにマッチし、コールグラフ経由で実装に展開
- symbol: "(net/http.Handler).ServeHTTP"
  match: implementations
  max_implementations: 300   # 省略時は全体の既定値（300。`-max-implementations`）
```

#### `match: implementations` の展開

- 展開先は、コールグラフ（呼び出し先の候補）から求める
- **上限を超えたら、展開を中止する。** 先頭 N 個だけ展開する（切り捨てる）ことはしない。部分的に展開すると「見ている部分」と「見ていない部分」の境界が見えなくなり、
  「列挙は網羅的に」に反するため。中止したときは `exact` と同じ扱いに縮退し、総数を `dynamic_dispatch`（`attributable_to: mixed`、サブ理由 `expansion_limit`）として記録する
- 上限の既定値は 300（`internal/model/expand.go` の `DefaultMaxImplementations`。ADR-030 の追記）。ルールごとに `max_implementations` で上書きできる。暫定値だった頃の記録は [history/ARCHITECTURE-plans.md](history/ARCHITECTURE-plans.md) の 5.4 の移動分
- **sanitizer への `implementations` は既定で禁止。** source / sink / propagator の展開が広すぎるとノイズが増えるだけだが、
  sanitizer を広げ過ぎると、本当は sanitize していない実装にまで「通れば安全」が及び、汚染が消えて見逃しが生まれるため。
  許可するときはルール側で明示する（`allow_sanitizer_expansion: true`）

- シンボルは `go/types` の `FullName` 形式（`(*net/http.Request).FormValue`、`os/exec.Command`、`(io.Writer).Write`）
- source の種類は `call_result`（呼び出しの結果）、`param_type`（その型の引数すべて）、`field`（構造体フィールドの読み出し）、`callback_param`（呼び出しに渡された関数の引数。ADR-106）。
  sanitizer は `drops`（除去するラベル。`"*"` は全部）と `adds`（落とした値に付くラベル。`html_escaped` など）を持つ
- ジェネリクスは**型引数を消去して一致**させる
- 各モデルに `origin: builtin | human | llm`。`llm` の場合は `decided_at`、`rationale`、`model_version` が必須

#### モデルと判定の分離

| | モデル | 判定 |
|---|---|---|
| 対象 | 関数 1 つの振る舞い | 検出結果 1 件（経路全体） |
| 再利用 | する | **しない** |
| 保存先 | モデル設定ファイル | 検出結果の注釈のみ |

#### モデルのキャッシュキー

**その関数のボディのハッシュ ＋ 直接呼び出している関数の「モデル ID」の集合。**
推移的なボディハッシュにしない（無関係な変更で無効化されすぎるため）。
モデルの依存グラフは、SCC 縮約したコールグラフの部分グラフとして既存の構造から得る。

#### 決定論性の担保

- 出力に、モデルが決めた所を記録する（`models`、`llm_influenced`、`suppressions`）。LLM 由来モデルが結論に影響した検出結果は、その旨がトレースできる。記録は、`origin: llm` の sanitizer の抑制、`builtin` 以外の propagator の `pure` と結果に届かない `flows`（`suppressions`。#138）、`numeric_stop`、`global_holds_nothing`、`approximate`。`builtin` の `pure` と `flows`、`builtin` や人が書いた sanitizer の停止は残らない（確認済みで、全部残すと出力が膨らむため。PRINCIPLES.md 2 節）。`suppressions` に `origin: llm` の記録がある関数を通り、同じ source から出る finding は、そのモデルを `models` に入れて `llm_influenced` を付ける
- `--deterministic-only` は `origin: llm` のモデルをすべて無視して実行される

#### モデルカバレッジレポート

```
rivus models coverage ./...
```

モデル未定義の外部関数の呼び出しを頻度順に並べる。`unmodeled_library_call` を集計する（`internal/report/coverage.go`）。


### 5.5 依存パッケージ

- **依存の本体は、既定で読む。** 標準ライブラリは読まず、モデルで扱う。依存のソースを読むと、依存の中への呼び出しを推測でなく追えるので、メモリは増える。`-no-deps` で読まなくなり、そのとき依存への呼び出しは、すべてモデルか `unmodeled_library_call` になる（`cmd/rivus/common.go` のフラグの説明。メモリの数字は 16 節）
- Dart は、プロジェクトの外の依存を、頼まれたものだけ読む（`-allow-deps-under`。`package_config.json` は信用しない）。Rust の依存の取得は、macOS では既定でサンドボックスの下（15 節）
- **依存の関数の中では、source と sink を探さない**(`internal/sites/enumerate.go`。ADR-099)。依存の本体は、流れを追うために読む。結果、ライブラリの中で危険な処理に届く形は、プロジェクトがその API を呼ぶ所に sink のモデルがないと検出されない。依存の関数が 1 つでもあれば、`summary.dependency_sites` がそのことを毎回言う
- モデルのあるインタフェースのメソッドの動的な呼び出しで、候補に本体のある依存の関数があるとき、本体は読まず、`dependency_body_unread` で示す（ADR-075）。実装の多いインターフェースのメソッドで、実装どうしが同じメソッドに委譲する形（委譲のクリーク）は、そのメソッドの行を `flows` のモデルにして、実装全体の本体を読まない（ADR-098。照合した対象と、注記の条件は `docs/MODELING.md` 4a 節）
- **モデルが何を省くか**（`internal/taint/calls.go`、`internal/model/model.go` の `HasModel`）: シンボルに sanitizer か propagator の行があると、呼び出しの所ではモデルが使われ、その関数の本体の要約は使われない。省かれないものもある。(1) 本体の要約の計算（その関数は、IR にあるかぎり、ほかの関数と同じく解析される）。(2) source だけ、sink だけの行は `HasModel` にならないので、本体が使われる。(3) 同じパッケージで、モデルのない関数は、本体が読まれる。(4) パッケージ単位で本体を読まない宣言は、解析の側だけ実装済み（下の項目、ADR-100）。extractor に本体を出さない指定は、Go と Dart が実装済み（下の項目、ADR-105）、Rust は未実装（設計: `docs/DESIGN-package-models.md`）
- **確認したバージョンの記録**（`internal/model/verified.go`。DESIGN-package-models.md 6.7）: モデルのファイルのトップレベルの `verified_versions`（`package`、`versions`、`checked_with`、`checked`）。`Parse` が書式を検証し（範囲の形、`checked_with` が範囲の内、日付、重複、未知のキーは失敗）、`Merge` が `Rules.Verified` に集める。解析は読まない（記録だけ）。`rivus models versions` が JSON で出す。lock ファイルのバージョンとの照合は未実装。
- **カテゴリで finding を絞る**(`cmd/rivus/analyze.go` の `categoryFlags`、`internal/report/report.go` の `CategoryFilter`): `rivus analyze` の `-categories` / `-skip-categories`(同時には使えない。名前は、読み込んだルールの sink のカテゴリにあるものだけで、ほかはエラーに有効な名前を並べる)。taint 解析が終わって、報告を組み立てる前に finding の一覧だけを切る(解析、`graph`、`index`、`uncertainties`、`sites` は変わらない)ので、`summary.findings` と `on_findings` は出力に残った finding の数になる。外した finding は `summary.categories_excluded`(`category`、`findings`。カテゴリの順)に数を出し、何も外さないとき(既定)は項目が出ない。複数のカテゴリを持つ finding は、どれかが残る対象なら残し、全部が外す対象のときだけ外す。終了コードは変わらない(`analyze` は finding があっても 0 で終わる)。
- **確認していないバージョンへの適用の注記**(`internal/model/verified.go` の `UnverifiedVersions`、`internal/frontend/gossa/load.go` の `ModuleVersions`、`cmd/rivus/analyze.go`。DESIGN-package-models.md 6.7 の 3、6.10): 解析した Go の対象が使っている module のバージョン(`go list` の `Module.Version`。ディレクトリへの置き換えと、別の module への置き換えは、バージョンがないので出さない)が、その module の `verified_versions` の範囲(`/v*` の記録を含む)の外のとき、`summary.unverified_versions` に 1 件(`package`、`used`、`verified[]`、`source`)。モデルの行は当てたまま(止めない)。記録のない module と、記録を持つモデルが当たっていないときは、出さない。Dart（`pubspec.lock`）と Rust（`Cargo.lock`）も、同じ形で出す（6.10 節。課題 #162）。
- **パッケージ単位で本体を読まない宣言**（ADR-100。`internal/model/bodyreplaced.go`、`cmd/rivus/bodyreplaced.go`、`internal/taint/bodyreplaced.go`）: モデルのファイルの `body_replaced_packages`（`package` と `reason`）、または `-body-replaced` が、依存のパッケージを指定する。書いたパッケージだけに効く（暗黙の接頭辞一致はしない。配下も指すときは `.../...`）。単位は `taint.UnitOf` と同じ（Go: import パス、Dart: `package:<名前>`、Rust: crate 名）。`analyze` は、IR を読んだあと、`sites.Enumerate` と `sg.BuildConsuming` の前に、宣言のあるパッケージの依存の関数（`Func.Dep`）を IR から落とす（`dropReplacedBodies`。プロジェクトの関数は落とさない）。落とした関数は、`-no-deps` のときと同じ、本体のない関数になる。呼び出しは、モデルの行があればモデル、なければ `unmodeled_library_call`（注記に、パッケージの本体を読んでいないことを書く）。呼び返し: モデルに `calls:` の行がない関数に、関数の値が渡されたときは、呼ばれるものとして扱い（ほかのオペランドを渡し、結果を呼び出しの結果にする。`assumed_call`）、プロジェクトの型のインターフェースの値（Go の `make_interface`）が渡されたときは、その型のメソッド（8 個まで）を同じく呼ばれるものとして扱う（`replacedRows`、`replacedCallbacks`。呼び出しの順序のために `callbackFuncs` にも入る）。出力: `summary.body_replaced_packages`（読まなかったパッケージ、宣言の場所、理由、関数の数。ソート済み）。`-read-declared-bodies` で宣言を無効にして本体を読める（`summary.body_replaced_ignored`）。index の鍵に、`-body-replaced` の一覧と `-read-declared-bodies` が入る（ルールファイルの宣言は、ファイルの内容のハッシュで、組み込みは組み込みのハッシュで入る）。減るのは、解析の時間とメモリ(ADR-100)
- **宣言を extractor に渡す**(ADR-105。`cmd/rivus/common.go` の `load`、`cmd/rivus/lang.go`): 宣言のあるパッケージは、extractor の段階から本体を読まない。結果(finding の ID の集合、`stats.functions`、`summary` の注記)は、IR のフィルタ(`dropReplacedBodies`)だけのとき(`-drop-after-read`)と同じで、`summary.body_replaced_packages` の項目だけが違う: extractor が読まなかったパッケージは、`functions_declared`(パッケージが宣言している関数の数。IR の `body_not_read`)を出し、`functions` は出さない(読んでいないので、落とした数はない)。`functions` は、読んで落とした数(`-drop-after-read` や Rust)のまま。**Go**: `gossa.Options.ReplacedPackage` が、`packages.Load` のあと、`go/ssa` の前に、宣言のあるパッケージ(依存のみ。標準ライブラリとプロジェクトは除く)の `Syntax` と `TypesInfo` を捨てる。`go/ssa` は本体のない関数だけを作り、`ReplacedFunc` が、型ごとに作られるラッパーや `init` も IR に出さない。型検査(`NeedDeps`)は省けない。**Dart**: `--body-replaced package:<名前>`(繰り返し可)。そのパッケージのファイルは、解決(`getResolvedUnit`)も、IR への下ろしもしない。クラスの階層(`extends:`)、動的呼び出しの候補、本体のある演算子、コンストラクタは、ライブラリの要素から作る(`_Index.library`。要素から分からないこと: 引数のないコンストラクタの本体と初期化子に仕事があるか。ないものとして扱う)。ライブラリの要素モデルを作ること(宣言と型のために要る)と、`import` をたどることは、今までどおり(読まないパッケージだけが使う依存は、読むが、IR に出ない。`PruneDeps` で落ちる)。読まなかった関数の数は、IR の `body_not_read` レコード(`unit`、`functions`)で言う。**Rust**: `--body-replaced <クレート名>`(繰り返し可。`-` と `_` は同じクレート)。ドライバが、宣言のある依存のクレートの本体を下ろさず、`body_not_read`(関数の数と、`merge` が、トレイト経由の呼び出しと `drop` の結びつけに使う名前)だけを書く。`cargo check` はそのクレートもコンパイルするので、extractor の時間とメモリは減らず、IR が小さくなる(7a 節の測定)。読まなかったクレートは `fn_skipped` と、「同じ名前の関数が 2 つのクレートにある」の注記に届かないので、そこはフィルタの実行と数が違う
- 組み込みモデルの範囲は、(a) Google Cloud（Firebase を含む。範囲の 1 つ目）、(b) 外部のサービスのクライアント（公開の指標の上位）、(c) 標準ライブラリ、(d) 代わりのないパッケージ、(e) pub の人気の上位（ADR-096、097。README、`docs/DART-MODEL-CANDIDATES.md`、`docs/GCP-MODELS.md`）。広げる理由は、コストを下げるためと、依存の中の sink を検出するため（ADR-099）。ユーザーは、ルールを足せる

### 5.6 値依存サニタイズと由来追跡

優先順:

1. **手書きモデルによる上書き** — 最優先
2. **値依存ノードとして記録** — サマリのガード（4.3）から、`conditional_nodes` に「条件と、条件ごとの結果の対応」を書き出す。**判定はしない**
3. （余裕があれば）サマリの特殊化 — 呼び出し側の引数がリテラルなら、定数値ごとに別サマリ

述語付きサマリ、シンボリック実行、SMT は実装しない。

由来追跡（provenance）は、値依存ノードの引数について、呼び出し側で何が渡されているかを後ろ向きに式のまま提示する。
たどれなくなったところで terminal kind を記録する。
値を断定してよいのは、定数そのものか、定数の単純なコピー(変数・フィールドへの格納と読み出し、呼び出しの結果が返す定数)だけ。途中に演算(否定、算術、比較、変換)があれば、オペランドを別々にたどって定数を畳み、畳んだ値を報告する。畳めなければ `unresolved`(オペランドの値を、結果の値として報告することはしない)。
`zero_value` は、その場所に届く書き込みが(フィールドへの格納、丸ごとの代入、関数の結果、ポインタ越しのコピー、配列の要素のコピー、アドレスを取られた変数のどれも)1 つもないと分かるときだけ(ADR-078)。

---

## 6. エントリポイント

コールグラフの根（プログラムの起点）は、extractor が `Func.Entry` で示す。外から来る値の入口は、ルールの `kind: param` の source で書く。
`exported_api_param` を定義した以上、「どこから先が外部か」を明示する必要がある。

| 対象 | 起点（`Func.Entry`） |
|---|---|
| Go | `main` パッケージの `main` と、パッケージの `init`（`internal/frontend/goir/goir.go` の `isEntry`）。変数の初期化は `Init`。ライブラリには起点がなく、エクスポートされた関数が起点になる |
| Dart | トップレベルの `main`（`extractor/dart/lib/extract.dart`）。変数の初期化は `Init` |
| Rust | extractor が決めた `main`（`extractor/rust/src/main.rs`） |

- **ユーザー追加指定:** ルールの source に `kind: param` を書く。シンボルの関数の引数（`params`、`recv`）を、フレームワークが渡す入力として宣言する。`marker`（インタフェースのメソッド名の glob。生成コードのサービス）と `extends` / `with_param_type`（継承の階層を持つ言語のサブクラス）は、複数の関数をまとめて宣言する（`internal/model/model.go`）。`kind: callback_param` は、シンボルの呼び出しのオペランド（`operand`。数え方は `calls:` と同じ。MODELING.md 4d 節）に渡された関数（`make_closure` を、コピーと変換をたどって引く）の、`from` から `to` の引数を source にする（`internal/sites/callback.go`。taint は変えない）。引けない呼び出しは `unresolved_callee` の注記になる（`sites.Result.UnresolvedCallbacks`）。注記の detail は `callback_param_unresolved:` で始まり（`sites.CallbackUnresolvedPrefix`）、`summary.callback_param_unresolved`（`count`、`sites[]` の `func`・`position`・`symbol`。位置でソート済み。ルールの行がなければ項目なし。finding が 0 件でも出る）がその数と場所を示す。ルールファイルに `entrypoints:` という節はない
- 起点から届くかは、検出結果の `reachability` が示す（4.5）。フレームワークが呼ぶメソッド（外の型のメソッドを上書きしたもの。`Func.Overrides`）は、`from_framework_callback`（ADR-073）
- ライブラリの公開関数の引数は `exported_api_param` の起点になる

---

## 7. 出力量の制御

過大近似を維持するため、素朴に全経路を出すと LLM 判定のコストが破綻する。実運用で最初に詰まる箇所なので、設計に入れる。

- 検出結果の単位は **(source サイト, sink サイト) の組**。経路はその中に入れる（シンボルの組ではない。シンボルの組だと、プロジェクト中の同じ関数の呼び出しがすべて 1 件に混ざってしまうため）
- **経路は 1 本だけ出す**（`internal/taint/engine.go`）。ADR-023 の「3 本まで。あふれは `omitted_paths` に数える」は、実装されていない（`omitted_paths` というフィールドはない）。経路が複数あっても、残りは出力に出ない。1 本の経路は、起点から届くもの、未解決の少ないもの、短いものを選ぶ。同じ sink に届く別の source は、`other_sources` に、本数と経路の素性だけを出す
- 経路が 200 段（`maxPathSteps`）を超えると、途中を落とす。これは不確実性の注記を残さない（PRINCIPLES.md 1 節）
- 検出結果とサイトに**安定した ID と指紋**を持たせる（4.6、ADR-028）。「前回から新規に増えたもの」を、前回の結果（ベースライン）との比較で CI が扱えるようにする
- **サイト ID**（`rivus sites` の各項目）も同じ方針で安定させる。ID を `rivus query --site` の seed に使える
- **出力の詳細度を選べる。** `-detail` は `summary`（経路とスニペットを省く）と `snippet`（全部。既定）の 2 つ（`internal/report/report.go`）。関数全文を付ける段階はない。エージェントのコンテキストを圧迫しないため

---

## 8. テスト戦略

後から足せないので最初に決めた。今の層は、次のとおり（使い方は README の開発の節、各ベンチの README）。

1. **マイクロベンチマーク集（`testdata/`）** — 言語ごとに作る。
   「検出されるべきケース」と「**意図的に検出できないケース**」の両方を置き、後者は「検出されないこと」を assert する。
   各ケースに期待される terminal kind も assert する
2. **differential test** — コアの SSA 化と、lifted `go/ssa` を比較する（`internal/ssacheck`）。
   - (1) NaiveForm → IR → コアの SSA 化
   - (2) `go/ssa` の lifted 出力
   - 上記の φ の配置と到達定義が一致することを確認する
   SSA 化は自作部分で最もバグを埋め込みやすく、症状が「なぜか汚染が消える / 増える」という分かりにくい形で出るため（ADR-036）
3. **zoo と curated、言語間の比較** — `bench/zoo`、`bench/zoo-go`、`bench/zoo-dart`、`bench/zoo-rust` は、流れの形を 1 つずつ、同じ意味のコードを 3 言語で書く。期待値は、ケースを実際に動かした結果（[DESIGN-zoo.md](DESIGN-zoo.md)）。`bench/*-curated` は、実プロジェクトと CVE の形を写したもの。`bench/parity` は、組み込みモデルの整備の言語間の差を数える。IR の中立性の検証にもなる
   - `calls:` の行の確認: 標準ライブラリの行は、道具が実際に呼んで照合する（`TestZooCallbacks*`）。パッケージの行は、第三者のコードを実行しないので、各行に `checked_by: cell` を書き、それを通るセルがあること（`@sink` か `@miss` のマーク付き）を `TestCallsRowsAreHeldByCells` が確認する（MODELING.md 3 節）
4. **ドッグフーディング** — rivus 自身を rivus で解析する（`cmd/rivus/dogfood_test.go`）。`attributable_to: rivus` の比率を監視する
5. **隔離のテスト** — 敵対的なビルドを、サンドボックスが閉じ込めること（`TestSandbox*`、`cmd/rivus/isolate_sandbox_darwin_test.go`。15 節）

---

## 9. 性能目標

履歴・計画は [history/ARCHITECTURE-plans.md](history/ARCHITECTURE-plans.md) の 9 節にある（実測前の仮説と、規模の階層）。今の測定値は、この文書には書かない。数字は、測った日とコミットを付けて、ベンチの記録（`bench/README.md`、`bench/LEDGER.md`）と [DESIGN-memory.md](DESIGN-memory.md) にある。

## 10. Phase 1 の細分化

履歴・計画は [history/ARCHITECTURE-plans.md](history/ARCHITECTURE-plans.md) の 10 節にある（Phase 1 のステップ表、Dart スパイク、Phase 2 以降の見通し）。Phase 1 と 2 は完了した。Go、Dart、Rust の 3 言語の extractor は、実装されている。

## 11. レビューで確認したい点

履歴・計画は [history/ARCHITECTURE-plans.md](history/ARCHITECTURE-plans.md) の 11 節にある。全項目が決定済みで、決めた内容は ADR（`docs/adr/`）にある。

---

## 12. グラフ層（`rivus index` / `rivus graph`）

解析結果（データの流れ）ではなく、**呼び出しの関係**を、1 回作って保存し、あとから検索する層。taint 解析を実行しない。出力には必ず `relation: "calls"` と `flows: "not_checked"` が付く（`internal/graphq/graphq.go`）。
使い方は [GRAPH.md](GRAPH.md)、設計の理由は [DESIGN-graph.md](DESIGN-graph.md)。

- **保存（`internal/graphdb`）:** 1 つの SQLite ファイル（純 Go の `modernc.org/sqlite`。cgo なし）。表は `meta`、`files`、`funcs`、`callsites`、`edges`、`refs`（関数が値として参照される所）、`sites`（`schema.go`）。**使い捨てのキャッシュ**で、移行はしない。スキーマのバージョン（今は 2）が違うファイルは拒み、作り直しを案内する。
- **層:** 今あるのは構造の層（関数、呼び出し点、エッジ、サイト）だけ。taint の層（findings、不確実性）は、このファイルにはない（`index -taint` は、JSON の index に出す）。`meta` が、どの層があるかを記録するので、層がないことを「findings がない」と読み違えない。
- **書き先:** `-graph <file>`、なければユーザーのキャッシュ（`os.UserCacheDir()/rivus/<-C の絶対パスのハッシュ>/graph.db`。`RIVUS_CACHE_DIR` で変えられる）。解析対象のディレクトリには作らない（`cmd/rivus/graph.go`）。
- **鍵（source hash。`internal/graphdb/key.go`、`cmd/rivus/graphkey.go`）:** 設定（フラグ、ルール、rivus のバージョン、環境、ツールチェイン）、読んだファイルの内容のハッシュ、ファイル一覧のハッシュ（新しく読めるようになったファイルに気づくため）で作る。疑わしいときは、いつも「作り直す」ほうを選ぶ。偽の「新しい」は変更を見逃し、偽の「古い」は index の時間だけで済むため。`graph info -C` が、今のソースと照合して `fresh` を返す。
- **問い合わせ（`internal/graphq`）:** `callers`、`callees`、`path`、`impls`、`dispatchers`。1 関数ずつ、ファイルから隣を引くので、グラフを全部メモリに持たない。必ず、深さと件数に上限を持つ（深さは既定 1、最大 6。件数は既定 50。経路は既定 5 本）。動的な呼び出しは、候補が 3 以下なら展開し、それより多いものは境界（フロンティア）として見せ、20 を超えるもの（ハブ）は `path` に使わない（`hub_skipped`）。**切ったものと、絞ったもの（`--via`、`--avoid`、`--only-impl`）は、必ず出力に書く。**

## 13. 関数値の呼び出しの候補（`internal/valuecalls`）

関数値の呼び出し（`call` の `value`）の候補を、**IR の上で 1 か所で決める**。サイトの列挙（ルールが、変数が持つ関数を指定できる。ADR-045）と、解析のグラフが、同じ候補を見るため（`sites` と `sg` が `valuecalls.Resolve` を呼ぶ）。

- **候補の出所:** Go は、extractor の VTA の結果。ほかの言語は、`valuecalls` が、値に流れ込む関数を、コピー、φ、引数（呼び出し元が渡すもの）を通して探す。メモリは追わない（フィールドやリストに入れた関数は見つからず、呼び出しは未解決のまま）。
- **限界:** 候補が 32 を超えたとき（`ir.ValueCandidateMost`）、呼び出し元を 3 段より上までたどるとき（`maxDepth`）は、候補を付けない。理由は `Callee.ValueLimit`（`candidate_limit`、`caller_depth_limit`）に残り、不確実性の detail になる。
- **引数を通って届く関数:** どの候補が呼ばれるかは、呼び出し元に依る。候補は、通った引数の番号を持ち（`ValueCand.Param`）、呼び出し元が、渡した関数以外を通る流れを外す。
- **絞り込み（`narrow.go`）:** Rust の `Fn::call` / `FnMut::call_mut` / `FnOnce::call_once` の候補は、extractor が「型の合うクロージャすべて」で作る。ここでは、**レシーバに流れ込む関数に絞る**（ADR-079、[DESIGN-capture.md](DESIGN-capture.md)）。
  - 規則は「**全部たどれたときだけ絞る**」。絞ると流れを失いうるが、作り出すことはない。たどれない物（規則のない命令、構造体のフィールド、呼び出しの結果、グローバル、トレイトオブジェクト、呼び出し元が全部は分からない引数、上限）に達したら、候補を全部残す。
  - 引数をたどるときは、呼び出し元が全部読まれていることが条件。`Func.Private`（`pub` がない）か、**閉じた世界**（起点があり、外からシンボルで呼べる関数がなく、extractor が `caller_marks` を付けた）。閉じた世界は前提を 1 つ入れるので、絞った不確実性に `basis: closed_world_entry` を付けて示す（「起点のあるプログラムとして読んだので、外からの呼び出しは数えていない」）。絞る前の候補数は `narrowed_from`。
  - Dart の動的な呼び出しへの同じ絞り込みは、まだない（設計だけ）。
- **関数値のパスで補う候補（`-resolve-func-values`、既定 off）:** 候補が 0 件の呼び出しに、`valuecalls` ではなく、関数値を汚染の要素として運ぶ別の走り（`taint.ResolveFuncValues`）が、候補を補う。補った呼び出しは `unresolved_callee` の注記を残す（詳細は `func_values_supplemented:`）。`rivus index` で on のときは、補ってから索引を書く（補った呼び出しは `meta` の `func_values_supplemented`。キーにフラグが入る）。`-timeout` は補う回の全部と通常のパスで共有する。設計と指標は [DESIGN-callouts.md](DESIGN-callouts.md) の 5.6。

## 14. `parts`（sink が引数のフィールドを指定する）

sink は、位置の引数のうち、**どのフィールドが危険か**を指定できる（ADR-074。設計は [DESIGN-field-sinks.md](DESIGN-field-sinks.md)、書き方は [MODELING.md](MODELING.md)）。

- **ルール:** `parts: [{arg: n | recv: true, field: "A.B"}]`。1 か所だけなら短い形（`args: [n]` ＋ `field:`）。フィールドは、IR にある名前を `.` でつなぐ。`[]` は、スライス、配列、map の要素。`args` で指定した位置は、全体が危険で、`parts` はそれ以外の位置のためにある（`internal/model/model.go` の `Sink`、`SinkPart`、`WholeArgs`）。
- **照合（`internal/taint/fieldsink.go` の `matchParts`）:** モデルの名前は、値からのアクセスパスの**部分列**として照合する。名前でない段（フィールドの後ろのポインタ、インタフェースの中身、コンテナの要素）は飛ばせる。名前のほうは、飛ばせない。
- **結果:** そのフィールド（かその下）にある汚染は、確かな検出。フィールドの隣の汚染は、流れではない。フィールドにあるとは言えない汚染（値全体が汚染されている、パスの上限で切れた、コンテナの要素、読まれていない関数を通った）は、検出として出し、不確実性 `field_imprecision`（detail は `whole_value` / `path_limit` / `element` / `opaque_callee`）を付ける。フィールド名がプログラムにないときは、`field_unresolved` を付けて、sink を全体として扱う。何も報告せずに落とすことはしない。

## 15. 隔離（信頼できない Rust の読み込み）

Rust の extractor は、対象をビルドする（`cargo check`）ので、対象の `build.rs`、proc macro、`[build-dependencies]` が動く。（`-lang` なしでは、`Cargo.toml` があるとき `[Y/n]` と聞く。端末でないときは `-yes` が必要。読まなかった言語は `summary.unread.skipped_languages` に残る。docs/rust-isolation.md）そこで `rivus analyze -lang rust` と `rivus extract -lang rust` は、**macOS では既定で、OS のサンドボックス（Seatbelt、`sandbox-exec`）の下で動かす**（ADR-093。以前の Docker のコンテナ（ADR-064）の経路は、なくなった）。プロファイルは、既定ですべて拒否し、必要なものだけを許す。2 段階で、取得（ネットワークあり。`cargo fetch --locked` だけ。ビルドしない。対象の `.cargo` の設定は使わない）と、抽出（ネットワークなし。`cargo check --offline --locked`。書けるのは作業ディレクトリだけ）。ホストの環境変数は渡さず、対象と取得したものは読み取り専用で、IR は標準出力でだけ返す。時間の上限と、プロセスグループごとの kill がある（メモリ・CPU・プロセス数の強い上限はない）。**Linux には隔離の手段がないので、既定では何も実行せずに止まる。**ホストで直接動かすのは、`-unsafe-host-exec`（信頼できるコードだけ）。コードは `cmd/rivus/isolate.go`、`isolate_fetch.go`、`isolate_sandbox_darwin.go`、`internal/sandbox/`、`lang.go`。詳しくは [rust-isolation.md](rust-isolation.md)。ルールは、解析対象の `.rivus/rules` からも読まれる（`-no-project-rules` で外せ、消せる量は `project_rules` が出す）。Go と Dart の extractor は、サンドボックスの下では動かさない。macOS のサンドボックスでは、`-isolation-env` に `DEVELOPER_DIR` がなく、コマンドラインツールがあるとき、ビルドの `DEVELOPER_DIR` をそこに向ける（Xcode.app は読ませないため。`summary.isolation.developer_dir`。`cmd/rivus/isolate_devdir.go`）。

## 16. メモリ

ピークは、解析ではなく、**読み込みと go/ssa の構築**にある（Go）。次の手で下げている（設計と測定は [DESIGN-memory.md](DESIGN-memory.md)）。

- **GC の設定（`tuneGC`、`cmd/rivus/main.go`）:** GC の目標を 50% にして（Go の既定は 100%）、ごみを多く持たない。環境変数 `GOGC` があれば、それを優先する。
- **go/ssa の解放:** IR とテキストを作ったあと、go/ssa のプログラムと、読み込んだパッケージを手放す（`ReleaseSSA`、`internal/frontend/gossa/load.go`。呼ぶのは `cmd/rivus/common.go`）。
- **依存ごとの解放:** 依存のパッケージのソースと型情報は、そのパッケージを構築し終えるたびに手放す（`load.go` の `buildInOrder` のコールバック）。
- 起点から届く関数だけを IR にする。`-no-deps` は、依存を読まないので、メモリが大きく減る（その分、依存への呼び出しは推測になる）。

測定値（ピーク GB、時間）は、測った日とコミットを付けて DESIGN-memory.md にある。この文書には書かない。未実装の案（段階的な構築など）も DESIGN-memory.md にある。

---

## 診断出力 `-v`

`-v` の `slow function` の行(300 ms 超の関数)は `(cpu …)` の列を持つ: その関数の解析中の、goroutine 自身のスレッドの CPU 時間(`CLOCK_THREAD_CPUTIME_ID`。`-v` のときだけ、解析の間 goroutine をスレッドに固定して読む。時計のない OS では 0)。壁時計の時間は GC と他のワーカーで 4〜5 倍に膨らむので、関数のコストは cpu の列で読む。出力の追加だけで、解析結果、finding、JSON の report は変わらない。
