// Package github is a skeleton of github.com/google/go-github/v93 v93.0.0 for the zoo (docs/DESIGN-package-models.md, 6.8): the
// public signatures of the methods that the built-in model (internal/model/builtin/go-github.yaml) has rows for, and bodies that
// only record what they were given (the sink rows) or return the marker of the case (the source rows), so that the cells run.
// It is not go-github: the analysis does not read these bodies where the model has a row, and the real go-github is never
// fetched or run by the zoo. The functions marked "not in go-github" are the zoo's way to make a client or a value.
package github

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"

	"zoo/mark"
)

// Client is the client of the API. v is the text that the calls of the skeleton return where the API returns text that a user of
// GitHub wrote.
type Client struct {
	Repositories *RepositoriesService
	Issues       *IssuesService
	PullRequests *PullRequestsService
	Git          *GitService
	v            string
}

type service struct{ client *Client }

type RepositoriesService service
type IssuesService service
type PullRequestsService service
type GitService service

type ClientOptionsFunc func(*Client) error
type RequestOption func(*http.Request)

func newClient(v string) *Client {
	c := &Client{v: v}
	c.Repositories = &RepositoriesService{c}
	c.Issues = &IssuesService{c}
	c.PullRequests = &PullRequestsService{c}
	c.Git = &GitService{c}
	return c
}

