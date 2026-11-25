package main

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"sort"
	"strings"
	"testing"
)

func TestMaxMemoryParse(t *testing.T) {
	t.Parallel()
	ok := map[string]int64{
		"6GiB": 6 << 30, "6gib": 6 << 30, "6GB": 6_000_000_000, "512MiB": 512 << 20, "512MB": 512_000_000,
		"1KiB": 1024, "1KB": 1000, "1TiB": 1 << 40, "12345": 12345, "100B": 100, " 2 GiB ": 2 << 30,
	}
	for in, want := range ok {
		n, off, err := parseMemorySize(in)
		if err != nil || off || n != want {
			t.Errorf("parseMemorySize(%q) = %d, %v, %v; want %d", in, n, off, err, want)
		}
	}
	if n, off, err := parseMemorySize("off"); err != nil || !off || n != 0 {
		t.Errorf("off = %d, %v, %v", n, off, err)
	}
	for _, in := range []string{"", "GiB", "-1GiB", "0", "0MiB", "6 gigs", "1.5GiB", "x", "9223372036854775807GiB"} {
		if _, _, err := parseMemorySize(in); err == nil || !strings.Contains(err.Error(), "off") {
			t.Errorf("parseMemorySize(%q): err = %v, want an error that shows the formats", in, err)
		}
	}
}

func TestMemoryLimitDefault(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ phys, cgroup, want int64 }{
		{24 * gib, 0, 6 * gib},
		{16 * gib, 0, 4 * gib},
		{8 * gib, 0, 4 * gib},
		{4 * gib, 0, 2 * gib},
		{64 * gib, 0, 16 * gib},
		{24 * gib, 8 * gib, 4 * gib}, // the cgroup is smaller: 8 GiB base
		{24 * gib, 2 * gib, 1 * gib}, // 2 GiB base: max(0.5, min(4, 1)) = 1 GiB
		{8 * gib, 64 * gib, 4 * gib}, // the cgroup is larger: the machine
	} {
		got, ok := defaultMemoryLimit(c.phys, c.cgroup)
		if !ok || got != c.want {
			t.Errorf("defaultMemoryLimit(%d GiB, %d GiB) = %d GiB, %v; want %d GiB", c.phys/gib, c.cgroup/gib, got/gib, ok, c.want/gib)
		}
	}
	if _, ok := defaultMemoryLimit(0, 0); ok {
		t.Error("an unknown memory gave a limit")
	}
}

func TestMemoryLimitPrecedence(t *testing.T) {
	t.Parallel()
	none := int64(math.MaxInt64)
	// default
	m, err := decideMemoryLimit("", "", none, 24*gib, 0, "")
	if err != nil || m.Source != "default" || m.Bytes != 6*gib || !m.set || m.Note != "" {
		t.Errorf("default: %+v, %v", m, err)
	}
	// the memory of the machine is not known
	m, _ = decideMemoryLimit("", "", none, 0, 0, "no /proc")
	if m.Source != "none" || m.Bytes != 0 || m.set || !strings.Contains(m.Note, "no /proc") {
		t.Errorf("unknown: %+v", m)
	}
	// the environment wins over the default and is left as it is
	m, _ = decideMemoryLimit("", "3GiB", 3*gib, 24*gib, 0, "")
	if m.Source != "env" || m.Bytes != 3*gib || m.set {
		t.Errorf("env: %+v", m)
	}
	// the flag wins over the environment, and says so
	m, err = decideMemoryLimit("5GiB", "3GiB", 3*gib, 24*gib, 0, "")
	if err != nil || m.Source != "flag" || m.Bytes != 5*gib || !m.set || !strings.Contains(m.Note, "GOMEMLIMIT=3GiB") {
		t.Errorf("flag over env: %+v, %v", m, err)
	}
	m, _ = decideMemoryLimit("5GiB", "", none, 24*gib, 0, "")
	if m.Source != "flag" || m.Note != "" {
		t.Errorf("flag: %+v", m)
	}
	// off: no limit, the default and the environment's too
	m, _ = decideMemoryLimit("off", "3GiB", 3*gib, 24*gib, 0, "")
	if m.Source != "none" || m.Bytes != 0 || !m.set || !strings.Contains(m.Note, "GOMEMLIMIT=3GiB") {
		t.Errorf("off: %+v", m)
	}
	if _, err := decideMemoryLimit("junk", "", none, 24*gib, 0, ""); err == nil {
		t.Error("a bad size was taken")
	}
}

