# rivus アーキテクチャ

状態: 初版（レビュー待ち）。決定の経緯と理由は [DECISIONS.md](DECISIONS.md)、用語は [GLOSSARY.md](GLOSSARY.md) を参照。

---

## 1. 全体構成

rivus は「脆弱性を報告する SAST」ではなく、**到達可能性に関する事実を問い合わせるクエリエンジン**である。
安全か危険かの判定は行わない。決定論的に確定できる事実と、確定できなかった理由を構造化して出す。

```
                 言語ごとの extractor（言語固有の知識が要る部分）
 ┌──────────────────────────────────────────────────────────────┐
 │ Go   extractor (Go 製・同梱)   go/packages → go/ssa(NaiveForm)│
 │ Rust extractor (Rust 製・外部) rustc_public → MIR             │──┐
 │ Dart extractor (Dart 製・外部) package:analyzer → 自作 CFG    │  │ サブプロセス起動
 └──────────────────────────────────────────────────────────────┘  │ 中立 IR (JSON Lines, schema_version 付き)
                                                                    ▼
 ┌──────────────────────────────────────────────────────────────────────────┐
 │ コアエンジン（Go 製・言語非依存）                                          │
 │                                                                          │
 │  IR 読み込み・検証                                                         │
 │     ↓                                                                    │
 │  SSA 化（dominator → φ 挿入）                                              │
 │     ↓                                                                    │
 │  コールグラフ（言語ごとのビルダーが供給。SCC 縮約 → トポロジカル順）        │
 │     ↓                       ┌──────────────┐                             │
 │  関数サマリ算出（bottom-up） ←─ モデル(YAML)  │ source/sink/sanitizer/      │
 │     ↓                       └──────────────┘ propagator。組み込みも同形式 │
 │  クエリ（前向き / 後ろ向き / source-sink 組）                              │
 │     ↓                                                                    │
 │  由来追跡 (provenance) ・ 条件ガード ・ 不確実性の記録                      │
 │     ↓                                                                    │
 │  出力: 構造化 JSON（本体）／ SARIF（副次）                                  │
 └──────────────────────────────────────────────────────────────────────────┘
```

責務の境界は「**言語固有の知識が要るか**」で引く。CFG 構築まで（言語固有の脱糖を含む）は extractor、SSA 化から先はコア。

### 想定する使われ方（エージェントの道具）

rivus の主な利用者の 1 つは、脆弱性の有無を判定する **LLM エージェント**である。
エージェントは「疑わしい箇所を軽く洗い出し → 気になる箇所だけ詳細に分析する」という 2 段構えで使う。

| レベル | 内容 | 必要な処理 |
|---|---|---|
| L0 | ルールの source / sink にマッチする**サイト**（箇所）を列挙する | ローダ ＋ シンボル照合のみ（ほぼ即時） |
| L1 | L0 に加えて、sink の引数が**リテラル定数だけ**のものに印を付ける。落とさず、事実として印を付けるだけ | 手続き内の簡単な確認 |
| L2 | 詳細分析（サマリ、ラベル、由来追跡、不確実性の記録） | 全部 |

```
rivus sites ./...                   # L0 / L1: 疑わしい箇所（サイト）の列挙
rivus query --site <ID> ./...       # L2: そのサイトの詳細分析
```

- L0 / L1 は判定ではなく**列挙**であり、ADR-001（事実を出し、判定はしない）と矛盾しない
- サイトには**安定した ID**（関数 ID ＋ 位置）を振り、L2 のクエリの seed として受け取れるようにする
- サイトの列挙は IR の `call`（呼び先 `FuncID`）で行うので、言語中立
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
| `FieldID` | 型 + フィールド名。フィールド感度のための名前 |
| `Pos` | ファイル、オフセット、長さ |
| `LocalID` | 関数内ローカルの番号 |
| `BlockID` | 関数内ブロックの番号 |

### 2.3 命令セット v0（27 種）

「15 種類程度」という当初の見積もりは過小だった。以下を初版とする。

**値の生成・変換**

