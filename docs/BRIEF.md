# rivus — プロジェクトブリーフ

> **この文書について（リポジトリ追加時のメモ）**
> 初期ブリーフの原文を、経緯を残すためにそのまま置いている。
> レビュー（[QUESTIONS.md](QUESTIONS.md)）で内容が変わった箇所には、`> 【改訂注記】` の引用ブロックを添えた。
> 注記のある節は、現在は [ARCHITECTURE.md](ARCHITECTURE.md) と [DECISIONS.md](DECISIONS.md) が優先する。
> 注記が付いているのは 5.3、5.5、6.1、7.1、7.2、7.4 の 6 か所。

---

## 0. このドキュメントについて

これは新規プロジェクト `rivus` の初期ブリーフです。
**いきなり実装を始めないでください。** まずこれを読んで、第10節の指示に従ってください。

---

## 1. 作りたいもの

`rivus` は **Dart / Go / Rust に対応した taint 解析（汚染解析）ツール** です。

ただし「脆弱性を報告する SAST」ではなく、**到達可能性に関する事実を問い合わせるクエリエンジン**
として設計します。最終的な正誤判定は LLM や人間が行う前提で、
rivus は **決定論的に確定できる事実だけを、構造化して提示する** ことに責務を限定します。

名前の `rivus` はラテン語で「小川・水路」。英語 *derive* の語源。

---

## 2. 私について / 進め方の希望

- 私は情報セキュリティのエンジニアで、Web アプリケーションセキュリティが専門
- 言語は Go / Python / PHP。Go は書ける
- **プログラム解析・コンパイラ理論は学習中**。AST、CFG、SSA、データフロー解析、
  手続き間解析、IFDS の概要までは理解している。実装経験はない
- ソフトウェアエンジニアとしての能力は高くないと自認している。
  抽象的な理解をもとに方針を決め、実装は委ねる進め方をしたい

したがって:

- **判断が必要な箇所は、勝手に決めずに私に聞いてください**
- 私が間違った理解を口にしたら、はっきり訂正してください
- 専門用語を使うときは、初出時に一言補足してください
- 大きな単位で一気に実装せず、段階を区切って、都度確認を挟んでください

---

## 3. なぜ作るのか

### 動機

プライベートリポジトリのコードに対して、**ファイルを跨ぐ taint 解析**をしたい。
しかし無償の選択肢がほぼ存在しない。

- CodeQL — プライベートコードには GitHub Code Security（$30/active committer/月）が必要。
  そもそも Dart に非対応
- Opengrep / Semgrep CE — 関数は跨げるが **ファイルを跨げない**（`--taint-intrafile`）。
  ファイル内で足りるなら ast-grep でも相当やれるので、これでは不足
- Joern — ファイル跨ぎができて Apache-2.0 だが、**Dart と Rust に非対応**
- SonarQube — Dart のファイル跨ぎ taint に対応しているが Developer Edition 以上（有償）

対象言語の優先度は **Dart > Rust > Go**。Dart は Flutter クライアント / Flutter Web /
バックエンド Dart の3層すべてで使っている。
**最優先の言語が、最もツールの手当てが薄い**という状況。

### 設計の出発点

完全な精度を目指さない。**LLM に判定させる前提で、判定に必要な材料を決定論的に揃える**
という分担にする。これにより、従来の SAST が苦労してきた path sensitivity や
エイリアス解析の完全性を追わずに済む。

---

## 4. 提供したい機能

### 4.1 source → sink の経路解析（基本）

汚染された値が危険な処理に到達する経路を、**ファイル・パッケージを跨いで**検出する。

### 4.2 片側だけのクエリ（重要）

これが rivus の特徴。source と sink の組ではなく、片方だけを指定できる。

- **ある sink に到達し得る source の列挙**（sink から逆向きに引く）
- **ある source が到達し得る sink の列挙**（source から前向きに引く）

