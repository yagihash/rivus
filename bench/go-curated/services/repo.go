package services

import (
	"curated/models"
	"curated/orm"
)

// User is the row type the callers use; it is an alias of the model (as Gitea's
// services use "type User = user_model.User").
type User = models.User

// CreateUser stores a user: the fields are put in a statement.
func CreateUser(db *orm.DB, u *User) error {
	return db.Exec("INSERT INTO users (name, email) VALUES ('" + u.Name + "', '" + u.Email + "')") // @sink(alias_user)
}

// ProfileLine builds a line from the stored bio; the stored data is not a source.
func ProfileLine(db *orm.DB, id int64) error {
	var users []User
	if err := db.Find(&users); err != nil {
		return err
	}
	for _, u := range users {
		if err := db.Exec("UPDATE users SET bio = '" + u.Bio + "'"); err != nil { // @sink(none)
			return err
		}
	}
	return nil
}
