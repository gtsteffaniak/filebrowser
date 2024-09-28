package storage

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/asdine/storm/v3"
	"github.com/gtsteffaniak/filebrowser/auth"
	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/share"
	"github.com/gtsteffaniak/filebrowser/users"
	"github.com/gtsteffaniak/filebrowser/utils"
	"golang.org/x/mod/sumdb/storage"
)

// Storage is a storage powered by a Backend which makes the necessary
// verifications when fetching and saving data to ensure consistency.
type Storage struct {
	Users    users.Store
	Share    *share.Storage
	Auth     *auth.Storage
	Settings *settings.Storage
}

func InitializeDb(path string) (*Storage, error) {
	db, err := storm.Open(path)
	utils.CheckErr(fmt.Sprintf("storm.Open path %v", path), err)
	exists, err := dbExists(path)

	if !exists {
		quickSetup(db)
	}

	if err != nil {
		panic(err)
	} else if exists && cfg.noDB {
		log.Fatal(path + " already exists")
	} else if !exists && !cfg.noDB && !cfg.allowNoDB {
		log.Fatal(path + " does not exist. Please run 'filebrowser config init' first.")
	}

	data.hadDB = exists
	db, err := storm.Open(path)
	utils.CheckErr(fmt.Sprintf("storm.Open path %v", path), err)

	defer db.Close()

	userStore := users.NewStorage(bolt.usersBackend{db: db})
	shareStore := share.NewStorage(bolt.shareBackend{db: db})
	settingsStore := settings.NewStorage(bolt.settingsBackend{db: db})
	authStore := auth.NewStorage(bolt.authBackend{db: db}, userStore)

	err := save(db, "version", 2) //nolint:gomnd
	if err != nil {
		return nil, err
	}

	return &Storage{
		Auth:     authStore,
		Users:    userStore,
		Share:    shareStore,
		Settings: settingsStore,
	}, nil
}

func dbExists(path string) (bool, error) {
	stat, err := os.Stat(path)
	if err == nil {
		return stat.Size() != 0, nil
	}

	if os.IsNotExist(err) {
		d := filepath.Dir(path)
		_, err = os.Stat(d)
		if os.IsNotExist(err) {
			if err := os.MkdirAll(d, 0700); err != nil { //nolint:govet,gomnd
				return false, err
			}
			return false, nil
		}
	}

	return false, err
}

func quickSetup(store *storage.Storage) {
	settings.Config.Auth.Key = generateKey()
	if settings.Config.Auth.Method == "noauth" {
		err := d.store.Auth.Save(&auth.NoAuth{})
		utils.CheckErr("d.store.Auth.Save", err)
	} else {
		settings.Config.Auth.Method = "password"
		err := d.store.Auth.Save(&auth.JSONAuth{})
		utils.CheckErr("d.store.Auth.Save", err)
	}
	err := d.store.Settings.Save(&settings.Config)
	utils.CheckErr("d.store.Settings.Save", err)
	err = d.store.Settings.SaveServer(&settings.Config.Server)
	utils.CheckErr("d.store.Settings.SaveServer", err)
	user := users.ApplyDefaults(users.User{})
	user.Username = settings.Config.Auth.AdminUsername
	user.Password = settings.Config.Auth.AdminPassword
	user.Perm.Admin = true
	user.Scope = "./"
	user.DarkMode = true
	user.ViewMode = "normal"
	user.LockPassword = false
	user.Perm = settings.Permissions{
		Create:   true,
		Rename:   true,
		Modify:   true,
		Delete:   true,
		Share:    true,
		Download: true,
		Admin:    true,
	}
	err = d.store.Users.Save(&user)
	utils.CheckErr("d.store.Users.Save", err)
}