func TestMaxMemoryFlagAndSummary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module m\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "m.go"), []byte("package m\n\nfunc F() int { return 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(env string, args ...string) (int, map[string]any) {
		cmd := exec.Command(binary, append([]string{"analyze", "-no-builtin-rules", "-no-deps", "-detail", "summary"}, append(args, "./...")...)...)
		cmd.Dir = dir
		cmd.Env = append(cmd.Environ(), "GOMEMLIMIT="+env)
		out, err := cmd.Output()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
		var rep map[string]any
		_ = json.Unmarshal(out, &rep)
		return code, rep
	}
	field := func(rep map[string]any, k string) any {
		s, _ := rep["summary"].(map[string]any)
		return s[k]
	}
	// a bad value is a usage error (2) that shows the formats
	cmd := exec.Command(binary, "analyze", "-max-memory", "lots")
	out, err := cmd.CombinedOutput()
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 2 || !strings.Contains(string(out), "512MiB") {
		t.Errorf("-max-memory lots: %v\n%s", err, out)
	}
	if _, rep := run("", "-max-memory", "512MiB"); rep == nil {
		t.Fatal("no JSON output")
	} else {
		if field(rep, "memory_limit_source") != "flag" || field(rep, "memory_limit_bytes") != float64(512<<20) {
			t.Errorf("flag summary: %v %v", field(rep, "memory_limit_source"), field(rep, "memory_limit_bytes"))
		}
	}
	if _, rep := run("", "-max-memory", "off"); rep == nil {
		t.Fatal("no JSON output")
	} else {
		if field(rep, "memory_limit_source") != "none" || field(rep, "memory_limit_bytes") != nil {
			t.Errorf("off summary: %v %v", field(rep, "memory_limit_source"), field(rep, "memory_limit_bytes"))
		}
	}
	if _, rep := run("1GiB"); rep == nil {
		t.Fatal("no JSON output")
	} else {
		if field(rep, "memory_limit_source") != "env" || field(rep, "memory_limit_bytes") != float64(1<<30) {
			t.Errorf("env summary: %v %v", field(rep, "memory_limit_source"), field(rep, "memory_limit_bytes"))
		}
	}
	if _, rep := run("1GiB", "-max-memory", "2GiB"); rep == nil {
		t.Fatal("no JSON output")
	} else {
		if n, _ := field(rep, "memory_limit_note").(string); !strings.Contains(n, "GOMEMLIMIT=1GiB") {
			t.Errorf("override note = %q", n)
		}
	}
	if _, rep := run(""); rep == nil {
		t.Fatal("no JSON output")
	} else {
		if src := field(rep, "memory_limit_source"); src != "default" && src != "none" {
			t.Errorf("default summary source = %v", src)
		}
	}
}

