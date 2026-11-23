# dio の MultipartFile.fromFile と fromFileSync の path が、まだ sink の行にない

## 何が見逃されるか(または、何が誤って報告されるか)

`MultipartFile.fromFile(String filePath, ...)` と `fromFileSync` の第 1 引数(arg0)は、読んで送るファイルの path で、path_traversal の sink にする方針が決まっている。行がないあいだは、利用者が決めた path でファイルを読んで送る流れが見逃される。

## 原因

`make model PKG=dio LANG=dart` が候補として出した(層 1 の規則: path の語)。行にする PR は別に出る。それまで、モデルのファイルの `deferred:` に置いて、未決として数えない。

## 再現するセル

なし(行が入る PR で足す)。行が入ったら、`deferred:` の 2 件を消す。