事実上「source だけのルールエンジン」「sink だけのルールエンジン」として使える。
たとえば「`exec.Command` に届く入力は全部で何か」を棚卸しする、といった用途。

### 4.3 source / sink / sanitizer のセットによるルールエンジン

従来型の taint ルール。3つ組を定義して検出する。

### 4.4 カスタムルールセット

ユーザーが自分のフレームワーク・自社ライブラリに合わせて
source / sink / sanitizer / propagator を定義できる。
**組み込みルールもすべて同じ形式で書かれていること**（ドッグフーディング）。

### 4.5 LLM 検証を支援する構造分析（最重要）

rivus の出力は、**LLM が正誤判定するための材料**として設計する。
「検出結果」ではなく「判定に必要な決定論的な事実」を出す。

具体的に出力に含めたいもの:

- 汚染経路の各ノードと、**周辺のソースコードスニペット**（行番号だけでは LLM が判断できない）
- **値依存のサニタイズ**の明示。
  例: `func render(s string, escape bool)` のように、引数の値によって
  sanitize するかが変わる関数。
  「ここは引数に応じて汚染・非汚染が切り替わる」という事実と、
  どの条件でどちらになるかの対応関係を書き出す。**判定はしない**
- **引数の由来（provenance）追跡**。
  値依存ノードについて、呼び出し側で実際に何が渡されているかを式のまま提示する
- **不確実性の所在の列挙**。
  由来追跡や経路探索を打ち切った場合、**なぜ打ち切ったかを必ず分類して記録する**。
  これが rivus の出力のなかで最も価値が高い部分

---

## 5. アーキテクチャ方針（決定済み）

### 5.1 extractor と解析エンジンの分離

言語ごとのフロントエンドを Go で再実装するのは非現実的（型解決・トレイト解決・
手続きマクロなどが壁）。そこで CodeQL や Joern と同じ構造を取る。

```
Dart extractor (Dart 製)  ─┐
Rust extractor (Rust 製)  ─┼→ 中立 IR → Go 製コアエンジン
Go   extractor (Go 製)    ─┘             (SSA → データフロー → taint → 出力)
```

### 5.2 IR は CFG のレベルで切る

**これは重要な決定事項です。**

- AST レベルで切ると、言語ごとの構文差が大きすぎて中立化できない
- SSA レベルで切ると、SSA 化のロジックを3言語ぶん書くことになり無駄

**CFG まで下げれば制御構造が「基本ブロックと分岐」に均され、言語差がほぼ消える。**
`for` / `while` / `for-in` / イテレータのどれであっても CFG では同じ形になる。

したがって:

- **extractor の責務**: ソース → 解決済み AST → **CFG** → 中立 IR
- **コアエンジンの責務**: 中立 IR → SSA 化（φ 挿入） → データフロー解析 → taint → 出力

**SSA 化と φ 挿入は言語非依存のグラフアルゴリズムなので、Go 側に一度だけ実装する。**

判断基準は「**言語固有の知識が要るか**」。CFG 構築までは要る、SSA 化から先は要らない。

### 5.3 IR に必要な情報

taint 解析に必要な情報は、コンパイラ IR よりずっと少ない。コード生成をしないため、
型は「sink を識別できる」「フィールドを区別できる」程度の粒度で足りる。

- 基本ブロックと後続エッジ
- 命令: `call`（解決済み呼び先 ID + 仮想呼び出しの候補リスト）、`binop`、
  `field_load` / `field_store`、`index_load` / `index_store`、`alloc`、`param`、
  `return`、`const`
- 関数の一意な ID（完全修飾名）とシグネチャ
- 型 ID（同一性が判定できれば十分。構造は不要）
- ソース位置（ファイル、オフセット、長さ）

`async` / `await` と例外は、**CFG のエッジとして表現**すれば追加の命令種別は不要。
命令セットは15種類程度に収まるはず。

