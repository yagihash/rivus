# ADR-032: 不確実性の記録の範囲と集計（ステップ 1.9）

- 状態: 有効
- 関連: ARCHITECTURE 4.4
- 状態の詳細: 訂正あり; 訂正・追記あり(書き方を保つ)
- 要旨: 全体一覧と経路区間に分ける。x/tools の件数を訂正

- **状況:** ADR-007 は「扱わなかった箇所は必ず出力に記録する」とした。ステップ 1.3〜1.8 までは、呼び出し関連（モデルなし、解決できない、インタフェース越し）しか記録していなかった。
  別名を介した書き込み、goroutine と channel、パッケージ変数、reflection は、何も報告せずに通り過ぎていた。
- **決定:** 記録の置き場所を 2 つに分け、kind を次のように記録する。
  - **プログラム全体の一覧（`uncertainties`）:** その場所そのものが分かっていないもの。`unmodeled_library_call`（`rivus`）、`dynamic_dispatch`（`mixed`、候補つき）、`unresolved_callee`（`mixed`）、`reflection`（`target`）、`ffi_boundary`（`target`）、`depth_limit`（`rivus`、関数の予算超過、時間切れ、展開の上限超過）、`recursion_approximation`（`rivus`）、`missing_source`（`target`、ロードエラー）
  - **検出結果の経路の区間（`findings[].uncertainties`、各ステップの `notes`）:** その区間を通った検出結果にだけ付く。`container_element_imprecision`（`rivus`。スライス・配列・map・channel の要素の出入り）、`concurrency`（`mixed`。channel、`select`、`go` 文）、`global_var`（`target`。パッケージ変数の読み出し。直接の書き込み側を候補として列挙する）、ほか `dynamic_dispatch` と `unmodeled_library_call`
    「結論に影響した場合だけ」記録するというルール（ADR-014）を、経路上にあるかどうかで実現した。1 つの区間が複数の不確実性を持てる
  - **集計（`summary`）:** プログラム全体の一覧を kind と `attributable_to` ごとに数え、`rivus` の比率と `depth_limit` の比率、各 kind が付いた検出結果の数を出す。ADR-008 のフィードバックループ
- **未実装・未検証:**
  - `exported_api_param`、`config_read`、`literal` は、由来追跡（ステップ 1.10）の打ち切りの種類で、まだ出ない
  - cgo（`C` パッケージ、`_Cfunc_`）の検出は実装したが、cgo を使ったテストは書いていない。アセンブリやリンクネームで本体のない関数は、`unmodeled_library_call` になる
  - `container_element_imprecision` は、要素への出入りに必ず付くので、スライスを使う経路ではほぼ毎回付く。見た目のノイズになるか、実測と運用で見る
- **実測（プログラム全体の一覧の内訳。`dynamic_dispatch` の帰属を変えた後、完全に収束した実行）:**

  | 対象 | 件数 | `rivus` | `mixed` | `target` | `depth_limit` の比率 |
  |---|---|---|---|---|---|
  | ghmint | 288 | 87% | 13% | 0% | 0% |
  | rivus 自身 | 390 | 93% | 7% | 0% | 0% |
  | x/tools（15.6 万行、96 秒、2 パスで収束） | 12,350 | 78% | 19% | 3% | 0.02% |

  `rivus` の大半は `unmodeled_library_call` と、実装が分からない `dynamic_dispatch` で、どちらもモデルを書けば減る。
  上位は `fmt.Fprintf`（505）、`(*go/types.object).Type`（232）、`strings.HasPrefix`（164）、`(go/types.Type).Underlying`（91）、`(context.Context).Err`（37）など。
  `summary.top_symbols` が、この一覧を出す（ステップ 1.11 のモデルカバレッジレポートの土台）
- **訂正:** この ADR の初版には、x/tools の件数を 5,444（`rivus` 67%、`mixed` 30%）と書いたが、**これは 255 秒の時間制限で打ち切られた、途中までの実行の値だった**（計測用に広い sink ルールを足したため。下の「性能の注意」参照）。上の表が正しい
- **`dynamic_dispatch` の帰属（ステップ 1.9 の相談で決定）:** 呼び出し先の候補が 0 件のとき（標準ライブラリなど、実装が読み込んだコードの外にある）は `rivus`（モデルを書くと減る）、1 件以上のときは `mixed`（VTA の精度向上で減らせる余地がある）。
  種類は変えず、`attributable_to` を呼び出しごとに決める。x/tools の候補数の分布は 0 件 616 / 1 件 217 / 2〜5 件 623 / 6〜20 件 425 / 21 件以上 213（`summary.dispatch_candidates`）。
  候補 1 件の記録を残す是非と、`mixed` の細かい分け方は、ステップ 1.12 で、より多くのコードの分布を見て決める
- **性能の注意:** `(error).Error`、`(io.Writer).Write`、`(fmt.Stringer).String` のように、**至る所で呼ばれるメソッドに sink ルールを書くと、解析が大幅に遅くなる**。
  x/tools で、こうしたインタフェースメソッドを 6 つ sink にしたところ、96 秒の解析が 255 秒でも終わらなかった（展開なしでも同じ。原因は展開ではなく、呼び出しの数）。
  全ての呼び出しが sink になり、各関数のサマリに「引数が sink に届く」記録が大量に残るため。ルールを書くときは、狭く書く。サマリの `SinkReach` を粗くする対策（出力要素の widening と同様）は未実装
- **帰結:** 経路の区間に付く不確実性は、出力の `notes` を見れば、LLM が「この経路のどこが怪しいか」を種類別に読める。
  `mixed` の割り当て（ADR-008）を見直す材料として、`dynamic_dispatch` の候補数の分布などの追加の計測が必要（レビュー待ち）

