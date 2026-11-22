# rivus の設計思想

rivus の設計の判断は、ここに書いた原則に沿う。ほかの文書は、原則そのものを繰り返さず、ここを参照する。

- この文書は **原則** と、**今のコードがどこまで届いているか** を分けて書く。理想を現実と読み違えないため。
- 根拠は ADR（`docs/DECISIONS.md`）とコードの場所で示す。ADR は決めた時点の記録で、今の姿の説明は [ARCHITECTURE.md](ARCHITECTURE.md) にある。
- 現状の確認は、2026-11-22、コミット `9c26a39` 時点。古くなる前提で読むこと。
- 根拠の ADR が **まだ無い** 原則には、そう書く（「根拠の ADR: 未決」）。ADR は決めたことの記録なので、決めていないことをここで先に決めない。

読者の入口は [README.md](../README.md)。用語は [GLOSSARY.md](GLOSSARY.md) に合わせる。

---

## 1. 判定ではなく、事実を出す

**原則**

- rivus は、Go / Dart / Rust のコードについて、外から来た値が危険な処理に届く経路を、関数・ファイル・パッケージをまたいで組み立てる。
- 安全か危険かは判定しない。判定は LLM や人が行い、rivus はその材料になる事実を出す。
- 出力は、次を含む。
  - 経路と、各ステップのコード
  - 確かめられなかったことと、その理由
  - 条件によって結果が変わる所と、その条件の値の出所
- rivus の結果を「通れば安全」のゲートとして使わない。

**根拠の ADR:** 001（判定をしない）、006、007、008、029、034、035、052、078（断定の誤りは、誤った注記の中で最も重い）

**コード:** `internal/taint/result.go` の `Finding`（`Paths[].Steps[].Snippet`、`Uncertainties`、`ConditionalNodes`）。`confidence` の値は `no_uncertainty_on_path` と `requires_review` だけで、安全・危険ではない。条件の出所は `internal/taint/conditional.go`、`provenance.go`。

**現状と、まだ届いていない所**

- 経路は 1 本だけ出す（`internal/taint/engine.go:1390`）。ADR-023 の「3 本まで、あふれは数える」は実装されていない。
- 経路が 200 段を超えると、途中の段を見せない（`rope.flatten`）。省いたステップ数は `paths[].omitted_steps` と、`depth_limit`（detail が `path_steps_omitted:`）に残る。省いたステップが持つ不確実性・モデル・到達性は、見せるステップと同じに集計に入る。source までさかのぼる途中の打ち切りも、同じマークを付ける。

---

## 2. 省いたものは、必ず報告する

これが rivus で一番大事な規律。

**原則**

- 解析は soundy。原理的には見逃しのない側（過大近似）を目指し、扱わないもの（リフレクションの一部、並行性の一部など）は意図して決める。
- 扱わなかったもの、打ち切ったもの、絞ったもの、モデルで近似したものは、必ず出力に記録する（不確実性の `kind`、`attributable_to`）。
- この規律は、解析だけでなく、次にも同じく当てはまる。
  - コールグラフの検索: 「探し尽くした」と言ってよいのは、出所をもれなくたどれたときだけ。たどれなければ全候補を残す。
  - モデル: 流れを止めるモデルは、止めた所を記録する。
  - テストの正解: 見逃しや誤検出のマーク（`@miss`、`@fp`、`@na` など）で、理由と参照を付けて残す。
- 失敗の重さは、重い順に次のとおり。
  1. サイレントな見逃し（注記なしで流れが消える）
  2. 誤った注記（理由や帰属の誤り）
  3. 注記つきの見逃し
  4. 構造的な誤検出
  5. 注記つきの過大近似

**根拠の ADR:** 007、008（+059 の追記）、030（`expansion_limit`）、050（`candidate_limit`）、053（`capture_unbound`）、054、056（`global_holds_nothing`）、062（`tests_skipped`）、067（`model_approximation`）、070、074（`field_imprecision`）、076（`summary.unread`）、079（`closed_world_entry`）

**失敗の重さの順について:** この 5 段の順序を 1 か所に書いた ADR は **まだ無い**（根拠の ADR: 未決）。断片は次にある。

- ADR-078: 断定の誤りは、誤った注記の中で最も重い
- `docs/DESIGN-field-sinks.md:142`: 何も報告せずに落とすことは、同じ流れが 2 件になることより重い
- `docs/DESIGN-zoo.md:26`、`bench/zoo/README.md`: 伝播のモデルの誤りを優先する。見逃しと誤検出のマークを分ける
- 「構造的な誤検出」という語は、どの文書にも出てこない

