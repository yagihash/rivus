// Package services has the business logic behind interfaces, as Gitea's services
// and gin-vue-admin's service layer do.
package services

import (
	"curated/orm"
	"curated/valid"
)

// Searcher finds things by a text.
type Searcher interface {
	Search(q string) error
}

// rawSearcher puts the text in the statement.
type rawSearcher struct{ db *orm.DB }

func (s *rawSearcher) Search(q string) error {
	return s.db.Exec("SELECT * FROM t WHERE name = '" + q + "'") // @sink(iface_search)
}

// quotedSearcher quotes it first.
type quotedSearcher struct{ db *orm.DB }

func (s *quotedSearcher) Search(q string) error {
	return s.db.Exec("SELECT * FROM t WHERE name = " + valid.Quote(q)) // @sink(none)
}

// NewSearchers returns the implementations (the caller picks one at run time).
func NewSearchers(db *orm.DB) []Searcher {
	return []Searcher{&rawSearcher{db}, &quotedSearcher{db}}
}

// Service is a service with its repository given when it was made (dependency injection).
type Service struct {
	search Searcher
	users  UserStore
}

// UserStore is the repository the service needs.
type UserStore interface {
	ByName(name string) error
	Rename(id int64, name string) error
}

// NewService wires a service.
func NewService(s Searcher, u UserStore) *Service { return &Service{search: s, users: u} }

// Find asks the searcher.
func (s *Service) Find(q string) error { return s.search.Search(q) }

// Lookup asks the store.
func (s *Service) Lookup(name string) error { return s.users.ByName(name) }

// Rename asks the store.
func (s *Service) Rename(id int64, name string) error { return s.users.Rename(id, name) }
