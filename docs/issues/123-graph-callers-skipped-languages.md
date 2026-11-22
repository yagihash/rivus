# 123: graph callers / callees / dispatchers の答えが、読まなかった言語を示さない

`rivus graph path` の「経路なし」は、index に `skipped_languages`(読まなかった言語。Rust を断った、など)があると `exhaustive: false` で、理由が `not_exhaustive_because.skipped_languages` に出る。

`callers` / `callees` / `dispatchers` の答えには、「これで全部」を示す欄(`exhaustive` や `complete`)がそもそも無く、index が読まなかった言語を持つことも、答えに出ない。読まなかった言語の呼び出し元は、結果に出ないのに、答えは全部に見える。

修正の案: 答えに `caveats`(か `limits`)の 1 行を足して、`meta.unread.skipped_languages` を示す。

## 案(PR #107 のレビューで出た。やるかは、あとで決める)

`-lang go` のように言語を指定したとき、ディレクトリに `Cargo.toml` や `pubspec.yaml` があっても、`skipped_languages` には何も残らない(今までと同じ。利用者が範囲を自分で決めたため)。ただ、エージェントが `-lang go` で作った索引の「経路なし」を `exhaustive` と読む場面は、auto で断ったときと同じ形になる。`why: "not-selected"` として残す案。`-lang` の範囲外の言語を、marker のファイルで見つけたときだけ(読まない)。
