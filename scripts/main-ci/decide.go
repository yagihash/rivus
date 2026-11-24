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
	State       string // success, pending, failure
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

// isPush reports whether a run is a push to main. A run of another branch is not main's.
func isPush(r Run) bool { return r.Event == "push" && r.Branch == "main" }

// isPeriodic reports whether a run is a scheduled or by-hand run on main.
func isPeriodic(r Run) bool {
	return (r.Event == "schedule" || r.Event == "workflow_dispatch") && r.Branch == "main"
}

// ignored reports whether a completed run says nothing about the health of main (cancelled or skipped).
func ignored(r Run) bool { return r.Conclusion == "cancelled" || r.Conclusion == "skipped" }

// latestPush gives the newest push run to main that is not cancelled. Concurrent pushes each have a group of their own, so a cancelled one is rare.
func latestPush(runs []Run) (Run, bool) {
	for _, r := range newestFirst(runs) {
		if isPush(r) && !(r.done() && ignored(r)) {
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

// Decide gives the status of `main-ci` for one pull request.
//
//	fixes-main label                                  success  (the way out: a pull request that fixes main)
//	newest push to main completed and not a success   failure
//	newest scheduled / by-hand run failed for 24 h+   failure
//	newest push to main still running                 pending
//	otherwise                                         success
//
// A failure of a scheduled run younger than 24 hours does not stop anything. A failure of the push to main always does.
// A run that is cancelled or skipped is passed over.
func Decide(now time.Time, runs []Run, labels []string) Result {
	for _, l := range labels {
		if l == FixesMainLabel {
			return Result{State: "success", Description: limit("fixes-main: main の CI が失敗していても、この PR は止めない")}
		}
	}
	push, havePush := latestPush(runs)
	if havePush && push.done() && push.Conclusion != "success" {
		p := push
		return Result{
			State: "failure", TargetURL: push.URL, Failing: &p,
			Description: limit(fmt.Sprintf("main の CI が失敗。main を直すか、直す PR に %s のラベルを付ける%s", FixesMainLabel, jobs(push))),
		}
	}
	if since, newest, ok := periodicFailure(runs); ok && now.Sub(since) >= StaleAfter {
		n := newest
		return Result{
			State: "failure", TargetURL: newest.URL, Failing: &n,
			Description: limit(fmt.Sprintf("定期の CI が失敗して 24 時間たった。直すか、直す PR に %s のラベルを付ける%s", FixesMainLabel, jobs(newest))),
		}
	}
	if havePush && !push.done() {
		return Result{State: "pending", TargetURL: push.URL, Description: limit("main の CI を実行中。終わるまで待つ")}
	}
	return Result{State: "success", Description: limit("main の CI は成功")}
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
