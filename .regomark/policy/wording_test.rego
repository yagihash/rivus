package wording_test

import data.wording

doc(kind, text) := {"documents": [{"path": "docs/a.md", "name": "a.md", "meta": {}, "links": [], "blocks": [{"kind": kind, "level": 0, "line": 3, "end_line": 3, "text": text, "lang": ""}]}]}

denied(kind, text) if count(wording.deny) > 0 with input as doc(kind, text)

test_coined_terms_are_denied if {
	every text in [
		"時間の減り", "減りは誤差", "減りも", "残る増え", "増えは鈍る", "出力の増えを見る", "増えが止まらない",
		"動物園のセルを足す", "黙った見逃しになる", "痕跡を出す", "汚れた値", "受け手の型",
		"呼び元をたどる", "見立てを書く", "確かめが済んだ", "赤の直しだけ", "直し方を示す",
		"旗を渡す", "印を付ける", "費用を測る", "本線に戻る", "本筋の経路", "捕獲した変数", "おとりの関数", "出どころを辿る", "書き手が多い", "zoo のマスを足す", "黙って捨てる", "報告の紙", "ジョブが飛ぶ", "main が赤い間", "安全な側に倒す",
		"受け入れの物差し", "雑音が増える", "観測との突き合わせ", "欄を名指す sink", "kind を名指したとき", "試験を先に書く", "隔離の試験",
		"報告の雛形", "ルールのひな形を出す", "rivus の版を記録する", "スキーマ版は上げない", "出力の欄を足す", "欄の名前",
		"動的な辺を足す", "辺のフィルタ", "グラフの辺", "50 辺で切る",
		"深さ 3 と 2 の比べ", "比べ方を決める", "## 4. 比べ方", "まとめての採用の設計", "まとめての却下の書式", "型でまとめて却下する", "まとめて採用する形",
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

test_red_and_green_for_a_test_result_are_denied if {
	every text in [
		"main が赤い", "main が十数回のマージの間、赤いままだった", "赤い間にマージした PR", "main を緑に戻す", "CI が緑になる",
		"PR が赤くなる", "赤を確かめる", "緑にするために注釈は変えない", "単独では緑 / 赤", "切られたら赤", "赤", "緑",
	] {
		denied("paragraph", text)
	}
}

test_red_and_green_as_colours_pass if {
	every text in ["赤色で表示する", "緑色のバッジ", "赤字で書く", "赤外線"] {
		not denied("paragraph", text)
	}
}

test_red_and_green_message_names_the_replacement if {
	some v in wording.deny with input as doc("paragraph", "main が赤いまま")
	contains(v.message, "赤 / 緑")
	contains(v.message, "失敗している")
}

test_ordinary_words_pass if {
	every text in [
		"時間が減った", "減る分", "減って、増える", "減りにくい", "減らす", "減り方", "出力が増えている", "増えた分", "増える原因", "増えない", "増えて、減る", "増えず", "増えにくい", "ふえ",
		"ファイルを読み込む", "読み取りの権限", "値を読み取る", "読み出し側", "読み方を決める", "コードを読み、確認する", "コードの読み込み",
		"戻り値を返す", "元に戻す", "1 に戻る", "先へ進む", "次へ進み、続ける", "進捗を見る",
		"矢印の向き", "目印を置く", "国旗を表示する", "見直しをする", "やり直しになる", "書き直しが要る",
		"値を確かめる", "確かめた結果", "バグを直す", "入口の関数", "zoo のセル", "コストを測る", "マスクする", "クリスマス", "たどる", "フラグを渡す",
		"指標を決める", "ノイズが増える", "観測との照合", "名指しで許す", "試験的に入れる", "テストを先に書く",
		"テンプレートを出す", "rivus のバージョン", "初版の割り当て", "Go 版の表", "ホスト版と同じ", "出力のフィールド", "「回答」欄に書く",
		"A と B を比べ、差を見る", "値を比べる", "出力を見比べる", "コードの見比べ", "周辺のコード", "この辺りで", "支配辺境", "エッジを足す", "ノードとエッジ",
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

test_merge_conflict_wording_is_denied if {
	every text in [
		"main と衝突した", "衝突を解消する", "ブランチの衝突を解決した", "マージで衝突する", "PR が衝突している", "リベースの衝突", "取り込みで衝突した", "push の衝突を直す",
	] {
		denied("paragraph", text)
	}
}

test_merge_conflict_message_names_the_replacement if {
	some v in wording.deny with input as doc("paragraph", "main と衝突した")
	contains(v.message, "コンフリクト")
}

test_name_and_value_collisions_pass if {
	every text in [
		"名前の衝突", "ADR の番号が衝突する", "ハッシュの衝突", "フラグが衝突する", "クロージャの名前が衝突する", "キーの衝突を避ける",
		"main の関数の名前の衝突を調べる", "コンフリクトを解消する", "main とコンフリクトした", "名前の衝突を解決する",
	] {
		not denied("paragraph", text)
	}
}

test_merge_conflict_in_code_is_not_checked if {
	not denied("code", "main と衝突した")
	not denied("paragraph", "`衝突を解消` の行")
}

test_a_sentence_without_conflict_word_passes if {
	not denied("paragraph", "main にマージした。名前が重なる")
}

test_terminology_rules_are_denied if {
	every text in [
		"生きている量が上限に近い", "生きているヒープの最大", "読み込みの段の山", "要約の段で増える", "解析の段", "確保の速い段", "要約を適用する段",
		"経路の段に注記が付く", "段の単位の注記", "段ごとに数える", "原因の段", "名前でない段を飛ばす",
		"合否の線を決める", "合格の線を上げる", "合格線", "線を越えた", "線の中に収まる", "線が上がる",
		"この決まりを守る", "決まりが増える", "決まりは変えない", "守る決まりを", "決まりごと",
	] {
		denied("paragraph", text)
	}
}

test_terminology_rules_pass_other_words if {
	every text in [
		"段階 2 に進む", "取得の段階", "経路の段階", "原因の段階を言う", "手段がない", "値段", "2 段", "1 段目の隣", "3 段で切る", "普段使う", "後段の LLM",
		"値が決まります", "結果が決まりました", "順序が決まる", "決まらない場合", "値が決まり、次に進む", "きまりを書く", "決まり文句", "決まり手", "ライブヒープ（強制 GC のあと）",
		"合格基準を厳しくする", "ルールを守る",
	] {
		not denied("paragraph", text)
	}
}

test_bridge_lift_peak_depth_are_denied if {
	every text in [
		"つなぎの注記を付ける", "つなぎの対策", "持ち上げありの SSA", "持ち上げなしの形", "読み込みの山", "メモリの山が高い", "山を下げる", "山の高さを測る", "ヒープの山", "要約の山",
		"最大 4 段まで見る", "3 段より上にたどる", "何段もの関数", "1 段深く見る", "4 段のパスなど", "1 段粗い", "5 段のヘルパー",
	] {
		denied("paragraph", text)
	}
}

test_bridge_lift_peak_depth_pass_other_words if {
	every text in [
		"山を越える", "山道を歩く", "山田さん", "2 段階に分ける", "1 段目の隣", "2 段構えで使う", "段階 4", "3 段の探索", "200 段を超える経路", "持ち上げる", "暫定の注記", "ピークが高い", "3 階層まで",
	] {
		not denied("paragraph", text)
	}
}
