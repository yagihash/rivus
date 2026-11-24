# #205 道具・ベンチ: `bench/parity/TABLE.md`(言語どうしの機能の差の表)が古い。作り直す手順が重く、検査の対象にもなっていない

- 状態: **open**
- 種類: 道具・ベンチ
- 言語: 全
- ROADMAP の節: 0
- 守るもの: -
- 題: 言語間の差の表(`bench/parity/TABLE.md`)が、実際の zoo の数とずれたまま、どの検査にも引っかからない

## 何か

`bench/parity/TABLE.md` の zoo-dart の行は `cells 55 / cases 187`(最後の更新は 2026-11-22、`ec1629f4`)。その後、Dart のセルが増え、今は 77 以上ある(`bench/zoo-dart/cells/lib` の Dart のファイルは 126。セルの数は `TestZooDart` の出力 `zoo-dart: cells N` が正)。前のセルの追加の PR でも、表は更新されていない。

## なぜ古くなるか

- 表を作り直せるのは `go run ./bench/parity report` だけ。入力を集める `bench/parity/collect.sh` が、全対象(実コードの対象を含む)の IR を作るので重い(Rust はサンドボックスの下)。
- `make docs-gen` の対象ではない(課題と ADR の表だけ)。CI の `docs-tests`(`make test-docs`)も、表と実際の数を比べない。だから、食い違っても何も失敗しない。

## 表の使われ方(`git grep -n 'parity/TABLE\|bench/parity'` で確認)

- 表そのものを読む所: `docs/DESIGN-zoo.md`(298、312 行: `@na` と `@todo` の件数を、表の 3 節に並べる、と書く)、`bench/parity/README.md`、`bench/README.md`(222 行)、`docs/ARCHITECTURE.md`(733 行)、`docs/PRINCIPLES.md`(118 行)、`docs/design-package-models/06-11-model-authoring-script.md`(103 行: 言語を横断する順を、表で見る)。
- コードが表を読む所は無い(`bench/parity` が書くだけ)。`bench/parity` の別の部分、`dead-allow` と `.dead-allow` は、`TestBuiltinRowsAreHitByTheZoo` が読む生きたゲートで、この課題の対象ではない。
- PARITY の情報そのものは、zoo のマーク(`@todo(PARITY)`、`@ref(PARITY)`、`@na`)と、`TestZooRust` / `TestZooDart` の parity の検査(`bench/zoo` のゲート)が持つ。表はその集計の写しで、量・需要・死んだ行(`dead-allow`)の数は、表にしかない。

## 修正の候補(修正は、今の作業の順(メモリと収束の作業)のあと)

- (a) **表を検査の対象にする**: 重い入力(実コードの IR)を使わない部分(zoo の行と、マークの件数)を、軽い形で作れるようにして、`make docs-gen` と CI の `docs-tests` に入れる。コミットした表と食い違えば失敗する。重い部分(量、需要、死んだ行)は、手順を分けて、更新するときだけ作り直す。
- (b) **表をやめる**: PARITY の進み具合は、`@todo(PARITY)` の件数と `bench/zoo` のゲートで見られる。上の参照元の記述を修正し、`TABLE.md` と作る道具の該当部分を消す(`dead-allow` は残す)。量・需要の数をどこで見るかを、先に決める。

どちらを選ぶかは、量と需要の数を、これからも表で見たいかで決まる。
