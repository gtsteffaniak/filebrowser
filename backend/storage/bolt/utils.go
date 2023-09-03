package bolt

import (
	"log"

	"github.com/asdine/storm/v3"

	"github.com/gtsteffaniak/filebrowser/errors"
)

func get(db *storm.DB, name string, to interface{}) error {
	log.Printf("name, %v , to %#v", name, to)
	err := db.Get("config", name, to)
	if err == storm.ErrNotFound {
		return errors.ErrNotExist
	}

	return err
}

func save(db *storm.DB, name string, from interface{}) error {
	log.Printf("name, %v , from %#v", name, from)
	return db.Set("config", name, from)
}
