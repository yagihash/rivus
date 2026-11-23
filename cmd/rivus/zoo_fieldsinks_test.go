package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/report"
	"github.com/yagihash/rivus/internal/taint"
)

// zooDeepParts are the rows whose path is longer than the engine keeps (K steps, 3): their findings are
// told by field_imprecision (path_limit) and never by the field itself, until the path is kept as far as the
// rule asks (docs/DESIGN-field-sinks.md, stage 4). The reason is the one of the @fp of the cell.
var zooDeepParts = map[string]string{
	"zoo/mark.SinkTask": "struct-field-deep: Task.*.MessageType.*.HTTPRequest.*.URL is 7 steps",
}

// TestZooFieldSinkRowsAreHit is the gate of the dead rows for the parts of a sink (docs/DESIGN-field-sinks.md 3): a
// misspelled path, or a field that a version of a library renamed, would be quiet for ever, and look like
// a decoy. Every sink of the rules of the zoo that names a field must be reached by a flow that the cells
// have, through the field itself (a finding with no uncertainty of the field); none of them may have
// field_unresolved (a name the program does not have).
func TestZooFieldSinkRowsAreHit(t *testing.T) {
	dir := zooRoot(t)
	f, err := os.Open(filepath.Join(dir, "rules.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rs, err := model.Parse(f, "zoo rules")
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]bool{}
	for i := range rs.Sinks {
		if rs.Sinks[i].HasParts() {
			rows[rs.Sinks[i].Symbol] = true
		}
	}
	if len(rows) == 0 {
		t.Fatal("the rules of the zoo have no sink with parts")
	}
	cmd := exec.Command(binary, "analyze", "-sites", "-rules", "rules.yaml", "./cells/...")
	cmd.Dir = dir
	out, err := cmd.Output()
	err = notFailed(err) // exit code 4: the zoo has cells that stop by the pass limit on purpose (F/state/global-chain); TestZoo* hold that the others converge
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	var rep report.Report
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatal(err)
	}
	sure, told := map[string]int{}, map[string]int{}
	for _, f := range rep.Findings {
		sym := f.Sink.Symbol
		if !rows[sym] {
			continue
		}
		told[sym]++
		unsure := false
		for _, u := range f.Uncertainties {
			unsure = unsure || u.Kind == taint.KindFieldImprecision || u.Kind == taint.KindFieldUnresolved
		}
		if !unsure {
			sure[sym]++
		}
	}
	for _, u := range rep.Uncertainties {
		if u.Kind == taint.KindFieldUnresolved {
			t.Errorf("a path of the rules has a name that the program does not have: %s", u.Detail)
		}
	}
	for sym := range rows {
		switch {
		case sure[sym] > 0:
		case zooDeepParts[sym] != "" && told[sym] > 0:
			t.Logf("%s: told only by field_imprecision: %s", sym, zooDeepParts[sym])
		default:
			t.Errorf("%s: the path of its parts is not met by any flow of the cells (%d findings, none through the field itself): a misspelling, or a form the cells lack", sym, told[sym])
		}
	}
	for sym := range zooDeepParts {
		if sure[sym] > 0 && strings.HasPrefix(sym, "zoo/") {
			t.Errorf("%s is told by the field itself now: take it out of zooDeepParts", sym)
		}
	}
}
