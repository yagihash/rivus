// Package issues is a cell of the package go_github.
//
// @cell(F/pkg/go-github/issues)
// @shape: a value of a source goes to the owner, the repository, the path or the ref of a call of the service of go-github (the rows of resource_name of the model); the skeleton records it
package issues

import (
	"context"

	"github.com/google/go-github/v93/github"
	"zoo/mark"
)

func use(...any) {}

func init() {
	mark.Case("F/pkg/go-github/issues/vuln", vuln)
	mark.Case("F/pkg/go-github/issues/fixed", fixed)
	mark.Case("F/pkg/go-github/issues/decoy", decoy)
}

func vuln() { // @vuln
	ctx := context.Background()
	c, _ := github.NewClient()
	v := mark.Src("pkg-go-github-issues-vuln")                                                // @source(pkg-go-github-issues-vuln)
	use(c.Issues.Get(ctx, v, v, 1))                                                           // @sink(pkg-go-github-issues-vuln)
	use(c.Issues.ListByRepo(ctx, v, v, nil))                                                  // @sink(pkg-go-github-issues-vuln)
	use(c.Issues.Create(ctx, v, v, github.CreateIssueRequest{}))                              // @sink(pkg-go-github-issues-vuln)
	use(c.Issues.ListComments(ctx, v, v, 1, nil))                                             // @sink(pkg-go-github-issues-vuln)
	use(c.Issues.GetComment(ctx, v, v, 1))                                                    // @sink(pkg-go-github-issues-vuln)
	use(c.Issues.CreateComment(ctx, v, v, 1, github.IssueCommentRequest{}))                   // @sink(pkg-go-github-issues-vuln)
	use(c.PullRequests.Get(ctx, v, v, 1))                                                     // @sink(pkg-go-github-issues-vuln)
	use(c.PullRequests.GetRaw(ctx, v, v, 1, github.RawOptions{}))                             // @sink(pkg-go-github-issues-vuln)
	use(c.PullRequests.Create(ctx, v, v, github.CreatePullRequest{}))                         // @sink(pkg-go-github-issues-vuln)
	use(c.PullRequests.Edit(ctx, v, v, 1, nil))                                               // @sink(pkg-go-github-issues-vuln)
	use(c.PullRequests.ListFiles(ctx, v, v, 1, nil))                                          // @sink(pkg-go-github-issues-vuln)
	use(c.PullRequests.ListCommits(ctx, v, v, 1, nil))                                        // @sink(pkg-go-github-issues-vuln)
	use(c.PullRequests.ListComments(ctx, v, v, 1, nil))                                       // @sink(pkg-go-github-issues-vuln)
	use(c.PullRequests.CreateComment(ctx, v, v, 1, github.CreatePullRequestCommentRequest{})) // @sink(pkg-go-github-issues-vuln)
	use(c.PullRequests.ListReviews(ctx, v, v, 1, nil))                                        // @sink(pkg-go-github-issues-vuln)
	use(c.PullRequests.CreateReview(ctx, v, v, 1, nil))                                       // @sink(pkg-go-github-issues-vuln)
	use(c.PullRequests.Merge(ctx, v, v, 1, "x", nil))                                         // @sink(pkg-go-github-issues-vuln)

}

func fixed() { // @fixed
	ctx := context.Background()
	c, _ := github.NewClient()
	mark.SinkCmd(mark.CleanCmd(mark.Src("pkg-go-github-issues-fixed"))) // @sink(none)
	use(c.Issues.Get(ctx, "o", "r", 1))                                 // @sink(none)
	use(c.PullRequests.Get(ctx, "o", "r", 1))                           // @sink(none)
}

func decoy() { // @decoy
	ctx := context.Background()
	c, _ := github.NewClient()
	v := mark.Src("pkg-go-github-issues-decoy")         // @source(pkg-go-github-issues-decoy)
	use(c.PullRequests.Merge(ctx, "o", "r", 1, v, nil)) // @sink(none) @nosink(pkg-go-github-issues-decoy)
	use(c.Issues.Get(ctx, v, "r", 1))                   // @sink(pkg-go-github-issues-decoy)
}
