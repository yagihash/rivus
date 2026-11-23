# ADR-004: bottom-up の関数サマリ（functional approach）

- 状態: 有効
- 関連: ARCHITECTURE 5.2
- 要旨: SCC 縮約・トポ順の functional approach。seed なしクエリのために選ぶ

- **状況:** 汎用の IFDS を実装すると重い。
- **決定:** コールグラフを SCC に縮約し、トポロジカル順（呼び出し先から）に関数サマリを算出して、呼び出し側で展開する（SCC 内のみ不動点まで反復）。
  これは Sharir & Pnueli (1981) の **functional approach** であり、IFDS の tabulation の非形式版ではない。
  「IFDS の summary edge に相当するものを functional approach で計算する」と表現を統一する。
- **理由（単なる用語の問題ではなく、方式選択の根拠）:**
  rivus の要件 4.2「ある sink に到達し得る source の列挙」は **seed となる source を与えずに投げるクエリ**である。
  tabulation は entry point と seed fact から前向きに探索する設計なので、この形に素直に乗らない。
  bottom-up の functional approach は全関数の完全なサマリを先に作るので、source 側・sink 側どちらからでも引ける。
  つまり rivus の目玉機能のために bottom-up を選んでいる。
- **帰結:** サマリは関数単位で独立させる（将来の増分解析の土台。ADR-021）。