| 命令 | 内容 |
|---|---|
| `const` | リテラル定数。値と型を持つ |
| `param` | 関数引数の受け取り。位置番号を持つ（レシーバは 0 番） |
| `copy` | 単純代入 `dst = src` |
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
| `slice` | 部分スライス `base[lo:hi]`。要素への汚染をそのまま引き継ぐ |
| `global_load` | パッケージ変数・static の読み出し |
| `global_store` | パッケージ変数・static への書き込み |

**関数**

| 命令 | 内容 |
|---|---|
| `make_closure` | クロージャ生成。呼び先 `FuncID` と捕捉した値のリスト |
| `call` | 関数呼び出し（2.4 参照） |

**例外・不明**

| 命令 | 内容 |
|---|---|
| `exception_value` | 例外ハンドラブロックの先頭で、捕捉した値を受ける |
| `unsupported` | extractor が脱糖できなかった構文。理由の文字列を持ち、`missing_source` 等の記録の起点になる |

**終端命令（各ブロックに 1 つ）**

| 命令 | 内容 |
|---|---|
| `jump` | 無条件分岐 |
| `branch` | 条件分岐。`cond=<ローカル> true=<ブロック> false=<ブロック>` と、条件式のソース範囲 `Pos` |
| `return` | 戻り（複数戻り値は値のリスト） |
| `throw` | 例外送出 |
| `unreachable` | 到達不能（`panic` 後など） |

合計: 値 7 ＋ メモリ 11 ＋ 関数 2 ＋ 例外・不明 2 ＋ 終端 5 ＝ 27。「25 前後」の想定内。

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
`finally` が複数経路から到達されるときは、extractor が経路ごとにブロックを複製するか、
`finally` 終了後の行き先を値で分岐させる。どちらにするかは Dart スパイクの結果で決める。

### 2.6 ストリーム形式

JSON Lines。**先頭行は必ず `{"schema_version": ...}`**。以降は関数ごとに 1 行以上。
`rivus extract --lang=... --output=ir.jsonl` で IR を単独で出力でき、人間が読んで検証できることを一級の要件とする。

---

## 3. コアエンジンのパッケージ構成

Go モジュール `github.com/yagihash/rivus`。依存は標準ライブラリ ＋ `golang.org/x/tools`（最新）＋ YAML パーサ。

```
cmd/rivus/                CLI（extract / analyze / query / models）
internal/
  ir/                     IR の型定義、JSON Lines の読み書き、検証、ダンプ
  extractor/              extractor の解決層（Locator）と起動・ストリーム読み取り
    gosrc/                Go extractor（Phase 2 以降。Phase 1 では frontend/gossa）
  frontend/gossa/         Phase 1: go/packages → go/ssa。go/ssa に触れるコードはここだけ
  ssabuild/               IR → SSA（dominator tree、dominance frontier、φ 挿入）
  callgraph/              CallGraphBuilder インタフェース、SCC 縮約、トポロジカル順
    gocg/                 Go 用: CHA → VTA
  model/                  YAML ルールの読み込み、シンボル照合（exact / implementations）
    builtin/              組み込みルール（YAML。ユーザー定義と同形式）
  taint/                  手続き内ソルバ（worklist）、ラベル、アクセスパス
  summary/                関数サマリの定義、算出、呼び出し側での展開、SCC 内の不動点
  guard/                  引数由来の条件ガードの抽出と上限制御
  provenance/             引数の由来追跡（後ろ向き）
  uncertainty/            terminal kind、attributable_to、記録
  sites/                  サイト（source / sink の候補箇所）の列挙と ID 付け。L0 / L1
  query/                  前向き / 後ろ向き / source-sink 組。サイト ID を seed に取れる
  report/                 結果のグループ化、安定 ID、経路数の上限、JSON 出力
    sarif/                副次出力（JSON が固まってから）
  testutil/               testdata ハーネス
testdata/
  go/ …                   マイクロベンチマーク（検出すべき / 意図的に検出しない）
spike/dart-cfg/           Dart 実現性スパイク（捨てコード）
docs/
```

### 依存の向き

```
cmd → query → summary → taint → ssabuild → ir
                 ↘ callgraph        ↗
        model ──→ (taint, summary から参照)
        uncertainty ──→ (taint, summary, provenance, query から記録)
extractor / frontend/gossa → ir          ← Go 固有の型は gossa 内で完結させ、外へ出さない
```

