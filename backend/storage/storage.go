package storage

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/asdine/storm/v3"
	"github.com/gtsteffaniak/filebrowser/backend/auth"
	"github.com/gtsteffaniak/filebrowser/backend/errors"
	"github.com/gtsteffaniak/filebrowser/backend/files"
	"github.com/gtsteffaniak/filebrowser/backend/settings"
	"github.com/gtsteffaniak/filebrowser/backend/share"
	"github.com/gtsteffaniak/filebrowser/backend/storage/bolt"
	"github.com/gtsteffaniak/filebrowser/backend/users"
	"github.com/gtsteffaniak/filebrowser/backend/utils"
)

// Storage is a storage powered by a Backend which makes the necessary
// verifications when fetching and saving data to ensure consistency.
type Storage struct {
	Users    *users.Storage
	Share    *share.Storage
	Auth     *auth.Storage
	Settings *settings.Storage
}

var store *Storage

func InitializeDb(path string) (*Storage, bool, error) {
	exists, err := dbExists(path)
	if err != nil {
		panic(err)
	}
	db, err := storm.Open(path)

	utils.CheckErr(fmt.Sprintf("storm.Open path %v", path), err)
	authStore, userStore, shareStore, settingsStore, err := bolt.NewStorage(db)
	if err != nil {
		return nil, exists, err
	}

	err = bolt.Save(db, "version", 2) //nolint:gomnd
	if err != nil {
		return nil, exists, err
	}
	store = &Storage{
		Auth:     authStore,
		Users:    userStore,
		Share:    shareStore,
		Settings: settingsStore,
	}
	if !exists {
		quickSetup(store)
	}

	return store, exists, err
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
	user := settings.ApplyUserDefaults(users.User{})
	user.Username = settings.Config.Auth.AdminUsername
	user.Password = settings.Config.Auth.AdminPassword
	user.Perm.Admin = true
	user.Scope = "./"
	user.DarkMode = true
	user.ViewMode = "normal"
	user.LockPassword = false
	user.Perm = settings.AdminPerms()
	err = store.Users.Save(&user)
	utils.CheckErr("store.Users.Save", err)
}

// create new user
func CreateUser(userInfo users.User, asAdmin bool) error {
	// must have username or password to create
	if userInfo.Username == "" || userInfo.Password == "" {
		return errors.ErrInvalidRequestParams
	}
	newUser := settings.ApplyUserDefaults(userInfo)
	if asAdmin {
		newUser.Perm = settings.AdminPerms()
	}
	// create new home directory
	userHome, err := settings.Config.MakeUserDir(newUser.Username, newUser.Scope, files.RootPaths["default"])
	if err != nil {
		log.Printf("create user: failed to mkdir user home dir: [%s]", userHome)
		return err
	}
	newUser.Scope = userHome
	log.Printf("user: %s, home dir: [%s].", newUser.Username, userHome)
	idx := files.GetIndex("default")
	_, _, err = idx.GetRealPath(newUser.Scope)
	if err != nil {
		log.Println("user path is not valid", newUser.Scope)
		return nil
	}
	err = store.Users.Save(&newUser)
	if err != nil {
		return err
	}
	return nil
}
