# #212 メモリ: 依存の関数が作る uncertainty(出力に出ないもの)が、ピークのヒープの 25%(`newUncertainty`)と、その詳細の文字列を持つ

- 状態: **open**(設計メモだけ。実装はまだ)
- 優先度: 高(`dart_services` のピークのヒープの約半分)
- 種類: 道具・ベンチ(メモリ)
- 言語: 全(確認したのは Dart の `dart_services`。Go も同じ経路)
- ROADMAP の節: -
- 守るもの: `--graph-check`(`bench/compare.sh --graph-check`)で前後が同じこと、`TestCuratedGo$` / `TestCuratedDart`、zoo
- 題: uncertainty の重複の共有・intern の見積もりと、推奨(作らない・持たない)

関連: #474(`applySummary` の割り当て削減。下の 4 節: 同じ行の文字列を扱う)、docs/PRINCIPLES.md(正確さが時間と資源に先立つ)。

## 1. 問い

`dart_services`(ピーク = 解析の最初の約 50 秒、ヒープ 3.57 GB)で、`taint.newUncertainty` が 907 MB(25%、1 位)。同じ中身の uncertainty がどれだけ重複しているか。共有(同じ中身は 1 つのオブジェクト)や intern(文字列)でどれだけ減るか。

## 2. 結論(先に)

**重複を共有する(案 a、b)より前に、作らない・持たない方が、桁が違う。** 理由は、ピークの 3.3 M 個の uncertainty の、ほとんど全部が、**出力に出ない**ものだから。

- `collect`(`internal/taint/engine.go`、`for _, u := range s.uncertain`)は、`fn.Dep`(依存の関数)の状態の `uncertain` を、**読む前に `continue` で飛ばす**。依存の関数が作った uncertainty は、最終の `uncertainties` に 1 件も入らない。
- それでも、`newNote`(`calls.go`)は、依存の関数でも毎回 `Uncertainty` を作って `s.uncertain` に足し、`kept()` が(`r.s = s.kept()`)結果に持たせ、大きい成分(`largest_scc` 13,983)の結果は、ラウンドが終わるまで全部持たれる。
- `dart_services` の解析の時間は、依存 3,773 s、自分のコード 6 s(`dep_spent_ms` / `own_spent_ms`)。作られる uncertainty も、ほとんどが依存の側。

## 3. 測ったこと

### 3.1 `dart_services` の heap プロファイル(`ld/out/peak.pb.gz`、`inuse_space` / `inuse_objects`)

| 項目 | 値 |
| --- | --- |
| `newUncertainty` | 907 MB、3.30 M 個(1 個 約 275 B。構造体 288 B の区分) |
| 呼び出し元 | `newNote` 895 MB のうち、`applySummary` 829 MB(93%)、`callBody` 63 MB、`pathNote` 43 MB |
| `applySummary` の行 464(`numeric_stop`、`char_read_in_callee`) | 構造体 2.23 M 個 + 詳細の文字列 2.23 M 個(flat 686 MB。1 個 約 307 B) |
| `applySummary` の行 439(`unresolved_callee`) | 構造体 0.88 M 個 + 詳細の文字列 0.88 M 個(flat 185 MB。1 個 約 211 B) |

**詳細の文字列は、`applySummary` の flat 889 MB の、ほぼ全部**(行 439 と 464 の 2 行で 871 MB)。つまり、#474 が対象にしている 889 MB と、**同じ呼び出しが作るもの**。構造体 907 MB と文字列 871 MB は、1 回の `newNote` 呼び出しが作る 2 つで、数える対象が同じ(重ねて数えない)。

### 3.2 出力側(`ld/out/out.json`)との差

最終の `uncertainties` は **878 件**、(kind, symbol, func, position)の一意は 876。内訳は dynamic_dispatch 478、dependency_body_unread 158、unmodeled_library_call 86、numeric_stop 55、assumed_call 39、depth_limit 34、unresolved_callee 27、missing_source 1。`detail` の一意は 106 通り、長さの中央値 43、90% 点 214、最大 377 文字。`candidates` は 573 件が持ち、合計 21,604 個、最大 13,895 個の 1 件(それ以外は小さい)。

ピークの 3.30 M 個に対して、出力は 878 件(0.03%)。ピークは途中(半分も進んでいない)だが、一意な組の数は単調に増えるので、**ピークの時点の一意な組は 878 件以下。99.97% 以上が、出力に出ないか、同じ組の重複**。これは「依存の関数のもの」と「同じ呼び出し点の再実行の重複」の和で、2 つの内訳は、`dart_services` を再実行しないと分けられない(重い実行はしない)。ただ、`collect` のコードから、依存の関数のものは、**どれだけ重複がなくても、出力に出ない**ので、持つ意味がない。

### 3.3 軽い対象で、作られた件数と一意な組(一時の測定。コミットしない)

