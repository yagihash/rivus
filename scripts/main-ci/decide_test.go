package main

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

var t0 = time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)

func run(id int64, event, status, conclusion string, ago time.Duration) Run {
	return Run{ID: id, Event: event, Branch: "main", Status: status, Conclusion: conclusion,
		URL: "https://example.test/runs/x", CreatedAt: t0.Add(-ago), UpdatedAt: t0.Add(-ago)}
}

// The table is the combinations of the push runs on main (none, running, success, failure, cancelled, several), of the
// scheduled CI (none, success, a failure younger than 24 hours, one at 24 hours, one that a later success fixed) and of the label.
// Nothing is pending: a run that has not ended counts as nothing.
func TestDecide(t *testing.T) {
	h := time.Hour
	m := time.Minute
	none := []Run(nil)
	one := func(status, conclusion string) []Run { return []Run{run(10, "push", status, conclusion, m)} }
	sched := func(conclusion string, ago time.Duration) []Run {
		return []Run{run(20, "schedule", "completed", conclusion, ago)}
	}
	tests := []struct {
		name     string
		push     []Run
		periodic []Run
		labels   []string
		want     string
	}{
		{"push: no run at all", none, none, nil, "success"},
		{"push: queued, nothing before", one("queued", ""), none, nil, "success"},
		{"push: running, nothing before", one("in_progress", ""), none, nil, "success"},
		{"push: success", one("completed", "success"), none, nil, "success"},
		{"push: failure", one("completed", "failure"), none, nil, "failure"},
		{"push: timed out", one("completed", "timed_out"), none, nil, "failure"},
		{"push: only a cancelled run", one("completed", "cancelled"), none, nil, "success"},
		{"push: running, the one before succeeded", []Run{run(11, "push", "in_progress", "", m), run(10, "push", "completed", "success", 10*m)}, none, nil, "success"},
		{"push: running, the one before failed", []Run{run(11, "push", "in_progress", "", m), run(10, "push", "completed", "failure", 10*m)}, none, nil, "failure"},
		{"push: cancelled, the one before succeeded", []Run{run(11, "push", "completed", "cancelled", m), run(10, "push", "completed", "success", 10*m)}, none, nil, "success"},
		{"push: cancelled, the one before failed", []Run{run(11, "push", "completed", "cancelled", m), run(10, "push", "completed", "failure", 10*m)}, none, nil, "failure"},
		{"push: the newer completed run counts", []Run{run(11, "push", "completed", "success", m), run(10, "push", "completed", "failure", h)}, none, nil, "success"},
		{"push: a run of another branch is ignored", []Run{{ID: 1, Event: "push", Branch: "feat/x", Status: "completed", Conclusion: "failure", CreatedAt: t0.Add(-m), UpdatedAt: t0.Add(-m)}}, none, nil, "success"},
		{"push: a run of another event is not the push", []Run{run(10, "schedule", "completed", "failure", h)}, none, nil, "success"},

		{"label: push failure", one("completed", "failure"), none, []string{"x", FixesMainLabel}, "success"},
		{"label: no run", none, none, []string{FixesMainLabel}, "success"},
		{"label: stale scheduled failure", one("completed", "success"), sched("failure", 25*h), []string{FixesMainLabel}, "success"},

		{"schedule: success", one("completed", "success"), sched("success", h), nil, "success"},
		{"schedule: failed 1 h ago", one("completed", "success"), sched("failure", h), nil, "success"},
		{"schedule: failed 23 h 59 m ago", one("completed", "success"), sched("failure", 24*h-m), nil, "success"},
		{"schedule: failed exactly 24 h ago", one("completed", "success"), sched("failure", 24*h), nil, "failure"},
		{"schedule: failed 30 h ago, push running", one("in_progress", ""), sched("failure", 30*h), nil, "failure"},
		{"schedule: failed 30 h ago, no push run", none, sched("failure", 30*h), nil, "failure"},
		{"schedule: failed 30 h ago, push cancelled", one("completed", "cancelled"), sched("failure", 30*h), nil, "failure"},
		{"schedule: failed 30 h ago, push failed", one("completed", "failure"), sched("failure", 30*h), nil, "failure"},
		{"schedule: by-hand run failed 30 h ago", one("completed", "success"), []Run{run(21, "workflow_dispatch", "completed", "failure", 30*h)}, nil, "failure"},
		{"schedule: a later success fixed it", one("completed", "success"), []Run{run(22, "schedule", "completed", "success", h), run(21, "schedule", "completed", "failure", 30*h)}, nil, "success"},
		{"schedule: 24 h counts from the oldest failure of the streak", one("completed", "success"),
			[]Run{run(23, "schedule", "completed", "failure", h), run(22, "workflow_dispatch", "completed", "failure", 25*h), run(21, "schedule", "completed", "success", 50*h)}, nil, "failure"},
		{"schedule: a new failure after a success starts a new streak", one("completed", "success"),
			[]Run{run(23, "schedule", "completed", "failure", h), run(22, "schedule", "completed", "success", 5*h), run(21, "schedule", "completed", "failure", 40*h)}, nil, "success"},
		{"schedule: a running run does not hide an old failure", one("completed", "success"),
			[]Run{run(22, "schedule", "in_progress", "", m), run(21, "schedule", "completed", "failure", 30*h)}, nil, "failure"},
		{"schedule: a cancelled run is passed over", one("completed", "success"),
			[]Run{run(22, "schedule", "completed", "cancelled", m), run(21, "schedule", "completed", "failure", 30*h)}, nil, "failure"},
		{"schedule: a run of another branch is ignored", one("completed", "success"),
			[]Run{{ID: 1, Event: "schedule", Branch: "feat/x", Status: "completed", Conclusion: "failure", CreatedAt: t0.Add(-30 * h), UpdatedAt: t0.Add(-30 * h)}}, nil, "success"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Decide(t0, tt.push, tt.periodic, tt.labels)
			if got.State != tt.want {
				t.Fatalf("state = %s (%s), want %s", got.State, got.Description, tt.want)
			}
			if utf8.RuneCountInString(got.Description) > 140 || strings.Contains(got.Description, "\n") || got.Description == "" {
				t.Fatalf("bad description: %q", got.Description)
			}
			if got.State == "failure" && (got.TargetURL == "" || got.Failing == nil) {
				t.Fatalf("a failure names its run: %+v", got)
			}
		})
	}
}

func TestDescriptionNamesJobsAndIsCut(t *testing.T) {
	r := run(1, "push", "completed", "failure", time.Hour)
	r.FailedJobs = []string{"test (shard 3)", "lint"}
	got := Decide(t0, []Run{r}, nil, nil)
	if !strings.Contains(got.Description, "test (shard 3), lint") || !strings.Contains(got.Description, FixesMainLabel) {
		t.Fatalf("description: %q", got.Description)
	}
	r.FailedJobs = []string{strings.Repeat("あ", 200)}
	if n := utf8.RuneCountInString(Decide(t0, []Run{r}, nil, nil).Description); n != 140 {
		t.Fatalf("cut to %d characters, want 140", n)
	}
}
