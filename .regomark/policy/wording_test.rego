package wording_test

import data.wording

doc(kind, text) := {"documents": [{"path": "docs/a.md", "name": "a.md", "meta": {}, "links": [], "blocks": [{"kind": kind, "level": 0, "line": 3, "end_line": 3, "text": text, "lang": ""}]}]}

denied(kind, text) if count(wording.deny) > 0 with input as doc(kind, text)

test_coined_terms_are_denied if {
	every text in [
		"動物園のセルを足す", "黙った見逃しになる", "痕跡を出す", "汚れた値", "受け手の型",
		"呼び元をたどる", "見立てを書く", "確かめが済んだ", "赤の直しだけ", "直し方を示す",
		"旗を渡す", "印を付ける", "費用を測る", "本線に戻る", "本筋の経路", "捕獲した変数", "おとりの関数", "出どころを辿る", "書き手が多い", "zoo のマスを足す", "黙って捨てる", "報告の紙", "ジョブが飛ぶ", "main が赤い間", "安全な側に倒す",
		"受け入れの物差し", "雑音が増える", "観測との突き合わせ", "欄を名指す sink", "kind を名指したとき", "試験を先に書く", "隔離の試験",
		"報告の雛形", "ルールのひな形を出す", "rivus の版を記録する", "スキーマ版は上げない", "出力の欄を足す", "欄の名前",
		"動的な辺を足す", "辺のフィルタ", "グラフの辺", "50 辺で切る",
	] {
		denied("paragraph", text)
	}
}

test_reading_noun_is_denied if {
	every text in [
		"コードの読みが浅い", "ボクの読みでは", "私の読みを書く", "テスト読みは", "設定読みで打ち切る", "読みの精度",
	] {
		denied("paragraph", text)
	}
}

test_agent_return_and_progress_are_denied if {
	every text in [
		"サブエージェントの戻りを待つ", "戻りを待つ", "エージェントの戻りが届く", "戻ってくるまで", "進みが遅い", "進みを報告する", "作業の進み",
	] {
		denied("paragraph", text)
	}
}

test_ordinary_words_pass if {
	every text in [
		"ファイルを読み込む", "読み取りの権限", "値を読み取る", "読み出し側", "読み方を決める", "コードを読み、確認する", "コードの読み込み",
		"戻り値を返す", "元に戻す", "1 に戻る", "先へ進む", "次へ進み、続ける", "進捗を見る",
		"矢印の向き", "目印を置く", "国旗を表示する", "見直しをする", "やり直しになる", "書き直しが要る",
		"値を確かめる", "確かめた結果", "バグを直す", "入口の関数", "zoo のセル", "コストを測る", "マスクする", "クリスマス", "たどる", "フラグを渡す",
		"指標を決める", "ノイズが増える", "観測との照合", "名指しで許す", "試験的に入れる", "テストを先に書く",
		"テンプレートを出す", "rivus のバージョン", "初版の割り当て", "Go 版の表", "ホスト版と同じ", "出力のフィールド", "「回答」欄に書く",
		"周辺のコード", "この辺りで", "支配辺境", "エッジを足す", "ノードとエッジ",
	] {
		not denied("paragraph", text)
	}
}

test_code_and_quotes_are_not_checked if {
	not denied("code", "動物園")
	not denied("quote", "動物園")
	not denied("paragraph", "フィールド名は `痕跡` のまま")
}

test_violation_has_the_line_and_the_replacement if {
	some v in wording.deny with input as doc("heading", "動物園")
	v.path == "docs/a.md"
	v.line == 3
	contains(v.message, "zoo")
}
