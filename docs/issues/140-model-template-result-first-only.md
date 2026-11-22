# 140: モデルの `to: result` は最初の戻り値だけ。テンプレートと組み込みの行が、error への流れを落とす

- 状態: **closed**
- 種類: サイレントな見逃し(直した)
- 言語: Go(Rust と Dart は戻り値が 1 つ)
- 見つけた経緯: Go の実際のプロジェクトでの利用から。型から生成したモデル 1,475 行のうち 19 行が、`(値, error)` を返す関数の error への流れを落としていた(`to: results` にすると 0 になった)。別の報告で、`{from: args, to: args}` が共有のオブジェクトにも汚染を書き、finding が約 8 倍になった。

`to: result` は result0 だけを指す。全部の戻り値は `to: results`。`rivus models coverage -yaml` のテンプレートは `to: result` と書いていたので、2 つ以上の戻り値を持つ関数では、error への流れが落ちた(`err.Error()` が sink に届かない)。テンプレートは、広い書き込み(`args → args`、`recv → args`)も既定で出していた。

## 直した内容

- テンプレートは `to: results` を書く。広い書き込みの行はコメントにし、書き込み先は `arg<N>` で指すこと、共有のオブジェクトへの書き込みは finding を増やすことを書いた。
- `docs/MODELING.md` に「`result` は最初の戻り値だけ。全部は `results`」と、広い書き込みの注意を書いた。`context.Context` を流れの `from` から外す `args_except: [N]` は案として書いた。実装していない(解析の側の選択子の変更で、別の課題にする)。
- 組み込みの Go のモデルを、`go/packages` で読んだシグネチャから点検した。落としていたのは `(*os/exec.Cmd).Output`(error の `Stderr` に出力が入る)の 1 行で、`to: results` にした。`CombinedOutput`、`StdoutPipe`、`StderrPipe` の error は固定の文なので、意図した行として `testdata/result-first-only.txt` に理由と書いた。
- `TestGoRowsThatSayResultAreForOneResult` が、2 つ以上の戻り値を持つ関数の `to: result` の行を、許可の一覧にあるものだけにする。
- Dart と Rust は、1 回の呼び出しの戻り値が 1 つ(Rust の tuple、Dart の Record も 1 つの値)なので、対象にならない。dart-http と dart-gcp(別のブランチ)の行も同じ。