`newNote` に一時の計測を入れて(`(kind, symbol, func, position)` と、`detail` の一意を数えた。計測は戻した)、軽い対象を実行した。

| 対象 | 作られた件 | 一意な組(kind, symbol, func, position) | 一意な detail | 重複の倍率 |
| --- | ---: | ---: | ---: | ---: |
| go-curated | 151 | 84 | 6 | 1.8 |
| dart-curated | 144 | 134 | 16 | 1.1 |
| zoo-dart | 734 | 669 | 32 | 1.1 |
| go-synth S | 69 | 25 | 3 | 2.8 |
| rivus 自身(`./internal/...`、`-no-deps`) | 1,770 | 1,057 | 25 | 1.7 |

- 小さい対象(依存を読まない、または浅い)では、重複は **1.1〜2.8 倍**。共有しても、構造体は半分にもならない。
- `(kind, symbol, func, position)` が同じなら、`detail` も同じだった(「組」と「組と detail」の数が、全対象で一致)。**同じ組の detail は同じ**(`dynamic_dispatch` の `call through an interface or function value` のような定型が多く、一意な detail は件数の 1〜10%)。
- 3.1 の巨大な倍率は、小さい対象には出ない。**依存を読む大きい対象で、依存の関数の数(`dart_services` は 27,933 関数、再解析 289,770 回)が、そのまま件数になる**ため。memos と dart-gcp は、重い実行になるので測らなかった。

## 4. 作られる所と持たれ方

- 作る: `newUncertainty`(`result.go`)の構造体は 約 288 B(`Kind` `AttributableTo` `Detail` `Symbol` `Func` `Snippet` `Basis` の文字列 7 本、`Candidates` `Operands` のスライス 2 本、`Position`、`pos`、`Recv`、`global` ほか)。大半は、出力のときに初めて埋まる(`fill`: `Position`、`Snippet`)か、まれにしか使わない。
- 呼び出し点(`s.newNote`): `unresolved_callee`(`applySummary`、`calls.go`、`summary.go`)、`dynamic_dispatch`(`calls.go`、`callrows.go`)、`unmodeled_library_call`、`depth_limit`、`numeric_stop`(`char_read_in_callee`)、`assumed_call` ほか約 40 か所。**戻り値を使う所(経路のステップの uncertainty として `why` に付ける)と、捨てる所**(`s.uncertain` に入るだけ。`summary.go` の 439、464、473 行、`calls.go` の 135、809、846 行、`bodyreplaced.go`、`transfer.go` ほか)がある。ピークの大半は、後者の 439 と 464。
- 持たれ方: `state.uncertain`(関数ごと)→ `kept()` → `e.states[fn]`(関数ごと 1 つ。再解析で置き換わる)。`collect` が、`Kind|Symbol|Func|posKey` で重複をなくして(最初の 1 件が残る)出す。`char_read_in_callee` は、さらに (呼ぶ関数, 呼ばれる関数) の組で 1 件にして、位置は最小のもの。**その並びに、`state.uncertain` を読む所は、`collect` と `keepEarlier`(前の訪問のものを足す)の 2 つだけ**。

## 5. 案と見込み

結果を変えない条件: 最終の `Result.Uncertainties` が、並びも中身も同じ(`collect` が、関数の ID 順、関数の中は `s.uncertain` の作った順、重複は最初の 1 件、と決めている)。`--graph-check` の same(順序と機械に依らない)。経路のステップの uncertainty(`Step.Notes`)は変えない。

### 案 0(推奨): 依存の関数は、出力に出ない uncertainty を持たない

