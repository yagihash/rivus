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

// isPeriodic reports whether a run is a scheduled or by-hand run on main.
func isPeriodic(r Run) bool {
	return (r.Event == "schedule" || r.Event == "workflow_dispatch") && r.Branch == "main"
}

// ignored reports whether a completed run says nothing about the health of main (cancelled or skipped).
func ignored(r Run) bool { return r.Conclusion == "cancelled" || r.Conclusion == "skipped" }

// newestHeadRun gives the newest push run of ci.yml for the head commit of main. The caller passes only the runs of that commit
// (`head_sha=<head>`); a re-run keeps its run id and replaces the result, so the newest run is the current state of the commit.
func newestHeadRun(headRuns []Run) (Run, bool) {
	for _, r := range newestFirst(headRuns) {
		if r.Event == "push" && r.Branch == "main" {
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
// headRuns are the push runs of ci.yml for the head commit of main (empty: the CI of the head has not started yet);
// periodic are the recent scheduled and by-hand runs of ci.yml on main.
//
//	fixes-main label                                       success  (the way out: a pull request that fixes main)
//	CI of the head of main completed, not a success        failure
//	CI of the head of main was cancelled                   pending  (nothing says main is green; re-run it, or a newer push replaces it)
//	scheduled / by-hand run failed for 24 hours or more    failure
//	CI of the head of main not started, queued, running    pending
//	CI of the head of main succeeded                       success
//
// A failure of a scheduled run younger than 24 hours does not stop anything. A failure of the head of main always does.
func Decide(now time.Time, headRuns, periodic []Run, labels []string) Result {
	for _, l := range labels {
		if l == FixesMainLabel {
			return Result{State: "success", Description: limit("fixes-main: main の CI が失敗していても、この PR は止めない")}
		}
	}
	head, haveHead := newestHeadRun(headRuns)
	if haveHead && head.done() && head.Conclusion != "success" && head.Conclusion != "cancelled" {
		p := head
		return Result{
			State: "failure", TargetURL: head.URL, Failing: &p,
			Description: limit(fmt.Sprintf("main の CI が失敗。main を直すか、直す PR に %s のラベルを付ける%s", FixesMainLabel, jobs(head))),
		}
	}
	if since, newest, ok := periodicFailure(periodic); ok && now.Sub(since) >= StaleAfter {
		n := newest
		return Result{
			State: "failure", TargetURL: newest.URL, Failing: &n,
			Description: limit(fmt.Sprintf("定期の CI が失敗して 24 時間たった。直すか、直す PR に %s のラベルを付ける%s", FixesMainLabel, jobs(newest))),
		}
	}
	switch {
	case !haveHead:
		return Result{State: "pending", Description: limit("main の先頭のコミットの CI がまだ始まっていない。始まって終わるまで待つ")}
	case head.done() && head.Conclusion == "cancelled":
		return Result{State: "pending", TargetURL: head.URL, Description: limit("main の先頭のコミットの CI が取り消された。再実行するか、新しい push を待つ")}
	case !head.done():
		return Result{State: "pending", TargetURL: head.URL, Description: limit("main の CI を実行中。終わるまで待つ")}
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