**コアから `go/ssa` や `go/callgraph` を直接 import してよいのは `frontend/gossa` と `callgraph/gocg` のみ。**
これを守れば Phase 2 の IR 切り出しは「gossa を extractor に置き換える」作業に収まる。

### 差し替えを前提にする境界

1. **Extractor の在り処の解決層** — `Locator` インタフェース。実装は「PATH から探す」「設定で指定されたパス」「埋め込みバイナリを展開」。
   初期は前 2 つのみ。`go:embed` は動作確認後の最適化として後から追加できる形にする
2. **コールグラフビルダー** — `CallGraphBuilder` インタフェース。
   Go は CHA → VTA、Dart は `pkg/vm` の type flow analysis、Rust は monomorphize collector 等。言語ごとに別実装で、Go 固有の手法をコアに漏らさない
3. **エントリポイント解決** — 言語・用途ごとの既定 ＋ ユーザー追加指定（6 節）

### Go のコールグラフに関する実装メモ

`vta.CallGraph(funcs map[*ssa.Function]bool, initial *callgraph.Graph)` を実物で確認済み（`golang.org/x/tools` v0.52.0）。

- `initial` に CHA（または RTA）の結果を渡して絞り込むのが標準的な使い方。`initial == nil` なら内部で CHA を使う
- 渡す SSA 関数は **`ssa.InstantiateGenerics` 付き**で構築しておく必要がある
- 結果にはルートノードがない
- 結果は reflection と unsafe を除いて健全。これらは `reflection` / `ffi_boundary` として記録する側の責務

---

## 4. 主要なデータ構造（案）

Go 風の擬似コード。実装時に調整する。

### 4.1 IR

```go
type FuncID string
type TypeID string

type Pos struct {
    File   string
    Offset int
    Length int
}

type Func struct {
    ID       FuncID
    Params   []Param          // 位置、名前、TypeID
    Results  []TypeID
    Locals   []Local          // LocalID → 名前、TypeID
    Blocks   []Block          // 先頭が entry
    Pos      Pos
    BodyHash string           // モデルのキャッシュキーに使う（5.4 参照）
    Exported bool
}

type Block struct {
    ID     BlockID
    Instrs []Instr
    Term   Terminator
    Succs  []Edge             // 種別付き
}

type Instr struct {
    Op   Op                   // 2.3 の命令
    Dst  LocalID              // なければ -1
    Args []LocalID
    Imm  any                  // const の値、FieldID、FuncID など Op ごとに決まる
    Call *CallInfo            // Op == Call のとき
    Pos  Pos
}

type Terminator struct {
    Kind     TermKind         // jump / branch / return / throw / unreachable
    Cond     LocalID          // branch のとき
    CondPos  Pos              // 条件式のソース範囲。人間可読な式はここから切り出す
    TrueBB   BlockID
    FalseBB  BlockID
    Values   []LocalID        // return
}
```

### 4.2 汚染

```go
type Label string             // 例 "user_input", "html_escaped", "sql_unsafe"

type LabelSet struct{ /* ビットセットまたは小さな sorted slice */ }

// アクセスパス。深さは k（既定 3）で打ち切る。打ち切りを起こしたときは記録する。
type AccessPath struct {
    Base   ValueRef           // ローカル / グローバル / 引数 / 戻り値
    Fields []FieldID          // 長さ <= k
}

// 汚染状態: アクセスパス → ラベル集合。コンテナ要素は区別しない（Fields には含めない）
type TaintState map[AccessPath]LabelSet
```

### 4.3 関数サマリ

```go
type Summary struct {
    Func     FuncID
    Flows    []Flow           // 引数 / グローバル → 戻り値 / 引数の副作用先 / sink
    Returns  []SourceReturn   // 戻り値が source 由来
    Sinks    []SinkReach      // 引数 → 関数内（またはその呼び先）の sink
    Uncertain []Uncertainty   // この関数の解析で打ち切った箇所
    Origin   ModelOrigin      // builtin / human / llm / derived（解析で算出）
}

type Flow struct {
    From   AccessPath         // 引数 i のフィールドなど
    To     AccessPath         // 戻り値、または引数 j のフィールド（副作用）
    Add    LabelSet           // 付与するラベル
    Drop   LabelSet           // 落とすラベル（sanitizer）
    Guards []Guard            // 引数由来の条件。空なら無条件
    Trace  []TraceStep        // 経路復元用
}

type Guard struct {
    Param     int             // 条件が依存する引数
    CondPos   Pos             // 条件式のソース範囲
    Polarity  bool            // true 側を通ったときこの Flow が成立するか
    Expr      string          // ソースから切り出した条件式（提示用。評価はしない）
}
```

