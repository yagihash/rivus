package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Labels and names that the workflow and the ruleset share.
const (
	FixesMainLabel = "fixes-main"
	// StaleAfter is how long a failed scheduled run may stay unfixed before pull requests are stopped.
	StaleAfter = 24 * time.Hour
	// maxDescription is the limit of a commit status description in GitHub's API.
	maxDescription = 140
)

// Run is one run of ci.yml (what `GET /actions/workflows/ci.yml/runs` returns, reduced to what the decision reads).
type Run struct {
	ID         int64
	Event      string // push, schedule, workflow_dispatch
	Branch     string
	Status     string // queued, in_progress, completed
	Conclusion string // success, failure, cancelled, skipped, ...
	URL        string
	CreatedAt  time.Time
	UpdatedAt  time.Time // for a completed run: when it finished
	// FailedJobs are the names of the failed jobs. Filled in only for the run that the decision names (see NeedJobs).
	FailedJobs []string
}

// Result is the commit status to set.
type Result struct {
	State       string // success, failure
	Description string // at most 140 characters, one line: what to fix
	TargetURL   string
	// Failing is the run behind a failure, so that the caller can look up its failed jobs and decide again.
	Failing *Run
}

func (r Run) done() bool { return r.Status == "completed" }

// newestFirst returns the runs sorted by creation time, newest first.
func newestFirst(runs []Run) []Run {
	out := append([]Run(nil), runs...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

// isPeriodic reports whether a run is a scheduled or by-hand run on main.
func isPeriodic(r Run) bool {
	return (r.Event == "schedule" || r.Event == "workflow_dispatch") && r.Branch == "main"
}

// ignored reports whether a completed run says nothing about the health of main (cancelled or skipped).
func ignored(r Run) bool { return r.Conclusion == "cancelled" || r.Conclusion == "skipped" }

// latestCompletedPush gives the newest completed push run of ci.yml on main that says something about its health (a cancelled or
// skipped run is passed over). It is not always the run of the head commit: while the CI of the head runs, the one before it counts.
func latestCompletedPush(pushRuns []Run) (Run, bool) {
	for _, r := range newestFirst(pushRuns) {
		if r.Event == "push" && r.Branch == "main" && r.done() && !ignored(r) {
			return r, true
		}
	}
	return Run{}, false
}

// periodicFailure looks at the scheduled and by-hand runs, newest first. If the newest completed one failed, it gives the
// oldest failure of the unbroken run of failures that ends there (the time from which main has been red) and the newest failed run.
func periodicFailure(runs []Run) (since time.Time, newest Run, ok bool) {
	for _, r := range newestFirst(runs) {
		if !isPeriodic(r) || !r.done() || ignored(r) {
			continue
		}
		if r.Conclusion == "success" {
			break
		}
		if !ok {
			newest = r
			ok = true
		}
		since = r.UpdatedAt // keeps moving back to older failures
	}
	return since, newest, ok
}

// Decide gives the status of `main-ci` for one pull request. It is a pure function of what the API says now (the caller reads it
// again before each write), never of the event that started the workflow, so events that arrive in any order give the same answer.
//
// pushRuns are the recent push runs of ci.yml on main (any commit); periodic are the recent scheduled and by-hand runs of ci.yml on main.
//
//	fixes-main label                                        success  (the way out: a pull request that fixes main)
//	newest completed push run on main failed                failure  (a cancelled run is passed over; a run still going is not waited for)
//	scheduled / by-hand run failed for 24 hours or more     failure
//	anything else (also: no completed run at all)           success
//
// Nothing is pending: a run that is queued or going counts as nothing yet, and if it ends in a failure, the `completed` event of
// the workflow run decides again and turns the check to failure. A failure of a scheduled run younger than 24 hours does not stop anything.
func Decide(now time.Time, pushRuns, periodic []Run, labels []string) Result {
	for _, l := range labels {
		if l == FixesMainLabel {
			return Result{State: "success", Description: limit("fixes-main: main の CI が失敗していても、この PR は止めない")}
		}
	}
	if last, ok := latestCompletedPush(pushRuns); ok && last.Conclusion != "success" {
		p := last
		return Result{
			State: "failure", TargetURL: last.URL, Failing: &p,
			Description: limit(fmt.Sprintf("main の CI が失敗。main を直すか、直す PR に %s のラベルを付ける%s", FixesMainLabel, jobs(last))),
		}
	}
	if since, newest, ok := periodicFailure(periodic); ok && now.Sub(since) >= StaleAfter {
		n := newest
		return Result{
			State: "failure", TargetURL: newest.URL, Failing: &n,
			Description: limit(fmt.Sprintf("定期の CI が失敗して 24 時間たった。直すか、直す PR に %s のラベルを付ける%s", FixesMainLabel, jobs(newest))),
		}
	}
	return Result{State: "success", Description: limit("main の CI に失敗の記録は無い")}
}

func jobs(r Run) string {
	if len(r.FailedJobs) == 0 {
		return ""
	}
	return " 失敗: " + strings.Join(r.FailedJobs, ", ")
}

// limit cuts a description to what the API accepts (140 characters, counted as characters).
func limit(s string) string {
	rs := []rune(s)
	if len(rs) <= maxDescription {
		return s
	}
	return string(rs[:maxDescription-1]) + "…"
}
