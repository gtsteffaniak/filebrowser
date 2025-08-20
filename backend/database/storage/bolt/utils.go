package bolt

import (
	storm "github.com/asdine/storm/v3"

	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
)

func get(db *storm.DB, name string, to interface{}) error {
	err := db.Get("config", name, to)
	if err == storm.ErrNotFound {
		return errors.ErrNotExist
	}

	return err
}

func Save(db *storm.DB, name string, from interface{}) error {
	return db.Set("config", name, from)
}

func SaveAccessRules(db *storm.DB, name string, from interface{}) error {
	return db.Set("access_rules", name, from)
}

func GetAccessRules(db *storm.DB, name string, to interface{}) error {
	err := db.Get("access_rules", name, to)
	if err == storm.ErrNotFound {
		return errors.ErrNotExist
	}

	return err
}
