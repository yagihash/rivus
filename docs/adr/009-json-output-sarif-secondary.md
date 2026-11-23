# ADR-009: 出力は構造化 JSON が本体、SARIF は副次

- 状態: 有効
- 関連: README
- 状態の詳細: SARIF は未実装。Phase 1 範囲外と明記; 訂正・追記あり(書き方を保つ)(ブリーフの誤り)
- 要旨: SARIF は private repo で GHAS 無しには使えない(訂正)。JSON が一級

- **状況:** ブリーフと初版の GLOSSARY は「プライベートリポジトリでも SARIF アップロード自体は GHAS なしで可能」としていた。
- **訂正:** **誤り。** GitHub の code scanning が使えるのは、パブリックリポジトリ、または GitHub Code Security を有効にした組織のリポジトリに限られる。
  プライベートリポジトリで Code Security なしには使えない。SARIF アップロードは code scanning の機能なので同じ制約下にある。
  （出典: GitHub Docs "Uploading a SARIF file to GitHub"）
- **決定:** 構造化 JSON を唯一の一級市民とする。SARIF は副次出力で、用途は次の 3 つ。
  - パブリックリポジトリでの利用
  - VS Code の SARIF Viewer など、ローカルのビューワで見る
  - 将来 Code Security を契約した場合や他の SARIF 対応基盤での互換性
  SARIF 生成は Phase 1 のスコープ外とし、JSON が固まってから実装する。
- **理由:** プライベートリポジトリで無償で使いたい、というのが rivus の出発点であり、Security タブへの出力はその目的に使えない。
- **帰結:** SARIF に載せる場合、LLM 向けの付加情報は `properties` バッグに入れる（ブリーフ 7.1 節の方針は維持）。

