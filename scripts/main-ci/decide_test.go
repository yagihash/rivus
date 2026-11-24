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

// The table is the combinations of the state of the head of main (none, queued, running, success, failure, cancelled), of the
// scheduled CI (none, success, a failure younger than 24 hours, one at 24 hours, one that a later success fixed) and of the label.
func TestDecide(t *testing.T) {
	h := time.Hour
	none := []Run(nil)
	head := func(status, conclusion string) []Run { return []Run{run(10, "push", status, conclusion, time.Minute)} }
	sched := func(conclusion string, ago time.Duration) []Run {
		return []Run{run(20, "schedule", "completed", conclusion, ago)}
	}
	tests := []struct {
		name     string
		head     []Run
		periodic []Run
		labels   []string
		want     string
	}{
		{"head: no run yet (main just got a push)", none, none, nil, "pending"},
		{"head: queued", head("queued", ""), none, nil, "pending"},
		{"head: running", head("in_progress", ""), none, nil, "pending"},
		{"head: success", head("completed", "success"), none, nil, "success"},
		{"head: failure", head("completed", "failure"), none, nil, "failure"},
		{"head: timed out", head("completed", "timed_out"), none, nil, "failure"},
		{"head: cancelled and nothing newer", head("completed", "cancelled"), none, nil, "pending"},
		{"head: re-run running after a failure", head("in_progress", ""), none, nil, "pending"},
		{"head: two runs, the newer one counts", []Run{run(11, "push", "completed", "success", time.Minute), run(10, "push", "completed", "failure", h)}, none, nil, "success"},
		{"head: cancelled, an older run of the same commit failed", []Run{run(11, "push", "completed", "cancelled", time.Minute), run(10, "push", "completed", "failure", h)}, none, nil, "pending"},
		{"head: run of another event is not the push", []Run{run(10, "schedule", "completed", "failure", h)}, none, nil, "pending"},

		{"label: head failure", head("completed", "failure"), none, []string{"x", FixesMainLabel}, "success"},
		{"label: head running", head("in_progress", ""), none, []string{FixesMainLabel}, "success"},
		{"label: head not started", none, none, []string{FixesMainLabel}, "success"},
		{"label: stale scheduled failure", head("completed", "success"), sched("failure", 25*h), []string{FixesMainLabel}, "success"},

		{"schedule: success", head("completed", "success"), sched("success", h), nil, "success"},
		{"schedule: failed 1 h ago", head("completed", "success"), sched("failure", h), nil, "success"},
		{"schedule: failed 23 h 59 m ago", head("completed", "success"), sched("failure", 24*h-time.Minute), nil, "success"},
		{"schedule: failed exactly 24 h ago", head("completed", "success"), sched("failure", 24*h), nil, "failure"},
		{"schedule: failed 30 h ago, head running", head("in_progress", ""), sched("failure", 30*h), nil, "failure"},
		{"schedule: failed 30 h ago, head not started", none, sched("failure", 30*h), nil, "failure"},
		{"schedule: failed 30 h ago, head cancelled", head("completed", "cancelled"), sched("failure", 30*h), nil, "failure"},
		{"schedule: failed 30 h ago, head failed", head("completed", "failure"), sched("failure", 30*h), nil, "failure"},
		{"schedule: by-hand run failed 30 h ago", head("completed", "success"), []Run{run(21, "workflow_dispatch", "completed", "failure", 30*h)}, nil, "failure"},
		{"schedule: a later success fixed it", head("completed", "success"), []Run{run(22, "schedule", "completed", "success", h), run(21, "schedule", "completed", "failure", 30*h)}, nil, "success"},
		{"schedule: 24 h counts from the oldest failure of the streak", head("completed", "success"),
			[]Run{run(23, "schedule", "completed", "failure", h), run(22, "workflow_dispatch", "completed", "failure", 25*h), run(21, "schedule", "completed", "success", 50*h)}, nil, "failure"},
		{"schedule: a new failure after a success starts a new streak", head("completed", "success"),
			[]Run{run(23, "schedule", "completed", "failure", h), run(22, "schedule", "completed", "success", 5*h), run(21, "schedule", "completed", "failure", 40*h)}, nil, "success"},
		{"schedule: a running run does not hide an old failure", head("completed", "success"),
			[]Run{run(22, "schedule", "in_progress", "", time.Minute), run(21, "schedule", "completed", "failure", 30*h)}, nil, "failure"},
		{"schedule: a cancelled run is passed over", head("completed", "success"),
			[]Run{run(22, "schedule", "completed", "cancelled", time.Minute), run(21, "schedule", "completed", "failure", 30*h)}, nil, "failure"},
		{"schedule: a run of another branch is ignored", head("completed", "success"),
			[]Run{{ID: 1, Event: "schedule", Branch: "feat/x", Status: "completed", Conclusion: "failure", CreatedAt: t0.Add(-30 * h), UpdatedAt: t0.Add(-30 * h)}}, nil, "success"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Decide(t0, tt.head, tt.periodic, tt.labels)
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
