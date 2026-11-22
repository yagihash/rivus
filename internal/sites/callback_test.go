package sites

import (
	"sort"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/testutil"
)

func callbackRules(t *testing.T, src string) *model.Rules {
	t.Helper()
	rs, err := model.Parse(strings.NewReader(src), "cb")
	if err != nil {
		t.Fatal(err)
	}
	return model.Merge(rs)
}

// paramsOf lists "function:parameter index" of the sites of the kind callback_param, sorted.
func callbackSitesOf(res *Result) []string {
	var out []string
	for _, s := range res.Sites {
		if s.Kind == model.KindCallbackParam {
			fn := s.Func[strings.LastIndex(s.Func, "/")+1:]
			out = append(out, fn+":"+itoa(s.IR.Param))
		}
	}
	sort.Strings(out)
	return out
}

func TestCallbackParamMarksTheParametersOfTheFunctionThatIsGiven(t *testing.T) {
	rules := callbackRules(t, `
sources:
  - kind: callback_param
    symbol: example.com/cases/cases/cbparam.Route
    operand: 1
    from: 1
    labels: [user_input]
  - kind: callback_param
    symbol: (*example.com/cases/cases/cbparam.Router).Get
    operand: 1
    from: 1
    labels: [user_input]
`)
	res := enumerateGo(t, testutil.Testdata(), []string{"./cases/cbparam"}, rules)
	got := strings.Join(callbackSitesOf(res), " ")
	// the literal, the named function and the local that holds one: the parameters 1 and 2 (after the first); the
	// method: the parameter 1 (the receiver is not an operand of the rule)
	for _, want := range []string{"cbparam.Literal$1:1", "cbparam.Literal$1:2", "cbparam.named:1", "cbparam.named:2", "cbparam.ThroughLocal$1:1", "cbparam.OnMethod$1:1"} {
		if !strings.Contains(" "+got+" ", " "+want+" ") {
			t.Errorf("no site %s in: %s", want, got)
		}
	}
	for _, not := range []string{":0", "OnMethod$1:2"} {
		if strings.Contains(got, not) {
			t.Errorf("a site %q that the rule does not mark in: %s", not, got)
		}
	}
	// the helper that registers what it is given, and the variable of two functions, are said, not marked
	miss := map[string]string{}
	for k, v := range res.UnresolvedCallbacks {
		miss[string(k.Func)[strings.LastIndex(string(k.Func), "/")+1:]] = v
	}
	for _, fn := range []string{"cbparam.Registers", "cbparam.Chosen"} {
		if miss[fn] == "" {
			t.Errorf("%s: the call whose function is not known is not recorded: %v", fn, miss)
		}
	}
	for _, fn := range []string{"cbparam.Literal", "cbparam.Named", "cbparam.ThroughLocal", "cbparam.OnMethod"} {
		if miss[fn] != "" {
			t.Errorf("%s: recorded as unresolved: %s", fn, miss[fn])
		}
	}
}

func TestCallbackParamFromAndTo(t *testing.T) {
	rules := callbackRules(t, `
sources:
  - kind: callback_param
    symbol: example.com/cases/cases/cbparam.Route
    operand: 1
    from: 1
    to: 1
    labels: [user_input]
`)
	res := enumerateGo(t, testutil.Testdata(), []string{"./cases/cbparam"}, rules)
	for _, s := range callbackSitesOf(res) {
		if strings.HasSuffix(s, ":2") || strings.HasSuffix(s, ":0") {
			t.Errorf("%s: outside from..to", s)
		}
	}
	if got := callbackSitesOf(res); len(got) != 3 {
		t.Errorf("want the parameter 1 of the three functions that are found, got %v", got)
	}
}

func TestNoCallbackParamRuleChangesNothing(t *testing.T) {
	res := enumerateGo(t, testutil.Testdata(), []string{"./cases/cbparam"}, callbackRules(t, "sources:\n  - symbol: a.B\n    labels: [x]\n"))
	if len(callbackSitesOf(res)) != 0 || len(res.UnresolvedCallbacks) != 0 {
		t.Errorf("no rule, and a site or a note: %v %v", callbackSitesOf(res), res.UnresolvedCallbacks)
	}
}
