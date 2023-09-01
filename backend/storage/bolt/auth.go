package bolt

import (
	"fmt"

	"github.com/asdine/storm/v3"
	"github.com/gtsteffaniak/filebrowser/auth"
	"github.com/gtsteffaniak/filebrowser/errors"
)

type authBackend struct {
	db *storm.DB
}

func (s authBackend) Get(t string) (auth.Auther, error) {
	var auther auth.Auther
	switch t {
	case "password":
		auther = &auth.JSONAuth{}
	case "proxy":
		auther = &auth.ProxyAuth{}
	case "hook":
		auther = &auth.HookAuth{}
	case "noauth":
		auther = &auth.NoAuth{}
	default:
		return nil, errors.ErrInvalidAuthMethod
	}
	fmt.Println("auth.go", t)

	return auther, get(s.db, "auther", auther)
}

func (s authBackend) Save(a auth.Auther) error {
	return save(s.db, "auther", a)
}
