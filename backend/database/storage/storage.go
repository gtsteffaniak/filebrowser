package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	storm "github.com/asdine/storm/v3"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/storage/bolt"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
)

var userStore *users.Storage

func InitializeDb(path string) (*bolt.BoltStore, bool, error) {
	exists, err := dbExists(path)
	if err != nil {
		panic(err)
	}
	db, err := storm.Open(path)
	if err != nil {
		if strings.Contains(err.Error(), "timeout") {
			logger.Fatal("the database is locked, please close all other instances of filebrowser before starting.")
		}
		logger.Fatalf("could not open database: %v", err)
	}
	store, err := bolt.NewStorage(db)
	if err != nil {
		return nil, exists, err
	}
	// Load access rules from DB on startup
	// ignoring errors because
	_ = store.Access.LoadFromDB()
	userStore = store.Users
	err = bolt.Save(db, "version", 2)
	if err != nil {
		return nil, exists, err
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

func quickSetup(store *bolt.BoltStore) {
	settings.Config.Auth.Key = utils.GenerateKey()
	err := store.Settings.Save(&settings.Config)
	utils.CheckErr("store.Settings.Save", err)
	err = store.Settings.SaveServer(&settings.Config.Server)
	utils.CheckErr("store.Settings.SaveServer", err)
	user := &users.User{}
	settings.ApplyUserDefaults(user)
	user.Username = settings.Config.Auth.AdminUsername
	if settings.Config.Auth.AdminPassword == "" {
		settings.Config.Auth.AdminPassword = "admin"
	}
	user.Password = settings.Config.Auth.AdminPassword
	user.Permissions.Admin = true
	user.Scopes = []users.SourceScope{}
	for _, val := range settings.Config.Server.Sources {
		user.Scopes = append(user.Scopes, users.SourceScope{
			Name:  val.Path, // backend name is path
			Scope: "",
		})
	}
	user.LockPassword = false
	user.Permissions = settings.AdminPerms()
	logger.Debugf("Creating user as admin: username=%v password=%v", user.Username, user.Password)
	err = store.Users.Save(user, true, true)
	utils.CheckErr("store.Users.Save", err)
}

// create new user
func CreateUser(userInfo users.User, asAdmin bool) error {
	newUser := &userInfo
	if userInfo.LoginMethod == "password" {
		if userInfo.Password == "" {
			return fmt.Errorf("password is required to create a password login user")
		}
	} else {
		hashpass, err := users.HashPwd(userInfo.Username)
		if err != nil {
			return err
		}
		userInfo.Password = hashpass
	}
	userInfo.Permissions = settings.Config.UserDefaults.Permissions
	// must have username
	if userInfo.Username == "" {
		return fmt.Errorf("username is required to create a user")
	}
	logger.Debugf("Creating user: %v %v", userInfo.Username, userInfo.Scopes)
	settings.ApplyUserDefaults(newUser)
	if asAdmin {
		userInfo.Permissions = settings.AdminPerms()
	}
	// create new home directories
	err := userStore.Save(newUser, true, false)
	if err != nil {
		return err
	}
	return nil
}
