# ADR-066: 組み込みの `pure` から、入力を出力に伝える関数を外す。基準を決め、実行で確かめる(R-03)

- 状態: 有効
- 関連: MODELING §1
- 要旨: 基準を決め、実行で確かめる

**背景:** `go-pure.yaml` / `go-stdlib-more.yaml` の `pure: true` は「戻り値に汚染が乗らない」という宣言。実行で確かめると(zoo、`TestZooPureGo`)、文字列・バイト列の引数を持つ `pure` の行 50 件のうち 18 件が誤りだった: 値に入る(`(*json.Decoder).Token`、`(*http.Cookie).String`、`(*url.URL).RequestURI`、`(time.Time).Format`、`hex.AppendEncode`、`hex.Encode`(書き込み先)、`url.UserPassword`、`regexp.Compile`)と、**error の文字列に入る**(`os.Stat`、`exec.LookPath`、`netip.ParseAddr`、`strconv.Atoi/ParseBool/ParseFloat/ParseUint`、`time.Parse/ParseInLocation/ParseDuration`)。error の文字列は、応答やログに書かれる(エラー文をそのまま返す XSS)。

**決めたこと:**
1. **`pure` の基準**(`docs/MODELING.md` の「When a function may be `pure`」): 結果と、書き込み先(引数、レシーバ)のどれにも、引数・レシーバの中身が入らない。**error の文字列も含める**。
2. 修正方法は、**結果ごとに書く**(`pure` を外して「引数 → 全部の結果」にしない): 数や時刻を作る関数(`strconv.Atoi` ほか 4、`time.Parse` ほか 2、`ParseDuration`)は、入力を `result1`(error)へだけ流し、`result0`(数、時刻)へは流さない(`numeric_stop` が止める値を汚すと過検出になる)。`os.Stat`、`exec.LookPath`、`netip.ParseAddr`、`regexp.Compile` は、`result0` と `result1` の両方。`hex.Encode` は、`arg1 → arg0`。`Token`、`Cookie.String`、`RequestURI` は、レシーバ → 結果。`Format` は、レイアウト(`arg0`) → 結果(日時の値は数なので止める)。`UserPassword`、`AppendEncode`、`x509.CreateCertificate`、`ParsePKIXPublicKey`、`sql.Named`、`parser.ParseFile`、`tls.Client`、`slog.Group` ほか、全部で 30 の行(レシーバの値に目印を入れて呼ぶ検査が、`(*log/slog.Level).UnmarshalText` の error の文字列に入力が入る 19 件目の誤りを見つけた)。
3. 検査: `TestPureFunctionsThatReturnDataAreReviewed`(型から、文字列・バイト列・構造体などを返す標準ライブラリの `pure` を列挙し、`testdata/pure-carriers.txt` の 84 件と比べる。増えても減っても失敗する)。`TestZooPureGo`(実行): 既知の誤りの一覧(ratchet)は空になった。264 の Go の `pure` の行のうち、呼んで確かめたのは 22(引数に目印を入れる 20 と、レシーバの型の値に目印を入れて呼ぶ 2(`(*net/url.URL).Port`、`(*regexp.Regexp).Match`))、確かめていないのは 242(`bench/zoo-go/pure/unchecked.txt`、理由つき。呼んでいない行が増えるとテストが失敗する)。

**検証:** `compare.sh`: R-03 の変更だけを、CORE-K の上で測ると、全ベンチで、finding の追加 0、削除 0、不確実性の変更 0(error や値に新しく流れが乗っても、sink に届く形が、これらのコードにはない)。Gitea は、R-03 だけでは測っていない(CORE-K と合わせて測った: ADR-065)。

**限界:** 呼んで確かめていない 242 の行(レシーバが唯一の入口の関数、プロセス・ファイル・ネットワーク・時計に触る関数)は、基準で読んだだけ。Dart と Rust の `pure` の同じ検査は、各言語の zoo の段階で。

