# ADR-065: 本体のない関数に sink のルールしかないとき、ルールをモデルとみなさない(CORE-K)

- 状態: 有効
- 関連: -

**背景:** `callBody` の `ruleOnly`(`CallSources` か `Sinks` にシンボルがある)が真のとき、「ルールがモデル」として、既定の「入ったものは出る」と、`unmodeled_library_call` の両方を通さなかった。sink のルールは「ここへ汚染が来てはならない」とだけ宣言し、戻り値に何が乗るかは宣言しない。組み込みの Go の sink 53 個のうち 48 個は、propagator がなく(`os.Open`、`http.Get`、`(*sql.DB).Prepare`)、戻り値に引数の汚染が乗らず、不確実性も出なかった(サイレントな見逃し)。`io.WriteString` は、ResponseWriter のときだけの sink なのに、`io.WriteString(&b, src)` で、`b` へ流れず、不確実性もなかった。

**決めたこと:** `ruleOnly` は **source のルールがあるとき**だけ(source は結果が何かを示す)。sink だけのシンボルは、本体がなければ、既定(引数から結果へ)と `unmodeled_library_call` を通す。`valueCandidates` も同じ。`io.WriteString` には、引数 1 から引数 0 への propagator を足した(`fmt.Fprint*` と同じ形。`flows` を持てば、モデルになり、不確実性は出ない)。

**検証:** `testdata/go/cases/sinkret`(`lib.Open`、`(*lib.DB).Prepare` の戻り値が、次の sink へ届く。`io.WriteString` から `bytes.Buffer`)と、`unmodeled_library_call` が出ること(直す前は、2 つの finding が落ち、不確実性が出なかった)。`compare.sh`(軽い層 + Gitea v1.26.4): 削除 0。追加は gogs の 1 件(2 リビジョンで同じコード): `getServiceType`(`r.FormValue("service")`、`upload-pack` か `receive-pack` に等しいときだけ通る)から `gitCommand`(`exec.Command(...).Output()`)の出力を、`ResponseWriter.Write` へ、の経路。`exec.Command`(sink のみ)の戻り値に引数が乗るようになり、`cmd.Output()` へ届く。入力は 2 つの定数のどちらかに限られ、出力は引数の文字列でない(git の出力)。**偽陽性として分類**(実コードの追加は、この 1 件のみ)。この finding の経路の `exec.Command` と `(*exec.Cmd).Output` の 2 つの呼び出しには、`unmodeled_library_call` が付いていて(`confidence` は `requires_review`)、不確実性の注記のある過大近似。`Output` の流れを切るモデルは入れない(`exec.Command("sh", "-c", "echo "+x).Output()` のように、引数の中身が標準出力に出る形は実在する。この偽陽性の理由は、値が定数の集合と等しいことを確かめてから使う許可リストで、モデルの理由ではない)。go-test-bench の 2 件は、同じ `sink` の `other_sources` の 6 件の集合が同じで、代表の source が替わっただけ。Gitea: finding 259 のまま、パス 6、不確実性 `unmodeled_library_call` 2087 → 2313(+226)。`unmodeled_library_call` は、gogs で 172 → 302、gin-vue-admin で 53 → 113 に増える(本来の「分からない」を示すようになった)。

**限界:** `exec.Command` のような sink は、結果(`*exec.Cmd`)の中身が、引数の汚染を運ぶとして扱われる(既定)。sink のシンボルごとに、戻り値に乗せるかを決める(`flows:` を書く)ことで、ノイズを減らせる。

