package bolt

import (
	storm "github.com/asdine/storm/v3"
	"github.com/gtsteffaniak/filebrowser/backend/auth"
	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
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
	return auther, get(s.db, "auther", auther)
}

func (s authBackend) Save(a auth.Auther) error {
	return Save(s.db, "auther", a)
}
