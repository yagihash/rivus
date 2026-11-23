// Package repos is a cell of the package go_github.
//
// @cell(F/pkg/go-github/repos)
// @shape: a value of a source goes to the owner, the repository, the path or the ref of a call of the service of go-github (the rows of resource_name of the model); the skeleton records it
package repos

import (
	"context"

	"github.com/google/go-github/v93/github"
	"zoo/mark"
)

func use(...any) {}

func init() {
	mark.Case("F/pkg/go-github/repos/vuln", vuln)
	mark.Case("F/pkg/go-github/repos/fixed", fixed)
	mark.Case("F/pkg/go-github/repos/decoy", decoy)
}

func vuln() { // @vuln
	ctx := context.Background()
	c, _ := github.NewClient()
	v := mark.Src("pkg-go-github-repos-vuln")                                                                                      // @source(pkg-go-github-repos-vuln)
	use(c.Repositories.Get(ctx, v, v))                                                                                             // @sink(pkg-go-github-repos-vuln)
	use(c.Repositories.ListCommits(ctx, v, v, nil))                                                                                // @sink(pkg-go-github-repos-vuln)
	use(c.Repositories.GetCommit(ctx, v, v, v, nil))                                                                               // @sink(pkg-go-github-repos-vuln)
	use(c.Repositories.GetCommitRaw(ctx, v, v, v, github.RawOptions{}))                                                            // @sink(pkg-go-github-repos-vuln)
	use(c.Repositories.GetCommitSHA1(ctx, v, v, v, "x"))                                                                           // @sink(pkg-go-github-repos-vuln)
	use(c.Repositories.CompareCommits(ctx, v, v, v, v, nil))                                                                       // @sink(pkg-go-github-repos-vuln)
	use(c.Repositories.GetReadme(ctx, v, v, &github.RepositoryContentGetOptions{Ref: v}))                                          // @sink(pkg-go-github-repos-vuln)
	use(c.Repositories.GetContents(ctx, v, v, v, &github.RepositoryContentGetOptions{Ref: v}))                                     // @sink(pkg-go-github-repos-vuln)
	use(c.Repositories.DownloadContents(ctx, v, v, v, &github.RepositoryContentGetOptions{Ref: v}))                                // @sink(pkg-go-github-repos-vuln)
	use(c.Repositories.DownloadContentsWithMeta(ctx, v, v, v, &github.RepositoryContentGetOptions{Ref: v}))                        // @sink(pkg-go-github-repos-vuln)
	use(c.Repositories.CreateFile(ctx, v, v, v, nil))                                                                              // @sink(pkg-go-github-repos-vuln)
	use(c.Repositories.UpdateFile(ctx, v, v, v, nil))                                                                              // @sink(pkg-go-github-repos-vuln)
	use(c.Repositories.DeleteFile(ctx, v, v, v, nil))                                                                              // @sink(pkg-go-github-repos-vuln)
	use(c.Repositories.GetArchiveLink(ctx, v, v, github.ArchiveFormat("tarball"), &github.RepositoryContentGetOptions{Ref: v}, 1)) // @sink(pkg-go-github-repos-vuln)
	use(c.Repositories.GetBranch(ctx, v, v, v, 1))                                                                                 // @sink(pkg-go-github-repos-vuln)
	use(c.Repositories.GetBranchProtection(ctx, v, v, v))                                                                          // @sink(pkg-go-github-repos-vuln)
	use(c.Repositories.ListReleases(ctx, v, v, nil))                                                                               // @sink(pkg-go-github-repos-vuln)
	use(c.Repositories.GetRelease(ctx, v, v, 1))                                                                                   // @sink(pkg-go-github-repos-vuln)
	use(c.Repositories.GetLatestRelease(ctx, v, v))                                                                                // @sink(pkg-go-github-repos-vuln)
	use(c.Repositories.GetReleaseByTag(ctx, v, v, v))                                                                              // @sink(pkg-go-github-repos-vuln)
	use(c.Repositories.CreateRelease(ctx, v, v, github.CreateReleaseRequest{}))                                                    // @sink(pkg-go-github-repos-vuln)
	use(c.Repositories.DownloadReleaseAsset(ctx, v, v, 1, nil))                                                                    // @sink(pkg-go-github-repos-vuln)
	// one position at a time (the line above puts the source in each of the positions of the row)
	use(c.Repositories.GetContents(ctx, v, "r", "p", nil)) // @sink(pkg-go-github-repos-vuln)
	use(c.Repositories.GetContents(ctx, "o", v, "p", nil)) // @sink(pkg-go-github-repos-vuln)
	use(c.Repositories.GetContents(ctx, "o", "r", v, nil)) // @sink(pkg-go-github-repos-vuln)
}

func fixed() { // @fixed
	ctx := context.Background()
	c, _ := github.NewClient()
	mark.SinkCmd(mark.CleanCmd(mark.Src("pkg-go-github-repos-fixed")))                                            // @sink(none)
	use(c.Repositories.GetContents(ctx, "o", "r", "README.md", &github.RepositoryContentGetOptions{Ref: "main"})) // @sink(none)
	use(c.Repositories.Get(ctx, "o", "r"))                                                                        // @sink(none)
}

func decoy() { // @decoy
	ctx := context.Background()
	c, _ := github.NewClient()
	v := mark.Src("pkg-go-github-repos-decoy") // @source(pkg-go-github-repos-decoy)
	msg := v
	use(c.Repositories.CreateFile(ctx, "o", "r", "p", &github.RepositoryContentFileOptions{Message: &msg})) // @sink(none) @nosink(pkg-go-github-repos-decoy)
	use(c.Repositories.GetRelease(ctx, "o", "r", 1))                                                        // @sink(none) @nosink(pkg-go-github-repos-decoy)
	use(c.Repositories.GetContents(ctx, v, "r", "p", nil))                                                  // @sink(pkg-go-github-repos-decoy)
}
