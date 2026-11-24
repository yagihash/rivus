# ADR-067: モデルの流れに「これは近似」と書く(`approximate`)。通った finding に不確実性 `model_approximation` が 1 件付く(C-8 の最初の実例)

- 状態: 有効
- 関連: MODELING §2
- 要旨: `model_approximation` が通った finding に付く

**背景:** CORE-K(ADR-065)で、`exec.Command` の戻り値に引数が載り、`(*exec.Cmd).Output` の結果へ運ばれる流れが、gogs に 1 件増やした(`git` の出力を、許可リストで絞った引数から応答に書く形)。この流れは**実在する**(`exec.Command("sh", "-c", "echo "+x).Output()`、`git log --format=<x>`、`cat <x>` は、引数の中身が標準出力に出る)ので、モデルで切ると、この形がサイレントな見逃しになる。gogs の +1 が偽陽性なのは、値が定数の集合と等しいことを確かめてから使う許可リストのためで、モデルの理由ではない。モデルを切ると、理由の違う finding をたまたま消す。ただ、外部プログラムの出力が引数を運ぶかは、プログラムによる、**近似**で、そう示さずに「ふつうの流れ」として運ぶと、確信度が `no_uncertainty_on_path`(uncertainty なし)になりうる。

**決めたこと:**
1. `flows` の要素に `approximate: "<理由>"` を書ける(`internal/model` の `Flow.Approximate`)。理由は、不確実性の `detail` になる。
2. その流れを通った finding の経路に、不確実性が **1 件**付く。種類は **`model_approximation`**(1 つに決める。`attributable_to: rivus`、`symbol` はモデルのシンボル、`detail` は理由)。確信度は `no_uncertainty_on_path` にならない(`requires_review`)。流れは消さない。
3. 適用した所(`go-stdlib-more.yaml`、`go-pure.yaml`): `(*os/exec.Cmd).Output` / `CombinedOutput` / `StdoutPipe` / `StderrPipe` の recv → 結果(外部プログラムの出力は、引数を運ぶかがプログラムによる)。`Run` / `Start` / `Wait` の recv → 結果(error): `exec.LookPath` の失敗の文面にコマンド名が入る(zoo の `(*os/exec.Cmd).Run` で確かめた)ので、`pure` から外して、近似つきで伝える。`exec.Command(name, args…)` の引数 → 戻り値は、近似でない本物の流れ(`cmd.Args` や `cmd.String()` を読む流れを失わない)。
4. **スキーマ**: 足すだけ(ルールの `approximate`、不確実性の新しい `kind`)。`rivus.result/1` のバージョンは上げない(ADR-034 の、足すだけの変更の扱い)。
5. zoo に `conv/exec-output-echo`(引数が `echo` で出力に出る形。`cmd.Args` に入れて、構築子が sink にならないようにした。`@sink` が出る)。`testdata/go/cases/approx`(近似の流れの finding は `model_approximation` が 1 件と、確信度が clear でない。近似でない流れには付かない)。

**ほかの使い道(今後):** 「引数がそのまま出るかは、呼ぶ先による」モデル(外部プログラム、リフレクション、プラグイン)。ルール作者(利用者や AI)が、確信のない流れを、消さずに、近似と書ける。
**限界:** 呼んで確かめていない 246 の行(レシーバが唯一の入口の関数、プロセス・ファイル・ネットワーク・時計に触る関数)は、基準で読んだだけ。Dart と Rust の `pure` の同じ検査は、各言語の zoo の段階で。

