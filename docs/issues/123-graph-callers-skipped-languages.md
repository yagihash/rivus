# 123: graph callers / callees / dispatchers の答えが、読まなかった言語を言わない

`rivus graph path` の「経路なし」は、index に `skipped_languages`(読まなかった言語。Rust を断った、など)があると `exhaustive: false` で、理由が `not_exhaustive_because.skipped_languages` に出る。

`callers` / `callees` / `dispatchers` の答えには、「これで全部」を言う欄(`exhaustive` や `complete`)がそもそも無く、index が読まなかった言語を持つことも、答えに出ない。読まなかった言語の呼び出し元は、結果に出ないのに、答えは全部に見える。

直し方の案: 答えに `caveats`(か `limits`)の 1 行を足して、`meta.unread.skipped_languages` を言う。