**ガードの制約:** 記録するのは引数由来の条件のみ。「その条件値が引数からのデータフローで到達可能か」で判定し、
この判定には taint ソルバを流用する（source = 引数、sink = 分岐条件）。
1 サマリあたり上限 4 個。超えたらガードなしの過大近似（全経路の和集合）に畳み、
`attributable_to: rivus` の不確実性（`guard_limit`、サブ理由）として記録する。

### 4.4 不確実性

```go
type TerminalKind string
const (
    KindLiteral TerminalKind = "literal"
    KindConfigRead = "config_read"
    KindExportedAPIParam = "exported_api_param"
    KindGlobalVar = "global_var"
    KindDynamicDispatch = "dynamic_dispatch"
    KindUnmodeledLibraryCall = "unmodeled_library_call"
    KindReflection = "reflection"
    KindConcurrency = "concurrency"
    KindContainerElementImprecision = "container_element_imprecision"
    KindDepthLimit = "depth_limit"
    KindFFIBoundary = "ffi_boundary"                   // 追加
    KindUnresolvedCallee = "unresolved_callee"         // 追加
    KindMissingSource = "missing_source"               // 追加
    KindRecursionApproximation = "recursion_approximation" // 追加
)

type Uncertainty struct {
    Kind          TerminalKind
    AttributableTo string      // "target" | "rivus" | "mixed"
    Detail        string       // サブ理由（missing_source など）
    Chain         []string     // 辿った式の連鎖
    Candidates    []FuncID     // dynamic_dispatch の候補
    Pos           Pos
    Snippet       string
}
```

#### kind ごとの `attributable_to`（初版の割り当て）

| kind | attributable_to | 備考 |
|---|---|---|
| `literal` | （なし） | 確定。不確実性ではない |
| `config_read` | target | |
| `exported_api_param` | target | |
| `global_var` | target | 書き込み箇所は列挙する |
| `reflection` | target | |
| `ffi_boundary` | target | |
| `missing_source` | target | ワークフロー前提（`go generate` / `build_runner` 未実行）に由来 |
| `dynamic_dispatch` | mixed | 言語機能として呼び先が決まらないが、候補の絞り込み精度（VTA など）は rivus の改善余地がある |
| `unresolved_callee` | mixed | 関数値は実行時に決まるが、生成箇所を追う型フロー解析で候補を絞れる |
| `concurrency` | mixed | 実行順序に依存するが、channel を単純なキューとしてモデル化すれば追える範囲が広がる |
| `unmodeled_library_call` | rivus | モデルを書けば減る |
| `container_element_imprecision` | rivus | 結論に影響した場合のみ記録（ノイズ防止） |
| `depth_limit` | rivus | |
| `recursion_approximation` | rivus | |
| `guard_limit`（サブ理由） | rivus | ガード上限超過。専用 kind にはせず、`depth_limit` ではない別の記録として残す |

`mixed` は「言語機能由来で、かつ rivus の精度向上でも減らせる」もの。
集計では `target` / `rivus` / `mixed` を**別々に**出し、`mixed` を `rivus` に含めない（投資で減らせる量を過大に見積もらないため）。
`mixed` の割り当ては暫定で、Phase 1 のステップ 1.12（ドッグフーディング）で実測してから見直す。

### 4.5 出力

```go
type Finding struct {
    ID           string        // (source シンボル, sink シンボル) 組から安定生成
    Source       Endpoint
    Sink         Endpoint
    Paths        []Path        // 上限あり。超過分は OmittedPaths に件数のみ
    OmittedPaths int
    Conditional  []ConditionalNode
    Provenance   []Provenance
    Uncertain    []Uncertainty
    ModelsUsed   []ModelRef    // origin: llm のモデルが影響したらここに現れる
    Confidence   string        // "requires_review" など。判定ではなく、確定度の分類
}
```

