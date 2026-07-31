package state

import (
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/sqldb"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/errors"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

func noAuthAdminUsername() string {
	admin := settings.Config.Auth.AdminUsername
	if admin == "" {
		return "admin"
	}
	return admin
}

// ResolveNoAuthUser returns the user noauth mode impersonates at runtime: auth.adminUsername when
// present, otherwise the sole user when exactly one account exists.
func ResolveNoAuthUser() (users.User, error) {
	target := noAuthAdminUsername()
	if user, err := GetUserByUsername(target); err == nil {
		return user, nil
	}

	allUsers, err := GetAllUsers()
	if err != nil {
		return users.User{}, err
	}
	if len(allUsers) == 1 {
		return allUsers[0], nil
	}
	return users.User{}, errors.ErrNotExist
}

// EnsureNoAuthAdminUserAfterMigration runs once after Bolt→SQLite user import when noauth is enabled.
// It never renames or modifies existing users. A new admin account is created only when
// auth.adminUsername is missing and more than one user was migrated (a single migrated user is
// used at runtime via ResolveNoAuthUser).
func EnsureNoAuthAdminUserAfterMigration(store *sqldb.SQLStore) error {
	if store == nil || !settings.Config.Auth.Methods.NoAuth {
		return nil
	}

	target := noAuthAdminUsername()
	if _, err := store.GetUserByUsername(target); err == nil {
		return nil
	}

	allUsers, err := store.ListUsers()
	if err != nil {
		return fmt.Errorf("list users for noauth migration setup: %w", err)
	}
	if len(allUsers) <= 1 {
		return nil
	}

	if err := createNoAuthAdminUser(store); err != nil {
		return fmt.Errorf("create noauth admin user: %w", err)
	}
	logger.Infof("noauth: created admin user %q after migration", target)
	return nil
}

func createNoAuthAdminUser(store *sqldb.SQLStore) error {
	target := noAuthAdminUsername()

	user := &users.User{}
	ApplyUserDefaults(user)
	user.Username = target

	password := settings.Config.Auth.AdminPassword
	if password == "" {
		password = "admin"
	}
	hashedPassword, err := utils.HashPwd(password)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}
	user.Password = hashedPassword
	user.LoginMethod = users.LoginMethodPassword
	user.LockPassword = false
	user.Permissions = settings.AdminPerms()
	user.BackendScopes = []users.BackendScope{}
	for _, val := range settings.Config.Server.Sources {
		user.BackendScopes = append(user.BackendScopes, users.BackendScope{
			Path:  val.Path,
			Scope: "/",
		})
	}
	adminPerms := settings.AdminSourceFilePermissions()
	for i := range user.BackendScopes {
		user.BackendScopes[i].Permissions = adminPerms
	}
	users.SyncBackendSourcePermissionsMap(user)
	user.Version = users.CurrentUserMigrationVersion

	nid, err := utils.RandomUint64ID()
	if err != nil {
		return fmt.Errorf("allocate admin user id: %w", err)
	}
	user.ID = nid

	return store.CreateUser(user)
}
