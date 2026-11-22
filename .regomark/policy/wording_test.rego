package wording_test

import data.wording

doc(kind, text) := {"documents": [{"path": "docs/a.md", "name": "a.md", "meta": {}, "links": [], "blocks": [{"kind": kind, "level": 0, "line": 3, "end_line": 3, "text": text, "lang": ""}]}]}

denied(kind, text) if count(wording.deny) > 0 with input as doc(kind, text)

test_coined_terms_are_denied if {
	every text in [
		"動物園のセルを足す", "黙った見逃しになる", "痕跡を出す", "汚れた値", "受け手の型",
		"呼び元をたどる", "見立てを書く", "確かめが済んだ", "赤の直しだけ", "直し方を示す",
		"旗を渡す", "印を付ける", "捕獲した変数", "おとりの関数", "出どころを辿る", "書き手が多い", "zoo のマスを足す", "黙って捨てる", "報告の紙", "ジョブが飛ぶ", "main が赤い間", "安全な側に倒す",
	] {
		denied("paragraph", text)
	}
}

test_ordinary_words_pass if {
	every text in [
		"矢印の向き", "目印を置く", "国旗を表示する", "見直しをする", "やり直しになる", "書き直しが要る",
		"値を確かめる", "確かめた結果", "バグを直す", "入口の関数", "zoo のセル", "マスクする", "クリスマス", "たどる", "フラグを渡す",
	] {
		not denied("paragraph", text)
	}
}

test_code_and_quotes_are_not_checked if {
	not denied("code", "動物園")
	not denied("quote", "動物園")
	not denied("paragraph", "欄の名前は `痕跡` のまま")
}

test_violation_has_the_line_and_the_replacement if {
	some v in wording.deny with input as doc("heading", "動物園")
	v.path == "docs/a.md"
	v.line == 3
	contains(v.message, "zoo")
}
