// Package fake is a synthetic package for the tests of the expansion of `adopt_rules:` (expand_test.go). Its import path is example.com/fake/v7.
package fake

import "context"

type OrgService struct{}
type IssueService struct{}
type Client struct{}

type Slug struct{ Owner, Name string }
type Opt struct{ Branch string }

// Doc has the text that a user wrote (Body, Title) and a value that the service decides (ID).
type Doc struct {
	Body  string
	Title string
	ID    int
}

func (s *OrgService) Get(ctx context.Context, owner, repo string) error { return nil }
func (s *OrgService) List(ctx context.Context, owner string) error      { return nil }

// Edit has a hand row: the hand row wins.
func (s *OrgService) Edit(ctx context.Context, owner, repo string, o *Opt) error { return nil }

// Old is rejected: it is not generated.
func (s *OrgService) Old(owner string) {}

// Number has no word of the table in an argument.
func (s *OrgService) Number(ctx context.Context, n int) error { return nil }

// Find and Move take a Slug (sink.arg-type).
func (s *OrgService) Find(ctx context.Context, slug Slug) error              { return nil }
func (s *IssueService) Move(slug *Slug, to *Slug, note string)               {}
func (s *IssueService) Create(ctx context.Context, owner, repo string) error { return nil }

// Query has a sql word, Fetch an url word: other rules, other words.
func (s *OrgService) Query(ctx context.Context, query string) error      { return nil }
func (s *OrgService) Fetch(ctx context.Context, owner, url string) error { return nil }

// Body is a source by its result.
func (s *OrgService) Body() string { return "" }

// Client is not a Service.
func (c *Client) Get(owner string) error { return nil }
func (c *Client) Find(slug Slug) error   { return nil }
func Top(owner string)                   {}
