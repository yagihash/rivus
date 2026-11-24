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

func TestDecide(t *testing.T) {
	h := time.Hour
	tests := []struct {
		name   string
		runs   []Run
		labels []string
		want   string
	}{
		{"no runs", nil, nil, "success"},
		{"push success", []Run{run(1, "push", "completed", "success", h)}, nil, "success"},
		{"push failure", []Run{run(1, "push", "completed", "failure", h)}, nil, "failure"},
		{"push failure, label", []Run{run(1, "push", "completed", "failure", h)}, []string{"x", FixesMainLabel}, "success"},
		{"push running", []Run{run(2, "push", "in_progress", "", time.Minute), run(1, "push", "completed", "success", h)}, nil, "pending"},
		{"push running after a failure is pending", []Run{run(2, "push", "queued", "", time.Minute), run(1, "push", "completed", "failure", h)}, nil, "pending"},
		{"newer push fixed it", []Run{run(2, "push", "completed", "success", time.Minute), run(1, "push", "completed", "failure", h)}, nil, "success"},
		{"cancelled push is passed over", []Run{run(2, "push", "completed", "cancelled", time.Minute), run(1, "push", "completed", "success", h)}, nil, "success"},
		{"cancelled push over a failure", []Run{run(2, "push", "completed", "cancelled", time.Minute), run(1, "push", "completed", "failure", h)}, nil, "failure"},
		{"schedule failed 1 h ago", []Run{run(1, "schedule", "completed", "failure", h)}, nil, "success"},
		{"schedule failed 23 h 59 m ago", []Run{run(1, "schedule", "completed", "failure", 24*h-time.Minute)}, nil, "success"},
		{"schedule failed exactly 24 h ago", []Run{run(1, "schedule", "completed", "failure", 24*h)}, nil, "failure"},
		{"schedule failed 25 h ago, label", []Run{run(1, "schedule", "completed", "failure", 25*h)}, []string{FixesMainLabel}, "success"},
		{"by-hand run failed 30 h ago", []Run{run(1, "workflow_dispatch", "completed", "failure", 30*h)}, nil, "failure"},
		{"schedule fixed by a later success", []Run{run(2, "schedule", "completed", "success", h), run(1, "schedule", "completed", "failure", 30*h)}, nil, "success"},
		{"the 24 hours count from the oldest failure of the streak",
			[]Run{run(3, "schedule", "completed", "failure", h), run(2, "workflow_dispatch", "completed", "failure", 25*h), run(1, "schedule", "completed", "success", 50*h)}, nil, "failure"},
		{"failure then success then new failure: the streak is the new one",
			[]Run{run(3, "schedule", "completed", "failure", h), run(2, "schedule", "completed", "success", 5*h), run(1, "schedule", "completed", "failure", 40*h)}, nil, "success"},
		{"schedule still running does not hide an old failure",
			[]Run{run(2, "schedule", "in_progress", "", time.Minute), run(1, "schedule", "completed", "failure", 30*h)}, nil, "failure"},
		{"stale schedule failure wins over a running push",
			[]Run{run(2, "push", "in_progress", "", time.Minute), run(1, "schedule", "completed", "failure", 30*h)}, nil, "failure"},
		{"run of another branch is ignored",
			[]Run{{ID: 1, Event: "push", Branch: "feat/x", Status: "completed", Conclusion: "failure", CreatedAt: t0.Add(-h), UpdatedAt: t0.Add(-h)}}, nil, "success"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Decide(t0, tt.runs, tt.labels)
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
	got := Decide(t0, []Run{r}, nil)
	if !strings.Contains(got.Description, "test (shard 3), lint") || !strings.Contains(got.Description, FixesMainLabel) {
		t.Fatalf("description: %q", got.Description)
	}
	r.FailedJobs = []string{strings.Repeat("あ", 200)}
	if n := utf8.RuneCountInString(Decide(t0, []Run{r}, nil).Description); n != 140 {
		t.Fatalf("cut to %d characters, want 140", n)
	}
}