// NewClient makes a client.
func NewClient(opts ...ClientOptionsFunc) (*Client, error) {
	c := newClient("")
	for _, o := range opts {
		if err := o(c); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// NewClientFor makes a client whose calls return the marker of tag where go-github returns what a user of GitHub wrote. (Not in
// go-github: the real way is a server that answers.)
func NewClientFor(tag string) *Client { return newClient(mark.Src(tag)) }

// WithEnterpriseURLs sets the URLs of a GitHub Enterprise server.
func WithEnterpriseURLs(baseURL, uploadURL string) ClientOptionsFunc {
	mark.SinkAny(baseURL)
	mark.SinkAny(uploadURL)
	return func(*Client) error { return nil }
}

func (c *Client) NewRequest(ctx context.Context, method, urlStr string, body any, opts ...RequestOption) (*http.Request, error) {
	mark.SinkAny(urlStr)
	return nil, nil
}

func (c *Client) NewFormRequest(ctx context.Context, urlStr string, body io.Reader, opts ...RequestOption) (*http.Request, error) {
	mark.SinkAny(urlStr)
	return nil, nil
}

func (c *Client) NewUploadRequest(ctx context.Context, urlStr string, reader io.Reader, size int64, mediaType string, opts ...RequestOption) (*http.Request, error) {
	mark.SinkAny(urlStr)
	return nil, nil
}

var _ = url.URL{}

func (s *RepositoriesService) Get(ctx context.Context, owner, repo string) (*Repository, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	return nil, nil, nil
}

func (s *RepositoriesService) ListCommits(ctx context.Context, owner, repo string, opts *CommitsListOptions) ([]*RepositoryCommit, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	return nil, nil, nil
}

func (s *RepositoriesService) GetCommit(ctx context.Context, owner, repo, sha string, opts *ListOptions) (*RepositoryCommit, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	mark.SinkAny(sha)
	return nil, nil, nil
}

func (s *RepositoriesService) GetCommitRaw(ctx context.Context, owner, repo, sha string, opts RawOptions) (string, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	mark.SinkAny(sha)
	return s.client.v, nil, nil
}

func (s *RepositoriesService) GetCommitSHA1(ctx context.Context, owner, repo, ref, lastSHA string) (string, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	mark.SinkAny(ref)
	return "", nil, nil
}

func (s *RepositoriesService) CompareCommits(ctx context.Context, owner, repo, base, head string, opts *ListOptions) (*CommitsComparison, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	mark.SinkAny(base)
	mark.SinkAny(head)
	return nil, nil, nil
}

func (s *RepositoriesService) GetReadme(ctx context.Context, owner, repo string, opts *RepositoryContentGetOptions) (*RepositoryContent, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	if opts != nil {
		mark.SinkAny(opts.Ref)
	}
	return &RepositoryContent{v: s.client.v, Content: &s.client.v}, nil, nil
}

func (s *RepositoriesService) GetContents(ctx context.Context, owner, repo, path string, opts *RepositoryContentGetOptions) (fileContent *RepositoryContent, directoryContent []*RepositoryContent, resp *Response, err error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	mark.SinkAny(path)
	if opts != nil {
		mark.SinkAny(opts.Ref)
	}
	return &RepositoryContent{v: s.client.v, Content: &s.client.v}, nil, nil, nil
}

func (s *RepositoriesService) DownloadContents(ctx context.Context, owner, repo, filepath string, opts *RepositoryContentGetOptions) (io.ReadCloser, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	mark.SinkAny(filepath)
	if opts != nil {
		mark.SinkAny(opts.Ref)
	}
	return io.NopCloser(strings.NewReader(s.client.v)), nil, nil
}

func (s *RepositoriesService) DownloadContentsWithMeta(ctx context.Context, owner, repo, filepath string, opts *RepositoryContentGetOptions) (io.ReadCloser, *RepositoryContent, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	mark.SinkAny(filepath)
	if opts != nil {
		mark.SinkAny(opts.Ref)
	}
	return io.NopCloser(strings.NewReader(s.client.v)), &RepositoryContent{v: s.client.v, Content: &s.client.v}, nil, nil
}

func (s *RepositoriesService) CreateFile(ctx context.Context, owner, repo, path string, body *RepositoryContentFileOptions) (*RepositoryContentResponse, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	mark.SinkAny(path)
	return nil, nil, nil
}

func (s *RepositoriesService) UpdateFile(ctx context.Context, owner, repo, path string, body *RepositoryContentFileOptions) (*RepositoryContentResponse, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	mark.SinkAny(path)
	return nil, nil, nil
}

func (s *RepositoriesService) DeleteFile(ctx context.Context, owner, repo, path string, opts *RepositoryContentFileOptions) (*RepositoryContentResponse, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	mark.SinkAny(path)
	return nil, nil, nil
}

func (s *RepositoriesService) GetArchiveLink(ctx context.Context, owner, repo string, archiveformat ArchiveFormat, opts *RepositoryContentGetOptions, maxRedirects int) (*url.URL, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	if opts != nil {
		mark.SinkAny(opts.Ref)
	}
	return nil, nil, nil
}

func (s *RepositoriesService) GetBranch(ctx context.Context, owner, repo, branch string, maxRedirects int) (*Branch, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	mark.SinkAny(branch)
	return nil, nil, nil
}

func (s *RepositoriesService) GetBranchProtection(ctx context.Context, owner, repo, branch string) (*Protection, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	mark.SinkAny(branch)
	return nil, nil, nil
}

func (s *RepositoriesService) ListReleases(ctx context.Context, owner, repo string, opts *ListOptions) ([]*RepositoryRelease, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	return nil, nil, nil
}

func (s *RepositoriesService) GetRelease(ctx context.Context, owner, repo string, id int64) (*RepositoryRelease, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	return nil, nil, nil
}

func (s *RepositoriesService) GetLatestRelease(ctx context.Context, owner, repo string) (*RepositoryRelease, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	return &RepositoryRelease{v: s.client.v, Body: &s.client.v}, nil, nil
}

func (s *RepositoriesService) GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*RepositoryRelease, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	mark.SinkAny(tag)
	return nil, nil, nil
}

func (s *RepositoriesService) CreateRelease(ctx context.Context, owner, repo string, body CreateReleaseRequest) (*RepositoryRelease, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	return nil, nil, nil
}

func (s *RepositoriesService) DownloadReleaseAsset(ctx context.Context, owner, repo string, id int64, followRedirectsClient *http.Client) (rc io.ReadCloser, redirectURL string, err error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	return
}

func (s *IssuesService) Get(ctx context.Context, owner, repo string, number int) (*Issue, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	return &Issue{v: s.client.v, Title: &s.client.v, Body: &s.client.v}, nil, nil
}

func (s *IssuesService) ListByRepo(ctx context.Context, owner, repo string, opts *IssueListByRepoOptions) ([]*Issue, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	return nil, nil, nil
}

func (s *IssuesService) Create(ctx context.Context, owner, repo string, body CreateIssueRequest) (*Issue, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	return nil, nil, nil
}

func (s *IssuesService) ListComments(ctx context.Context, owner, repo string, number int, opts *IssueListCommentsOptions) ([]*IssueComment, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	return []*IssueComment{{v: s.client.v, Body: &s.client.v}}, nil, nil
}

func (s *IssuesService) GetComment(ctx context.Context, owner, repo string, commentID int64) (*IssueComment, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	return &IssueComment{v: s.client.v, Body: &s.client.v}, nil, nil
}

func (s *IssuesService) CreateComment(ctx context.Context, owner, repo string, number int, body IssueCommentRequest) (*IssueComment, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	return nil, nil, nil
}

func (s *PullRequestsService) Get(ctx context.Context, owner, repo string, number int) (*PullRequest, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	return &PullRequest{v: s.client.v, Title: &s.client.v, Body: &s.client.v}, nil, nil
}

func (s *PullRequestsService) GetRaw(ctx context.Context, owner, repo string, number int, opts RawOptions) (string, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	return s.client.v, nil, nil
}

func (s *PullRequestsService) Create(ctx context.Context, owner, repo string, body CreatePullRequest) (*PullRequest, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	return nil, nil, nil
}

func (s *PullRequestsService) Edit(ctx context.Context, owner, repo string, number int, pull *PullRequest) (*PullRequest, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	return nil, nil, nil
}

func (s *PullRequestsService) ListFiles(ctx context.Context, owner, repo string, number int, opts *ListOptions) ([]*CommitFile, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	return nil, nil, nil
}

func (s *PullRequestsService) ListCommits(ctx context.Context, owner, repo string, number int, opts *ListOptions) ([]*RepositoryCommit, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	return nil, nil, nil
}

func (s *PullRequestsService) ListComments(ctx context.Context, owner, repo string, number int, opts *PullRequestListCommentsOptions) ([]*PullRequestComment, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	return []*PullRequestComment{{v: s.client.v, Body: &s.client.v}}, nil, nil
}

func (s *PullRequestsService) CreateComment(ctx context.Context, owner, repo string, number int, body CreatePullRequestCommentRequest) (*PullRequestComment, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	return nil, nil, nil
}

func (s *PullRequestsService) ListReviews(ctx context.Context, owner, repo string, number int, opts *ListOptions) ([]*PullRequestReview, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	return []*PullRequestReview{{v: s.client.v, Body: &s.client.v}}, nil, nil
}

func (s *PullRequestsService) CreateReview(ctx context.Context, owner, repo string, number int, body *PullRequestReviewRequest) (*PullRequestReview, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	return nil, nil, nil
}

func (s *PullRequestsService) Merge(ctx context.Context, owner, repo string, number int, commitMessage string, options *PullRequestOptions) (*PullRequestMergeResult, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	return nil, nil, nil
}

func (s *GitService) GetCommit(ctx context.Context, owner, repo, sha string) (*Commit, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	mark.SinkAny(sha)
	return &Commit{v: s.client.v, Message: &s.client.v}, nil, nil
}

func (s *GitService) GetBlob(ctx context.Context, owner, repo, sha string) (*Blob, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	mark.SinkAny(sha)
	return &Blob{v: s.client.v, Content: &s.client.v}, nil, nil
}

func (s *GitService) GetBlobRaw(ctx context.Context, owner, repo, sha string) ([]byte, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	mark.SinkAny(sha)
	return []byte(s.client.v), nil, nil
}

func (s *GitService) GetTree(ctx context.Context, owner, repo, sha string, recursive bool) (*Tree, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	mark.SinkAny(sha)
	return nil, nil, nil
}

func (s *GitService) GetRef(ctx context.Context, owner, repo, ref string) (*Reference, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	mark.SinkAny(ref)
	return nil, nil, nil
}

func (s *GitService) ListMatchingRefs(ctx context.Context, owner, repo, ref string) ([]*Reference, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	mark.SinkAny(ref)
	return nil, nil, nil
}

func (s *GitService) UpdateRef(ctx context.Context, owner, repo, ref string, body UpdateRef) (*Reference, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	mark.SinkAny(ref)
	return nil, nil, nil
}

func (s *GitService) DeleteRef(ctx context.Context, owner, repo, ref string) (*Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	mark.SinkAny(ref)
	return nil, nil
}

func (s *GitService) CreateRef(ctx context.Context, owner, repo string, body CreateRef) (*Reference, *Response, error) {
	mark.SinkAny(owner)
	mark.SinkAny(repo)
	return nil, nil, nil
}

// The types of the signatures. Most are empty here: the cells do not look inside them.

type ArchiveFormat string

type Blob struct {
	v       string
	Content *string
}

type Branch struct{}

type Commit struct {
	v       string
	Message *string
}

type CommitFile struct{}

type CommitsComparison struct{}

type CommitsListOptions struct {
	SHA  string
	Path string
	ListOptions
}

type CreateIssueRequest struct{}

type CreatePullRequest struct{}

type CreatePullRequestCommentRequest struct{}

type CreateRef struct{ Ref, SHA string }

type CreateReleaseRequest struct{}

type Issue struct {
	v     string
	Title *string
	Body  *string
}

type IssueComment struct {
	v    string
	Body *string
}

type IssueCommentRequest struct{}

type IssueListByRepoOptions struct{}

type IssueListCommentsOptions struct{}

type ListOptions struct{ Page, PerPage int }

type Protection struct{}

type PullRequest struct {
	v     string
	Title *string
	Body  *string
}

type PullRequestComment struct {
	v    string
	Body *string
}

type PullRequestListCommentsOptions struct{}

type PullRequestMergeResult struct{}

type PullRequestOptions struct{}

type PullRequestReview struct {
	v    string
	Body *string
}

type PullRequestReviewRequest struct{}

type RawOptions struct{ Type RawType }

type RawType int

type Reference struct{}

type Repository struct{}

type RepositoryCommit struct{}

type RepositoryContent struct {
	v       string
	Content *string
}

type RepositoryContentFileOptions struct {
	Message *string
	Content []byte
}

type RepositoryContentGetOptions struct {
	Ref string `url:"ref,omitempty"`
}

type RepositoryContentResponse struct{}

type RepositoryRelease struct {
	v    string
	Body *string
}

type Response struct{ *http.Response }

type Tree struct{}

type UpdateRef struct{ SHA string }

func (i *Issue) GetTitle() string { return i.v }

func (i *Issue) GetBody() string { return i.v }

func (i *IssueComment) GetBody() string { return i.v }

func (p *PullRequest) GetTitle() string { return p.v }

func (p *PullRequest) GetBody() string { return p.v }

func (p *PullRequestComment) GetBody() string { return p.v }

func (p *PullRequestReview) GetBody() string { return p.v }

func (c *Commit) GetMessage() string { return c.v }

func (r *RepositoryRelease) GetBody() string { return r.v }

func (r *RepositoryContent) GetContent() (string, error) { return r.v, nil }

func (b *Blob) GetContent() string { return b.v }

// What the service decides, not a user: not sources (ADR-108).
func (i *Issue) GetHTMLURL() string         { return "https://github.com/example/example/issues/1" }
func (i *Issue) GetNumber() int             { return 1 }
func (c *Commit) GetSHA() string            { return "0000000000000000000000000000000000000000" }
func (r *RepositoryContent) GetSHA() string { return "0000000000000000000000000000000000000000" }
