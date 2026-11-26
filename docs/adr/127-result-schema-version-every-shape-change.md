# ADR-127: 結果の形が変わるときは、足しただけでも `rivus.result` のバージョンを上げる(2026-11-25)

- 状態: 有効
- 関連: ADR-008、ADR-034、ADR-043、ADR-125、ADR-126、ARCHITECTURE 4.4、#109
- 要旨: `analyze` の出力のスキーマ `rivus.result/N` を、形(キーと型)と `kind` と `attributable_to` の値が変わるたびに上げる。上げ忘れはテストで落とす。今の形を `rivus.result/2` にする

**背景:** ADR-008 は、不確実性の `kind` を「閉じた集合。追加はスキーマのバージョンを上げて行う」と決めた。実際には、`kind` もフィールドも、`rivus.result/1` のまま足されてきた(ADR-043 の追記、ADR-034 と ADR-126 の「足すだけなら上げない」)。ADR と運用が食い違い(PRINCIPLES に未決として載せていた)、結果を読む側(利用者のスクリプト、保存した結果を読む `rivus query`、エージェント)は、`rivus.result/1` と書いてあっても、どの `kind` とフィールドがあるかを、バージョンから知れなかった。IR では、同じ作りの「足すだけなら上げない」のせいで、古いものが何も報告せずに読まれた(#109、ADR-125)。結果の側でも、古い rivus が書いた結果を、新しい `kind` を知らない読む側が、何も言われずに読む事例が起こり得る。

**決定:**
1. **固定するもの**: `internal/report.Report` から届く型のすべてについて、JSON のキーと型(`Finding`、`Uncertainty`、`Stats`、`Summary` など。`json:"-"` と書き出されない(小文字の)フィールドは除く。`omitempty` は、キーの一部として含める。キーの並びと型の宣言の順は、ハッシュに影響しない)と、`internal/taint/result.go` の `Kind*` 定数の値と、`attributable_to` の値(`Attributable*`)を、1 つの SHA-256 にする。ADR-125 の `shapeOf` と同じ作りで、型は reflect で、定数は構文木で読む。
2. **テスト**: `TestResultSchemaVersionMatchesShape`(`internal/report/schema_shape_test.go`)が、今のハッシュを、表 `resultShapes`(バージョン → ハッシュ)の `SchemaVersion` のエントリと比べ、合わなければ落ちる。失敗のメッセージは、新しいハッシュと、直す 2 か所(`internal/report/report.go` の `SchemaVersion` と、表の新しいエントリ)を、そのまま出す。**バージョンを上げる手順は、この 2 か所だけ**(extractor は関係しない)。リリースしたバージョンのエントリは、書き換えない。
3. **今の形を `rivus.result/2` にする**(今まで `rivus.result/1`)。今の形のハッシュが、`/2` の最初のエントリ。**`/1` のエントリは作らない**: `/1` は、`kind` とフィールドが足されてきた期間の全体(下の一覧)で、1 つの形に戻せない。`rivus.result/1` と書いてある結果は、「`/2` より前に書かれた結果。形は書いた時点の rivus による」ものとして読む。
4. **フィールドの追加でも上げる**(`kind` の追加と同じ。`omitempty` で、既定では出ないものも)。`kind` や `attributable_to` の値の追加、キーの追加、型の変更、キーの削除が、すべて対象。

**固定しないもの(利用者が依存してはいけない範囲):**
- 自由なキーの map のキー(`fingerprint` のキー、`by_kind` の `kind` の名前、`by_output` などの集計の見出し)。名前はデータで、形ではない。ただし、`kind` の値は、定数として固定している。
- `detail`、`note`、`why` などの、文面の中身。形ではなく中身。読む側が文面を解析して判断してはいけない。
- 件数、時間、順序などの、値。

**理由:** #109 と同じ。読む側は、バージョンが一致すれば、キーも `kind` も揃っていると期待する。「足すだけなら上げない」は、足したものを知らない読む側を、何も報告せずに通す。`kind` の追加は、利用者のスクリプトにとって、新しい分岐が要る変更なので、フィールドの追加と同じに扱う。バージョンの上がる頻度は、テストが決める(形が変わったときだけで、`detail` の文面や値だけの変更では上がらない)。

**覆すもの:**
- ADR-008 の「追加はバージョンを上げる」は、運用と合わず逆になっていた。今回、元に戻す(ADR-008 の文面が正しくなる)。ADR-043 の追記が「運用で逆」と認めた部分も、これで解消する。
- ADR-034 と ADR-126(9 項)の「足すだけなら上げない」のうち、`rivus.result` の部分。ほかの決定(ADR-126 の `summary.extractor` の中身など)は有効。
- 各 ADR が個別に書いた「`rivus.result/1` のバージョンは上げない」(ADR-061、067、069、087)は、書いた時点の記録として残す。今後は、この ADR に従う。

**`/1` の期間に足された `kind` とフィールド(事実の一覧。`rivus.result/1` から `/2` の間に、バージョンを上げずに足されたもの):**
調べ方: `internal/taint/result.go` の `Kind*` の値と、`Report` から届く型のフィールドについて、その値(フィールドは、`json` のキーと Go のフィールド名が並ぶ行)が最初に入ったコミットを、`git log -S` と `git log -G` で探した。PR の番号は、そのコミットを取り込んだマージ(`Merge pull request`)による。「PR なし(main に直接)」は、PR の運用の前に、main に直接入ったもの(リリース PR #100 が取り込んだ)。同じ Go のフィールド名とキーが、同じファイルの別の型にもあるフィールド(`Symbol`、`Position` など)は、日付が早めに出ているものが混じりうる。IR(`ir.Position` など)から来る型は数えていない。

*`kind`(`Kind*` 定数。2026-11-15 のものは、実装の初日):*
- `dynamic_dispatch`: 2026-11-15、ceee8de1、PR なし(main に直接)
- `unmodeled_library_call`: 2026-11-15、ceee8de1、PR なし(main に直接)
- `dependency_body_unread`: 2026-11-21、5130626d、PR なし(main に直接)
- `project_body_unread`: 2026-11-23、76d01d38、#226
- `unresolved_callee`: 2026-11-15、ceee8de1、PR なし(main に直接)
- `container_element_imprecision`: 2026-11-15、ceee8de1、PR なし(main に直接)
- `depth_limit`: 2026-11-15、ceee8de1、PR なし(main に直接)
- `access_path_truncated`: 2026-11-23、3474da57、#309
- `recursion_approximation`: 2026-11-15、ceee8de1、PR なし(main に直接)
- `char_transform`: 2026-11-24、522dc4b1、#389
- `missing_source`: 2026-11-15、ceee8de1、PR なし(main に直接)
- `reflection`: 2026-11-15、645e2263、PR なし(main に直接)
- `concurrency`: 2026-11-15、645e2263、PR なし(main に直接)
- `ffi_boundary`: 2026-11-15、645e2263、PR なし(main に直接)
- `global_var`: 2026-11-15、645e2263、PR なし(main に直接)
- `config_read`: 2026-11-15、7391729a、PR なし(main に直接)
- `exported_api_param`: 2026-11-15、7391729a、PR なし(main に直接)
- `stored_data_boundary`: 2026-11-19、51c1817a、PR なし(main に直接)
- `model_approximation`: 2026-11-21、7bb8d939、PR なし(main に直接)
- `numeric_stop`: 2026-11-20、5902fdcc、PR なし(main に直接)
- `unsupported_construct`: 2026-11-20、05cab585、PR なし(main に直接)
- `assumed_call`: 2026-11-21、5dfec7ff、PR なし(main に直接)
- `func_value_into_dependency`: 2026-11-25、f6f852d1、#475
- `field_imprecision`: 2026-11-21、680104bf、PR なし(main に直接)
- `field_unresolved`: 2026-11-21、680104bf、PR なし(main に直接)
- `named_unresolved`: 2026-11-23、3185849e、#169
- `pointer_alias`: 2026-11-23、c1d327f4、#295

*フィールド(型: キー)。2026-11-15 は、実装の初日で、形が固まる前だったので、列挙しない(`git log --since=2026-11-15 --until=2026-11-16 -- internal/taint internal/report`)。2026-11-16 以降に足されたもの:*
- 2026-11-16、c67c7d15、PR なし(main に直接): Finding: other_sources, other_sources_count
- 2026-11-18、d9d2429d、PR なし(main に直接): OtherSource: path_steps; Finding: path_steps
- 2026-11-19、5754550a、PR なし(main に直接): OtherSource: reachability; Finding: reachability
- 2026-11-19、61683e81、PR なし(main に直接): Stats: dep_spent_ms, own_spent_ms
- 2026-11-19、6cff04d3、PR なし(main に直接): OtherSource: dispatch_widened, dispatch_candidates; Finding: dispatch_widened, dispatch_candidates
- 2026-11-19、b21ada3f、PR なし(main に直接): Expansion: missing
- 2026-11-19、e509d0bb、PR なし(main に直接): Stats: levels, largest_scc, widest_level
- 2026-11-20、05cab585、PR なし(main に直接): OtherSource: uncertainties
- 2026-11-20、a7d721e8、PR なし(main に直接): Report: project_rules; ProjectRules: file, sources, sinks, sanitizers, propagators, pure_models, drop_all_sanitizers
- 2026-11-21、2884c016、PR なし(main に直接): Stats: dropped_globals; DroppedGlobal: symbol, functions
- 2026-11-21、2d4a5133、PR なし(main に直接): Stats: unstable_count, run_timeouts, load_dependent; Summary: load_dependent, load_dependent_note
- 2026-11-21、42c6ee59、PR なし(main に直接): Stats: tests_skipped; TestsSkipped: files, targets, dev_dependencies, dev_dependency_names, notes
- 2026-11-21、5dfec7ff、PR なし(main に直接): Stats: identity_global_stores
- 2026-11-21、81a869a1、PR なし(main に直接): Uncertainty: narrowed_from, basis; Stats: dispatch_not_narrowed; NotNarrowed: open_callers, unknown_origin, limit; Summary: dispatch_narrowed; DispatchNarrowed: calls, candidates_before, candidates_after
- 2026-11-21、b01a35c6、PR なし(main に直接): LoadError: unread, unread_functions; NumericStopCharReads: functions, top; NumericStopOther: functions; Summary: unread; Unread: functions, top; UnreadPackage: package, functions, why; SkippedLanguage: why; BodyReplacedPackage: package; FunctionsSkipped: functions; SkippedCrate: functions, why
- 2026-11-21、b0ed28b3、PR なし(main に直接): Finding: shared_state; SharedState: place, path, writers, readers; SharedPlace: name; SharedWriters: flow, all, test
- 2026-11-21、bbc2cad7、PR なし(main に直接): Stats: pass_changes, unmodeled_top, packages_top, dispatch_candidates; PassChange: pass, changed, new_flow, added_elems, new_elems, returned_elems, guard_only, shrunk; UnmodeledSymbol: symbol, reached, on_path; PackageStat: package, dep, functions, reanalyses, spent_ms, summary_elems, largest_summary, largest_summary_elems; SCCPackage: package, functions; DispatchCandidates: one, two_to_8, 9_to_16, over_16, none, largest, largest_at
- 2026-11-21、d725bd65、PR なし(main に直接): Finding: reachability_note
- 2026-11-21、efd59b96、PR なし(main に直接): OtherSource: via_shared_state
- 2026-11-22、1d560ef3、#135: Stats: largest_scc_packages
- 2026-11-22、5824866a、#150: Stats: summary_sizes; SummarySizes: functions, empty, median, p90, p99, max, over_100, over_1000, over_10000, top_1pct_share_permille, own_elems, dep_elems, by_output, by_element, by_path_depth; ReachSpread: max, median, p90, over_100, over_1000
- 2026-11-22、8284fd6e、#153: SummarySizes: real_origins, origin_reach, globals_top, globals_with_elems
- 2026-11-22、93adcd75、PR なし(main に直接): TestsSkipped: outside_build_files, outside_build_file_names; Summary: outside_configuration; OutsideConfiguration: files
- 2026-11-22、ad296447、PR なし(main に直接): TestsSkipped: skipped_dirs, skipped_dir_names; Summary: skipped_dirs; SkippedDirs: examples; OutsideConfiguration: examples
- 2026-11-22、b92ac469、#156: Summary: on_findings_top, on_findings_dispatch_candidates; NoteSymbol: notes
- 2026-11-22、b99440df、PR なし(main に直接): Path: omitted_steps
- 2026-11-22、ca6f6e7f、PR なし(main に直接): Summary: isolation; Isolation: mode, detail
- 2026-11-22、d4549a27、#107: Unread: skipped_languages; SkippedLanguage: lang
- 2026-11-23、05363855、#237: UnverifiedVersion: all_used
- 2026-11-23、0b0e25f0、#256: Isolation: developer_dir; DeveloperDir: value, source
- 2026-11-23、1eaa5b37、#163: Uncertainty: recv, operands; Operand: type, class
- 2026-11-23、2490082d、#193: BodyReplacedPackage: functions, functions_declared
- 2026-11-23、463665b8、#365: Stats: budget_exhausted, func_timeouts
- 2026-11-23、62b3696d、#302: SummarySizes: distinct, distinct_dep; DistinctElems: all, ignoring_guards, ignoring_path, ignoring_path_and_guards
- 2026-11-23、6682b38c、#259: Summary: categories_excluded; CategoryExcluded: category
- 2026-11-23、68f39ac6、#309: Summary: access_path_depth
- 2026-11-23、76d01d38、#226: AccessPathDepth: project, dependency; Summary: functions_skipped; FunctionsSkipped: project, dependencies, crates; SkippedCrate: crate, dependency, why_other, examples
- 2026-11-23、8cac9e7e、#372: Summary: live_heap_max_bytes
- 2026-11-23、978f3b64、#332: Stats: out_widened_funcs, out_widened_outputs
- 2026-11-23、9f6572a5、#176: Site: categories; Finding: categories
- 2026-11-23、a3c84a11、#179: Summary: body_replaced_packages, body_replaced_ignored; BodyReplacedPackage: declared_in, reason
- 2026-11-23、b4776a2b、#294: ProjectRules: sanitizers_without_neutralises
- 2026-11-23、b766adda、#354: Summary: memory_limit_bytes, memory_limit_source, memory_limit_note
- 2026-11-23、c75619e2、#200: Summary: callback_param_unresolved; CallbackParamUnresolved: sites; CallbackParamUnresolvedSite: func, position
- 2026-11-23、cfe4ec55、#312: NumericStops: char_reads, other
- 2026-11-23、d3108509、#229: Summary: unverified_versions; UnverifiedVersion: used, verified
- 2026-11-23、d54e5559、#359: Summary: memory_peak_bytes, memory_limit_exceeded, gc_cpu_fraction
- 2026-11-23、d55391b8、#165: Summary: dependency_sites; DependencySites: searched, dependency_functions
- 2026-11-23、d89de829、#353: Summary: incomplete, incomplete_reasons
- 2026-11-23、e3dc3079、#312: Summary: numeric_stops; NumericStopCount: function, scope
- 2026-11-23、eb18ef49、#192: UnverifiedVersion: package, source
- 2026-11-24、73a6620e、#491: Stats: largest_scc_deps
- 2026-11-24、9041f141、#470: Stats: scc_coarsened
- 2026-11-24、fb129dc9、#382: Stats: adds_total, adds_max_single_run, adds_max_function_total, adds_per_function; AddsBucket: from, to, adds
- 2026-11-25、65095f1c、#537: Summary: extractor; ExtractorCheck: lang, path, schema, source_hash, expected_hash, match, detail
- 2026-11-25、7d921ca8、#490: Uncertainty: coarsened_scc; CoarsenedSCC: id, packages, packages_count, functions; CoarsenedFunctions: group, dependencies; Finding: through_coarsened_scc

**同じ作りに広げる候補(この ADR では、`rivus.result` だけを固定する。課題 #223):** 次のスキーマも、同じ「キーと型と値のハッシュを表に固定し、変わったらバージョンを上げる」にできる。`rivus.query/1`(`internal/report/query.go`)、`rivus.sites/1`(`cmd/rivus/sites.go`)、`rivus.resolve/1`(`internal/report/resolve.go`)、`rivus.coverage/0`(`internal/report/coverage.go`。`/0` のまま)、`rivus.graph`(`internal/graphdb`。名前 `rivus.graph/1` と、整数のバージョン 2 の 2 つがある)。これらは、`rivus.result` の型を共有する部分があり(`rivus.query` は `Finding` を含む)、`rivus.result` の形が変わると、これらの出力も変わる。バージョンを揃えるか、別々に上げるかは、広げるときに決める。

**帰結:** 結果が変わる(`schema_version` が `rivus.result/2` になる。ほかは同じ)。形を変える PR は、`SchemaVersion` と `resultShapes` の 2 か所を直す(テストの失敗メッセージが、そのまま写せる行を出す)。`kind` の一覧の表(ARCHITECTURE 4.4)と、定数との照合は、次の作業(完了条件 A(2))。

**追記(2026-11-25、`rivus.result/3`): 値が決まった文字列の列挙の扱い、`interface` 型のフィールド、`summary.go_version`**

*1. 列挙の扱い。* 上の決定は、ハッシュに入れる列挙を `Kind*` と `Attributable*` の 2 つだけにしていた。結果の中の、値が決まった文字列を、`internal/report` から届く型のすべてについて調べ、次のように分けた。`schema_shape_test.go` の表 `resultEnums` が、(a) の定数の取り方(ファイルと、定数の名前の前置き)で、1 行足せば、その列挙が形に入る。
- (a) ハッシュに入れる(値の追加は、利用者のスクリプトにとって新しい分岐が要る変更で、形の変更に当たる): `finding.confidence`(`Confidence*`、2 値)、`reachability`(`Reach*`、3 値)、`conditional_nodes[].kind`(`Cond*`、2 値)、`summary.incomplete_reasons` の値(`Reason*`、5 値)、`summary.extractor[].match`(`cmd/rivus/extractorcheck.go` の `match*`、6 値)、`summary.go_version.match`(`Go*`、3 値)。これで、ハッシュは `kind`、`attributable_to` と合わせて 8 つの列挙の値を持つ。
- (b) 固定しない(値が定数でなく、ほかのパッケージの定数や、文中のリテラルで、今のテストの取り方では読めない。値の追加は、バージョンを上げずに起こり得る。**迷って(b)にしたもの。判断を仰ぐ**): `role`(`ModelRef`、`Site`、`Expansion`: `source` / `sink` / `sanitizer` / `propagator`)、`ModelRef.origin`、`Site.kind`、`SharedPlace.kind`(`global`)、`numeric_stops` の `scope`(`project` / `dependency`)、`summary.isolation.mode`(`sandbox` / `host`)、`developer_dir.source`(`user` / `default`)、`memory_limit_source`、`Uncertainty.basis`(`closed_world_entry`)、`certainty`、`terminal_kind`、`operands[].class`(`ir.TypeClass`。IR の側のハッシュ(ADR-125)には入っている)、`extractor[].lang`(`rust` / `dart`)。利用者は、これらの値の一覧が閉じているとは、まだ思ってはいけない。
- 列挙ではないもの: `category` と `labels` と `categories`(ルールのファイルで決まる名前で、データ)、`why` と `reason` と `detail`(文面)、`target.revision` などの値。上の「固定しないもの」と同じ。

*2. `interface` 型のフィールド。* 今の結果の型には、`interface`(`any`)型のフィールドがない。足すときは、その中身の形は、ハッシュに入らない(reflect が型の名前までしか分からない)。固定しない範囲として、中身を利用者が決まった形と思ってはいけない。足す PR は、この ADR にその旨を追記し、中身の形を別に固定するかを決める。

*3. `summary.go_version`(`rivus.result/3`)。* 課題 #109 の「古いものが何も報告せずに読まれる」の仲間で、こちらは、読む対象のコードの言語が、rivus を確かめた Go より新しいとき。`summary` に `go_version`(`target`、`verified`、`match`、`newest_module`、`newest_version`)を足し、`rivus.result/3` に上げる。
- 検証済みの Go は、定数 `gossa.VerifiedGo`(`"1.27"`)の 1 か所。rivus 自身の `go.mod` にある `go` の指定の minor で、`go.mod` を上げて定数を忘れるとテストが落ちる。
- 比べるのは minor まで(patch まで比べると、toolchain の更新のたびに誤った警告が出る)。
- **対象のモジュールだけでなく、読んだ依存のモジュールの `go` 行も見る**(`packages.Module.GoVersion`。依存の `go` 行が新しければ、その依存の言語の機能を、rivus は確かめていない)。読んだモジュール全体の最大で決める。最大が対象自身なら、`newest_module` は対象のモジュールパス。同じ値のときは、対象、次に、パスの小さい順。
- `unknown` は、どのモジュールにも読める `go` の指定が 1 つもないとき(古い `go.mod`、`go.mod` なし)だけ。対象に `go` の指定がなくても、依存に読めるものがあれば比べる: 依存が新しければ `newer`(`target` は空のまま、`newest_module` と `newest_version` に依存を入れる)、新しくなければ `yes`。対象に `go` の指定がないことと、依存の言語が新しいことは別の事実で、依存のコードも同じように読むため。
- **止めない**。`newer` のときだけ標準エラーに 1 行を出し、終了コードは変えない。ベンチと合格基準は、`newer` を失敗にしない(記録だけ)。
- 実際の `go` コマンドは、新しすぎる `go` 行を、`GOTOOLCHAIN=local` では読み込みの失敗にする(auto では新しい toolchain を取りに行く)ので、`newer` を、読み込みを通して確かめるテストは書かない。読み込みの経路(`Program.GoVersions`)と、判定(`CheckGoVersion`)を別々に確かめる。
- Dart の言語バージョン(`pubspec.yaml` の `environment.sdk`)は次の作業。同じ形で `summary` に足せる。Rust の edition も同様。

**追記(2026-11-25、`rivus.result/4`): kind の一覧の表、`role` と `origin` を列挙に入れる**

*1. kind の一覧の表(完了条件 A(2))。* kind ごとの `attributable_to` と 1 文の説明を、コードの 1 か所(`internal/taint/kinds.go` の `Kinds`)に持つ。`attributable()` はその表を引く(各 kind を作る所で `attributable_to` を手で書かない。表と違う値を付けると、`TestAttributableNotAssignedByHand` が落ちる)。`Kind*` の定数に表の項目がないと `TestKindsTableCoversConstants` が落ちる。ARCHITECTURE 4.4 の表は、`make docs-gen WHAT=kinds` の出力で、違えば `TestDocsSync` が落ちる。表で値が 2 つになる kind は `dynamic_dispatch` だけ(候補が 0 件のとき、calls.go で `rivus`。表の値は `mixed` で、その旨を説明に書く)。結果は変わらない。
- `KindGlobalHoldsNothing` の定数は、`heldnothing.go` にあり、ハッシュの取り方(`result.go` の `Kind*`)から外れていた。`result.go` に移した。これで、この kind がハッシュに入る(値は変わらない)。

*2. 列挙に入れたもの(上の (b) から)。* `role` と `ModelRef.origin`。取り方を構文木で読めるように、`role` の値を `internal/model/model.go` の定数 `RoleSource`、`RoleSink`、`RoleSanitizer`、`RolePropagator` にまとめた(それまでは、taint と model の文中のリテラル)。`ModelRef.role` と `Expansion.role` はこの 4 値、`Site.role` は `internal/sites/sites.go` の `RoleSource`、`RoleSink`(2 値)、`ModelRef.origin` は `model.go` の `Origin*`(4 値)。`resultEnums` に 3 項目を足した。値は変えていない。ハッシュが変わるので、`rivus.result/4` に上げた(`rivus.result/3` のハッシュは消さない)。ほかの (b)(`Site.kind`、`SharedPlace.kind` など)は、上のとおり固定しない。

**追記(2026-11-26、`rivus.result/5`): `summary.dart_version`**

`summary.go_version`(`rivus.result/3`)の Dart の分。課題 B(1) の残り。`summary` に `dart_version`(`target`、`minimum`、`verified`、`match`)を足し、`rivus.result/5` に上げる。`rivus.result/4` のハッシュは消さない。
- 検証済みの Dart は、定数 `VerifiedDart`(`cmd/rivus/dartversion.go`、`"3.13"`)の 1 か所。CI の `DART_SDK`(extractor とベンチを動かす SDK)の minor で、ずれるとテストが落ちる。比べるのは minor まで。
- 比べるのは、`pubspec.yaml` の `environment.sdk` が許す**最も低い SDK**(`minimum`)。Dart の言語バージョンは、この下限で決まる(上限は言語を決めない)。`^3.0.0` も `>=3.0.0 <4.0.0` も 3.0.0。`>=` の項が複数あれば高いほう。下限が検証済みより新しければ `newer`、新しくなければ `yes`、下限がない(`any`、`<4.0.0` だけ、`sdk` なし、`pubspec.yaml` なし、読めない)なら `unknown`。
- **対象のパッケージ(`-C` の `pubspec.yaml`)だけを読む**。`go_version` は、ローダーが読んだ依存のモジュールの `go` 行も渡すので、いちばん新しいもので決めたが、Dart の依存は、`pubspec.lock` に `sdk` の制約がなく、依存の `pubspec.yaml` を pub のキャッシュから読む経路が rivus にまだない。そのため `newest_module` と `newest_version` に当たる欄は持たない。依存を読むかは、判断を仰ぐ(次の作業)。モノレポで `-C` にない子のパッケージの `pubspec.yaml` も読まない(`pubspec.lock` の読みと同じ範囲)。
- **止めない**。`newer` のときだけ標準エラーに 1 行。終了コードは変えない。ベンチと合格基準は、`newer` を失敗にしない。
- `match` の値は、`resultEnums` の行(`Dart*`、3 値)で、ハッシュに入る。Rust の edition は、同じ形で次に足せる。
