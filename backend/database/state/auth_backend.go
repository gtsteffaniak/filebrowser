package state

import (
	"github.com/gtsteffaniak/filebrowser/backend/auth"
	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
)

// authBackend implements auth.StorageBackend using in-memory defaults from settings
type authBackend struct{}

func (a authBackend) Get(t string) (auth.Auther, error) {
	switch t {
	case "password":
		authCfg := settings.Config.Auth.Methods.PasswordAuth
		var recaptcha *auth.ReCaptcha
		if authCfg.Recaptcha.Host != "" && authCfg.Recaptcha.Secret != "" {
			recaptcha = &auth.ReCaptcha{
				Host:   authCfg.Recaptcha.Host,
				Key:    authCfg.Recaptcha.Key,
				Secret: authCfg.Recaptcha.Secret,
			}
		}
		return &auth.JSONAuth{ReCaptcha: recaptcha}, nil
	case "proxy":
		return &auth.ProxyAuth{}, nil
	case "noauth":
		return &auth.NoAuth{}, nil
	default:
		return nil, errors.ErrInvalidAuthMethod
	}
}

func (a authBackend) Save(auther auth.Auther) error {
	// No-op: auth config comes from settings, not persisted in state
	return nil
}
