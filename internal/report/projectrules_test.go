package report

import (
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/model"
)

// A rule file that comes with the analysed code can remove detections of that code. The report
// says that it was read, and counts what it can remove (yagihash/rivus #57).
func TestProjectRulesAreCounted(t *testing.T) {
	parse := func(name, text string) *model.RuleSet {
		rs, err := model.Parse(strings.NewReader(text), name)
		if err != nil {
			t.Fatal(err)
		}
		return rs
	}
	project := parse(".rivus/rules/x.yaml", `
ruleset: project/x@1
sources:
  - symbol: example.com/a.In
    labels: [user_input]
sanitizers:
  - symbol: example.com/a.Clean
    drops: ["*"]
  - symbol: example.com/a.Escape
    drops: [user_input]
propagators:
  - symbol: example.com/a.Quiet
    pure: true
  - symbol: example.com/a.Pass
    flows: [{from: args, to: result}]
`)
	project.Project = true
	own := parse("extra.yaml", "ruleset: user/y@1\nsanitizers:\n  - symbol: example.com/b.C\n    drops: [\"*\"]\n")
	got := projectRules(model.Merge(project, own))
	if len(got) != 1 {
		t.Fatalf("%d project rule files, want 1 (only what the analysed directory brought): %+v", len(got), got)
	}
	p := got[0]
	if p.File != ".rivus/rules/x.yaml" || p.Sources != 1 || p.Sanitizers != 2 || p.DropAll != 1 || p.Propagators != 2 || p.PureModels != 1 {
		t.Errorf("counts: %+v", p)
	}
}
