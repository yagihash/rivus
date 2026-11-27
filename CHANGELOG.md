# Changelog

## [v0.0.21](https://github.com/yagihash/rivus/compare/v0.0.20...v0.0.21) - 2026-11-27

### Features
- Rust: Fn::call の絞り込みの出所の上限を 1024 にし、Dart の列挙の上限と分ける(#239) by @yagihash in https://github.com/yagihash/rivus/pull/641
- summary: value_candidates と dispatch_candidates_applied を足す(rivus.result/6) by @yagihash in https://github.com/yagihash/rivus/pull/642

## [v0.0.20](https://github.com/yagihash/rivus/compare/v0.0.19...v0.0.20) - 2026-11-26

### Features
- feat: ベンチと合格基準で extractor のハッシュが yes でないと失敗にする by @yagihash in https://github.com/yagihash/rivus/pull/539
- bench: Go の呼び出しの位置を IR と比べる道具(B(3) の Go の分) by @yagihash in https://github.com/yagihash/rivus/pull/564
- 合格基準のメモリの判定を 3 状態にする(PASS、余裕不足、FAIL) by @yagihash in https://github.com/yagihash/rivus/pull/572
- bench: Go のセルを 4 件足す(Dart にあって Go にない形) by @yagihash in https://github.com/yagihash/rivus/pull/568
- Rust の zoo: helper-overflow と conv の 2 セルを足す(PARITY 3 件) by @yagihash in https://github.com/yagihash/rivus/pull/585
- Rust のモデル 3 行と zoo の 3 セル(downcast_mut、retain、symlink) by @yagihash in https://github.com/yagihash/rivus/pull/587
- feat: summary.dart_version: 新しい Dart の sdk 指定を知らせる(result/5) by @yagihash in https://github.com/yagihash/rivus/pull/592
- Rust の zoo に callback_param のセル 3 つとルールを足す by @yagihash in https://github.com/yagihash/rivus/pull/598
- zoo: Dart の ptr-for-in を ptr-range-slice に改名し PARITY を付ける by @yagihash in https://github.com/yagihash/rivus/pull/601
- zoo: Dart のパッケージ 16 セルと range-func の @na(承認済み) by @yagihash in https://github.com/yagihash/rivus/pull/600
- Rust: 環境の alloc にクロージャの関数の ID を出し、コアは ID を先に引く(#233) by @yagihash in https://github.com/yagihash/rivus/pull/610
- feat: 組み込みの sources と sinks だけを外す -no-builtin-sources-sinks を足す by @yagihash in https://github.com/yagihash/rivus/pull/621
### Bug Fixes
- fix: sink の site を、calls: の行に渡された関数参照から作る by @yagihash in https://github.com/yagihash/rivus/pull/538
- 子の stderr の制御文字を除去する(SBX-4) by @yagihash in https://github.com/yagihash/rivus/pull/565
- sandbox の待ち方を堅くする(WaitDelay、取り消しの区別、mountable、取得段階の環境) by @yagihash in https://github.com/yagihash/rivus/pull/566
- 取得元の検査を、全ての Cargo.toml に当てる(Cargo.lock も TOML として読む) by @yagihash in https://github.com/yagihash/rivus/pull/567
- 取得元の検査: 外を指すシンボリックリンクのディレクトリと、外へ出る path 依存を拒否 by @yagihash in https://github.com/yagihash/rivus/pull/575
- seccomp: 拒否リストの番号に上限を置き、超える call は ENOSYS by @yagihash in https://github.com/yagihash/rivus/pull/579
- sandbox: 外の inode を内へ持ち込む link / rename のプローブ by @yagihash in https://github.com/yagihash/rivus/pull/580
- sanitizeForTerminal: 双方向の制御文字とゼロ幅の文字を可視にする by @yagihash in https://github.com/yagihash/rivus/pull/581
- sites: Rust のクロージャを callback_param の関数として引く(#231) by @yagihash in https://github.com/yagihash/rivus/pull/596
- fix: merge-pr.sh が main-ci の check を名指しで要求する(G1) by @yagihash in https://github.com/yagihash/rivus/pull/595
- test: TestThreadCPU を負荷に左右されない形にする(#220) by @yagihash in https://github.com/yagihash/rivus/pull/602

## [v0.0.19](https://github.com/yagihash/rivus/compare/v0.0.18...v0.0.19) - 2026-11-25

### Features
- feat: Dart の Iterable.singleWhere と expand に calls: の行を足す by @yagihash in https://github.com/yagihash/rivus/pull/534
- feat: リリースにソースの tarball を足し、tap の url をそれに替える by @yagihash in https://github.com/yagihash/rivus/pull/536
- feat: 解析の前に extractor のソースのハッシュを読んで照合し、記録する by @yagihash in https://github.com/yagihash/rivus/pull/537

## [v0.0.18](https://github.com/yagihash/rivus/compare/v0.0.17...v0.0.18) - 2026-11-25

### Features
- bench: rustypaste の truth と detect を足す(CVE-2026-90774、INFO) by @yagihash in https://github.com/yagihash/rivus/pull/524
- feat: Dart の標準ライブラリと collection の関数値を取る API に calls: を足す(#216) by @yagihash in https://github.com/yagihash/rivus/pull/525
- feat: extractor のソースのハッシュを、ビルドのときに埋める(ADR-126) by @yagihash in https://github.com/yagihash/rivus/pull/529
- test: 関数値を受けて呼ぶ API のセルを足す(to_later、to_sort、to_lib_call) by @yagihash in https://github.com/yagihash/rivus/pull/530
- bench: --graph-check を zoo-rust に広げる(手元、macOS、隔離の下) by @yagihash in https://github.com/yagihash/rivus/pull/533
- feat: Dart の sort、reduce、fold の関数にリストの要素を渡す(#221) by @yagihash in https://github.com/yagihash/rivus/pull/532
- ci: Rust の extractor のビルドにソースのハッシュを渡す by @yagihash in https://github.com/yagihash/rivus/pull/531
### Bug Fixes
- fix: バージョンの違う extractor の IR を読まずに止める(#109) by @yagihash in https://github.com/yagihash/rivus/pull/527

## [v0.0.17](https://github.com/yagihash/rivus/compare/v0.0.16...v0.0.17) - 2026-11-25

### Features
- model: dart:io の Link.create / createSync の target を sink にする by @yagihash in https://github.com/yagihash/rivus/pull/504
- bench: Rust の INFO の行(shlex 1.3.0)を合格基準の道具に足す by @yagihash in https://github.com/yagihash/rivus/pull/505
- ci: main-ci は main の CI を待たず、失敗の記録が無ければ success にする by @yagihash in https://github.com/yagihash/rivus/pull/513
- bench: rustypaste を INFO の対象として足し、最新の main で測る by @yagihash in https://github.com/yagihash/rivus/pull/508
- feat: rivus cache list / clean -older-than(課題 #199 の一部) by @yagihash in https://github.com/yagihash/rivus/pull/522
### Bug Fixes
- fix: irspill が大きな位置の数を読み戻せず解析が止まるのを直す by @yagihash in https://github.com/yagihash/rivus/pull/515
- fix: cpos の offset を 64 ビットにし、行 21,475 以降の手順のスニペットを守る by @yagihash in https://github.com/yagihash/rivus/pull/521
- fix: 読む対象が無いときは、結果を出さず、エラー(1)で終わる by @yagihash in https://github.com/yagihash/rivus/pull/520

## [v0.0.16](https://github.com/yagihash/rivus/compare/v0.0.15...v0.0.16) - 2026-11-25

### Features
- ci: Actions の lint(ghalint、actionlint)を ci.yml のジョブにして ci-ok に入れる by @yagihash in https://github.com/yagihash/rivus/pull/496
- taint: 関数値を FUNC の要素で運ぶ(Dart、#210 段階 1) by @yagihash in https://github.com/yagihash/rivus/pull/475
- scripts: 断るパスの一覧を .github/guardrail-paths に集め、merge-pr.sh が読む by @yagihash in https://github.com/yagihash/rivus/pull/489
- ci: main の CI の結果を PR の commit status(main-ci)にする by @yagihash in https://github.com/yagihash/rivus/pull/488
- taint: scc_coarsened の文言に、通った依存の関数、深さ 0 の意味、確かめ方を入れる by @yagihash in https://github.com/yagihash/rivus/pull/481
- taint: 粗く解いた SCC の構造化した項目と、finding の through_coarsened_scc を足す by @yagihash in https://github.com/yagihash/rivus/pull/490
- taint: 巨大な SCC の安全網を既定で有効にする(-scc-coarsen、既定 5,500、ADR-124) by @yagihash in https://github.com/yagihash/rivus/pull/491
- bench: dart-synth にサイズ F(依存へ関数値を渡す形)を足す by @yagihash in https://github.com/yagihash/rivus/pull/501
- taint: applySummary の割り当てを減らす(結果は変えない) by @yagihash in https://github.com/yagihash/rivus/pull/474
- bench: コストの退行の警告(G12)を compare.sh と PR の CI に足す by @yagihash in https://github.com/yagihash/rivus/pull/502
### Bug Fixes
- taint: クロージャが捕まえた引数への書き込みを、Go でも要約に出す(TestDogfood の流れの喪失) by @yagihash in https://github.com/yagihash/rivus/pull/494
- cmd/rivus: sidesOf のグローバルな map を mutex で守る(TestOffByDefaultFlags の競合) by @yagihash in https://github.com/yagihash/rivus/pull/497
- bench: graph-check の収束しないだけの場合を終了コード 4 にし、zoo-go だけ CI で許す by @yagihash in https://github.com/yagihash/rivus/pull/498

## [v0.0.15](https://github.com/yagihash/rivus/compare/v0.0.14...v0.0.15) - 2026-11-24

### Features
- irspill: ディスクの Source(DiskSource)と、Go の Emit を足す(B1a。まだ繋がない) by @yagihash in https://github.com/yagihash/rivus/pull/413
- perf: Go の IR を Put 経由でディスクに書く(B1b。内部の切り替えあり) by @yagihash in https://github.com/yagihash/rivus/pull/414
- bench: 合格基準の A と C を測る道具に対応する(memos-4gib、gitea、検出 score) by @yagihash in https://github.com/yagihash/rivus/pull/418
- bench: --cost-check で memos-4gib も見られるようにし、古い passline-ledger を消す by @yagihash in https://github.com/yagihash/rivus/pull/419
- bench: compare.sh --graph-check で、前後のグラフの表現の同一を見る by @yagihash in https://github.com/yagihash/rivus/pull/421
- perf: Dart と Rust の IR を 1 行ずつ Put する(B2。内部の切り替えあり) by @yagihash in https://github.com/yagihash/rivus/pull/422
- perf: IR の読む回数を減らし、valuecalls の保持に上限をつける(B3) by @yagihash in https://github.com/yagihash/rivus/pull/427
- ci: bench のテストを PR のジョブに足し、graph-check を定期の実行に足す by @yagihash in https://github.com/yagihash/rivus/pull/429
- perf: IR をディスクに書いて読む形を既定にする(B4) by @yagihash in https://github.com/yagihash/rivus/pull/430
- taint: SCC の反復ごとに要約のダンプを書く内部の口を足す by @yagihash in https://github.com/yagihash/rivus/pull/433
- bench: dart_services の依存の版を、pubspec.lock で固定する by @yagihash in https://github.com/yagihash/rivus/pull/435
- taint: 大きな SCC の計算済みの state を、commit のあとに手放す(A が通る) by @yagihash in https://github.com/yagihash/rivus/pull/436
- taint: SCC の反復ごとのダンプに、出力のキー別の内訳を足す by @yagihash in https://github.com/yagihash/rivus/pull/440
- taint: SCC の反復ごとのダンプに、出力の完全なパスの上位を足す by @yagihash in https://github.com/yagihash/rivus/pull/442
- goir: Extract の経路を 1 つにする(lifted を使わない経路を消す。課題 #201) by @yagihash in https://github.com/yagihash/rivus/pull/452
- ci: Dart の extractor のテスト(dart test)を CI で回す by @yagihash in https://github.com/yagihash/rivus/pull/464
- bench: --graph-check に findings と uncertainties の順序のままの比較を足す by @yagihash in https://github.com/yagihash/rivus/pull/472
- taint: -resolve-func-values と関数値の別の通しを消す by @yagihash in https://github.com/yagihash/rivus/pull/469
- bench: --graph-check を Dart に広げる(収束しない対象は NOT CONVERGED、#211) by @yagihash in https://github.com/yagihash/rivus/pull/473
- taint: 大きい SCC は、依存の関数を最初から深さ 0 で解く入口(ADR-124、#187) by @yagihash in https://github.com/yagihash/rivus/pull/470
- bench: 時間を測る実行に負荷とプロセスを記録し、判定は 3 回の中央値にする by @yagihash in https://github.com/yagihash/rivus/pull/480
### Bug Fixes
- test: Rust のテストが、共有の場所に CARGO_HOME を作らないようにする by @yagihash in https://github.com/yagihash/rivus/pull/423
- goir: debug extract の IR の `$bound` の候補を、解析と同じにして揺れをなくす by @yagihash in https://github.com/yagihash/rivus/pull/425
- merge-pr.sh: チェックが全部終わって失敗が無いこと(ci-ok が pass)を確かめる by @yagihash in https://github.com/yagihash/rivus/pull/439
- dart: getter の呼びの候補に、同名の setter の本体を入れない by @yagihash in https://github.com/yagihash/rivus/pull/446
- dart: 演算子のメンバ名の鍵を壊さない(== は空の鍵、<= は <、[]= は [] に混ざっていた) by @yagihash in https://github.com/yagihash/rivus/pull/448
- dart: super.x と super の演算子、添字を静的な呼びにする by @yagihash in https://github.com/yagihash/rivus/pull/449
- bench: 観測した呼び出しの照合(Dart)の誤判定 3 つを直す(no_edge 92 → 80) by @yagihash in https://github.com/yagihash/rivus/pull/451
- dart: 継承した実装(mixin、上位クラス)への interface 呼び出しの候補を足す by @yagihash in https://github.com/yagihash/rivus/pull/453
- merge-pr.sh: 承認の日付つきの Zoo-Loosened の PR は、CI が通っていれば開発者がマージできる by @yagihash in https://github.com/yagihash/rivus/pull/455
- cmd: extractor のコマンドの値を、空白を含むパスと引用符で分ける(課題 #200) by @yagihash in https://github.com/yagihash/rivus/pull/459
- dart: extractor の dart test が test/sample を拾わないようにする(課題 #204) by @yagihash in https://github.com/yagihash/rivus/pull/463
- valuecalls: 返したクロージャを呼ぶ形を呼び出しごとにたどる(Dart、#78) by @yagihash in https://github.com/yagihash/rivus/pull/465
- valuecalls: コールバックのコールバックの呼び出しをもう一度解決する(Dart、#81) by @yagihash in https://github.com/yagihash/rivus/pull/466
- valuecalls: 代入し直した変数のセルの load をたどる(Dart、根 I) by @yagihash in https://github.com/yagihash/rivus/pull/471
- model: path/filepath.Walk のコールバックに root を 3 つの引数とも渡す(model-diff の MISS) by @yagihash in https://github.com/yagihash/rivus/pull/485
- model: dart:io の FileSystemEntity.path を足す(package:file の委譲で流れが注記なしで消える、#213) by @yagihash in https://github.com/yagihash/rivus/pull/486
- model: net/url.URL.Port の MISS に non-payload の miss-reviewed を付ける by @yagihash in https://github.com/yagihash/rivus/pull/487
- bench: graph-check に -func-timeout 24h を付け、時計に依存する実行を LOAD DEPENDENT と示す by @yagihash in https://github.com/yagihash/rivus/pull/492

## [v0.0.14](https://github.com/yagihash/rivus/compare/v0.0.13...v0.0.14) - 2026-11-23

### Features
- perf: Texts が依存の構文木を持たず、位置だけを持つ by @yagihash in https://github.com/yagihash/rivus/pull/401
- flags: 既定で off の機能の一覧を、コードとテストで持つ by @yagihash in https://github.com/yagihash/rivus/pull/400
- perf: lifted を NaiveForm より先に作って手放す(B の土台) by @yagihash in https://github.com/yagihash/rivus/pull/402
- accept: Go の extractor の受け入れ条件のテストの部品を足す by @yagihash in https://github.com/yagihash/rivus/pull/404
- accept: 読み込みの順を変えて、結果が同じことを確かめるテスト by @yagihash in https://github.com/yagihash/rivus/pull/409
- ir: Source を入れ、IR を読む側をその口に通す(B0。結果は同一) by @yagihash in https://github.com/yagihash/rivus/pull/410
- sites: IRRef.Func を関数の参照(ID と通し番号)に替える(B0) by @yagihash in https://github.com/yagihash/rivus/pull/411
- accept: 解析の finding、uncertainty、呼び出しのエッジを比較するテスト by @yagihash in https://github.com/yagihash/rivus/pull/412
### Bug Fixes
- goir: 同名の型の印を、読み込みの順に依らない形にする by @yagihash in https://github.com/yagihash/rivus/pull/403
- gossa: 関数の並べ替えの決め手を、Pos の数でなく位置にする by @yagihash in https://github.com/yagihash/rivus/pull/408

## [v0.0.13](https://github.com/yagihash/rivus/compare/v0.0.12...v0.0.13) - 2026-11-23

### Features
- 決定論のテストで収束を要求し、compare.py に完全でない実行の印を出す by @yagihash in https://github.com/yagihash/rivus/pull/361
- 関数ごとの打ち切りも、完全でない解析(終了コード 4)として扱う by @yagihash in https://github.com/yagihash/rivus/pull/365
- summary にメモリの最大と GC の割合を足す。models coverage に incomplete を出す by @yagihash in https://github.com/yagihash/rivus/pull/359
- 重い測定の記録を 1 つの台帳(bench/heavy-ledger.tsv)にまとめる by @yagihash in https://github.com/yagihash/rivus/pull/366
- sg: 同じ内容の InstrInfo を 1 つにまとめて、生きているヒープを減らす by @yagihash in https://github.com/yagihash/rivus/pull/367
- taint: 経路の段の記録を小さくして、生きているヒープを減らす by @yagihash in https://github.com/yagihash/rivus/pull/371
- 経路の段に、畳んだ出力から作った段の注記(out_widened)を付ける by @yagihash in https://github.com/yagihash/rivus/pull/370
- summary に、生きているヒープの最大(live_heap_max_bytes)を足す by @yagihash in https://github.com/yagihash/rivus/pull/372
- IR に char_read の属性を足し、isCharRead を属性と今の判定の 2 段にする by @yagihash in https://github.com/yagihash/rivus/pull/374
- bench: 規模を変えたときの、段ごとの生きている量の伸びを測る道具 by @yagihash in https://github.com/yagihash/rivus/pull/380
- bench: Go の zoo で、実行して観測したエッジと静的グラフを照合する(observed-edges) by @yagihash in https://github.com/yagihash/rivus/pull/381
- stats に、関数ごとの作業量(adds)の分布を足す(動作は変えない) by @yagihash in https://github.com/yagihash/rivus/pull/382
- goir: 読み込みの山を下げる(ブロックの配列をちょうどの大きさに、変換が済んだ関数の SSA を手放す) by @yagihash in https://github.com/yagihash/rivus/pull/383
- 合格基準の「検出」を、場所で判定する(memos)。検出と終わることを別の条件に by @yagihash in https://github.com/yagihash/rivus/pull/387
- goir: lifted な SSA を、変換の前に表へ直して手放す(読み込みのフェーズ) by @yagihash in https://github.com/yagihash/rivus/pull/391
- 暫定の注記: 依存の中で文字を 1 文字ずつ読んで止まった流れを、呼び出しの側に注記として出す by @yagihash in https://github.com/yagihash/rivus/pull/388
- フラグ -char-taint(既定 off): 文字として読んだ値を、文字列に戻る所まで運ぶ(Go) by @yagihash in https://github.com/yagihash/rivus/pull/389
- irspill: IR の符号化と保存(名前のない一時ファイル)を足す。まだ繋がない by @yagihash in https://github.com/yagihash/rivus/pull/397
### Bug Fixes
- Dart: ローカル関数が自分の名前で自分を呼ぶ形を、自分を作り直して扱う(#196) by @yagihash in https://github.com/yagihash/rivus/pull/377
- Dart: クロージャの中で、取り込んだ変数を呼ぶ形の候補を、クロージャを作った所からたどる(#99) by @yagihash in https://github.com/yagihash/rivus/pull/390

## [v0.0.12](https://github.com/yagihash/rivus/compare/v0.0.11...v0.0.12) - 2026-11-23

### Features
- zoo のゲート: Rust の lib.rs のマークを見る。@todo(PARITY)は承認なしで通す by @yagihash in https://github.com/yagihash/rivus/pull/336
- 完全でない解析(時間切れ、未収束)は、結果を出したうえで終了コード 4 で終わる by @yagihash in https://github.com/yagihash/rivus/pull/353
- 問い合わせ(query、resolve)の出力の最上位に、incomplete と incomplete_reasons を出す by @yagihash in https://github.com/yagihash/rivus/pull/356
- -max-memory を足し、指定がないときのメモリの上限を物理メモリの 1/4 にする by @yagihash in https://github.com/yagihash/rivus/pull/354
### Dependencies
- fix(deps): update dependency path to ^1.9.1 by @renovate[bot] in https://github.com/yagihash/rivus/pull/345
- chore(deps): update actions/setup-go action to v7.1.0 by @renovate[bot] in https://github.com/yagihash/rivus/pull/347
- fix(deps): update golang.org/x packages by @renovate[bot] in https://github.com/yagihash/rivus/pull/348
- chore(deps): lock file maintenance by @renovate[bot] in https://github.com/yagihash/rivus/pull/351
- renovate: minimumReleaseAge を 3 日から 7 日にする by @yagihash in https://github.com/yagihash/rivus/pull/352
- fix(deps): update dependency yaml to ^3.1.4 by @renovate[bot] in https://github.com/yagihash/rivus/pull/346
- renovate: Rust の nightly を Renovate で更新する。日付が必要でない記述は、日付を消す by @yagihash in https://github.com/yagihash/rivus/pull/355
- renovate: 同じ値を 2 つのマネージャが見ている 2 か所(Rust の nightly、Dart の SDK)を、1 つにする by @yagihash in https://github.com/yagihash/rivus/pull/357
- chore(deps): update dependency ubuntu to v26 by @renovate[bot] in https://github.com/yagihash/rivus/pull/349
- chore(deps): pin go module directive to 1.27.4 by @renovate[bot] in https://github.com/yagihash/rivus/pull/343
- fix(deps): update module go.yaml.in/yaml/v3 to v4 by @renovate[bot] in https://github.com/yagihash/rivus/pull/350

## [v0.0.11](https://github.com/yagihash/rivus/compare/v0.0.10...v0.0.11) - 2026-11-23

### Features
- feat(model): 一括採用の展開(段階 2)。決定から行を決まった形で生成する by @yagihash in https://github.com/yagihash/rivus/pull/278
- feat(model): 層 1 の resource_name の語に base、head、tag を足す by @yagihash in https://github.com/yagihash/rivus/pull/279
- feat: make merge で、開発者がマージしてよい PR だけをマージする by @yagihash in https://github.com/yagihash/rivus/pull/280
- ci: main の実行の取り消し、zoo-loosened、darwin の vet、文書のテストの穴を直す by @yagihash in https://github.com/yagihash/rivus/pull/283
- feat(model-diff): Rust のモデルの行の綴りを、extractor の綴りと照合する(ローカル) by @yagihash in https://github.com/yagihash/rivus/pull/284
- test: 追跡されたファイルに実行ファイルと 1 MB を超えるものがないことを確かめる by @yagihash in https://github.com/yagihash/rivus/pull/289
- bench: 依存の重い対象のベンチと LEDGER を足し、エンジン側のコストの状況をまとめる by @yagihash in https://github.com/yagihash/rivus/pull/292
- bench: memos と dart_services を対象に足し、合格の線を 1 行で出す by @yagihash in https://github.com/yagihash/rivus/pull/293
- bench: Dart の的を cocoon app_dart にして、合格の線と起動の案内を足す by @yagihash in https://github.com/yagihash/rivus/pull/296
- feat: ポインタの別名で近似した経路の finding に、不確実性の注記を付ける by @yagihash in https://github.com/yagihash/rivus/pull/295
- test: 変更したモデルのパッケージに差分テストを PR で回し、pure・sanitizer・source の行も比べる by @yagihash in https://github.com/yagihash/rivus/pull/291
- bench: 合格の線の検出を脆弱性の単位にし、位置の単位の数を並べて出す by @yagihash in https://github.com/yagihash/rivus/pull/301
- feat: 依存の関数だけアクセスパスの深さを下げる実験(フラグ、既定は変えない)と、診断を足す by @yagihash in https://github.com/yagihash/rivus/pull/302
- feat(model): アーカイブのエントリ名を source にする(Go の標準ライブラリと Dart の tar) by @yagihash in https://github.com/yagihash/rivus/pull/303
- feat(model): package:archive の 3.x の配置の行を足す by @yagihash in https://github.com/yagihash/rivus/pull/304
- bench: Dart の検出の的に archive(CVE-2023-39139)を足す by @yagihash in https://github.com/yagihash/rivus/pull/306
- bench: 時計で止めた関数がある実行を、合格の線の収束に数えない by @yagihash in https://github.com/yagihash/rivus/pull/310
- feat: 値がアクセスパスの切れた所を通ったとき、その段に注記(access_path_truncated)を付ける by @yagihash in https://github.com/yagihash/rivus/pull/309
- bench: 合格の線のメモリを終了時のピークにし、見張りと記録を分ける by @yagihash in https://github.com/yagihash/rivus/pull/313
- bench: 合格のメモリをプロセスの木の合計にし、本体のピークと並べて記録する by @yagihash in https://github.com/yagihash/rivus/pull/315
- bench: 検出の条件を detected と quiet_after_fix に分ける(#293 のマージのあとに push して入らなかった分の作り直し) by @yagihash in https://github.com/yagihash/rivus/pull/319
- tooling: マージ済みの PR のブランチへの push を、pre-push のフックで止める by @yagihash in https://github.com/yagihash/rivus/pull/320
- feat: 依存の関数のアクセスパスの深さの既定を 2 にする(自分のコードは 3 のまま。`-dep-k 3` で前の動作) by @yagihash in https://github.com/yagihash/rivus/pull/316
- tooling: compare.sh に時間とメモリの前後を足し、エンジンの PR の決まりに入れる by @yagihash in https://github.com/yagihash/rivus/pull/322
- マージの道具は、main の最新の CI が失敗しているとマージしない by @yagihash in https://github.com/yagihash/rivus/pull/326
- 重い実行の道具は、マシン全体で 1 つのロックを取る by @yagihash in https://github.com/yagihash/rivus/pull/327
- zoo: #290 の近くの形(添字のスライス、range のスライス、値のコピー)のセルを足す by @yagihash in https://github.com/yagihash/rivus/pull/330
- perf: 解析の割り当てを減らす(結果は変えない) by @yagihash in https://github.com/yagihash/rivus/pull/321
- 出力の widening を、不確実性 out_limit: と stats に記録する(#186 の 1) by @yagihash in https://github.com/yagihash/rivus/pull/332
- feat: 数値の型で汚染を止めた場所を、関数ごとに summary に出す by @yagihash in https://github.com/yagihash/rivus/pull/312
### Bug Fixes
- fix: 同じ module と path 依存のパッケージを、プロジェクトとして扱う by @yagihash in https://github.com/yagihash/rivus/pull/287
- fix: 結果が 2 つ以上ある命令で得たポインタ経由の書き込みを追う by @yagihash in https://github.com/yagihash/rivus/pull/290
- fix(model): pure の行の流れを本体に合わせ、確認できない行を減らす by @yagihash in https://github.com/yagihash/rivus/pull/298
- fix: 関数値を渡した、モデルのない呼び出しで、返り値が結果に届くと見なす by @yagihash in https://github.com/yagihash/rivus/pull/297
- fix(model): sanitizer の効く範囲を種類ごとに分け、コールバックを持つ pure の行を直す by @yagihash in https://github.com/yagihash/rivus/pull/294
- test: 1 文字ずつ読み書きするコードの流れの見逃しを、zoo のセルと課題に固定する by @yagihash in https://github.com/yagihash/rivus/pull/311
- main の CI を直す: TestScaleSynth の期待値を直し、PR でも実行する by @yagihash in https://github.com/yagihash/rivus/pull/324
- 解析する関数の集合を閉じる: 実行ごとに変わっていた標準ライブラリの関数をそろえる by @yagihash in https://github.com/yagihash/rivus/pull/329

## [v0.0.10](https://github.com/yagihash/rivus/compare/v0.0.9...v0.0.10) - 2026-11-23

### Features
- feat(model): まとめての採用(adopt_rules)の書式とローダー(段階 1) by @yagihash in https://github.com/yagihash/rivus/pull/267
- feat(model): まとめた行の decision に、重なった決定の id を全部書く by @yagihash in https://github.com/yagihash/rivus/pull/270
- ci: test のジョブを、arm64 の Linux のランナーで動かす by @yagihash in https://github.com/yagihash/rivus/pull/274
- perf(test): 重いテストの、同じ解析や exe のコンパイルの繰り返しを減らす by @yagihash in https://github.com/yagihash/rivus/pull/275

## [v0.0.9](https://github.com/yagihash/rivus/compare/v0.0.8...v0.0.9) - 2026-11-23

### Features
- feat: Rust でも summary.unverified_versions を出す(Cargo.lock から。#162 を閉じる) by @yagihash in https://github.com/yagihash/rivus/pull/237
- feat(model): make model に -gen-skeleton を足す(Go の骨組みとセルのテンプレートを生成する。段階 2) by @yagihash in https://github.com/yagihash/rivus/pull/238
- feat: Dart でも summary.unverified_versions を出す(pubspec.lock から。#162 の Dart の分) by @yagihash in https://github.com/yagihash/rivus/pull/236
- feat(model): 骨組みの生成を、モデルの行が指すものだけに絞る by @yagihash in https://github.com/yagihash/rivus/pull/241
- feat(model): モデルを作る手順のコマンド(段階 1。Go だけ)と rejected: の節 by @yagihash in https://github.com/yagihash/rivus/pull/233
- feat(model): dio の MultipartFile.fromFile と fromFileSync を path_traversal の sink にする by @yagihash in https://github.com/yagihash/rivus/pull/245
- ci: rust のジョブのテストを -v で実行する(テストごとの時間を測るため) by @yagihash in https://github.com/yagihash/rivus/pull/250
- perf: Dart の extractor を、コンパイルした実行ファイルで動かす(テストと利用者の起動の時間を減らす) by @yagihash in https://github.com/yagihash/rivus/pull/252
- feat(rust): 宣言のあるパッケージの本体を extractor が読まない(Rust の段階 4) by @yagihash in https://github.com/yagihash/rivus/pull/254
- chore: make check に TestDogfood を足す(capture_unbound の上限を、push の前に知る) by @yagihash in https://github.com/yagihash/rivus/pull/253
- feat(model): make model を Dart にも広げる(段階 1) by @yagihash in https://github.com/yagihash/rivus/pull/243
- feat(model): dart の googleapis の BigQuery で、テーブルのビュー、マテリアライズドビュー、ルーチンの SQL を sql_injection の sink にする by @yagihash in https://github.com/yagihash/rivus/pull/257
- feat: analyze にカテゴリで finding を含める・外すフラグを足す(外した件数は summary に出す) by @yagihash in https://github.com/yagihash/rivus/pull/259
- test(model): dart github の RepositorySlug の見逃しを @miss のセルにする by @yagihash in https://github.com/yagihash/rivus/pull/258
- ci: test、dart、rust のジョブを 1 種類のシャードにまとめ、どのテストも 1 回だけ実行する by @yagihash in https://github.com/yagihash/rivus/pull/260
### Bug Fixes
- fix: 確認していないバージョンの照合を、言語ごとの記録とだけ行う by @yagihash in https://github.com/yagihash/rivus/pull/240
- fix: サンドボックスの下で Rust のビルドスクリプトが動くように、DEVELOPER_DIR をコマンドラインツールに向ける by @yagihash in https://github.com/yagihash/rivus/pull/256

## [v0.0.8](https://github.com/yagihash/rivus/compare/v0.0.7...v0.0.8) - 2026-11-23

### Features
- feat: 差分テスト(段階 1)。bench/model-diff と make model-diff(Go と Dart) by @yagihash in https://github.com/yagihash/rivus/pull/186
- feat: 宣言のあるパッケージの本体を、extractor が読まない(Go と Dart。段階 4、ADR-105) by @yagihash in https://github.com/yagihash/rivus/pull/191
- fix: body_replaced_packages の functions の意味を段階で変えない(functions_declared を分ける)。ADR-105 と設計の実測を直す by @yagihash in https://github.com/yagihash/rivus/pull/193
- feat: モデルのファイルの verified_versions を、ローダーのフィールドにする(rivus models versions) by @yagihash in https://github.com/yagihash/rivus/pull/192
- feat: source の新しい種類 callback_param(渡された関数の引数が source。段階 1、ADR-106) by @yagihash in https://github.com/yagihash/rivus/pull/197
- feat(zoo): 外部のパッケージのセルを、ディレクトリから列挙する(Go と Rust)。Dart の pubspec を生成物にする by @yagihash in https://github.com/yagihash/rivus/pull/198
- shelf_router: ハンドラの追加の引数を callback_param の source にする(#181 の @miss を解消) by @yagihash in https://github.com/yagihash/rivus/pull/199
- feat: callback_param が引けなかった登録を summary に出す(callback_param_unresolved) by @yagihash in https://github.com/yagihash/rivus/pull/200
- feat(model): dart dio の組み込みモデル by @yagihash in https://github.com/yagihash/rivus/pull/202
- feat(model): go gin の組み込みモデル(例から昇格) by @yagihash in https://github.com/yagihash/rivus/pull/208
- feat(zoo): 外部のパッケージの骨組みが zoo-mark で記録する(Go と Rust) by @yagihash in https://github.com/yagihash/rivus/pull/209
- feat(model): dart shelf のモデルを広げる by @yagihash in https://github.com/yagihash/rivus/pull/207
- feat: パッケージの calls: の行を、実行でなく zoo のセルで確認する(checked_by: cell) by @yagihash in https://github.com/yagihash/rivus/pull/206
- feat(model): dart github の組み込みモデル by @yagihash in https://github.com/yagihash/rivus/pull/212
- feat(model): rust actix-web の組み込みモデル(例から昇格) by @yagihash in https://github.com/yagihash/rivus/pull/204
- feat(model): rust axum のモデルを広げる by @yagihash in https://github.com/yagihash/rivus/pull/213
- feat(model): go cloud.google.com/go/storage の組み込みモデル by @yagihash in https://github.com/yagihash/rivus/pull/214
- feat(model): go jmoiron/sqlx のモデルを広げる by @yagihash in https://github.com/yagihash/rivus/pull/216
- feat(model): rust google-cloud-storage の組み込みモデル by @yagihash in https://github.com/yagihash/rivus/pull/217
- feat(model): dart googleapis_auth の組み込みモデル by @yagihash in https://github.com/yagihash/rivus/pull/222
- feat(model): rust octocrab の組み込みモデル by @yagihash in https://github.com/yagihash/rivus/pull/224
- fix(sites): Rust の型名(`::` 区切り)で `kind: field` の source のキーを作る(#161) by @yagihash in https://github.com/yagihash/rivus/pull/230
- feat(zoo): Dart の骨組みを依存として読む(--dep-dir)。dart-github の kind: field の source を検出にする by @yagihash in https://github.com/yagihash/rivus/pull/231
- feat(rust): extractor が変換で落ちた関数を飛ばして続ける(fn_skipped、summary.functions_skipped、ADR-109。課題 #160 の (b)) by @yagihash in https://github.com/yagihash/rivus/pull/226
- feat(model): Go のモデルの行に、module の path の major を `/v*` と書けるようにする by @yagihash in https://github.com/yagihash/rivus/pull/228
- feat: 確認していない major / バージョンに当てたとき、summary.unverified_versions に出す(Go。#228 のあと) by @yagihash in https://github.com/yagihash/rivus/pull/229
- feat(model): go google/go-github の組み込みモデル by @yagihash in https://github.com/yagihash/rivus/pull/225
- # chore: rust-web.yaml をパッケージごとのファイルに分ける by @yagihash in https://github.com/yagihash/rivus/pull/235
### Bug Fixes
- test: dead-allow の「一致した」指摘を、通常は記録だけにする(モデルの PR が共有の .dead-allow を直さなくてよい) by @yagihash in https://github.com/yagihash/rivus/pull/187
- fix(model): 差分テストで見つかった標準ライブラリのモデルの MISS を直す(json、url、flag、template ほか)。miss-reviewed の注釈 by @yagihash in https://github.com/yagihash/rivus/pull/195
- fix(model): 差分テストの続き。miss-reviewed を種類つきに絞る、写しのビルド(wasm)の修正、標準ライブラリのモデルの追加の修正(課題 #150〜#155) by @yagihash in https://github.com/yagihash/rivus/pull/201
- fix(model): database/sql の Conn と Tx.Prepare の SQL 文字列を sink にする by @yagihash in https://github.com/yagihash/rivus/pull/219
- fix(rust): 依存のクレートのコンパイルで落ちたとき、クレート名とバージョン、依存の側で落ちたこと、-no-deps を出す(課題 #160 の (a)) by @yagihash in https://github.com/yagihash/rivus/pull/220
- feat(model): 標準ライブラリ os の、行がない関数に行を足す by @yagihash in https://github.com/yagihash/rivus/pull/223

## [v0.0.7](https://github.com/yagihash/rivus/compare/v0.0.6...v0.0.7) - 2026-11-22

### Features
- feat: -pass-stats で、finding の注記が集中するシンボルの上位と、動的な呼び出しの候補の数の分布を出す by @yagihash in https://github.com/yagihash/rivus/pull/156
- feat(model): package:http の Client の組み込みモデル(dart-http.yaml)と、委譲のクリーク全体をモデルで切る基準(ADR-098) by @yagihash in https://github.com/yagihash/rivus/pull/160
- feat(model): Google Cloud の Dart のクライアントの組み込みモデル(dart-gcp.yaml: Storage、Pub/Sub、Datastore、BigQuery の sink) by @yagihash in https://github.com/yagihash/rivus/pull/162
- feat(model): 名前つき引数を指す sink(named:)と、1 つのシンボルの複数カテゴリの sink(課題 #139、#140) by @yagihash in https://github.com/yagihash/rivus/pull/169
- feat: summary.dependency_sites で「依存の中の source と sink は探していない」ことを言う(ADR-099)。言い回しの規則の追加 by @yagihash in https://github.com/yagihash/rivus/pull/165
- feat: finding に categories(同じ位置を読む行の全部のカテゴリ)を足す。flows / sources の named を課題 #145 にする by @yagihash in https://github.com/yagihash/rivus/pull/176
- chore: モデルの PR が並んで出ても conflict しない置き方(marks.d、dead-allow の分割、cases.dart の生成、課題の model-*)と、ゲートの @ref の確認 by @yagihash in https://github.com/yagihash/rivus/pull/178
- feat: パッケージ単位の宣言(body_replaced_packages)。モデルが代わりをするパッケージは、解析の前に本体を落とす(段階 2、ADR-100) by @yagihash in https://github.com/yagihash/rivus/pull/179
### Bug Fixes
- fix: origin が builtin でないモデルの pure と、結果に届かない flows が流れを消した場所を、suppressions に記録する by @yagihash in https://github.com/yagihash/rivus/pull/161
- fix: models coverage のテンプレートと組み込みの行で、複数の戻り値の関数に to: results を使う(to: result は最初の戻り値だけ) by @yagihash in https://github.com/yagihash/rivus/pull/163
- fix: models coverage のテンプレートで、インターフェース、slice、map のレシーバへの書き込みの行も書く by @yagihash in https://github.com/yagihash/rivus/pull/164
- fix(ci): zoo-loosened を merge-base との差分で見て、新しいセルの @sink(none) / @miss / @fp の行の追加は承認の行を求めない by @yagihash in https://github.com/yagihash/rivus/pull/167

## [v0.0.6](https://github.com/yagihash/rivus/compare/v0.0.5...v0.0.6) - 2026-11-22

### Features
- feat: -pass-stats の summary_sizes に、source の箇所の広がりと、グローバルが持つ要素の数を足す by @yagihash in https://github.com/yagihash/rivus/pull/153
- feat: rivus completion zsh(コマンドの表から生成する zsh の補完。サブコマンドとフラグに自動で追従する) by @yagihash in https://github.com/yagihash/rivus/pull/155
### Bug Fixes
- fix: -h はどのコマンドでも終了コード 0 にする(課題 #131) by @yagihash in https://github.com/yagihash/rivus/pull/152

## [v0.0.5](https://github.com/yagihash/rivus/compare/v0.0.4...v0.0.5) - 2026-11-22

### Features
- feat: index -taint の既定の書き先を利用者のキャッシュにする(互換性を壊す)と、graph が記録された -rules を使う by @yagihash in https://github.com/yagihash/rivus/pull/149
- feat: -pass-stats に、要約の大きさの分布と、要素の内訳(stats.summary_sizes)を足す by @yagihash in https://github.com/yagihash/rivus/pull/150
### Bug Fixes
- fix: rivus sites が analyze と同じ言語の選択を通す(Dart と Rust のディレクトリが空の結果にならない) by @yagihash in https://github.com/yagihash/rivus/pull/145
- fix(dart): dart pub get をしていない(または古い)パッケージを、標準エラーの専用の 1 行で言う by @yagihash in https://github.com/yagihash/rivus/pull/151

## [v0.0.4](https://github.com/yagihash/rivus/compare/v0.0.3...v0.0.4) - 2026-11-22

### Features
- feat: -resolve-func-values(候補 0 件の値の呼び出しに、関数値のパスで候補を補う。既定 off) by @yagihash in https://github.com/yagihash/rivus/pull/120
- feat: -resolve-func-values で補った辺を索引に入れ、-timeout を全部の回で共有する by @yagihash in https://github.com/yagihash/rivus/pull/126
- feat: -pass-stats-top と -pass-stats-sort、最大の SCC のパッケージ別の内訳、巨大な SCC の中でも -timeout を効かせる by @yagihash in https://github.com/yagihash/rivus/pull/135
- feat: rivus extract を rivus debug extract に移し、-h は普段使うフラグだけを出す by @yagihash in https://github.com/yagihash/rivus/pull/136
### Bug Fixes
- fix(dart): 複数のパッケージの親を -C に指しても、依存が package: の ID で解析される(サイレントな読み落とし) by @yagihash in https://github.com/yagihash/rivus/pull/133
- fix(dart): 許可の外の依存を読まなかったことを、summary.unread と標準エラーに出す by @yagihash in https://github.com/yagihash/rivus/pull/139
- fix(dart): 同じ依存の 2 つのバージョンの報告に、どちらが読まれて解析されたかを書く by @yagihash in https://github.com/yagihash/rivus/pull/140

## [v0.0.3](https://github.com/yagihash/rivus/compare/v0.0.2...v0.0.3) - 2026-11-22

## [v0.0.2](https://github.com/yagihash/rivus/compare/v0.0.1...v0.0.2) - 2026-11-22

### Features
- feat: -lang なし(auto)でも、聞いたうえで Rust を読む。読まなかった言語は結果に残す by @yagihash in https://github.com/yagihash/rivus/pull/107
### Bug Fixes
- fix(taint): -workers 1 も SCC を回ごとに解析し、出力が -workers に依らないようにする(#96) by @yagihash in https://github.com/yagihash/rivus/pull/101
### Dependencies
- ci: Renovate を設定する(Go・Rust・Dart の依存と Actions) by @yagihash in https://github.com/yagihash/rivus/pull/106

## [v0.0.1](https://github.com/yagihash/rivus/commits/v0.0.1) - 2026-11-22
