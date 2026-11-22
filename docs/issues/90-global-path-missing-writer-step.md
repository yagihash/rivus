# #90 グローバルを通る流れの経路から書いた側の段が消える(経路が読んだ側から始まる)。同じ長さの経路は関数 ID の順で選ばれる

- 状態: **open**
- 種類: 誤った経路
- 言語: 全
- ROADMAP の節: 2(経路の欠け・揺れ)
- 守るもの: -
- GitHub: #90(移行前の番号は同じ。2026-11-21 に開いた。移行時の状態: OPEN)

## 本文(GitHub から移した写し)

## 確度
実行で確認(param の source + `os.Args` の形)。グローバルを通る流れ一般で起きるかは推測。CORE-C は `internal/taint/engine.go` のコード読解。

## 場所
- `internal/taint/summary.go:105` 付近(「read from the global store, not produced here」)
- `internal/taint/engine.go` の `globalStore`、`commitGlobals`
- 経路の選び方: `internal/taint/engine.go:927` 付近の `collect`(`rf.steps.len() < cur.steps.len()` のときだけ置き換える)

## 再現(Go)
同じ issue 群の CORE-A の例で、`r` を `os.Args[2:]` にする(source は `runB` の param)。
実際: 経路は `main: call runA > runA: load > runA: call helper > sink` で、source の関数 `runB` が 1 度も出ない。`confidence` は `no_uncertainty_on_path`、`global_var` の不確実性もない。
一般の形: 関数 W がグローバルに source の値を書き、関数 R が読んで sink へ。

## 問題 1(CORE-B): 書いた側が経路に入らない
グローバルの格納に入った要素は、読む側の関数で、理由(why)を持たない。経路は読んだところから始まる。読み手に、書いた関数と位置が見えない誤った注記になる。

## 問題 2(CORE-C): 同じ長さの経路の選び方が関数 ID の順
(origin, sink) ごとに短い経路を残す。同じ長さなら、関数 ID の順で先に来たものが残る。CORE-B で段が欠けた経路は、正しい経路と同じ長さになり、`main` が先なので、欠けたほうが残る。

## 修正の方向
- CORE-B(方針は承認済み): 格納に入れるときに、書いた側の経路(少なくとも書いた関数と位置)を持たせ、読んだ側の経路の前につなぐ。持てない設計上の理由があるなら、その finding に `global_var` の不確実性を付け、`no_uncertainty_on_path` にしない
- CORE-C: CORE-B を直せば欠けた経路が長くなり、自然に負ける。念のため、同じ長さのときの規則を決める(source の関数から始まる経路を優先、など)。決めたら、代表の経路の選び方の ADR に書く
- 受け入れ: 経路のどこかに source のサイトがある関数が出ること(ベンチの採点の不変条件の issue と対)

## 関連
CORE-A の issue(順序は CORE-A → CORE-B → CORE-C)

別セッションのレビューの識別子: CORE-B、CORE-C

## コメント(GitHub から移した写し)

関連: param の source の汚染の書き戻し(#89。CORE-A)、グローバル経由の経路(この issue。CORE-B / CORE-C)。着手順は #89 → #90。採点の不変条件(経路に source の関数が出る)は #92。