> 【改訂注記】命令セットは 15 種類程度では収まらず、**27 種**とした
> （`branch` の条件保持、クロージャ、型アサーション、グローバル変数、`slice` などを追加）。
> `go` / `defer` / `await` は新しい命令にせず `call` のフラグで表す。
> イテレータは extractor が脱糖する。IR は **φ なしの CFG ＋ 書き換え可能ローカル変数**とする。
> → [ARCHITECTURE.md 2 節](ARCHITECTURE.md)、[DECISIONS.md ADR-003](DECISIONS.md)

### 5.4 言語別の実装コスト（調査済み）

| 言語 | CFG | コールグラフ | 備考 |
|---|---|---|---|
| Go | `go/ssa` が提供 | `go/callgraph` が提供 | IR への変換のみ。数日 |
| Rust | MIR が既に CFG の形 | monomorphize collector | `rustc_public`（旧 `stable_mir`）経由。nightly 依存。`stable-mir-json` 等の既存ツール流用も可 |
| Dart | **自作が必要** | **自作が必要** | 最も重い。数週間〜数ヶ月 |

Dart の詳細:

- `package:analyzer`（pub.dev 公開）が解決済み AST と element model を提供。
  型解決・名前解決・パッケージ解決はこれに任せられる
- **CFG は公開 API に存在しない。** AST から自作する。
  `RecursiveAstVisitor` があるので数百行で書けるが、
  **`async` / `await` と `try` / `catch` / `finally` のエッジを正しく張るのが難所**
- コールグラフも公開されたものがない。Dart は暗黙のインタフェースと
  Widget のオーバーライドが多く、素朴なクラス階層解析だと候補が爆発する。
  Dart SDK の `pkg/vm` にある type flow analysis（AOT の tree-shaking 用に
  devirtualization を行う）が使える可能性がある。ただし pub.dev 未公開
- 生成コード（`*.g.dart`、`*.freezed.dart`）は **解析対象から除外しないこと**。
  JSON デシリアライズや ORM の生成コードは taint の伝播経路そのもの。
  ただし `build_runner` を先に実行する必要があるので、それは rivus の責務外とし、
  ワークフロー側の前提条件とする

### 5.5 配布形態

**単一バイナリを目指す。ただし Rust は外部ツールチェイン依存を許容する。**

| extractor | 方式 |
|---|---|
| Go | Go 実装なのでコアに同梱 |
| Dart | `dart compile exe` でネイティブ化し `go:embed` で埋め込み（+20MB 程度） |
| Rust | 外部依存。対象プロジェクトの cargo / rustc を使う |

Rust が外部依存になるのは原理的な制約。proc-macro の展開には rustc で
任意のコードを実行する必要があり、サンドボックス内では不可能。
ただし **Rust を解析したい人は cargo を持っている**ので、実質的な問題にならない。

将来 wasm 化できれば Dart も外部依存を外せるが、dart2wasm は WasmGC を要求し、
純 Go の wasm ランタイム wazero は WasmGC 未対応のため、現時点では不可。
**後から置き換えられる設計にはしておく。**

> 【改訂注記】**Dart も Rust と同じ外部ツールチェイン依存とする。**
> `dart compile exe` はクロスコンパイルが限定的で、OS/arch ごとの埋め込みには各環境でのビルドが要るため。
> `go:embed` は動作確認後の最適化として後から追加できるよう、extractor の在り処を解決する層（`Locator`）を挟む。
> また Rust は `rustc_public` が nightly 専用のため、**固定した nightly でビルドが通るプロジェクト**が前提になる
> （「対象プロジェクトの cargo / rustc を使う」はそのままでは成立しない）。
> → [DECISIONS.md ADR-012, ADR-013](DECISIONS.md)

---

## 6. 解析アルゴリズムの方針（決定済み）

### 6.1 関数サマリ方式

