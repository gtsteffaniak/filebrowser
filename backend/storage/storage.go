package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/asdine/storm/v3"
	"github.com/gtsteffaniak/filebrowser/auth"
	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/share"
	"github.com/gtsteffaniak/filebrowser/storage/bolt"
	"github.com/gtsteffaniak/filebrowser/users"
	"github.com/gtsteffaniak/filebrowser/utils"
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
	exists, err := dbExists(path)
	if err != nil {
		panic(err)
	}
	db, err := storm.Open(path)

	utils.CheckErr(fmt.Sprintf("storm.Open path %v", path), err)
	authStore, userStore, shareStore, settingsStore, err := bolt.NewStorage(db)
	if err != nil {
		return nil, err
	}

	err = bolt.Save(db, "version", 2) //nolint:gomnd
	if err != nil {
		return nil, err
	}
	store := &Storage{
		Auth:     authStore,
		Users:    userStore,
		Share:    shareStore,
		Settings: settingsStore,
	}
	if !exists {
		quickSetup(store)
	}
	return store, err
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

func quickSetup(store *Storage) {
	settings.Config.Auth.Key = utils.GenerateKey()
	if settings.Config.Auth.Method == "noauth" {
		err := store.Auth.Save(&auth.NoAuth{})
		utils.CheckErr("store.Auth.Save", err)
	} else {
		settings.Config.Auth.Method = "password"
		err := store.Auth.Save(&auth.JSONAuth{})
		utils.CheckErr("store.Auth.Save", err)
	}
	err := store.Settings.Save(&settings.Config)
	utils.CheckErr("store.Settings.Save", err)
	err = store.Settings.SaveServer(&settings.Config.Server)
	utils.CheckErr("store.Settings.SaveServer", err)
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
	err = store.Users.Save(&user)
	utils.CheckErr("store.Users.Save", err)
}
