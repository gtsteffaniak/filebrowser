package state

import (
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

// QuickSetup creates the initial admin user on first run.
func QuickSetup() error {
	if settings.Env.IsPlaywright || settings.Env.IsDevMode {
		settings.Env.IsFirstLoad = false
	} else {
		settings.Env.IsFirstLoad = true
	}

	settings.Config.Auth.Key = utils.GenerateKey()

	passwordAuth := settings.Config.Auth.Methods.PasswordAuth.Enabled
	noAuth := settings.Config.Auth.Methods.NoAuth

	if !passwordAuth && !noAuth {
		return nil
	}

	user := &users.User{}
	ApplyUserDefaults(user)

	user.Username = settings.Config.Auth.AdminUsername
	if user.Username == "" {
		user.Username = "admin"
	}

	if settings.Config.Auth.AdminPassword == "" {
		settings.Config.Auth.AdminPassword = "admin"
	}

	user.Permissions.Admin = true
	user.LoginMethod = users.LoginMethodPassword

	user.BackendScopes = []users.BackendScope{}
	for _, val := range settings.Config.Server.Sources {
		user.BackendScopes = append(user.BackendScopes, users.BackendScope{
			Path:  val.Path,
			Scope: "/",
		})
	}

	user.LockPassword = false
	user.Permissions = settings.AdminPerms()
	adminPerms := settings.AdminSourceFilePermissions()
	for i := range user.BackendScopes {
		user.BackendScopes[i].Permissions = adminPerms
	}
	users.SyncBackendSourcePermissionsMap(user)
	user.Version = users.CurrentUserMigrationVersion
	user.ShowFirstLogin = settings.Env.IsFirstLoad && user.Permissions.Admin

	logger.Debugf("Creating user as admin: %v", user.Username)

	hashedPassword, hashErr := utils.HashPwd(settings.Config.Auth.AdminPassword)
	if hashErr != nil {
		return fmt.Errorf("failed to hash admin password: %w", hashErr)
	}
	user.Password = hashedPassword

	nid, err := utils.RandomUint64ID()
	if err != nil {
		return fmt.Errorf("failed to allocate admin user id: %w", err)
	}
	user.ID = nid

	if err := sqlDb.CreateUser(user); err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}
	logger.Debug("quickSetup created admin user",
		"username", user.Username,
		"userID", user.ID,
		"permAdmin", user.Permissions.Admin,
		"permShare", user.Permissions.Share,
		"permModify", user.Permissions.Modify,
		"permCreate", user.Permissions.Create,
		"permDelete", user.Permissions.Delete,
		"permDownload", user.Permissions.Download,
		"permApi", user.Permissions.Api,
	)
	return nil
}
