# ADR-088: 同じ位置の source(同じ関数の同じパラメータ)は、`other_sources` で 1 つに数える(メソッド値の `$bound` の双子)

- 状態: 有効
- 関連: -
- 要旨: 見せ方のみ。finding の集合は変わらない

**背景:** メソッド値(`s.HandleA`)は、`HandleA$bound` という関数になり、同じリクエスト型のパラメータを、**束ねたメソッドの位置**で持つ。`param_type` の source は、この双子と元の関数の両方に付き、`mergeParamSources` が別々の source として数えた。`bench/gcp` では、`*http.Request` を取る関数 33 個と `$bound` の双子 32 個が、`other_sources_count` を約 2 倍にしていた(同じ入力を 2 回数える)。

**決めたこと:** `mergeParamSources` は、代表と同じ位置・同じシンボルの source(と、すでに数えた位置の source)を、`other_sources` と `other_sources_count` に入れない。**findings の集合は変わらない**(双子は、同じ sink の同じグループに畳まれるだけで、代表になる関数は、並びの最後の決め手(関数名の順)で、元の関数が先になる)。見せ方の変更だけで、`other_sources_count` の数が減る。テスト: `testdata/go/cases/paramshare/bound`(3 から 1)。

