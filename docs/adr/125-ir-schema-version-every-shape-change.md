# ADR-125: IR の形が変わるときは、足しただけでも `schema_version` を上げる(2026-11-25)

- 状態: 有効
- 関連: ADR-017、ADR-044、ADR-062、ADR-073、ARCHITECTURE 2.7、#109
- 要旨: 古い extractor を何も報告せずに読まないため、足すだけの変更でもバージョンを上げる

**背景:** ADR-044、ADR-062、ADR-073 などは、IR にレコードやフィールドを足すとき、「足すだけなので `schema_version` は変えない(古いストリームはそのまま読める)」と決めた。その結果、バージョンは `rivus.ir/0` から動かず、足した情報を書かない古い extractor のストリームが、検査を通って読まれた。結果は、足した情報の分だけ欠け、出力に何も出なかった(#109。Docker のイメージが古く、finding が 298 件と出た(今の extractor では 174 件))。

**決定:**
1. IR の形(型とフィールド)が変わるときは、足しただけでも、`ir.SchemaVersion`(`rivus.ir/N`)を上げる。Rust と Dart の extractor が書くバージョンも、同じ PR で上げる。
2. 読む側は、バージョンが違うストリームを、読まずに止める。メッセージに、ストリームのバージョン、rivus のバージョン、作り直し方を書く。終了コードは 1。回避のフラグは作らない。
3. 上げ忘れを、テストで防ぐ。`TestIRSchemaVersionMatchesShape` が、`internal/ir` の型の形のハッシュをバージョンごとに固定する。`TestExtractorsWriteTheSchemaVersion` が、extractor が書くバージョンを照合する。
4. 今のバージョンを `rivus.ir/1` に上げた。

**覆すもの:** ADR-044、ADR-062、ADR-073 の「足すだけで `schema_version` は変えない」(その決定の、バージョンの扱いの部分だけ)。ほかの決定は有効。

**帰結:** 結果が変わる。バージョンの違う extractor は、エラーで止まる。`rivus setup rust` / `rivus setup dart` で作り直す。形を変える PR は、バージョンの 4 か所(`ir.go`、`irjson.rs`、`irjson.dart`、`rivus_dart.dart`)とフィクスチャの先頭行を直す。残り: `summary` に extractor のバージョンを出すこと(#109)。
