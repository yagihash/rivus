# ADR-087: `other_sources` の各項目に、パッケージ変数を経由したマーク `via_shared_state` を付ける(出力の追加だけ)。マップの要素の読み出しにも `global_var` の注記を付ける

- 状態: 有効
- 関連: -
- 状態の詳細: 069 の続き
- 要旨: 出力の追加のみ

**背景:** `other_sources`(ADR-043 の追記の `mergeParamSources`)の項目は、別のハンドラの入力が、共有の状態ごしに届いたものでも、それを示さなかった(finding 本体には `shared_state`(ADR-069)があるが、項目には何もない)。判定役(LLM)が、`other_sources_count` を「ほかの入口からも届く」と読むとき、パッケージ変数を通った経路かどうかを見分けられなかった(`bench/gcp` の誤った注記は、この形)。また、パッケージ変数のマップを**キーつきで読む**形(`M["codec"]`)の読み出しには、`global_var` の注記が付かず、`registry` の finding に `shared_state` が付かなかった(キーなしの読み出しと、フィールドの読み出しには付いていた)。

**決めたこと:**
1. `other_sources` の項目に `via_shared_state`(配列。パッケージ変数の名前。その source が finding になっていたときの `shared_state` の `place.name`。経由しなければ省く)を足す。出力の追加だけ。`rivus.result/1` のバージョンは上げない(ADR-034)。
2. キーつきのマップの読み出し(`index_load` の `keyed`)にも、グローバルの読み出しの注記(`global_var`)を付ける。ほかの読み出しと同じ注記で、`shared_state` の元になる。
3. テスト: go-scale の `longlived` と `registry` で、`other_sources` の全項目に `via_shared_state` が付くこと(`TestScaleSynth`)。

