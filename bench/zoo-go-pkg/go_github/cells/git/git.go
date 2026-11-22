// Package git is a cell of the package go_github.
//
// @cell(F/pkg/go-github/git)
// @shape: a value of a source goes to the owner, the repository, the path or the ref of a call of the service of go-github (the rows of resource_name of the model); the skeleton records it
package git

import (
	"context"

	"github.com/google/go-github/v93/github"
	"zoo/mark"
)

func use(...any) {}

func init() {
	mark.Case("F/pkg/go-github/git/vuln", vuln)
	mark.Case("F/pkg/go-github/git/fixed", fixed)
	mark.Case("F/pkg/go-github/git/decoy", decoy)
}

func vuln() { // @vuln
	ctx := context.Background()
	c, _ := github.NewClient()
	v := mark.Src("pkg-go-github-git-vuln")                               // @source(pkg-go-github-git-vuln)
	use(c.Git.GetCommit(ctx, v, v, v))                                    // @sink(pkg-go-github-git-vuln)
	use(c.Git.GetBlob(ctx, v, v, v))                                      // @sink(pkg-go-github-git-vuln)
	use(c.Git.GetBlobRaw(ctx, v, v, v))                                   // @sink(pkg-go-github-git-vuln)
	use(c.Git.GetTree(ctx, v, v, v, false))                               // @sink(pkg-go-github-git-vuln)
	use(c.Git.GetRef(ctx, v, v, v))                                       // @sink(pkg-go-github-git-vuln)
	use(c.Git.ListMatchingRefs(ctx, v, v, v))                             // @sink(pkg-go-github-git-vuln)
	use(c.Git.UpdateRef(ctx, v, v, v, github.UpdateRef{}))                // @sink(pkg-go-github-git-vuln)
	use(c.Git.DeleteRef(ctx, v, v, v))                                    // @sink(pkg-go-github-git-vuln)
	use(c.Git.CreateRef(ctx, v, v, github.CreateRef{}))                   // @sink(pkg-go-github-git-vuln)
	use(c.NewRequest(ctx, "GET", v, nil))                                 // @sink(pkg-go-github-git-vuln)
	use(c.NewFormRequest(ctx, v, nil))                                    // @sink(pkg-go-github-git-vuln)
	use(c.NewUploadRequest(ctx, v, nil, 0, "x"))                          // @sink(pkg-go-github-git-vuln)
	use(github.WithEnterpriseURLs(v, "https://uploads.example.invalid/")) // @sink(pkg-go-github-git-vuln)
	use(github.WithEnterpriseURLs("https://git.example.invalid/", v))     // @sink(pkg-go-github-git-vuln)
}

func fixed() { // @fixed
	ctx := context.Background()
	c, _ := github.NewClient()
	mark.SinkCmd(mark.CleanCmd(mark.Src("pkg-go-github-git-fixed"))) // @sink(none)
	use(c.Git.GetRef(ctx, "o", "r", "heads/main"))                   // @sink(none)
	use(c.NewRequest(ctx, "GET", "user", nil))                       // @sink(none)
}

func decoy() { // @decoy
	ctx := context.Background()
	c, _ := github.NewClient()
	v := mark.Src("pkg-go-github-git-decoy")        // @source(pkg-go-github-git-decoy)
	use(c.NewRequest(ctx, "POST", "user/repos", v)) // @sink(none) @nosink(pkg-go-github-git-decoy)
	use(c.Git.GetRef(ctx, v, "r", "heads/main"))    // @sink(pkg-go-github-git-decoy)
}
