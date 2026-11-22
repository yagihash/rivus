# 文書で使わない用語と言い回し。直訳や造語をやめて、日本語の技術文書で普通に使う語で書く。
# 見るのは、見出し・段落・リスト・表の本文。コードブロック、引用、バッククォートの中は見ない。
# 用語を足すときは terms に 1 行足し、wording_test.rego にテストを足す（opa test .regomark/policy）。
package wording

schema_version := 3

rule_ids := {"WORD001"}

# pattern は RE2 の正規表現。1 文字の語は、前後の文字を含めて書く（「矢印」「入口」などを巻き込まない）。
# その場合は、メッセージに出す語を word に書く。
terms := [
	{"pattern": `動物園`, "use": "zoo"},
	{"pattern": `黙った見逃し`, "use": "サイレントな見逃し"},
	{"pattern": `痕跡`, "use": "不確実性の注記（2 回目以降は「注記」）"},
	{"pattern": `関門`, "use": "ゲート（CI では「必須チェック」）"},
	{"pattern": `汚れ`, "use": "汚染（docs/GLOSSARY.md の用語）"},
	{"pattern": `受け手`, "use": "レシーバ"},
	{"pattern": `呼び(元|先)`, "use": "呼び出し元 / 呼び出し先"},
	{"pattern": `種まき`, "use": "シード"},
	{"pattern": `捕獲`, "use": "キャプチャ"},
	{"pattern": `おとり`, "use": "デコイ"},
	{"pattern": `出どころ`, "use": "出所"},
	{"pattern": `辿`, "use": "たどる（ひらがなで）"},
	{"pattern": `書き手`, "use": "書き込み側、書き込む関数"},
	{"pattern": `読み手`, "use": "読み出し側、読み出す関数（読者の意味なら「読者」）"},
	{"pattern": `(^|[^ァ-ヶー])マス($|[^ァ-ヶー])`, "word": "マス", "use": "セル"},
	{"pattern": `黙っ(て|た)`, "use": "何も報告せずに、サイレントに"},
	{"pattern": `費用`, "use": "コスト（計算の時間やメモリの意味）"},
	{"pattern": `物差し`, "use": "指標"},
	{"pattern": `雑音`, "use": "ノイズ"},
	{"pattern": `突き合わせ`, "use": "照合"},
	{"pattern": `名指(す|せ|さ|した)`, "word": "名指す", "use": "指定する（値として名前を挙げる意味なら「参照する」。「名指しで」は使ってよい）"},
	{"pattern": `試験($|[^的])`, "word": "試験", "use": "テスト（「試験的」は使ってよい）"},
	{"pattern": `見立て`, "use": "見積もり、推定、見込み"},
	{"pattern": `受け止め`, "use": "対応、解釈"},
	{"pattern": `見張り`, "use": "監視"},
	{"pattern": `手当て`, "use": "対策"},
	{"pattern": `言い切`, "use": "断定する"},
	{"pattern": `側に倒(す|し|せ|れ)`, "use": "〜として扱う、安全側に寄せる"},
	{"pattern": `確かめ(の|が|を|は|も|に|で|と|方|、|。|）|\)|$)`, "word": "確かめ", "use": "確認（動詞の「確かめる」は使ってよい）"},
	{"pattern": `(^|[^り見きてけえせ作取立建塗引出書焼])直し(の|が|を|は|も|に|で|と|だけ|方|、|。|）|\)|$)`, "word": "直し", "use": "修正（「やり直し」「見直し」などは使ってよい）"},
	{"pattern": `(^|[^国])旗(を|の|は|が|で|に|も|と|、)`, "word": "旗", "use": "フラグ"},
	{"pattern": `(^|[^矢目])印(を|が|は|に|で|も|と|の|、|つき|付き)`, "word": "印", "use": "マーク"},
	{"pattern": `(報告|引き継ぎ|設計)の紙`, "use": "報告書、文書、設計文書"},
	{"pattern": `(ジョブ|テスト|試験)(が|を|は)\s?飛(ぶ|ば|ん)`, "use": "スキップされる / スキップする"},
	{"pattern": `(CI|main)(の CI)?\s?(が|は)\s?(赤|緑)`, "use": "失敗 / 成功"},
]

checked_kinds := {"heading", "paragraph", "list", "table"}

deny contains {"rule": "WORD001", "path": doc.path, "line": block.line, "message": msg} if {
	some doc in input.documents
	some block in doc.blocks
	block.kind in checked_kinds
	prose := regex.replace(block.text, "`[^`]*`", "")
	some term in terms
	found := regex.find_n(term.pattern, prose, 1)
	count(found) > 0
	msg := sprintf("「%s」は使わない → %s", [object.get(term, "word", found[0]), term.use])
}
