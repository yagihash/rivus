package services

import (
	"curated/models"
	"curated/orm"
	"strconv"
)

// SQLUsers is the store of users.
type SQLUsers struct{ DB *orm.DB }

// ByName builds the statement from the name.
func (u *SQLUsers) ByName(name string) error {
	return u.DB.Where("name = '" + name + "'").Find(&[]models.User{}) // @sink(di_byname)
}

// Rename uses the id as a number.
func (u *SQLUsers) Rename(id int64, name string) error {
	return u.DB.Exec("UPDATE users SET name = ? WHERE id = "+strconv.FormatInt(id, 10), name) // @sink(none)
}
