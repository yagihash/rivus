## 1. 今の動作(コードの調査)

モデルがあるとき、今、何が省かれていて、何が省かれていないか。

1. **extractor が読む範囲は、モデルの有無で変わらない。**
   - Go: `cmd/rivus/common.go` の `load`(163 行)は `gossa.Options{Deps: !c.noDeps && withGraph}` で `internal/frontend/gossa/load.go` の `Load` を呼ぶ。`Deps` なら `NeedDeps` を足し(169 行)、`ssautil.AllPackages`(239 行)で依存のパッケージも全部、ソースから型検査して SSA にする。`loadGo`(174 行)の `rules.Expand` は、インターフェースのメソッドの規則を、プログラムの中の実装のメソッドに写すだけで、何も読まない・省かない。標準ライブラリだけは、`skip`(486 行の `buildInOrder`)で SSA の本体を作らない(モデルが代わりをする)。
   - Dart: extractor(`extractor/dart/lib/extract.dart`)に渡るのは `--no-deps`、`--include-tests`、`--allow-deps-under` だけ(`cmd/rivus/lang.go` 330〜340 行)。モデルは渡らない。
   - Rust: `--no-deps` だけ(`extractor/rust/src/main.rs` 295 行)。
   - 読む範囲を変えるのは `-no-deps`、`-tests`、`-allow-deps-under` だけ。
2. **依存の具体的な関数に、flows / pure / calls / sanitizer のモデルがあるとき、呼び出しの所では、モデルが使われ、本体の要約は使われない**(`internal/taint/calls.go` 223〜226 行。`rules.HasModel`(`internal/model/model.go` 1254 行)は、sanitizer か propagator の行があること。sink / source だけの行は `HasModel` ではないので、本体が使われる)。インターフェースやトレイトのメソッドにモデルがあるときは、依存の実装の本体は使わず(`dependency_body_unread` の注記。`noteDepBodies`、1143 行)、プロジェクト自身の実装は、読んでモデルと合わせる(ADR-075)。
3. **モデルで置き換えられた関数の本体の要約も、計算される。** `Engine.Run`(`internal/taint/engine.go` 529 行)は、本体のある全関数(`e.prog.Funcs`)を、ボトムアップに計算する。呼び出しグラフはモデルで変わらない。ADR-098 の測定で解析が 65〜90% 減ったのは、「呼び出しの所で、依存の実装の要約を適用して合流する」処理が省かれたから。依存の関数自身の解析は省かれない。
4. **同じパッケージの、モデルのない関数は、本体が読まれる。** パッケージ単位で切る方法がない(今は、全部読むか、全部読まないか)。