汎用の IFDS を実装しようとすると重いので、**関数サマリのボトムアップ伝播**で実装する。
（これは IFDS の tabulation アルゴリズムの非形式版にあたる）

1. コールグラフを構築し、強連結成分に縮約してトポロジカル順に処理
2. 各関数について関数内 taint を解き、サマリを算出
   - 引数 i → 戻り値に伝播するか
   - 引数 i → 引数 j のフィールドに伝播するか（副作用）
   - 引数 i → 関数内の sink に到達するか
   - 戻り値が source 由来か
3. 呼び出し側でサマリを展開、不動点まで反復

> 【改訂注記】「IFDS の tabulation の非形式版」は不正確。bottom-up のサマリ方式は
> Sharir & Pnueli (1981) の **functional approach** で、IFDS の tabulation（呼び出し元からのオンデマンド探索）とは別物。
> 「IFDS の summary edge に相当するものを functional approach で計算する」と表現を統一する。
> bottom-up を選ぶ理由は、4.2 の「seed なしで sink 側から source を引く」クエリが素直に実現できること。
> 不動点の反復は SCC の内部のみ。→ [DECISIONS.md ADR-004](DECISIONS.md)

### 6.2 汚染はラベル付きで持つ

ブール値ではなく **ラベル集合**で持つ。
`map[Value]LabelSet`。sanitizer は特定ラベルだけを落とし、sink は特定ラベルにのみ反応する。

理由: HTML 用にエスケープした値を JS コンテキストの sink に流したら安全ではない、
という種類のミスマッチが実務では頻出する。
**設計の初期から入れておかないと後で入れられない。**

### 6.3 値依存のサニタイズ（path sensitivity）

完全な経路感度は実装しない。以下の優先順で対応する。

1. **手書きモデルによる上書き** — 最優先で実装。投資対効果が桁違いに良い
2. **値依存ノードとして記録し、判定を LLM に委ねる** — rivus の中核的な立場
3. （余裕があれば）**サマリの特殊化** — 呼び出し側で引数がリテラルなら、
   その定数値ごとに別サマリを持つ。軽い実装で相当な割合が解決できる

**述語付きサマリやシンボリック実行 / SMT ソルバは実装しない。**
LLM が後段にいる前提だと、SMT が得意で LLM が苦手な領域
（複雑な算術制約）は Web アプリの injection 検出でほとんど出てこない。

### 6.4 健全性の立場

**soundy** を宣言する。原理的には健全（見逃しなし）を目指すが、
リフレクション、動的なコード生成、並行性の一部は意図的に扱わない。
**そして、扱わなかった箇所は必ず出力に記録する。**

過大近似は維持する。「LLM が後で落としてくれる」からといって見逃してよいわけではない。
**列挙は網羅的に、判定は保留**が原則。

---

## 7. 出力設計（最重要）

### 7.1 二段構え

- **SARIF** — GitHub の Security タブに載せる用。
  プライベートリポジトリでも SARIF アップロード自体は GHAS なしで可能
- **構造化 JSON** — LLM 判定パイプライン用。こちらが本体

SARIF に載せる場合、LLM 向けの付加情報は `properties` バッグに入れる。

> 【改訂注記】**「プライベートリポジトリでも SARIF アップロードは GHAS なしで可能」は誤り。**
> GitHub の code scanning が使えるのは、パブリックリポジトリ、または GitHub Code Security を有効にした
> 組織のリポジトリに限られる（出典: GitHub Docs "Uploading a SARIF file to GitHub"）。
> したがって**構造化 JSON を唯一の一級市民**とし、SARIF は副次出力（公開リポジトリ、ローカルのビューワ、
> 将来の互換性用）に格下げする。SARIF 生成は Phase 1 のスコープ外。
> → [DECISIONS.md ADR-009](DECISIONS.md)

### 7.2 不確実性の分類（terminal kind）

探索を打ち切ったとき、**なぜ打ち切ったかを必ず分類して記録する。**
この列挙型は**最初に確定させたい**。

