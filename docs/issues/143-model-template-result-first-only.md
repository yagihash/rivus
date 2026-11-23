# 143: モデルの `to: result` は最初の戻り値だけ。テンプレートと組み込みの行が、error への流れを落とす

- 状態: **closed**
- 種類: サイレントな見逃し(直した)
- 言語: Go
- ROADMAP の節: 1
- 守るもの: `TestGoRowsThatSayResultAreForOneResult`、`TestModelsCoverageStubsLoseNoFinding`
- 題: モデルの `to: result` は最初の戻り値だけ。テンプレートと組み込みの行が、error への流れを落とす
- 補足(言語): Go(Rust と Dart は戻り値が 1 つ)
- 見つけた経緯: Go の実際のプロジェクトでの利用から。型から生成したモデル 1,475 行のうち 19 行が、`(値, error)` を返す関数の error への流れを落としていた(`to: results` にすると 0 になった)。別の報告で、`{from: args, to: args}` が共有のオブジェクトにも汚染を書き、finding が約 8 倍になった。

`to: result` は result0 だけを指す。全部の戻り値は `to: results`。`rivus models coverage -yaml` のテンプレートは `to: result` と書いていたので、2 つ以上の戻り値を持つ関数では、error への流れが落ちた(`err.Error()` が sink に届かない)。テンプレートは、広い書き込み(`args → args`、`recv → args`)も既定で出していた。

## 直した内容

- テンプレートは `to: results` を書く。広い書き込み(`args → args`)は、書き込み先になりうる引数(ポインタ、スライス、map、インターフェース。型は呼び出しの operand から)ごとの `to: arg<N>` の行と、ポインタのレシーバへの行にした。既定で有効のままで、各行に「書き込まないなら消す。共有のオブジェクトを残すと finding が増える」と注釈した。`context.Context` は書き込みから外した(results の `from` には残す。`bench/gcp` の 3 件が `r.Context()` を通るため)。
- `docs/MODELING.md` に「`result` は最初の戻り値だけ。全部は `results`」と、広い書き込みの注意を書いた。`args_except` は、テンプレートが引数を並べるので作らない。
- 組み込みの Go のモデルを、`go/packages` で読んだシグネチャから点検した。落としていたのは `(*os/exec.Cmd).Output`(error の `Stderr` に出力が入る)の 1 行で、`to: results` にした。`CombinedOutput`、`StdoutPipe`、`StderrPipe` の error は固定の文なので、意図した行として `testdata/result-first-only.txt` に理由と書いた。
- `TestGoRowsThatSayResultAreForOneResult` が、2 つ以上の戻り値を持つ関数の `to: result` の行を、許可の一覧にあるものだけにする。
- Dart と Rust は、1 回の呼び出しの戻り値が 1 つ(Rust の tuple、Dart の Record も 1 つの値)なので、対象にならない。dart-http と dart-gcp(別のブランチ)の行も同じ。
- Rust は実機での確認をしていない(1 回の呼び出しが 1 つの値、という読みだけ)。
