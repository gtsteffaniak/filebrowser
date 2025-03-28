package storage

import (
	"fmt"
	"os"
	"path/filepath"

	storm "github.com/asdine/storm/v3"
	"github.com/gtsteffaniak/filebrowser/backend/auth"
	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/logger"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/database/storage/bolt"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
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

	err = bolt.Save(db, "version", 2)
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
	err := store.Settings.Save(&settings.Config)
	utils.CheckErr("store.Settings.Save", err)
	err = store.Settings.SaveServer(&settings.Config.Server)
	utils.CheckErr("store.Settings.SaveServer", err)
	user := &users.User{}
	settings.ApplyUserDefaults(user)
	user.Username = settings.Config.Auth.AdminUsername
	user.Password = settings.Config.Auth.AdminPassword
	user.Perm.Admin = true
	user.Scopes = []users.SourceScope{}
	for _, val := range settings.Config.Server.Sources {
		user.Scopes = append(user.Scopes, users.SourceScope{
			Name:  val.Path, // backend name is path
			Scope: "",
		})
	}
	user.LockPassword = false
	user.Perm = settings.AdminPerms()
	logger.Debug(fmt.Sprintf("Creating user as admin: %v %v", user.Username, user.Password))
	err = store.Users.Save(user, true)
	utils.CheckErr("store.Users.Save", err)
}

// create new user
func CreateUser(userInfo users.User, asAdmin bool) error {
	newUser := &userInfo
	// must have username or password to create
	if userInfo.Username == "" || userInfo.Password == "" {
		return errors.ErrInvalidRequestParams
	}
	logger.Debug(fmt.Sprintf("Creating user: %v %v", userInfo.Username, userInfo.Scopes))
	settings.ApplyUserDefaults(newUser)
	if asAdmin {
		userInfo.Perm = settings.AdminPerms()
	}
	if len(userInfo.Scopes) == 0 {
		userInfo.Scopes = settings.Config.UserDefaults.DefaultScopes
	}
	// create new home directories
	err := store.Users.Save(newUser, true)
	if err != nil {
		return err
	}
	return nil
}
