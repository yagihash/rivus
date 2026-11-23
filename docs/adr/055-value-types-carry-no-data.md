# ADR-055: 値の型は、値のデータを運ばない（`reflect.TypeOf` などのモデル）（採用）

- 状態: 有効
- 関連: -
- 状態の詳細: 採用; 訂正・追記あり(書き方を保つ)。仮説履歴
- 要旨: 仮説の履歴を本文に残す(収束の原因探索)

**背景:** Gitea（`-deps`）の解析は、30 分で収束しなかった（5〜6 パス、要素が増え続ける）。gogs は、`*net/http.Request` 型の引数が source になる、型ベースの source が、request と無関係な sink（`healthCheck` の `fmt.Fprintf`、`database` の各 sink）に届く 12 件の finding を出していた。追跡（`RIVUS_TRACE_ELEM`）で、この source が、グローバルに入る直前の命令を見つけた。`proto.encodeExtension` が `reflect.TypeOf(v)`（モデルなし）を呼び、その結果を `getMarshalInfo` に渡して、`proto.marshalInfoMap` に store する。`(*TextMarshaler).writeStruct` が `(reflect.Value).Type()`（モデルなし）を呼び、`proto.propertiesMap` の map key にする。protobuf の `impl.aberrantMessageDescCache` も同じ形。モデルも本体もない呼び出しは、「引数から結果へ流れる」とされるので、値のデータが、その値の型の記述に載り、型ごとのキャッシュ（グローバル）を読む全部の関数へ広がった。

**決めたこと:** 値の**型の同一性**は、汚染として追わない。
- 型を返すもの: `reflect.TypeOf`、`reflect.TypeFor`、`(reflect.Value).Type`、`(reflect.Type).String`、`(reflect.Type).PkgPath` は `pure: true`。
- 数や真偽を返すもの: `(reflect.Value).Len` / `Kind` / `IsNil` / `IsValid` / `NumField` は、`recv → result` のモデル（止めるのは `numeric_stop` の規則で、止めた場所が記録され、`-numeric-taint` で戻せる。同じ判断を 2 か所でしない）。
- データを運ぶもの（`reflect.ValueOf`、`(reflect.Value).Interface` / `String` / `Bytes` / `Index` / `Field` / `MapIndex`）は、触らない。
- 範囲外: デシリアライズで「どの型を作るか」を攻撃者が選ぶ形（型の選択そのもの）は、追わない。（この方針は、ユーザーが承認した（C-12、2026-11-21）。）

**測定（2026-11-21）:**
- gogs（`bench/gogs/rules.yaml`、新しいバージョン）: finding 12 → 1（残るのは `bind.go:116`）、パス 6 → 3、taint 19.9 秒 → 9.3 秒、uncertainty 2,900 → 2,463。11 件は、経路が、型のキャッシュ（`proto.*Map`、`aberrantMessageDescCache`）のグローバルを通る形だった。
- Gitea（`-deps`、`bench/gitea/rules.yaml`、`absorbWhole` なし、callouts なし）: **収束**（8 パス、145 秒、最大 RSS 13.2 GB、finding 37）。モデルなしは、30 分で収束しなかった。`stats.pass_changes` で、新しい要素が、パスごとに 124K、470K、277K、252K、171K、64K、15K、100 と減り、戻ってきた要素はほぼ 0。
- CVE 3 件（`bench/gitea/truth-v1.26.4.json`）の再現率は、モデルの前後で、どの出力も 1/3（`oauth.go:313` だけ。`release.go:338` と `openid.go:26` は、モデルとは関係なく、今の main で、見つからない）。
- 正解のあるバージョンでの再現率（`bench/compare.sh 2dd5022 7391fa1`、`bench/score.py`）: gogs-9a9388ac（TP 4、FN 0）、gogs-48946299（TP 2、FN 0）、gin-vue-admin（TP 2、FN 0）、go-test-bench（TP 18、FN 0、FP 2、TN 1）。前後で同じ。追加された finding は 0 件。削除は、gogs-48946299 の 21 件（同じ sink `misc/markdown.go:16` は残る。経路は 183〜191 ステップで、5 または 14 番目のステップで protobuf の型の処理に入る）と、go-curated の 2 件（新しいフィクスチャの `@sink(none)`）。
- 本物の v1.26.4 の Gitea での測り直しは、別に記録する（ここまでの Gitea の数字は、2026-11-18 の main の木で取った: `bench/README.md` の注意）。

**仮説の履歴（Gitea が収束しない原因。次に同じ形の問題が出たとき、要素の追跡と `stats.unmodeled_top` から始められるように）:**
1. 巨大な SCC（3,370 関数）が、反復を遅くする → 案 2（コールアウト、ADR-047）で、最大 SCC は 3,134（事前測定は 322）にしか割れず、収束には効果がなかった（収束は、モデルで足りた）。
2. 関数値の呼び出しの、中継（`WithTx` → `txWithNoCheck`）で、ガードが外へ伝わらず、渡していない関数へ漏れる（測定: 直接渡す形の適用の 97 %）→ 実在する漏れだが、収束の原因ではなかった（ガードを上限の外に置いても、新しい流れの増え方は変わらない）。
3. source が、同じパラメータの `OutParam` に書き戻される（C-1、ADR-048）→ 直したが、Gitea は収束しなかった。
4. 要約の join が単調でない（`widen` のあと、要素が往復する）→ 実在した（gogs の protobuf の `RegisterFile`）が、Gitea の増加は往復ではなく、新しい要素だった（`returned_elems` ≒ 0）。`absorbWhole`（別のコミット）で直した。
5. **型のキャッシュを通る合流（主因）**: `reflect.TypeOf` / `(reflect.Value).Type` が引数から結果へ流れ、型のキャッシュ（グローバル）を読む全部へ広がる。
6. 次の合流点: `fmt.Fprintf(os.Stderr, x)` が、`os.Stderr` の指す先を汚染し、`log.New(os.Stderr, ...)` で全ロガーが運ぶ（380 個の request の source が `certmagic` のグローバルまで）。別の ADR（`globals:` の `holds: nothing`）。

**限界:** 型の記述から、型の名前（`(reflect.Type).String`）を作って、攻撃者が選んだ型名が、sink に入る形（型の選択の注入）は、追わない。