| kind | 意味 | LLM が判断できるか |
|---|---|---|
| `literal` | リテラル定数に到達 | 不要（確定） |
| `config_read` | 設定ファイル / 環境変数 | ◎ リポジトリ内を読めば分かる |
| `exported_api_param` | 公開 API の引数。由来はリポジトリ外 | ○ |
| `global_var` | パッケージ変数。書き込み箇所は列挙済み | ○ |
| `dynamic_dispatch` | インタフェース。候補 N 件を列挙 | ○ 候補を見せれば絞れる |
| `unmodeled_library_call` | 外部ライブラリでモデル未定義 | ○ |
| `reflection` | reflect / DI。追跡不能 | △ |
| `concurrency` | channel / goroutine 境界 | △ |
| `container_element_imprecision` | コレクション要素を区別していない | △ |
| `depth_limit` | 探索上限に到達 | ✗ rivus 側の都合 |

`depth_limit` の比率は、**上限チューニングの指標として監視する**。
これは対象の性質ではなく rivus の限界なので、減らすべき数字。

分類を固定する理由:

- LLM 側のプロンプトを kind ごとに切り替えられる
- 「どの不確実性がどれだけ誤検知を生んでいるか」を集計でき、
  **rivus のどこに投資すべきかが分かる**。既製 SAST では得られないフィードバックループ

> 【改訂注記】レビューで 4 種を追加し（`ffi_boundary`、`unresolved_callee`、`missing_source`、
> `recursion_approximation`）、各 kind に `attributable_to: target | rivus | mixed` を持たせた。
> 列挙は閉じた集合とし、追加はスキーマのバージョンを上げて行う。
> → [DECISIONS.md ADR-008](DECISIONS.md)、[ARCHITECTURE.md 4.4](ARCHITECTURE.md)

### 7.3 出力例（イメージ）

```json
{
  "sink": { "symbol": "...", "location": "handler.go:88", "snippet": "..." },
  "source": { "symbol": "...", "location": "handler.go:71", "snippet": "..." },
  "path": [ "..." ],
  "conditional_nodes": [
    {
      "function": "render",
      "location": "render.go:12",
      "kind": "value_dependent_sanitization",
      "branch_condition": "escape",
      "derived_from": "parameter 1 (escape bool)",
      "outcomes": {
        "escape == true":  "sanitized by html.EscapeString",
        "escape == false": "tainted (passthrough)"
      },
      "call_site_argument": "renderUserBio(bio, cfg.AllowHTML)",
      "resolved": false,
      "function_source": "..."
    }
  ],
  "provenance": {
    "argument": "escape (param 1)",
    "resolved":   [ { "value": false, "call_site": "handler.go:40", "certainty": "literal" } ],
    "unresolved": [ { "terminal_kind": "config_read", "chain": ["cfg.AllowHTML", "loadConfig()", "os.ReadFile(path)"], "snippet": "..." } ]
  },
  "confidence": "requires_review"
}
```

`resolved` と `unresolved` を分けることで、LLM 側のプロンプトを
「確定した事実」と「調査が必要な項目」で自然に書き分けられる。

### 7.4 LLM の判定を還流させる

LLM が「このケースは常に sanitize される」と判定したら、
それを**モデルとして設定ファイルに書き戻す**仕組みを用意する。
次回以降は rivus の段階で解決され、LLM 呼び出しも減る。

キャッシュのキーは **関数のシグネチャ + ボディのハッシュ**。
コードが変わったら再判定される。

> 【改訂注記】LLM の出力を**モデル**（関数 1 つの振る舞い。再利用する）と**判定**（検出結果 1 件の評価。再利用しない）に分ける。
> キャッシュキーは「関数のボディのハッシュ ＋ 直接呼び出している関数のモデル ID の集合」とし、
> モデルには `origin: builtin | human | llm` を必須にする。`--deterministic-only` で `origin: llm` を無視して走れる。
> → [DECISIONS.md ADR-010](DECISIONS.md)

