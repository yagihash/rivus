# ブリーフへの質問・指摘

> **歴史的な文書。** 最初のブリーフ（[BRIEF.md](BRIEF.md)）への質問と指摘の記録で、各項目の「回答」欄は空のまま。回答に相当する判断は [DECISIONS.md](../DECISIONS.md) の ADR にある。今の設計とは異なる所がある（例: Q3 の推奨「自モジュールのみ解析」は、今の既定と逆で、依存のソースも既定で読む）。中身は書き換えていない。
> 今の正本は [PRINCIPLES.md](../PRINCIPLES.md)、[ARCHITECTURE.md](../ARCHITECTURE.md)、[DECISIONS.md](../DECISIONS.md)。

ARCHITECTURE.md を書く前に確認したい事項。回答は各項目の「回答」欄に書く。
「推奨どおり」とだけ書いてもよい。

---

## A. 訂正したい点

### A1. SARIF アップロードと GHAS

ブリーフは「プライベートリポジトリでも SARIF アップロード自体は GHAS なしで可能」としている。
GLOSSARY.md にも同じ記述がある。

一方、GitHub の code scanning はプライベートリポジトリでは GitHub Code Security が必要で、
SARIF アップロード API も code scanning の機能なので同じ制約がかかるはず、というのが私の理解。
どちらも一次情報では確認していない。

- 実際に試すか公式ドキュメントで確認してほしい
- 確認できるまでは、SARIF を主役にしない前提で設計する（JSON が本体という方針とは矛盾しない）
- 制約があった場合、SARIF は「公開リポジトリ向け」「VS Code の SARIF Viewer で見る用」に格下げする

**回答:**

### A2. 「IFDS の非形式版」という表現

IFDS の tabulation は呼び出し元からのオンデマンド探索。
ブリーフの bottom-up 関数サマリ方式は「関数的アプローチ（functional approach）」と呼ばれるもの。
GLOSSARY の「IFDS の summary edge に相当する」という言い方なら問題ない。方針の変更は不要。
文献を探すときは「functional approach / summary-based analysis」で探すと見つかりやすい。

---

## B. 見落としていそうな点・矛盾

### B1. Phase 1 → 2 の SSA の扱い（→ Q1）

`go/ssa` は CFG ではなく、すでに SSA 化された出力（φ入り）を返す。
Rust の MIR は SSA ではなく、書き換え可能なローカル変数を使う。
「extractor は CFG まで、SSA 化はコア側」とすると、Go extractor は SSA を一度ほどく必要が出る。

ただし `ssa.NaiveForm` を指定すると、φなしで `Alloc` / `Store` / `Load` が残る形で出力される。
これを使えば、Go も Rust も「CFG ＋ 書き換え可能ローカル変数」に揃えられる。

### B2. 命令セットは「15種類程度」より増えそう

第5.3節の一覧に入っていないもの:

- 分岐条件そのものを保持する終端命令（`branch`）
- クロージャ生成
- `go` / `defer`
- 型アサーション
- 単純代入、型変換、単項演算
- ポインタの参照外し、アドレス取得
- イテレータ操作

特に分岐条件の保持は必須。第7節の「値依存サニタイズ」は
`escape == true` のような条件を、どの taint 経路が通ったかとして記録するため。
要約の中に「経路に条件ガードを付けて記録する」軽い仕組みが必要。完全な経路感度ではない。

### B3. 第7.4節のキャッシュキー

キーが「シグネチャ＋ボディのハッシュ」だと、呼び出し先が変わっても判定が古いまま残る。

- 呼び出し先を含めた推移的なハッシュにする、または判定の適用範囲を関数単体に限る
- LLM 判定をモデルに書き戻すと「決定論的な事実だけを出す」原則と少しぶつかる。
  書き戻したモデルには `origin: llm | human`、判定日、根拠を付け、
  出力にも「LLM 由来モデルで抑制した」と明示する

### B4. 配布面のリスク

- **Dart**: `dart compile exe` はクロスコンパイルが限定的。OS/arch ごとのバイナリを `go:embed` するには、
  各環境でのビルドが必要。CI で解けるが、単一バイナリの前提がやや重くなる
- **Rust**: `rustc_public` は nightly 専用。extractor は rustc ドライバ（rustc をライブラリとして呼ぶプログラム）になり、
  特定の nightly 版に固定される。解析対象は stable のことが多いので、
  「対象の cargo/rustc を使う」はそのままでは成立しない場合がある。
  解決策は「extractor 用に固定 nightly を別に用意し、対象は `RUSTC_BOOTSTRAP` などで通す」あたり。
  Phase 3 で詰めれば足りる

### B5. terminal kind に足りないもの（→ Q7）

| 追加案 | 意味 |
|---|---|
| `ffi_boundary` | cgo、`dart:ffi`、Rust の `extern` / unsafe |
| `unresolved_callee` | 関数値・クロージャ経由の呼び出しで呼び出し先が確定しない |
| `missing_source` | 生成コード未生成、パッケージ解決失敗などで見えない |
| `recursion_approximation` | SCC（相互再帰する関数の集まり）を近似で閉じた |

これらを `depth_limit` に押し込むと「rivus の都合」と「対象の性質」が混ざる。

---

## C. 判断してほしい点

| # | 質問 | 推奨 | 回答 |
|---|---|---|---|
| Q1 | IR の SSA の扱い。案 A: φなし CFG ＋ ローカル変数（Go は NaiveForm）。案 B: IR に `phi` を任意で許し、コアの SSA 化は冪等にする | 案 A | |
| Q2 | ヒープの扱い。フィールド感度ありで、アクセスパスの深さ上限 k をいくつにするか。コンテナ要素は区別せず `container_element_imprecision` に落とす。エイリアス解析は作らない | k=3 | |
| Q3 | 依存パッケージ（stdlib・第三者ライブラリ）のソースを解析するか。解析すれば精度は上がるが遅くなり、しなければ手書きモデルが大量に必要 | 自モジュールのみ解析。依存との境界はモデルで扱い、無ければ `unmodeled_library_call`。選んだ依存だけ解析対象に含める設定を用意 | |
| Q4 | ルール（source / sink / sanitizer / propagator）の記述形式 | YAML。シンボルは完全修飾名（レシーバ・ジェネリクス込み） | |
| Q5 | extractor とコアの通信形式 | サブプロセス起動＋バージョン付き JSON Lines。重くなったら protobuf に移行 | |
| Q6 | Dart の実現性スパイク（`async`/`await` と `try`/`finally` の CFG が `package:analyzer` から素直に組めるかの確認、1〜2日）を Phase 1 の合間に挟むか。Phase の順序は変えない | 挟む | |
| Q7 | B5 の kind 追加案を採用するか。列挙は閉じた集合（追加はスキーマバージョン上げ）でよいか | 採用。集合は閉じる | |
| Q8 | GLOSSARY.md の扱い | 受領済み。`docs/GLOSSARY.md` として置いてよいか | |

---

## D. こちらで決めて進める点（異論があれば指摘してほしい）

- ライセンスは Apache-2.0
- Go のコールグラフは VTA（変数型解析で候補を絞る手法）を既定にし、CHA（クラス階層だけで候補を出す粗い手法）を比較用に残す
- Phase 1 では `go/ssa` に触れるコードを 1 パッケージ（例: `internal/frontend/gossa`）に隔離し、Phase 2 の IR 切り出しを楽にする
