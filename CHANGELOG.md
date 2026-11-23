# Changelog

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