- 0a: `kept()` が、`s.fn.Dep` の状態の `uncertain` を落とす(1 行)。結果を持つ間の保持(ラウンドの間、全関数の結果を持つ)が、なくなる。**`collect` が読まないものだけを落とすので、出力は同じ**(`keepEarlier` が足すのも、同じ `uncertain`)。
- 0b: 戻り値を捨てる呼び出し点では、依存の関数なら、**`newNote` も、詳細の文字列の連結も、しない**(`s.noteOnly(in, kind, symbol, func() string)` のような 1 つの入口に寄せる。または、呼び出し前に `if s.fn.Dep { skip }`)。作るコスト(構造体 + 文字列、約 3.3 M 回 ×2)と、GC の仕事が、なくなる。戻り値を使う所(経路のステップの uncertainty)は、そのまま。
- 見込み: **構造体 907 MB のうち、`applySummary` 経由の 829 MB はほぼ全部、`callBody` の 63 MB も戻り値を捨てる所なら全部。さらに、詳細の文字列 871 MB(上の 3.1)**。合計で最大 約 1.7 GB(ピーク 3.57 GB の約半分)。残る `pathNote` 43 MB は、戻り値を使うので別(#474 の対象)。**実測していない見込み**(重い実行をしないため)。実装後に `dart_services` のピークのプロファイルで確かめる。
- 大きさ: 小(0a は 1 行 + テスト。0b は、戻り値を捨てる約 15 か所を 1 つの入口に寄せる。約 40〜80 行)。
- 危険: `state.uncertain` を読む所が `collect` と `keepEarlier` だけ(4 節)という前提。今後、依存の関数の uncertainty を出す変更(たとえば、依存の中の不確実性を出力に出す)をするなら、この案と衝突する。**コメントで、「依存の関数の uncertainty は出力に出ない」と書いておく**。並列(`-workers`): 状態は関数ごとで、共有する変更がない(新しい競合なし)。決定性: 作るのを止めるだけで、並びは変わらない。

### 案 1: 自分のコードの関数の重複を、状態の中でなくす(0 の後に、まだ要るなら)

- 戻り値を捨てる所で、`(kind, symbol, position)` をキーにした状態ごとの map で、2 回目以降を作らない(最初の 1 件が残る = `collect` の重複をなくす規則と同じ。`Func` は状態で一定)。`char_read_in_callee` は、(symbol)をキーに、最小の位置に更新する(`collect` の規則を、作るときに行う)。
- 見込み: **自分のコードの関数の分は、`dart_services` で出力の 878 件以下**(0.03%)なので、0 の後に残る構造体は 0.3 MB 程度。重複の倍率は、軽い対象で 1.1〜2.8(3.3 節)で、効くのは、依存を読む大きい対象の自分のコードの側。**0 をやれば、実質足りる**。
- 大きさ: 小〜中。危険: 状態ごとの map なので、並列の競合なし。戻り値を使う所には使わない(同じ組で detail が違う場合に、経路のステップの uncertaintyが変わるため。軽い対象では同じ組の detail は同じだったが、保証はない)。

### 案 2: 共有(全体の intern。`sync.Map`、キーは (kind, detail, position))

- **推奨しない**。`Func` と `position` が状態の中身で、関数ごとに違うので、**関数をまたぐ共有はほとんど起きない**(同じ関数の再解析の結果は、置き換わって捨てられる)。`fill` が共有したオブジェクトに書く(`Position`、`Snippet`)ので、`-workers` で、書き込みの競合の確認が要る。得るものは、案 0、1 に包まれる。

### 案 3: `detail` の文字列の intern(案 b)

- 一意な detail は 106(878 件)なので、出力の側は 約 100 KB。**得るものがない**。構造体を作る前の連結の一時的な割り当てを減らすなら、0b(作らない)の方が根本。

### 案 4: 遅く作る(ID だけ持つ。案 c)

- **推奨しない**。`Uncertainty` は `Step.Notes`、finding、JSON に直接出る型で、構造体を中心に使われる。大きい変更になる。0 で、持つ数が出力の件数(878)まで落ちるので、要らない。

### 案 5: `candidates` の共有(案 d)

- **推奨しない**。出力に出る 573 件の合計が 21,604 個(最大 1 件 13,895 個 = 約 220 KB)。ピークの割り当てに出てこない大きさ。

## 6. 推奨と大きさ

1. **案 0(0a と 0b)を実装する**。大きさは小。ピークの約半分(最大 約 1.7 GB の見込み)を、**作らない・持たない**で減らす。**#474 と同じ 2 行(`summary.go` の 439、464)を触るので、#474 と同じ PR か、先に出す方を決めて、あとから入る方が合わせる**(#474 の「889 MB」と、この課題の「871 MB」は同じものを指す。二重に数えない)。
2. 案 0 の後に、`dart_services` のピークのプロファイルを取り直し、`newUncertainty` がまだ上位に残るなら、案 1(状態の中の重複をなくす)を足す。残らなければ、閉じる(案 2〜5 は、しない)。
3. 確認の方法: 結果が同じことは、`--graph-check`(`bench/compare.sh --graph-check dart-curated,zoo-dart,go-curated`)、`TestCuratedGo$` / `TestCuratedDart`、zoo、`rivus analyze` の JSON の、前後の比較(`uncertainties` の並びと中身)。メモリは、`dart_services` のピーク(開始 50 秒、`-memprofile` 相当)の比較(`bench/dart-services`、重い実行。担当に依頼)。

## 7. 限界(この文書の測り方)

- ピークの内訳の「依存の関数のもの」と「同じ呼び出し点の重複」の割合は、`dart_services` を再実行して(依存の関数の作った件数を数えて)分ける必要がある。この文書は、`collect` のコード(依存の関数の `uncertain` は出力に出ない)と、出力の 878 件を根拠に、**上限**を示した。
- 軽い対象の倍率(1.1〜2.8)は、依存を読まない小さい対象で、`dart_services` の再現ではない。memos と dart-gcp は、重いので測っていない。
- 構造体の大きさ 約 275 B は、ピークの `inuse_space` / `inuse_objects` の割り算(907 MB / 3.30 M)。
