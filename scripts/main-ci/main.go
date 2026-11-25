// main-ci sets the check run `main-ci` on the head of every open pull request of this repository, from the result of ci.yml on main
// (.github/workflows/main-ci.yml; the rules are in decide.go). It calls the GitHub API through `gh` and reads GH_TOKEN and
// GITHUB_REPOSITORY from the environment. It does not read or run the code of any pull request.
// Pull requests from forks are skipped: a fork is not in scope (the repository has no forks in use).
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func gh(args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

type apiRun struct {
	ID         int64     `json:"id"`
	Event      string    `json:"event"`
	Branch     string    `json:"head_branch"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion"`
	URL        string    `json:"html_url"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func toRuns(out []byte) ([]Run, error) {
	var body struct {
		Runs []apiRun `json:"workflow_runs"`
	}
	if err := json.Unmarshal(out, &body); err != nil {
		return nil, err
	}
	var runs []Run
	for _, r := range body.Runs {
		runs = append(runs, Run{ID: r.ID, Event: r.Event, Branch: r.Branch, Status: r.Status, Conclusion: r.Conclusion, URL: r.URL, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt})
	}
	return runs, nil
}

// readState reads, from the API now, the recent push runs of ci.yml on main and the recent scheduled / by-hand runs.
func readState(repo string) (pushRuns, periodic []Run, err error) {
	out, err := gh("api", fmt.Sprintf("repos/%s/actions/workflows/ci.yml/runs?branch=main&event=push&per_page=30", repo))
	if err != nil {
		return nil, nil, err
	}
	if pushRuns, err = toRuns(out); err != nil {
		return nil, nil, err
	}
	for _, ev := range []string{"schedule", "workflow_dispatch"} {
		out, err := gh("api", fmt.Sprintf("repos/%s/actions/workflows/ci.yml/runs?branch=main&event=%s&per_page=30", repo, ev))
		if err != nil {
			return nil, nil, err
		}
		rs, err := toRuns(out)
		if err != nil {
			return nil, nil, err
		}
		periodic = append(periodic, rs...)
	}
	return pushRuns, periodic, nil
}

// failedJobs gives the names of the jobs of a run that did not succeed (`ci-ok` is the sum of the others, so it is left out).
func failedJobs(repo string, id int64) []string {
	out, err := gh("api", fmt.Sprintf("repos/%s/actions/runs/%d/jobs?per_page=100", repo, id))
	if err != nil {
		return nil
	}
	var body struct {
		Jobs []struct {
			Name       string `json:"name"`
			Conclusion string `json:"conclusion"`
		} `json:"jobs"`
	}
	if json.Unmarshal(out, &body) != nil {
		return nil
	}
	var names []string
	for _, j := range body.Jobs {
		if j.Conclusion == "failure" && j.Name != "ci-ok" {
			names = append(names, j.Name)
		}
	}
	return names
}

type pull struct {
	Number int `json:"number"`
	Head   struct {
		SHA  string `json:"sha"`
		Repo struct {
			FullName string `json:"full_name"`
		} `json:"repo"`
	} `json:"head"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

// writeCheck creates the check run `main-ci` on a commit (the app that owns it is GitHub Actions: only an app can create a check run,
// so the ruleset can pin the app and nobody can write the value by hand). A required check is judged by the newest check run of that
// name, so a new one replaces the old one. It is not created again when the newest one already says the same.
func writeCheck(repo, sha string, res Result) error {
	status, conclusion := "completed", res.State
	cur, err := gh("api", fmt.Sprintf("repos/%s/commits/%s/check-runs?check_name=main-ci&per_page=1", repo, sha),
		"--jq", `.check_runs[0] | "\(.status) \(.conclusion // "") \(.output.title // "")"`)
	if err == nil && strings.TrimSpace(string(cur)) == strings.TrimSpace(status+" "+conclusion+" "+res.Description) {
		return nil
	}
	body := map[string]any{
		"name": "main-ci", "head_sha": sha, "status": status,
		"output": map[string]string{"title": res.Description, "summary": res.Description},
	}
	body["conclusion"] = conclusion
	if res.TargetURL != "" {
		body["details_url"] = res.TargetURL
	}
	in, _ := json.Marshal(body)
	cmd := exec.Command("gh", "api", "-X", "POST", fmt.Sprintf("repos/%s/check-runs", repo), "--input", "-")
	cmd.Stdin = strings.NewReader(string(in))
	cmd.Stderr = os.Stderr
	_, err = cmd.Output()
	return err
}

func main() {
	repo := os.Getenv("GITHUB_REPOSITORY")
	if repo == "" {
		fmt.Fprintln(os.Stderr, "GITHUB_REPOSITORY is not set")
		os.Exit(2)
	}
	pushRuns, periodic, err := readState(repo)
	if err != nil {
		fmt.Fprintln(os.Stderr, "list runs:", err)
		os.Exit(1)
	}
	out, err := gh("api", "--paginate", "--slurp", fmt.Sprintf("repos/%s/pulls?state=open&per_page=100", repo))
	if err != nil {
		fmt.Fprintln(os.Stderr, "list pull requests:", err)
		os.Exit(1)
	}
	var pages [][]pull
	if err := json.Unmarshal(out, &pages); err != nil {
		fmt.Fprintln(os.Stderr, "pull requests:", err)
		os.Exit(1)
	}
	now := time.Now()
	failed := 0
	filled := map[int64]bool{}
	for _, page := range pages {
		for _, p := range page {
			if p.Head.Repo.FullName != repo { // a fork (or a deleted fork): out of scope
				fmt.Printf("#%d: skipped (not from this repository)\n", p.Number)
				continue
			}
			var labels []string
			for _, l := range p.Labels {
				labels = append(labels, l.Name)
			}
			res := Decide(now, pushRuns, periodic, labels)
			if res.Failing != nil && !filled[res.Failing.ID] {
				// Look up the failed jobs once, then decide again with the names in the description.
				for _, rs := range [][]Run{pushRuns, periodic} {
					for i := range rs {
						if rs[i].ID == res.Failing.ID {
							rs[i].FailedJobs = failedJobs(repo, res.Failing.ID)
						}
					}
				}
				filled[res.Failing.ID] = true
			}
			res = Decide(now, pushRuns, periodic, labels)
			if err := writeCheck(repo, p.Head.SHA, res); err != nil {
				fmt.Fprintf(os.Stderr, "#%d: writing the check run failed: %v\n", p.Number, err)
				failed++
				continue
			}
			fmt.Printf("#%d %s: %s %s\n", p.Number, strings.TrimSpace(p.Head.SHA[:7]), res.State, res.Description)
		}
	}
	if failed > 0 {
		os.Exit(1)
	}
}