ブリーフ 7.3 節の出力例に沿う。`resolved` と `unresolved` を分け、確定した事実と調査が必要な項目を書き分けられるようにする。

---

## 5. 解析の仕組み

### 5.1 手続き内

- SSA 化されたローカルの上で worklist により不動点を求める
- 合流（φ）は和集合（may 解析）
- ラベル付き: sanitizer は特定ラベルだけを `Drop` し、sink は特定ラベルにのみ反応する
- 明示的フローのみ追う。暗黙的フロー（制御依存）は追わない（ただし条件ガードの抽出では分岐条件を見る）

### 5.2 手続き間（bottom-up の関数サマリ）

1. コールグラフを構築し、SCC に縮約してトポロジカル順（呼び先から）に処理する
2. 各関数の手続き内 taint を解き、サマリを算出する
3. 呼び出し側でサマリを展開する。**SCC の内部だけ**不動点まで反復する

サマリの独立性（関数単位で閉じていること）を保つ。これが将来の増分解析の土台になる。増分解析自体は Phase 1〜4 のスコープ外。

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
- symbol: "database/sql.(*DB).Query"
  match: exact

# インタフェースのメソッドにマッチし、コールグラフ経由で実装に展開
- symbol: "io.Writer.Write"
  match: implementations     # 展開数の上限と dynamic_dispatch の記録を伴う