**コード:** `internal/taint/result.go`（`kind` の一覧、`attributable()`）、`heldnothing.go`、`internal/ir/ir.go`（`ValueLimit`、`NarrowBasis`）、`internal/valuecalls/narrow.go`、`internal/report/report.go`。テストのマークは `bench/zoo/README.md` に定義がある。

**現状と、まだ届いていない所**

- **200 段を超える経路の切り詰め:** 記録している（`omitted_steps`、`path_steps_omitted:`。1 節に同じ）。
- **流れを止めるモデルの記録が、一部だけ:** 記録するのは、`origin: llm` の sanitizer の抑制、`builtin` 以外の `pure` と結果に届かない `flows`(#138)、`numeric_stop`、`global_holds_nothing`、`approximate`。`builtin` の `pure` と `flows`、`builtin` や人が書いた sanitizer の停止は、出力に残らない。ADR-010 の文面（「抑制した・伝播させたを必ず記録する」）のほうが、実装より広い。
- **`numeric_stop`:** サマリを通って止まる分は記録しない。ADR-043 の追記が自己申告している。
- **`kind` の追加と、スキーマのバージョン:** ADR-008 は「種類は閉じた集合で、追加はスキーマを上げる」と書くが、実際は `assumed_call`、`model_approximation`、`field_imprecision`、`numeric_stop` などを `rivus.result/1` のまま足している（ADR-043 の追記が認めている）。運用と ADR の文面が食い違っていて、どちらに合わせるかは **未決**。
- **ADR-079 の訂正:** 「全部たどれたときだけ絞る」が守られていなかった 6 件が、訂正として記録されている。

---

## 3. 価値の中心は、さまざまな記法を正確にグラフ化すること

**原則**

- 呼び出しの関係とデータの流れを、言語の多様な書き方にわたって取りこぼさないことが、rivus の価値の中心。
- 組み込みのモデルの範囲は、Google Cloud(範囲の 1 つ目)、外部のサービスのクライアント(公開の指標の上位)、標準ライブラリ、代わりのないパッケージ、pub の人気の上位(ADR-096、097)。依存を API の境界のモデルにするのは、解析のコストを下げるためと、依存の中の sink を検出するため(依存の関数の中では source と sink を探さないので、sink のモデルは検出に要る。ADR-099)。特定の利用者に特化したモデルは、rivus の外で作り、ルールとして差し込む。
- 知識が rivus のコード（extractor やコア）に埋まっていて、ルールの YAML から足せない状態は、直すべき問題として扱う。
- ルールの書式の安定と、書き間違いをはっきり弾く検証は、外から差し込む人のための約束。
- モデルの種類で扱いを分ける。
  - source / sink（何が危険か）は、コードからは分からない知識。
  - 流れのモデル（`flows`、`pure`、`calls:`）は、本体を読む代わりの近似。増やすほど本体を読まない場所が増える。後者は、必要が測定で示されたものに絞る(本体がない、収束しない、ライブラリが利用者のコードを呼ぶ、**解析に著しくコストがかかる**。最後のものは、パッケージ全体なら ADR-092 の条件を満たしたもの、実装の多いインターフェースのメソッドなら ADR-098 の基準を満たしたものだけ。パッケージ単位で読み込みの段階から読まない仕組みは、設計中で、決まっていない)。詳しくは [MODELING.md](MODELING.md)。

**根拠の ADR:** 002、003、015、016、044、063、066、067、071、074、086、092。096、097、098、099(範囲と、切る基準と、依存の中のサイト)。077 は Google Cloud のモデルの始まりで、ADR-097 で範囲の 1 つ目になった。

**コード:**

- モデル: `internal/model/builtin/*.yaml`、`internal/model/model.go`（`RuleSet` の `Sources` / `Sinks` / `Sanitizers` / `Propagators` / `Globals` / `ConfigReads`。propagator が `flows` / `pure` / `calls` を持つ）
- 外から差し込む例: `examples/rules/`
- 組み込みから外したフレームワークを使っているとき stderr に 1 行出す: `cmd/rivus/notbuiltin.go`
- 書き間違いの検証: `model.Parse` の `KnownFields(true)`、外部ファイルの `origin: builtin` の拒否、`origin: llm` の必須フィールド

**現状と、まだ届いていない所**

- **組み込みの範囲:** ADR-063 の「保留」は、2026-11-22 に決着した(ユーザーの判断): 組み込みもある程度充実させる。特定のフレームワークを組み込みにするかは、その都度、相談役が判断する。`rust-web.yaml` の axum / rocket / tide / rouille / warp と、`dart-packages.yaml` の shelf は、そのまま残る(その都度の判断の対象)。README の「特定のフレームワークは組み込まない」の文は、まだ直していない(ずれている)。
- **コアに埋まった知識:** 「直すべき問題として扱う」という原則を書いた ADR は **まだ無い**（根拠の ADR: 未決）。実例は次のとおり。コアに言語名の分岐（`if lang ==`）は無いが、シンボルの知識は持っている。
  - `calls.go:460`（`reflect.*`）、`calls.go:959`（`bytes.Buffer` / `strings.Builder`）
  - `callrows.go` の `isStdType`（Rust の `std` / `core` / `alloc`）
  - `valuecalls/narrow.go:46`（Rust の `Fn::call` のシンボル）
- **ルールの書式:** 書き間違いを弾く検証はある。ただし書式のバージョンは無く（`ruleset: name@N` はルールセットのバージョン）、README は「書式は変わる」と書く。安定の約束は、まだ文書になっていない。
- **MODELING.md:** `pure` / `approximate` / `calls:` / `parts` / 「行は一致しなければならない」の節は実装と合っている。基本の `flows` / `sources` / `sanitizers`、`globals` / `holds`（ADR-056）、`.content`（ADR-082）などの節は、まだ無い。
- **構文の多様さの精度:** Dart と Rust は Go より見逃しが多い。数字は `bench/zoo/README.md`、`bench/MISSES.md`、`bench/parity/` にある。

---

## 4. 言語に依らないコアと、言語ごとの extractor

**原則**

- 中立な IR を、CFG のレベルで切る。φ のない CFG と、再代入できるローカル変数。
- 判断の基準は「言語に固有の知識が必要か」。CFG を作るまでは extractor、SSA から先はコアで、1 回だけ実装する。
- 言語に固有の脱糖は extractor の責務。IR の命令を増やして逃げない。
- 型・名前・パッケージの解決は、言語のツールチェインに任せる。フロントエンドを自作しない。

**根拠の ADR:** 002、003、026、036、037（Dart の難所は IR を変えず extractor が展開する）、039、040、041、042。「例外・async・defer・go は新しい命令にしない」は ARCHITECTURE の 2.1。

**コード:** `internal/ir/ir.go`（φ は無い。`Op` 21 種と `Term` 5 種）、`internal/ssabuild`、`internal/irssa`、`internal/ssacheck`（go/ssa との差分の検査）、`internal/frontend/gossa/load.go`（`NaiveForm`）。ツールチェインへの委譲先は、Go が `go/packages`、Dart が `package:analyzer`、Rust が `rustc_public`。`internal/taint`、`sg`、`irssa`、`ssabuild` が `go/ssa`・`go/packages`・`go/types` に依存しないことは、`go list -deps` で確認した。

**現状と、まだ届いていない所**

- **IR は後から広がっている:** `exception_value`、`unsupported`、`type` レコード、`type_aliases` などが足された（ADR-043 の追記、083）。「命令を足して構文の差から逃げない」を、どこまで許すかの規則は、ADR にも ARCHITECTURE にも見つからない（根拠の ADR: 未決）。
- **コアのシンボル知識:** 3 節に同じ。
- **`internal/callgraph`:** `go/ssa` に依存する Go 専用の構築で（ADR-018）、コアのエンジンの依存先ではない。パッケージのコメント（`callgraph.go:1`）は、この実態と合っていない。

---

## 5. 関数サマリをボトムアップに作る

**原則**

- functional approach で、IFDS の tabulation ではない。
- 理由は機能から来る。「ある sink に届く source を全部挙げる」のような、起点を決めない問い合わせには、全関数の完全なサマリが先に必要。

**根拠の ADR:** 004（理由を明記）、021、031、047

**コード:** `internal/taint/engine.go`（`order(fns, callees)`、`levelsOf`。SCC ごとに不動点、レベルごとに並列）

**現状と、まだ届いていない所**

- 「サマリが関数単位で独立している」性質は、グローバルの共有状態で厳密には成り立たない。グローバルはプログラム全体で 1 つの、フローに依らない状態で、変化したときに読み出し側を再解析する（ADR-031。収束は 5〜7 パス）。`docs/DESIGN-shared-state.md` と ADR-069、087 が、この扱いの経緯。
- 増分解析（6 節の作らないもの）に備えて、この性質を壊さない方針は変わらない。

---

## 6. 決定論的であること

**原則**

- rivus 単体の出力は決定論的。結果を変える隠れた設定を持たない。
- LLM の判定から作ったモデルは `origin: llm` として区別し、`--deterministic-only` で外せる。

**根拠の ADR:** 010、034、043（追記: 非決定の 3 つの原因を直した）、054、061

**コード:** `--deterministic-only` は `cmd/rivus/common.go:64`、`internal/model/model.go:1159`（`origin: llm` を落とす）、`internal/report/report.go:52`（`deterministic_only`、`ignored_models`）。テストは `cmd/rivus/determinism_test.go` など。`origin: llm` には `decided_at` / `rationale` / `model_version` が必須（`model.go`）。

**現状と、まだ届いていない所**

- **時計の制限:** 既定の `FuncTimeout`（30 秒）で、負荷によって出力が変わりうる。これは `summary.load_dependent` で示す。ADR-010 の「rivus 単体は決定論的」は、この条件つきで読むこと。
- **x/tools の揺れ:** generic で 1 件の揺れが残る（ADR-043 の追記が自己申告している）。
- **結果を変える入力が、出力に記録されていない:**
  - フラグ: `-no-deps`、`-tests`、`-numeric-taint`、`-k`
  - 読み込んだルールファイルの一覧
  - Go の環境: `GOFLAGS`、`GOOS`、`CGO_ENABLED`、`GOWORK`（キャッシュキーには入っている）
  
  確認した範囲では、`stats` と `summary` に無い。

---

## 7. 測ってから変える

**原則**

- 仮説で大きな変更をしない。原因を追跡で特定してから直す。ADR に、仮説の履歴を残す。
- 精度を上げる変更は、結果が変わってよい。ただし、消えたものが本当に実在しない経路だったことを確かめる。
- 速度やメモリのための変更は、結果が同一であること。
- 正解は、zoo（形 × 言語の行列）と、公開のコードの実際の脆弱性（修正前のコミット）に置く。
- 正解のマーク（`@miss`、`@fp`、`@na` など）の変更は、正解そのものの変更として扱う。ケースを直して通すのではなく、マークと変更の理由を、同じコミットで見えるようにする。

**根拠の ADR:** 027、043、046、055、057（仮説の履歴を ADR に残す）、061、066、084〜090（「確かめたこと」の節）

**コード・手順:**

- 比較: `bench/compare.sh`（終了コード 1 = finding が消えた、2 = 比較の前提が崩れた）、`bench/compare.py`。手順は `bench/README.md`。
- zoo: `bench/zoo-go`、`zoo-dart`、`zoo-rust`、`bench/zoo/README.md`、`docs/DESIGN-zoo.md`
- 実際の脆弱性: `bench/wild/`（公開 ID、修正前の SHA。コードは複製せず `fetch.sh` で取る）
- マークの規則: `@ref(...)` を `@miss` と `@fp` に必須とする。`@na` は「言語に本当に無い形」だけ。`@manual` は 3% まで。

**現状と、まだ届いていない所**

- 「ADR に仮説の履歴を残す」は、ADR-055、057、043 で実践されているが、規則としては書かれていない。ADR の書き方も一定ではない（状態の注記が本文に複数並ぶものがある）。
- zoo の段階 5（組み合わせ）と表 S の全体は、未完（`docs/DESIGN-zoo.md`）。`wild` は Go の 3 件のみ。

---

## 8. 解析の対象は、攻撃者が制御しうる入力

**原則**

- 対象のコードは、解析する人のマシンで実行されうる。Rust のビルド、取得の段階、Go の `go list`、Dart の前提の手順。既定は隔離する。
- 対象のリポジトリの中のデータ（設定ファイル、git の設定、ルール、DB など）も、rivus への攻撃の入力になりうる。
- セキュリティの制御を外す設定は、外したことが分かる形にする。

**根拠の ADR:** 038（追記: `.git/config` を信頼せず、git を実行しない）、062、064（隔離。訂正を含む。コンテナの部分は 093 が覆した）、093（macOS のサンドボックス）。設計は [rust-isolation.md](rust-isolation.md) と `docs/DESIGN-fetch-network.md`。

**コード:**

- 隔離: `internal/sandbox`（macOS のサンドボックス。既定ですべて拒否し、必要なものだけ許す。ネットワークは取得の段階だけ）、`cmd/rivus/isolate_sandbox_darwin.go`（Rust の取得と抽出の 2 段階）、`isolate_fetch.go`（取得の段階で対象の設定を読まず、取得元を検査）
- サンドボックスが作れなければ、Linux では、何も実行せず止まる: `cmd/rivus/isolate_sandbox_darwin.go`、`isolate_sandbox_other.go`。隔離の下か、ホストでかは、`summary.isolation` に出る
- git の HEAD を直接読む: `internal/vcs`
- 対象内のルール: `.rivus/rules` は既定で読み、`-no-project-rules` で止める。symlink は拒否する。出力に、対象内ルールが消せる量を出す（`internal/report/report.go` の `ProjectRules`）
- テスト: `TestSandboxedBuildScriptCannotEscape`、`TestSandboxClosesEveryWayOut`、`TestSandboxFetchReadsNoConfigOfTheProject`、`TestSandboxFetchRunsNoCodeAndAcceptsUnstableManifestFeatures`、`TestSandboxTimeLimitStopsTheProgramAndItsGroup`（`internal/sandbox`、`cmd/rivus`。開発のマシンで `RIVUS_SBX_RED=1` / `RIVUS_SBX_NET=1`。CI は `ci.yml` の `sandbox` ジョブ で小さい部分）

**現状と、まだ届いていない所**

- **隔離が既定なのは、Rust だけ:** Go と Dart に隔離は無い。README が明記している。Rust の隔離は、macOS のサンドボックス(ADR-093)。メモリ・CPU・プロセス数の強い上限がなく、ユーザー境界もない。承知している弱さの一覧は、[rust-isolation.md](rust-isolation.md)。Linux では、信頼できないコードの Rust は読めない(止まる)。
- **auto で Rust を読むかは、聞く:** 端末でない実行(`claude -p`、CI)は、`-yes` か `-lang rust` がないと Rust を読まない(読まなかったことは `skipped_languages` に残る)。聞いた答えで別の index になる。`callers` / `dispatchers` の答えは、`skipped_languages` をまだ言わない(#123)。
- **取得の段階のネットワーク:** 開いたまま(宛先の許可リストがない)。`DESIGN-fetch-network.md` は案のまま(コンテナなしで実装する案は、後回しの課題)。
- **制御を外したことが、出力に出ない:** `-unsafe-host-exec`、`-allow-fetch-host`、`-allow-deps-under` を使ったことは、出力に記録されない。警告が出るかは、確認した範囲では見つからなかった。
- **Go の環境:** `go list` は `GOFLAGS`、`GOTOOLCHAIN`、`GOPROXY` などの環境をそのまま使う。対象の `go.mod` の `toolchain` 指定が、toolchain の取得を起こすかは、未確認。
- **結果の正しさへの攻撃:** 対象の `.cargo/config.toml` の `[env]` で IR を空にできる（R-21。ADR-064 に課題として記録）。隔離は、これを守らない。
- **保存した DB:** `rivus index` の SQLite を、攻撃の入力として検証しているかは、未確認。
- **ADR-001 との食い違い:** 「対象のビルドはしない」は、Rust の `cargo check` と合わない。README の「rivus は対象をビルドしない」にも、同じ食い違いがある。

---

## 9. 作らないもの

**原則**

- 判定そのもの
- 完全な経路感度、SMT、シンボリック実行、完全なポインタ解析、暗黙のフロー（制御依存）の追跡
- 独自の問い合わせ言語
- 増分解析（当面。ただし、サマリが関数単位で独立している性質は壊さない）
- Dart / Rust のフロントエンドの自作、IDE 連携、Web UI、SaaS

**根拠の ADR:** 001、002、006、014、016、021

**線の近くにあるもの（越えていないかを見る所）:**

- 経路感度: ADR-006 のガード（引数由来の条件）と、ADR-078 の定数畳み込み（`provfold.go`）。評価はせず、対応を書き出すだけ、という線は守られている。
- エイリアス: ADR-072、084、090 は、値の流れで別名を追う範囲に収まるが、積み重なっている。
- ビルド: 8 節のとおり、Rust は ADR-001 の例外になっている。
