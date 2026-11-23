// Package sources is a cell of the package go_github.
//
// @cell(F/pkg/go-github/sources)
// @shape: what a user of GitHub wrote (an issue, a comment, a commit message, the content of a file) comes out of a call of go-github, and reaches a sink; what the service decides (a URL it builds) does not (ADR-108)
package sources

import (
	"context"
	"io"

	"github.com/google/go-github/v93/github"
	"zoo/mark"
)

func init() {
	mark.Case("F/pkg/go-github/sources/vuln", vuln)
	mark.Case("F/pkg/go-github/sources/fixed", fixed)
	mark.Case("F/pkg/go-github/sources/decoy", decoy)
}

func vuln() { // @vuln
	ctx := context.Background()
	{
		c := github.NewClientFor("pkg-go-github-sources-vuln-title-issue")
		is, _, _ := c.Issues.Get(ctx, "o", "r", 1)
		x := is.GetTitle() // @source(pkg-go-github-sources-vuln-title-issue)
		mark.SinkCmd(x)    // @sink(pkg-go-github-sources-vuln-title-issue)
	}
	{
		c := github.NewClientFor("pkg-go-github-sources-vuln-body-issue")
		is, _, _ := c.Issues.Get(ctx, "o", "r", 1)
		x := is.GetBody() // @source(pkg-go-github-sources-vuln-body-issue)
		mark.SinkCmd(x)   // @sink(pkg-go-github-sources-vuln-body-issue)
	}
	{
		c := github.NewClientFor("pkg-go-github-sources-vuln-body-issue-comment")
		ics, _, _ := c.Issues.ListComments(ctx, "o", "r", 1, nil)
		x := ics[0].GetBody() // @source(pkg-go-github-sources-vuln-body-issue-comment)
		mark.SinkCmd(x)       // @sink(pkg-go-github-sources-vuln-body-issue-comment)
	}
	{
		c := github.NewClientFor("pkg-go-github-sources-vuln-title-pull")
		pr, _, _ := c.PullRequests.Get(ctx, "o", "r", 1)
		x := pr.GetTitle() // @source(pkg-go-github-sources-vuln-title-pull)
		mark.SinkCmd(x)    // @sink(pkg-go-github-sources-vuln-title-pull)
	}
	{
		c := github.NewClientFor("pkg-go-github-sources-vuln-body-pull")
		pr, _, _ := c.PullRequests.Get(ctx, "o", "r", 1)
		x := pr.GetBody() // @source(pkg-go-github-sources-vuln-body-pull)
		mark.SinkCmd(x)   // @sink(pkg-go-github-sources-vuln-body-pull)
	}
	{
		c := github.NewClientFor("pkg-go-github-sources-vuln-body-pull-comment")
		pcs, _, _ := c.PullRequests.ListComments(ctx, "o", "r", 1, nil)
		x := pcs[0].GetBody() // @source(pkg-go-github-sources-vuln-body-pull-comment)
		mark.SinkCmd(x)       // @sink(pkg-go-github-sources-vuln-body-pull-comment)
	}
	{
		c := github.NewClientFor("pkg-go-github-sources-vuln-body-review")
		rvs, _, _ := c.PullRequests.ListReviews(ctx, "o", "r", 1, nil)
		x := rvs[0].GetBody() // @source(pkg-go-github-sources-vuln-body-review)
		mark.SinkCmd(x)       // @sink(pkg-go-github-sources-vuln-body-review)
	}
	{
		c := github.NewClientFor("pkg-go-github-sources-vuln-message-commit")
		gc, _, _ := c.Git.GetCommit(ctx, "o", "r", "sha")
		x := gc.GetMessage() // @source(pkg-go-github-sources-vuln-message-commit)
		mark.SinkCmd(x)      // @sink(pkg-go-github-sources-vuln-message-commit)
	}
	{
		c := github.NewClientFor("pkg-go-github-sources-vuln-body-release")
		rel, _, _ := c.Repositories.GetLatestRelease(ctx, "o", "r")
		x := rel.GetBody() // @source(pkg-go-github-sources-vuln-body-release)
		mark.SinkCmd(x)    // @sink(pkg-go-github-sources-vuln-body-release)
	}
	{
		c := github.NewClientFor("pkg-go-github-sources-vuln-content-file")
		fc, _, _, _ := c.Repositories.GetContents(ctx, "o", "r", "p", nil)
		x, _ := fc.GetContent() // @source(pkg-go-github-sources-vuln-content-file)
		mark.SinkCmd(x)         // @sink(pkg-go-github-sources-vuln-content-file)
	}
	{
		c := github.NewClientFor("pkg-go-github-sources-vuln-content-blob")
		blob, _, _ := c.Git.GetBlob(ctx, "o", "r", "sha")
		x := blob.GetContent() // @source(pkg-go-github-sources-vuln-content-blob)
		mark.SinkCmd(x)        // @sink(pkg-go-github-sources-vuln-content-blob)
	}
	{
		c := github.NewClientFor("pkg-go-github-sources-vuln-raw-commit")
		x, _, _ := c.Repositories.GetCommitRaw(ctx, "o", "r", "sha", github.RawOptions{}) // @source(pkg-go-github-sources-vuln-raw-commit)
		mark.SinkCmd(x)                                                                   // @sink(pkg-go-github-sources-vuln-raw-commit)
	}
	{
		c := github.NewClientFor("pkg-go-github-sources-vuln-raw-pull")
		x, _, _ := c.PullRequests.GetRaw(ctx, "o", "r", 1, github.RawOptions{}) // @source(pkg-go-github-sources-vuln-raw-pull)
		mark.SinkCmd(x)                                                         // @sink(pkg-go-github-sources-vuln-raw-pull)
	}
	{
		c := github.NewClientFor("pkg-go-github-sources-vuln-raw-blob")
		x, _, _ := c.Git.GetBlobRaw(ctx, "o", "r", "sha") // @source(pkg-go-github-sources-vuln-raw-blob)
		mark.SinkCmdBytes(x)                              // @sink(pkg-go-github-sources-vuln-raw-blob)
	}
	{
		c := github.NewClientFor("pkg-go-github-sources-vuln-download")
		rc, _, _ := c.Repositories.DownloadContents(ctx, "o", "r", "p", nil) // @source(pkg-go-github-sources-vuln-download)
		dl, _ := io.ReadAll(rc)
		mark.SinkCmdBytes(dl) // @sink(pkg-go-github-sources-vuln-download)
	}
	{
		c := github.NewClientFor("pkg-go-github-sources-vuln-download-meta")
		rc, _, _, _ := c.Repositories.DownloadContentsWithMeta(ctx, "o", "r", "p", nil) // @source(pkg-go-github-sources-vuln-download-meta)
		dl, _ := io.ReadAll(rc)
		mark.SinkCmdBytes(dl) // @sink(pkg-go-github-sources-vuln-download-meta)
	}
	{
		c := github.NewClientFor("pkg-go-github-sources-vuln-field-title-issue")
		is, _, _ := c.Issues.Get(ctx, "o", "r", 1)
		x := *is.Title  // @source(pkg-go-github-sources-vuln-field-title-issue)
		mark.SinkCmd(x) // @sink(pkg-go-github-sources-vuln-field-title-issue)
	}
	{
		c := github.NewClientFor("pkg-go-github-sources-vuln-field-body-issue")
		is, _, _ := c.Issues.Get(ctx, "o", "r", 1)
		x := *is.Body   // @source(pkg-go-github-sources-vuln-field-body-issue)
		mark.SinkCmd(x) // @sink(pkg-go-github-sources-vuln-field-body-issue)
	}
	{
		c := github.NewClientFor("pkg-go-github-sources-vuln-field-body-issue-comment")
		ics, _, _ := c.Issues.ListComments(ctx, "o", "r", 1, nil)
		x := *ics[0].Body // @source(pkg-go-github-sources-vuln-field-body-issue-comment)
		mark.SinkCmd(x)   // @sink(pkg-go-github-sources-vuln-field-body-issue-comment)
	}
	{
		c := github.NewClientFor("pkg-go-github-sources-vuln-field-title-pull")
		pr, _, _ := c.PullRequests.Get(ctx, "o", "r", 1)
		x := *pr.Title  // @source(pkg-go-github-sources-vuln-field-title-pull)
		mark.SinkCmd(x) // @sink(pkg-go-github-sources-vuln-field-title-pull)
	}
	{
		c := github.NewClientFor("pkg-go-github-sources-vuln-field-body-pull")
		pr, _, _ := c.PullRequests.Get(ctx, "o", "r", 1)
		x := *pr.Body   // @source(pkg-go-github-sources-vuln-field-body-pull)
		mark.SinkCmd(x) // @sink(pkg-go-github-sources-vuln-field-body-pull)
	}
	{
		c := github.NewClientFor("pkg-go-github-sources-vuln-field-body-pull-comment")
		pcs, _, _ := c.PullRequests.ListComments(ctx, "o", "r", 1, nil)
		x := *pcs[0].Body // @source(pkg-go-github-sources-vuln-field-body-pull-comment)
		mark.SinkCmd(x)   // @sink(pkg-go-github-sources-vuln-field-body-pull-comment)
	}
	{
		c := github.NewClientFor("pkg-go-github-sources-vuln-field-body-review")
		rvs, _, _ := c.PullRequests.ListReviews(ctx, "o", "r", 1, nil)
		x := *rvs[0].Body // @source(pkg-go-github-sources-vuln-field-body-review)
		mark.SinkCmd(x)   // @sink(pkg-go-github-sources-vuln-field-body-review)
	}
	{
		c := github.NewClientFor("pkg-go-github-sources-vuln-field-message-commit")
		gc, _, _ := c.Git.GetCommit(ctx, "o", "r", "sha")
		x := *gc.Message // @source(pkg-go-github-sources-vuln-field-message-commit)
		mark.SinkCmd(x)  // @sink(pkg-go-github-sources-vuln-field-message-commit)
	}
	{
		c := github.NewClientFor("pkg-go-github-sources-vuln-field-body-release")
		rel, _, _ := c.Repositories.GetLatestRelease(ctx, "o", "r")
		x := *rel.Body  // @source(pkg-go-github-sources-vuln-field-body-release)
		mark.SinkCmd(x) // @sink(pkg-go-github-sources-vuln-field-body-release)
	}
	{
		c := github.NewClientFor("pkg-go-github-sources-vuln-field-content-file")
		fc, _, _, _ := c.Repositories.GetContents(ctx, "o", "r", "p", nil)
		x := *fc.Content // @source(pkg-go-github-sources-vuln-field-content-file)
		mark.SinkCmd(x)  // @sink(pkg-go-github-sources-vuln-field-content-file)
	}
	{
		c := github.NewClientFor("pkg-go-github-sources-vuln-field-content-blob")
		blob, _, _ := c.Git.GetBlob(ctx, "o", "r", "sha")
		x := blob.Content // @source(pkg-go-github-sources-vuln-field-content-blob)
		mark.SinkCmd(x)   // @sink(pkg-go-github-sources-vuln-field-content-blob)
	}
}

func fixed() { // @fixed
	ctx := context.Background()
	c := github.NewClientFor("pkg-go-github-sources-fixed")
	is, _, _ := c.Issues.Get(ctx, "o", "r", 1)
	x := is.GetBody()
	mark.SinkCmd(mark.CleanCmd(x)) // @sink(none)
}

func decoy() { // @decoy
	ctx := context.Background()
	c := github.NewClientFor("pkg-go-github-sources-decoy")
	is, _, _ := c.Issues.Get(ctx, "o", "r", 1)
	u := is.GetHTMLURL()
	mark.SinkCmd(u)   // @sink(none) @nosink(pkg-go-github-sources-decoy)
	b := is.GetBody() // @source(pkg-go-github-sources-decoy)
	mark.SinkCmd(b)   // @sink(pkg-go-github-sources-decoy)
}
