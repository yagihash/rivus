// Package models has the rows.
package models

// User is a row.
type User struct {
	ID    int64
	Name  string
	Email string
	Bio   string
}

// Repo is a row of a repository.
type Repo struct {
	ID    int64
	Owner string
	Name  string
}