---

## 8. 非目標（やらないこと）

明示しておきます。これらを実装しようとしないでください。

- **判定を下すこと。** rivus は事実を出す。安全か危険かは決めない
- 完全な経路感度、シンボリック実行、SMT ソルバ連携
- 完全なポインタ解析 / エイリアス解析（必要箇所の列挙に留める）
- 暗黙的フロー（制御依存）の追跡
- Dart / Rust のフロントエンド（パーサ・型解決器）の自作
- 対象言語のビルド・コード生成の面倒を見ること
  （`go generate` や `build_runner` は前提条件としてワークフロー側に切り出す）
- IDE 統合、Web UI、SaaS 化（当面）

---

## 9. 技術スタック

- **コアエンジン**: Go 1.27。依存は標準ライブラリ + `golang.org/x/tools` を基本とする
- `golang.org/x/tools` は最新版を使う。SSA / 型解決は新しい言語機能への対応が
  リリースごとに追加されるため、古いと Go 1.27 のコードで失敗することがある
- **Dart extractor**: Dart。`package:analyzer`
- **Rust extractor**: Rust。`rustc_public`（旧 `stable_mir`）または既存の MIR ダンプツール
- ライセンスは Apache-2.0 を想定（要相談）
- CI は GitHub Actions
- `rivus` 自身を `rivus` で解析できるようにする（ドッグフーディング）

---

## 10. 開発フェーズ

**この順序には理由があります。入れ替えないでください。**

### Phase 1: Go のみで完結させる

中立 IR を経由せず、`go/ssa` から直接 taint 解析まで作る。
ここで理論と設計を固める。関数内 → 関数サマリ → ファイル跨ぎ、の順。

### Phase 2: 中立 IR を切り出す

Phase 1 の実装を、IR を挟む形にリファクタする。Go extractor が最初の実装例になる。
**動く実装を1つ持ってから IR を設計するほうが、確実に良いものになる。**

### Phase 3: Rust を足す

MIR ダンプを流用。**IR の中立性を検証する2つ目の例**として機能する。

### Phase 4: Dart を作る

一番重いので最後。Phase 1〜3 で IR が固まっているので、
やることは「解決済み AST → CFG → IR」に限定される。
まず関数内解析を動かし、それからコールグラフに着手する。

Dart を最優先したいが Phase 4 に置くのは、
**IR の中立性は軽い言語で検証すべき**だから。
Dart の CFG ビルダーを書きながら IR も設計する、という二重の難しさは避ける。

---

## 11. 最初にやってほしいこと

**実装はまだ始めないでください。** この順で進めてください。

1. このブリーフを読んで、**不明確な点・矛盾している点・判断が必要な点を質問してください。**
   特に、私が技術的に見落としていそうな前提があれば指摘してほしい

2. 質問への回答を踏まえて、`docs/ARCHITECTURE.md` を書いてください。内容:
   - 全体構成図
   - 中立 IR の命令セットの初版（第5.3節をもとに具体化）
   - コアエンジンのパッケージ構成案
   - 主要なデータ構造の定義案
   - Phase 1 を更に細かいステップに分解したもの

3. あわせて `docs/DECISIONS.md` を作り、
   **このブリーフで既に決まっていること**と**その理由**を ADR 形式で記録してください。
   後から経緯を追えるようにしたい

4. `README.md` の骨子

5. そこまでできたら一度止めて、私のレビューを待ってください

### 補足

- `docs/GLOSSARY.md` として用語集を別途持ち込む予定です。
  語彙はそちらに揃えてください（なければ聞いてください）
- 各ステップの終わりに `git commit` してください。コミットメッセージは日本語で構いません
- **私が理解できていなさそうな箇所があれば、先に進むより質問を投げてください**