// TestMaxMemoryKeepsResult: the limit changes how the memory is used, not what is found (ADR-121).
func TestMaxMemoryKeepsResult(t *testing.T) {
	t.Parallel()
	root := "../.."
	gen := t.TempDir() + "/gen"
	build := exec.Command("go", "build", "-o", gen, "./bench/go-scale/gen")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the generator: %v\n%s", err, out)
	}
	dir := t.TempDir()
	if out, err := exec.Command(gen, "-shapes", "all", "-out", dir).CombinedOutput(); err != nil {
		t.Fatalf("gen: %v\n%s", err, out)
	}
	type result struct {
		ids   []string
		elems any
	}
	get := func(limit string) result {
		cmd := exec.Command(binary, "analyze", "-sites", "-workers", "1", "-rules", "rules.yaml", "-max-memory", limit, "./...")
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("analyze -max-memory %s: %v", limit, err)
		}
		var rep struct {
			Findings []struct {
				ID string `json:"id"`
			} `json:"findings"`
			Stats map[string]any `json:"stats"`
		}
		if err := json.Unmarshal(out, &rep); err != nil {
			t.Fatal(err)
		}
		var r result
		for _, f := range rep.Findings {
			r.ids = append(r.ids, f.ID)
		}
		sort.Strings(r.ids)
		r.elems = rep.Stats["summary_elems"]
		return r
	}
	a, b := get("512MiB"), get("off")
	if len(a.ids) == 0 {
		t.Fatal("no findings: the check would prove nothing")
	}
	if !reflect.DeepEqual(a, b) {
		t.Errorf("the limit changed the result:\n512MiB: %d findings, summary_elems %v\noff: %d findings, summary_elems %v", len(a.ids), a.elems, len(b.ids), b.elems)
	}
}

func TestGogcFor(t *testing.T) {
	mid := func(lim int64) int {
		return gogcLow + int(float64(gogcDefault-gogcLow)*float64(lim-gogcLowLimit)/float64(gogcHighLimit-gogcLowLimit)+0.5)
	}
	for _, c := range []struct {
		name  string
		limit int64
		want  int
	}{
		{"none", 0, 50},
		{"8 GiB", 8 * gib, 50},
		{"6 GiB (the default of a 24 GiB machine)", 6 * gib, 50},
		{"6442450944 (the same, in bytes)", 6442450944, 50},
		{"5 GiB: between, linear", 5 * gib, mid(5 * gib)},
		{"4 GiB: the constant", 4 * gib, gogcLow},
		{"2 GiB: the constant", 2 * gib, gogcLow},
	} {
		if got := gogcFor(c.limit); got != c.want {
			t.Errorf("%s: gogcFor = %d, want %d", c.name, got, c.want)
		}
	}
	// the line of the rule: it never goes above 50 and never below the constant, and it does not fall when the limit rises
	prev := 0
	for l := int64(1) * gib; l <= 9*gib; l += gib / 8 {
		g := gogcFor(l)
		if g < gogcLow || g > gogcDefault || g < prev {
			t.Fatalf("limit %d: %d (prev %d)", l, g, prev)
		}
		prev = g
	}
}

// GOGC in the environment wins: applyMemoryLimit does not change the percent then.
func TestApplyMemoryLimitKeepsGOGCOfTheEnvironment(t *testing.T) {
	old := debug.SetGCPercent(77)
	defer debug.SetGCPercent(old)
	oldLimit := debug.SetMemoryLimit(-1)
	defer debug.SetMemoryLimit(oldLimit)
	t.Setenv("GOGC", "77")
	if _, err := applyMemoryLimit("4GiB"); err != nil {
		t.Fatal(err)
	}
	if got := debug.SetGCPercent(77); got != 77 {
		t.Errorf("GOGC in the environment: the percent is %d, want 77", got)
	}
	t.Setenv("GOGC", "")
	if _, err := applyMemoryLimit("4GiB"); err != nil {
		t.Fatal(err)
	}
	if got := debug.SetGCPercent(77); got != gogcLow {
		t.Errorf("no GOGC, limit 4GiB: the percent is %d, want %d", got, gogcLow)
	}
	if _, err := applyMemoryLimit("8GiB"); err != nil {
		t.Fatal(err)
	}
	if got := debug.SetGCPercent(77); got != gogcDefault {
		t.Errorf("no GOGC, limit 8GiB: the percent is %d, want %d", got, gogcDefault)
	}
}