```

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

- 出力に「このモデルで抑制した / 伝播させた」を必ず記録する。LLM 由来モデルが結論に影響した検出結果は、その旨がトレースできる
- `--deterministic-only` は `origin: llm` のモデルをすべて無視して走る

#### モデルカバレッジレポート

```
rivus models coverage ./...
```

モデル未定義の外部関数の呼び出しを頻度順に並べる。`unmodeled_library_call` を集計するだけで、実装は軽い。**Phase 1 に含める。**

### 5.5 依存パッケージ

- **解析するのは自モジュールのみ。** 依存（stdlib・第三者ライブラリ）との境界は手書きモデルで扱い、無ければ `unmodeled_library_call`
- 設定で選んだ依存だけ解析対象に含められる
- 組み込みモデルの最低ライン: Go は `database/sql`、`os/exec`、`net/http`、`html/template`、`text/template`、`strings` など。
  Dart は `dart:*` と Flutter SDK の主要部分。Rust は `std` と頻出クレート（`serde`、`tokio`、`reqwest` など）

### 5.6 値依存サニタイズと由来追跡

優先順:

1. **手書きモデルによる上書き** — 最優先
2. **値依存ノードとして記録** — サマリのガード（4.3）から、`conditional_nodes` に「条件と、条件ごとの結果の対応」を書き出す。**判定はしない**
3. （余裕があれば）サマリの特殊化 — 呼び出し側の引数がリテラルなら、定数値ごとに別サマリ

述語付きサマリ、シンボリック実行、SMT は実装しない。

由来追跡（provenance）は、値依存ノードの引数について、呼び出し側で何が渡されているかを後ろ向きに式のまま提示する。
辿れなくなったところで terminal kind を記録する。

---

## 6. エントリポイント

コールグラフの根をどう決めるかは未定義だったので、ここで決める。
`exported_api_param` を定義した以上、「どこから先が外部か」を明示する必要がある。

| 対象 | 既定のエントリポイント |
|---|---|
| Go | `main.main` ＋ エクスポートされた関数・メソッド。テスト関数を含めるかは設定 |
| Dart バックエンド | ルートハンドラの登録箇所（フレームワークごとにモデルとして記述） |
| Flutter | `main`、`build` メソッド、イベントハンドラ |
| Rust | `main` ＋ `pub` な関数 ＋ ハンドラ登録（フレームワークごと） |

- **ユーザー追加指定:** ルールファイルに `entrypoints:` セクションを持てる（シンボル指定は 5.4 と同じ）
- エントリポイントの引数・非 `main` ライブラリ入口は `exported_api_param` の起点になる

---

## 7. 出力量の制御

過大近似を維持するため、素朴に全経路を出すと LLM 判定のコストが破綻する。実運用で最初に詰まる箇所なので、設計に入れる。

- 検出結果を **(source シンボル, sink シンボル) の組でグループ化**し、経路はその中に入れる
- 1 グループあたりの経路数に上限（既定 3）。超えたら代表経路だけ出し、`omitted_paths` に残件数を記録
- グループ単位で**安定した ID** を振る。同じ source/sink 組・同じ関数シグネチャなら、行番号が動いても同じ ID になるようにする。「前回から新規に増えたもの」を CI で扱えるようにする
- **サイト ID**（`rivus sites` の各項目）も同じ方針で安定させる。ID を `rivus query --site` の seed に使える
- **出力の詳細度を選べる**ようにする（概要のみ / スニペット付き / 関数全文付き）。エージェントのコンテキストを圧迫しないため

---

## 8. テスト戦略

後から足せないので最初に決める。

1. **マイクロベンチマーク集（`testdata/`）** — 言語ごとに作る。
   「検出されるべきケース」と「**意図的に検出できないケース**」の両方を置き、後者は「検出されないこと」を assert する。
   各ケースに期待される terminal kind も assert する
2. **differential test** — コアの SSA 化と、lifted `go/ssa` を比較する。
   - (1) NaiveForm → IR → コアの SSA 化
   - (2) `go/ssa` の lifted 出力
   - 上記の φ の配置と到達定義が一致することを確認する
   SSA 化は自作部分で最もバグを埋め込みやすく、症状が「なぜか汚染が消える / 増える」という分かりにくい形で出るため。
   導入時期は Phase 1 の終盤（1.11b）。Phase 1 の `testdata` が揃った段階で差し込み、SSA 化の問題を「検出結果の変化」として検知できるようにする
3. **クロス言語の等価性テスト** — Phase が進むごとに、同じ意味のコードを複数言語で書いて同じ結果を期待する。IR の中立性の検証になる
4. **ドッグフーディング** — rivus 自身を rivus で解析する。`attributable_to: rivus` の比率を監視する

---

## 9. 性能目標

叩き台（現実的でないと分かったらレビューで見直す）は 2 軸で置く。

1. **一括解析**: 20 万行規模のプロジェクトを、ノート PC で 5 分以内。CI の PR チェックで回せる上限として置く
2. **問い合わせ**: サマリが揃った後の 1 回のクエリ（`rivus query --site`）の応答時間。
   エージェントが対話的に何度も呼ぶため、数秒以内を目標にする（数値はステップ 1.8 の実測後に決める）。
   `rivus sites`（L0 / L1）はそれより速く、ほぼ即時を目標にする

問い合わせ型の使い方では、重いサマリ算出と軽いクエリ応答を分け、**サマリを保存して再利用できる**形にする必要がある。

- 増分解析は Phase 1〜4 のスコープ外。ただしサマリの関数単位の独立性は壊さない
- メモリに全部載せてよい前提で始める

---

## 10. Phase 1 の細分化

方針: 1 ステップごとに区切り、**各ステップの終わりで `git commit` ＋ ユーザーの確認**を挟む。
Phase 1 では中立 IR を経由せず、`go/ssa` から直接 taint 解析まで作る。`go/ssa` に触れるコードは `internal/frontend/gossa` に隔離する。

| ステップ | 内容 | 完了の目安 |
|---|---|---|
| 1.0 | 足場: `go.mod`（Go 1.27、`golang.org/x/tools` 最新）、CI（GitHub Actions）、LICENSE（Apache-2.0）、`testdata/` ハーネス | `go test ./...` が通る CI |
| 1.1 | ローダ: `go/packages` → `go/ssa`（`InstantiateGenerics` 付き）、`FuncID` 付与、ソース位置とスニペット切り出し | 小さな Go プロジェクトの全関数と位置を列挙できる |
| 1.2 | モデル: YAML 読み込み、シンボル照合（`exact` のみ先に）、組み込みの最小セット | source/sink/sanitizer を YAML で書いて照合できる |
| 1.2b | サイト列挙（`rivus sites`）: L0 の列挙、L1 のリテラル印、サイト ID、JSON 出力 | 型解決つきで sink の呼び出し箇所を列挙でき、grep との差（別名インポートなど）がテストで示せる |
| 1.3 | 手続き内 taint: ラベル、worklist、φ 相当の合流、sanitizer の `Drop` | 1 関数内の `testdata` ケース（検出する / しない）が通る |
| 1.4 | 出力 v0: 構造化 JSON（経路ノードとスニペット） | 1.3 の結果が JSON で読める |
| 1.5 | コールグラフ: `CallGraphBuilder` インタフェース、Go 実装（CHA → VTA）、SCC 縮約、トポロジカル順 | 呼び先から順に関数を列挙できる |
| 1.6 | 関数サマリ: 算出、展開、SCC 内の不動点。ファイル・パッケージ跨ぎ | 複数ファイル・複数パッケージにまたがる `testdata` が通る |
| 1.7 | ヒープ: フィールド感度（k=3）、クロージャ、グローバル、コンテナの扱い | 構造体・クロージャ経由のケースが通る |
| ★ Dart スパイク | **1.5〜1.7 のあいだ、ただし Phase 2 の前に**。2 日のタイムボックス（下記） | 可否の判断 ＋ IR 設計に反映すべき難所のリスト |
| 1.8 | 一方向クエリ: sink から source を引く / source から sink を引く。サイト ID を seed に取れる | seed なしのクエリと、サイト起点のクエリが動く。サマリの保存・再利用と、応答時間の初回計測 |
| 1.9 | 不確実性の記録: terminal kind、`attributable_to`、`dynamic_dispatch` の候補列挙、`match: implementations` | すべての打ち切りが分類されて出る |
| 1.10 | 値依存: 引数由来のガード抽出（taint ソルバ流用）、`conditional_nodes`、provenance、ガード上限 | ブリーフの `render` の例が期待どおりに出る |
| 1.11 | 運用機能: モデルカバレッジレポート、`origin` と `--deterministic-only`、結果のグループ化・安定 ID・経路数上限、`entrypoints` | `rivus models coverage` が動く |
| 1.11b | 自作 SSA 化（`ssabuild`）と differential test: `go/ssa` の NaiveForm 出力から SSA 化し、lifted `go/ssa` と φ の配置・到達定義を突き合わせる。Phase 1 の `testdata` を回帰テストにする | differential test が全 `testdata` で一致し、解析結果が変わらない |
| 1.12 | ドッグフーディング: rivus 自身を解析。`depth_limit` と `attributable_to: rivus` の比率を計測 | 計測値を README に記録 |

SARIF 出力は Phase 1 のスコープ外。JSON が固まってから。

### Dart スパイク（`spike/dart-cfg/`）

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
- **Phase 3:** Rust。最初のタスクは「`stable-mir-json` 等の既存ツールで、実在のクレートがどれくらい通るかの確認」。成果物は「固定した nightly でビルドが通る Rust プロジェクトを解析できるツール」。想定より通らなければ、Rust の優先度を下げる判断もあり得る
- **Phase 4:** Dart。解決済み AST → CFG → IR。まず関数内、それからコールグラフ

---

## 11. レビューで確認したい点

1. **【決定済み】** 4.4 の `dynamic_dispatch` / `unresolved_callee` / `concurrency` は `attributable_to: mixed`（3 値化）。
   Phase 1 のステップ 1.12 の実測後に見直す
2. **【決定済み】** 自作 SSA 化と differential test は **Phase 1 の終盤（ステップ 1.11b）**で導入する（ADR-026）。
   Phase 1 の間は lifted の `go/ssa` を使い、`testdata` が揃ってから差し込む
3. IR の命令数は 27（2.3）。`slice` や `global_load/store` を独立命令にしたが、`index_*` / `field_*` に畳む選択もある
4. `finally` の表現（経路ごとにブロック複製 / 行き先を値で分岐）は、Dart スパイクの結果で決める
5. 性能目標（20 万行 / 5 分 / ノート PC）が現実的か
6. 出力 `Finding.ID` の安定性の定義（関数シグネチャのみ、ボディハッシュ込み、など）
7. `match: implementations` の展開数の既定上限（案: 20）
