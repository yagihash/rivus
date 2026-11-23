package taint

import (
	"bytes"
	"encoding/json"
	"regexp"
	"testing"

	"github.com/yagihash/rivus/internal/frontend/goir"
	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/sg"
	"github.com/yagihash/rivus/internal/sites"
	"github.com/yagihash/rivus/internal/testutil"
)

// loaded is Go code read all the way to the SSA graph the engine runs on.
type loaded struct {
	gossa *gossa.Program
	ir    *ir.Program
	texts ir.Texts
	sg    *sg.Program
}

func loadIR(t testing.TB, dir string, patterns ...string) *loaded {
	t.Helper()
	p, err := gossa.Load(dir, patterns, gossa.Options{NaiveForm: true, WithLifted: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.LoadErrors) != 0 {
		t.Fatalf("load errors: %v", p.LoadErrors)
	}
	prog, err := goir.Extract(p, goir.Options{All: true})
	if err != nil {
		t.Fatal(err)
	}
	g, err := sg.Build(ir.MemSource(prog))
	if err != nil {
		t.Fatal(err)
	}
	return &loaded{gossa: p, ir: prog, texts: goir.NewTexts(p), sg: g}
}

func (l *loaded) sites(rules *model.Rules) *sites.Result {
	return sites.Enumerate(ir.MemSource(l.ir), l.texts, rules)
}

func (l *loaded) engine(st *sites.Result, rules *model.Rules, opts Options) *Engine {
	e := New(l.sg, l.texts, st, rules, opts)
	e.SetLoadErrors(l.gossa.LoadErrors)
	return e
}

// The IR that goes through the JSON Lines stream (as it will from another
// process) gives the result that the IR handed over directly gives.
func TestTheStreamGivesTheSameResultAsTheDirectIR(t *testing.T) {
	rules := testutil.Rules(t)
	for _, name := range []string{"inter", "heap", "guard", "entry"} {
		t.Run(name, func(t *testing.T) {
			direct := loadIR(t, testutil.Testdata(), "./cases/"+name+"/...")
			// Everything the core needs now comes through the stream: the
			// texts too, which are not asked of the extractor.
			goir.AddTexts(direct.texts.(*goir.Texts), direct.ir)
			var buf bytes.Buffer
			if err := ir.Write(&buf, direct.ir); err != nil {
				t.Fatal(err)
			}
			back, err := ir.Read(&buf)
			if err != nil {
				t.Fatal(err)
			}
			g, err := sg.Build(ir.MemSource(back))
			if err != nil {
				t.Fatal(err)
			}
			viaStream := &loaded{gossa: direct.gossa, ir: back, texts: ir.NewStreamTexts(back), sg: g}

			run := func(l *loaded) []byte {
				st := l.sites(rules)
				data, err := json.Marshal(l.engine(st, rules, DefaultOptions()).Run())
				if err != nil {
					t.Fatal(err)
				}
				// The run time is the one thing that differs.
				return regexp.MustCompile(`,"(duration|dep_spent|own_spent)_ms":\d+`).ReplaceAll(data, nil)
			}
			a, b := run(direct), run(viaStream)
			if !bytes.Equal(a, b) {
				i := 0
				for i < len(a) && i < len(b) && a[i] == b[i] {
					i++
				}
				lo := max(0, i-200)
				t.Errorf("the result differs at %d:\n  direct: %s\n  stream: %s", i, a[lo:min(len(a), i+300)], b[lo:min(len(b), i+300)])
			}
			if len(a) < 200 {
				t.Errorf("the result is too small to mean anything: %s", a)
			}
		})
	}
}
