# ADR の一覧

[DECISIONS.md](DECISIONS.md)(ADR-001〜092)の、番号、題、今の状態、関連する文書の一覧。ADR は決めた時点の記録で、書き換えない。**今どうなっているか**は、ここと、[ARCHITECTURE.md](ARCHITECTURE.md)、[PRINCIPLES.md](PRINCIPLES.md) で見る。

- 確認: 2026-11-22、9c26a39。状態は、ADR の本文と、後の ADR・実装を読んで付けた。古くなる前提で読むこと。
- 状態の意味:
  - **有効**: 今も、決めたとおり。
  - **一部覆った**: 後の ADR、同じ ADR の訂正や追記、または実装が、一部を置き換えた。覆した先を「状態の詳細」に書く。
  - **覆った**: 全体が置き換わったもの。今のところ 0 件。
- DECISIONS.md の冒頭は「覆った ADR は『置換』にする」と書くが、実際には 1 本も付いていない。この一覧が、その代わりに覆した先を示す。
- DECISIONS.md の冒頭の表は 037 で止まっている。038 以降は、この一覧で探す。
- DECISIONS.md の中では、057 が 056 より前に置かれている(番号順でない)。ADR-027 と 030 の追記は、ADR-031 の節の中にある。
- 要旨が「-」のものは、一覧を作った時点で要旨を付けていない。
- 関連する文書の `ARCHITECTURE N` は、ARCHITECTURE.md の節番号。再構成で変わりうる。

