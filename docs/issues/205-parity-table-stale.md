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

## 案の比較と推薦(2026-11-24。コード `bench/parity/` を読んで確かめた)

**表の中身の何が、何から作れるか**(`bench/parity/render.go`、`README.md`):

| 表の節 | 入力 | マークとセルの数だけで作れるか |
| --- | --- | --- |
| 3 節のうち `@na` / `@todo` の件数と名前 | zoo のマーク(Rust は `cells/src/lib.rs` の先頭、Dart は `bench/zoo-dart/na.txt`、Go のセル)。いまは `TestZoo*` が出す `parity marks:` の行を、正規表現で読んでいる | **作れる**(テストを動かさず、ファイルを読むだけで済む) |
| 3 節のうち、セル / ケースの数 | セルのファイルの数え上げ | 作れる(新しく数えるコードが要る) |
| 3 節のうち、出る / `@miss` / `@fp` | `TestZoo*` の実行結果(解析を動かす。言語ごとに数秒〜10 秒余り) | 作れない(軽くない。zoo のゲートがすでに検査している) |
| 3 節のうち `pure` の実行の検査 | `TestZooPureGo` の出力 | 作れない |
| 要約と 5 節: 組み込みの行数と、形ごとの数 | `internal/model/builtin/*.yaml` だけ | 作れる(IR は要らない) |
| 1 節 (A) 量 | Go の標準ライブラリの型の読み込み(`go/types`)。Dart と Rust は「未」 | 重くはないが、マークとは別の入力 |
| 2 節 (B) 需要、4 節 (D) 当たっているか、6 節、`DEAD.md` | 実コードの IR(`collect.sh` が、`casdoor`、`gogs`、`dart-pub`、`parseable` などを抽出する。Rust はサンドボックスの下) | **作れない**(IR でしか作れない) |

つまり、表の価値の大きい部分(量・需要・死んだ行)は IR でしか作れず、マークから作れる部分は、すでに `TestZoo*` の出力と、`bench/zoo` のゲート(`@na` と `@todo` が無いと失敗する)が持っている。

**(a) 軽い形を作って、`make docs-gen` と CI に入れる**

- 作る範囲は、マークの件数と名前、セルの数、組み込みの行数と形。ここだけを新しい道具(`bench/parity` に `-light` のような形)で作り、コミットした表と比べる。
- 欠点: (1) 表のほとんどの節(量・需要・当たっているか)は、手順を分けて残るので、「古い表」の問題の半分は残る。(2) 表はすべてのセルの追加の PR で変わるので、**全員が触る 1 つのファイル**になり、PR どうしでコンフリクトする(`docs/design-package-models/06-01-no-conflict-layout.md` が避けたい形)。(3) 数を数えるコードと、その検査を、持ち続ける。(4) 件数は `TestZoo*` がすでに出すので、同じ情報の 2 つ目の写しになる。
- 大きさ: 中。新しいコード 200〜300 行(マークとセルの読み込み、行数の数え上げ、`scripts/docs-gen` か `make test-docs` への接続)と、そのテスト。1〜2 日。

**(b) 表をやめる**

- PARITY の進み具合は、`@todo(PARITY)` の件数と、`TestZoo*` の出力の `parity marks:` の行と、`bench/zoo` のゲートで見る。量・需要・死んだ行の数は、必要なときに `go run ./bench/parity report` を `-table` / `-dead` に**Git の管理外の出力先**を指定して作る(道具は残す。ゲートである `dead-allow` と `.dead-allow` は、そのまま)。
- 作業: `bench/parity/TABLE.md` を消す(`DEAD.md` は、同じ道具が作る写しなので、同じ扱いにするか、別に決める)。参照元の記述を直す: `docs/DESIGN-zoo.md`(298、312 行: 「表の 3 節」を「テストの出力の行」に)、`bench/parity/README.md`(表を手順書にしない)、`bench/README.md`(222 行)、`docs/PRINCIPLES.md`(118 行)、`docs/design-package-models/06-11-model-authoring-script.md`(103 行: 言語を横断する順を見る所を、`parity marks:` の行に)、`docs/ARCHITECTURE.md`(733 行はそのままでよい)、`bench/parity/render.go` と `README.md` の「手で直さない」の文。
- 大きさ: 小。コードの変更はほぼなし(記述の修正が 6 か所ほどと、ファイルの削除)。半日以内。

**推薦: (b)**。作り直しが重く(IR を集める)、軽く作れる部分は、すでにテストの出力とゲートにある。(a) は、同じ情報の写しを、共有の 1 ファイルに増やし、コンフリクトの元になる。量・需要の数は、これからも必要なら、道具を残して、そのときに作る(コミットしない)。(b) を先に直してから、量・需要の数を表で見る必要がまた出たら、そのときに (a) を考える。実装は、案を読んでから(状態は `open` のまま)。