| # | 題 | 状態 | 状態の詳細(覆した先、注意) | 関連する文書 | 要旨 |
|---|---|---|---|---|---|
| 001 | 事実を出し、判定はしない | 有効 | 非目標の「ビルドの面倒を見ない」は Rust で例外: ADR-042/064; 覆した先・注意: ビルド非目標の例外 | ARCHITECTURE, BRIEF | 事実を構造化して出す。判定はしない。過大近似は維持、列挙は網羅的に |
| 002 | extractor と解析エンジンの分離 | 有効 |  | ARCHITECTURE 2 | 言語別 extractor が中立 IR を出し、Go 製コアが解析。型/名前/パッケージ解決は各言語の道具 |
| 003 | IR は φ なし CFG + 書き換え可能ローカル | 有効 | 命令数は 039/040 で具体化、IR 増分は ADR-043 追記 | ARCHITECTURE 2.1-2.3 | CFG の高さで切る。SSA 化はコア一本。Go は NaiveForm、lifted go/ssa は oracle |
| 004 | bottom-up の関数サマリ | 有効 |  | ARCHITECTURE 5.2 | SCC 縮約・トポ順の functional approach。seed なしクエリのために選ぶ |
| 005 | 汚染はラベル集合 | 有効 |  | ARCHITECTURE 4.2 | ラベル集合。sanitizer は特定ラベルだけ落とす |
| 006 | 値依存サニタイズと条件ガード | 有効 | 出どころ追跡は 035、訂正は 078; 覆した先・注意: 078 | ARCHITECTURE 5.6 | 引数由来のガードだけ記録(約 4 個まで)、評価はしない。SMT/シンボリック実行はしない |
| 007 | soundy を宣言、扱わなかった箇所は記録 | 有効 |  | ARCHITECTURE 4.4 | リフレクション等は意図的に扱わず、必ず出力に残す |
| 008 | 不確実性は閉じた kind + attributable_to | 一部覆った | 「追加は版を上げる」は運用で逆。ADR-043 追記 CORE-1、ARCH 4.4; 訂正・追記あり(書き方を保つ)。追記あり。覆した先・注意: 版運用 | ARCHITECTURE 4.4 | 10 種 + 4 種、`target`/`rivus`/`mixed`。追記: ADR-059 で `assumed_call` |
| 009 | 出力は JSON が本体、SARIF は副次 | 有効 | SARIF は未実装。Phase 1 範囲外と明記; 訂正・追記あり(書き方を保つ)(ブリーフの誤り) | README | SARIF は private repo で GHAS 無しには使えない(訂正)。JSON が一級 |
| 010 | LLM の出力は「モデル」と「判定」 | 有効 | 実装: 034 | ARCHITECTURE 5.4, MODELING | `origin`(builtin, human, llm)、`--deterministic-only`、suppressions |
| 011 | フェーズ順序と Dart スパイク | 有効 | 完了: 037 | BRIEF | Go→IR→Rust→Dart。Phase 1 の間に Dart 2 日スパイク |
| 012 | Dart/Rust は外部ツールチェイン依存 | 有効 | Rust の既定実行は 064 で隔離 | extractor README | 配布の負担を避け、Dart も外部 SDK 前提 |
| 013 | Rust は固定 nightly が前提 | 有効 | 固定値: 042 の `nightly-2026-11-15` | rust-isolation | 対象も固定 nightly でビルドできる必要。通る範囲を先に確認 |
| 014 | フィールド感度(k=3)、エイリアス解析なし | 有効 | ただし 072/084/090 は別名風の扱いを足す。要注意 | ARCHITECTURE 5.3 | フィールド名ベース、オブジェクト感度なし、完全エイリアス解析は作らない |
| 015 | 依存の境界はモデルで扱う | 一部覆った | 依存のソースを読む `-deps` が既定オン: ADR-043 追記。範囲: 044→063; 覆した先・注意: 043 追記 / 063 | ARCHITECTURE 5.5, README | 解析は自モジュールのみ、依存は手書きモデル。`models coverage` |
| 016 | ルールは宣言的 YAML、クエリ言語は作らない | 有効 | 書式の拡張: 034/071/074/080/082/083 | MODELING | `exact`/`implementations` を区別。名前空間 + 版 |
| 017 | extractor はサブプロセス + JSON Lines | 一部覆った | Go は同プロセスで IR を渡す: 039/040 | ARCHITECTURE 2.6 | バージョン付き JSON Lines、IR ダンプは一級 |
| 018 | コールグラフ構築は差し替え可能 | 有効 | 訂正・追記あり(書き方を保つ) | ARCHITECTURE 3 | CHA は VTA の入力として必須(訂正)。言語固有手法はコアに漏らさない |
| 019 | ライセンス Apache-2.0 | 有効 |  | - | - |
| 020 | Go 1.27 と x/tools 最新 | 有効 |  | - | go/ssa は隔離(`internal/frontend/gossa`) |
| 021 | 増分解析は当面スコープ外 | 有効 |  | ARCHITECTURE 9 | サマリの関数単位独立は壊さない |
| 022 | エントリポイントを明示 | 有効 | 拡張: 052/073/079 | ARCHITECTURE 6 | 言語ごとの既定 + `entrypoints`(実体は `kind: param`) |
| 023 | 出力量の制御と安定 ID | 一部覆った | 経路数上限 3 + 超過は件数、は未実装。実装は常に 1 本; 訂正・追記あり(書き方を保つ)あり(単位をサイトに)。覆した先・注意: 経路数が未実装だと明記 | ARCHITECTURE 7 | 単位は (source サイト, sink サイト)。経路数上限 |
| 024 | テスト戦略 | 有効 | zoo 等で拡張: DESIGN-zoo、ADR-043/046 | DESIGN-zoo, bench/README | 検出するケースと「検出されない」ケースを両方 assert |
| 025 | エージェントの道具としての使われ方、L0/L1/L2 | 有効 |  | ARCHITECTURE 1 | `rivus sites`/`query --site`。出力詳細度を選べる |
| 026 | 自作 SSA と differential test は Phase 1 終盤 | 有効 | 完了: 036 | ARCHITECTURE 2.7 | 案 R を採用 |
| 027 | 性能目標は仮説、実測で確定 | 有効 | 追記で実測; 訂正・追記あり(書き方を保つ)。追記(初回計測)。031 の節内にある | ARCHITECTURE 9 | 20 万行/5 分は仮説。ロードエラーでも続行し `missing_source` |
| 028 | サイト/検出結果 ID は指紋の入れ物 | 有効 |  | ARCHITECTURE 4.6 | `structural@1` から公開 ID。複数キーで対応付け |
| 029 | ブックマークと `rivus resolve` | 有効 |  | ARCHITECTURE 4.7 | rivus はステートレス。`absent_now` は事実で判定ではない |
| 030 | `implementations` の上限と sanitizer 展開禁止 | 有効 | 実装と実測は追記; 訂正・追記あり(書き方を保つ)。追記は 031 の節内にある(位置注意) | ARCHITECTURE 4.4 | 上限超過は中止して縮退、`expansion_limit`。sanitizer 展開は既定禁止 |
| 031 | Phase 1 の解析エンジン(M2 時点) | 一部覆った | 「go/ssa 直接依存」は 040、「モデルがあれば本体は読まない」は 075/086、数値は CORE-5; 覆した先・注意: 040, 075 | ARCHITECTURE 5 | 記号的サマリ、セル、widening、予算と timeout |
| 032 | 不確実性の記録範囲と集計 | 有効 | 訂正あり; 訂正・追記あり(書き方を保つ) | ARCHITECTURE 4.4 | 全体一覧と経路区間に分ける。x/tools の件数を訂正 |
| 033 | `rivus resolve` の実装 | 有効 |  | ARCHITECTURE 4.7 | - |
| 034 | モデルの運用機能 | 有効 |  | MODELING, README | `models coverage`、`pure`、`origin`、`--deterministic-only`。複数経路は見送り |
| 035 | 値依存サニタイズと由来追跡 | 有効 | 訂正 078、`when_arg_type` 追記; 訂正・追記あり(書き方を保つ)。追記あり | ARCHITECTURE 5.6 | 由来追跡、定数/設定読みで打ち切り |
| 036 | 自作 SSA 化と differential test | 有効 |  | ARCHITECTURE 2.7 | - |
| 037 | Dart スパイクの結果とドッグフーディング | 有効 |  | ARCHITECTURE 10 | 実現可能。例外辺とブロック切り、`finally` は複製 |
| 038 | 出力にリビジョン(`target`) | 一部覆った | `dirty` を廃止、git を実行しない; 訂正・追記あり(書き方を保つ)。追記あり。覆した先・注意: 冒頭に「`dirty` は廃止」 | ARCHITECTURE 4.5 | `target: {revision}`。追記で `.git/config` の実行を避ける |
| 039 | Phase 2 の IR(案) | 一部覆った | 「コアがテキストを切る」を 040 が修正; 覆した先・注意: 040(状態が「案」のまま) | ARCHITECTURE 2.7 | 命令数 26 のまま、型は TypeClass のみ |
| 040 | Phase 2 の結果: 中立 IR の上に移した | 有効 |  | ARCHITECTURE 2.7 | 旧経路を除く。コアは go/ssa を知らない |
| 041 | Dart extractor の最初の版 | 有効 | 追記多数: gRPC、型階層、ベンチ; 訂正・追記あり(書き方を保つ)。追記あり | extractor/dart | `package:analyzer` から IR |
| 042 | Rust extractor の最初の版 | 一部覆った | ホスト実行は 064 で既定が隔離に、テスト読みは 062; 訂正・追記あり(書き方を保つ)。追記あり。覆した先・注意: 064/062 | extractor/rust, rust-isolation | `rustc_public` で MIR、固定 nightly、`cargo check` |
| 043 | 精度のベンチマーク(go-test-bench) | 一部覆った | 追記「組み込みは標準まで」→ 044 → 063; 訂正・追記あり(書き方を保つ)。追記の運用。覆した先・注意: 044/063 | bench/README | 正解付きベンチ。追記が長大(依存読み、決定性、並列化、性能) |
| 044 | 組み込みの範囲(有名サードパーティ) | 一部覆った | 063。追記で gin を外した; 覆した先・注意: 063。本文内矛盾あり | README, GCP-MODELS | 有名サードパーティも組み込みに |
| 045 | 関数値の呼び出しに sink/source/モデルを当てる | 有効 | 拡張: 050/068/070 | DESIGN-callouts | 候補は IR 上で一元決定(`valuecalls`) |
| 046 | 独自ベンチ(go-curated、go-synth) | 有効 |  | bench/README | 精度回帰と性能スケールを分ける |
| 047 | パラメータの関数の呼び出しを記号的な出力に(コールアウト) | 有効 | 状態: 棚上げ→精度のため再開。段階 2 は 071 が実装; 訂正・追記あり(書き方を保つ)。状態注記 | DESIGN-callouts | 状態の注記が本文中に 2 つある |
| 048 | `kind: param` の source を呼び元へ書き戻さない | 有効 |  | - | 「種」だけ書き戻さない |
| 049 | グローバル経由の経路に書き手の段を入れる | 有効 | 証拠は 069/087 | DESIGN-shared-state | - |
| 050 | 上限で解決しなかった関数値は「知らない」と言わない | 一部覆った | 拡張: 070。追記(2026-11-22): 候補の上限を 8 から 32 にした | - | `candidate_limit` を区別。32 を超える形は黙った見逃し(#77) |
| 051 | `Reachable` が捕獲ありクロージャも辿る | 有効 | Rust は 058 | - | - |
| 052 | `not_from_entry` に説明を添える | 有効 | 073 が拡張 | - | - |
| 053 | 捕獲値が呼ぶ所で分からないとき黙って埋めない | 有効 | 059 が作った場所で当てる経路を足す。`capture_unbound` は残る | DESIGN-capture | `depth_limit`/`capture_unbound` |
| 054 | 作業量の上限と実時間の上限を区別 | 有効 |  | README | `budget:` / `func_timeout:` / `run_timeout:`、`load_dependent` |
| 055 | 値の型は値のデータを運ばない(reflect) | 有効 | 採用; 訂正・追記あり(書き方を保つ)。仮説履歴 | - | 仮説の履歴を本文に残す(収束の原因探索) |
| 057 | join した要約で全体が一部を吸収(`absorbWhole`) | 有効 | 位置注意 | - | ファイル上は 056 より前に置かれている(番号順でない) |
| 056 | `globals:` の `holds: nothing` | 有効 | 採用、ユーザー承認 | MODELING に記載なし | 流れを止めたことを `global_holds_nothing` で記録 |
| 058 | Rust の `Reachable`(再 export trait、環境 alloc) | 有効 | 051 の続き | - | - |
| 059 | 見えない呼び先のクロージャを作った場所で当てる | 有効 | 採用 | DESIGN-capture | `assumed_call`。008 に追記あり |
| 060 | ポインタレシーバ越しのインターフェース呼び出し | 有効 | 090 が続き | - | - |
| 061 | 診断出力(`-pass-stats` 等)は結果を変えない | 有効 |  | - | - |
| 062 | ビルドが除くものは既定で読まない(3 言語) | 有効 |  | README | 読まなかった数を `tests_skipped` に出す |
| 063 | 特定のフレームワークは `examples/rules` | 一部覆った | 2026-11-22 の追記で、線引きは固定でなくなった(組み込みに入れるかは、その都度判断。保留は決着); 覆した先・注意: 044 に逆リンク | README, examples/rules | 標準 + 代わりのないパッケージ + SQL ドライバ |
| 064 | 信頼しない Rust の隔離抽出 | 一部覆った | 同 ADR 内の訂正で「取得の段階は安全」を撤回; 訂正・追記あり(書き方を保つ)(ユーザー発見)。残課題の更新が要 | rust-isolation, DESIGN-fetch-network | Docker、2 段階、ネットワークなし抽出 |
| 065 | sink のルールだけの関数をモデル扱いしない | 有効 |  | - | - |
| 066 | 組み込み `pure` から入力を出力に伝える関数を外す | 有効 |  | MODELING §1 | 基準を決め、実行で確かめる |
| 067 | モデルの流れに「近似」(`approximate`) | 有効 |  | MODELING §2 | `model_approximation` が通った finding に付く |
| 068 | インターフェース越しの呼び出しで候補の sink を当てる | 有効 | 045 の延長 | MODELING §4 | - |
| 069 | グローバル経由の finding に `shared_state` | 有効 | 087 が拡張 | DESIGN-shared-state | 出力の追加のみ |
| 070 | 関数値の候補が一部しか追えないことを言う | 有効 |  | - | `unresolved_callee`(`partial`) |
| 071 | モデルの `calls:` | 有効 | 080 が `method:` を追加 | MODELING §3 | 段階 2 |
| 072 | Rust の可変捕獲を指す先と同一視 | 有効 |  | - | - |
| 073 | フレームワークが呼ぶメソッドを入口に足す | 有効 |  | README | `from_framework_callback` |
| 074 | sink が引数のフィールドを言える(`parts`) | 有効 | 084 が黙った見逃しを直す | MODELING §4, DESIGN-field-sinks | `field_imprecision` |
| 075 | モデルありの interface で自前実装の本体も読む | 一部覆った | sanitizer を対象外にした点を 086 が変更; 訂正・追記あり(書き方を保つ)。追記あり。覆した先・注意: 086 | - | 和を取る。追記で `dependency_body_unread` |
| 076 | 型エラーで SSA が作られなかったパッケージを言う | 有効 |  | README | `summary.unread` |
| 077 | Google Cloud を組み込む(メンテナの例外) | 有効 | 例外は 044/063 の基準の外と明記 | GCP-MODELS | Spanner / BigQuery の SQL |
| 078 | ガード出どころの言い切りは定数/単純コピーのみ | 有効 | F4/F5 訂正が本文内; 訂正・追記あり(書き方を保つ) | ARCHITECTURE 5.6 | 誤った痕跡の中で最も重い、と明記 |
| 079 | Rust `Fn*::call` の候補を出どころで絞る | 有効 | 訂正が本文内。085 が拡張; 訂正・追記あり(書き方を保つ) | DESIGN-generics-dispatch | `closed_world_entry` |
| 080 | `calls:` に `method:` | 有効 |  | MODELING §3 | - |
| 081 | Rust のモデル位置は dynamic でも `arg0` が受け手(`self_first`) | 有効 |  | DESIGN-generics-dispatch | `Box<dyn Fn>` は入れない |
| 082 | 書き込み器モデルは `.content` に置く | 有効 | 訂正が本文内。090 が続き; 訂正・追記あり(書き方を保つ) | MODELING に記載なし | - |
| 083 | Go の型の別名はルール側で同義扱い(`type_aliases`) | 有効 | 訂正が本文内; 訂正・追記あり(書き方を保つ) | GCP-MODELS | - |
| 084 | 「読まない関数に渡った」印をセルに付ける | 有効 | 074 の穴を直す。訂正が本文内; 訂正・追記あり(書き方を保つ) | DESIGN-field-sinks | - |
| 085 | Rust の async 内 `Fn::call` も絞る | 有効 | 079 の続き | DESIGN-async-dispatch | - |
| 086 | sanitizer のある interface メソッドは実装の本体を読む | 有効 | 075 を一部置換 | - | 組み込みの sanitizer の影響は 0 行 |
| 087 | `other_sources` に `via_shared_state` | 有効 | 069 の続き | - | 出力の追加のみ |
| 088 | 同じ位置の source を 1 つに数える | 有効 |  | - | 見せ方のみ。finding の集合は変わらない |
| 089 | Rust `write!` のモデル | 有効 |  | - | - |
| 090 | ポインタ包みの interface への書き込みを指す先にも | 有効 | 060/082 の続き | - | - |
| 091 | 下げられなかった式が値を返すとき、`unsupported` が元の値と結果を持つ | 有効 | 相談役の判断(2026-11-22)。Dart の解決できないコンストラクタ呼び出しから | ARCHITECTURE の IR の命令 | Go と Rust の extractor は未確認 |
| 092 | 依存のパッケージを、流れのモデルで切る(本体を読まない)のは、費用が著しいときも。ただし条件つき | 有効 |  | PRINCIPLES 3 節、GCP-MODELS P12、MODELING | 費用の測定の記録、突き合わせの検査、`calls:`、保守的な痕跡、切ったことの表示。実装は未(ROADMAP) |

## 訂正・追記の書き方を保つ ADR

誤りに気づいて直した記録が、本文にある ADR。日付と理由を残す書き方を保つ(書き換えない)。

009、018、023、032、038、064、078、079、082、083、084、043(長い追記群)、044(追記)、047(状態の注記: 棚上げから再開)、055(仮説の履歴)

## 本文に古い記述が残るもの

覆った印は付けず、読むときの注意として挙げる。

- ADR-023: 経路数の上限(既定 3)は実装されていない。常に 1 本(`internal/taint/engine.go:1390`)。
- ADR-044: 本文の「Go は gin」は、同じ ADR の追記で外れた。題も古い。ADR-063 が範囲を絞り直した。
- ADR-064: 本文の「残り」のうち、取得元の検査は実装済み。ネットワークの絞り込みは設計のみ([DESIGN-fetch-network.md](DESIGN-fetch-network.md))。
- ADR-031: 「go/ssa に直接依存」は ADR-040 で解消した。
- ADR-039: 状態が「案」のまま。ADR-040 で実装された。
- ADR-008: 「kind の追加はスキーマ版を上げる」は、実際は上げていない(ADR-043 の追記が認めている)。
