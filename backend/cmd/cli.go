package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

var (
	configPath string
)

func setRule(backendSourcePath, indexPath, ruleCategory, value string, allow bool) error {
	if backendSourcePath == "" {
		return fmt.Errorf("backend source path is missing; check --source / -s")
	}
	if indexPath == "" {
		return fmt.Errorf("--path / -p is required (index path within the source, e.g. /)")
	}
	if ruleCategory == "" {
		return fmt.Errorf("role is required: use --role / -r <user|group|all>")
	}
	if ruleCategory != "all" && value == "" {
		return fmt.Errorf("value is required when role is 'user' or 'group': use --value / -v <username|groupname>")
	}

	parsedPath, err := utils.ParseSanitizedIndexPath(indexPath, true)
	if err != nil {
		return err
	}

	if allow {
		switch ruleCategory {
		case "user":
			err = state.AllowUser(backendSourcePath, parsedPath, value)
		case "group":
			err = state.AllowGroup(backendSourcePath, parsedPath, value)
		default:
			return fmt.Errorf("invalid role for allow: must be 'user' or 'group'")
		}
	} else {
		switch ruleCategory {
		case "user":
			err = state.DenyUser(backendSourcePath, parsedPath, value)
		case "group":
			err = state.DenyGroup(backendSourcePath, parsedPath, value)
		case "all":
			err = state.DenyAll(backendSourcePath, parsedPath)
		default:
			return fmt.Errorf("invalid role: must be 'user', 'group', or 'all'")
		}
	}

	if err != nil {
		return fmt.Errorf("failed to add or update rule: %w", err)
	}

	action := "deny"
	if allow {
		action = "allow"
	}
	if ruleCategory == "all" {
		fmt.Printf("successfully added %s rule for all users on index path '%s' in filesystem path '%s'\n", action, indexPath, backendSourcePath)
	} else {
		fmt.Printf("successfully added %s rule for %s '%s' on index path '%s' in filesystem path '%s'\n", action, ruleCategory, value, indexPath, backendSourcePath)
	}
	return nil
}

func setUser(username, password string, asAdmin bool) error {
	user, err := state.GetUserByUsername(username)
	if err != nil {
		newUser := users.User{
			FrontendUser: users.FrontendUser{
				Username:    username,
				LoginMethod: users.LoginMethodPassword,
			},
		}
		for _, source := range settings.Config.Server.SourceMap {
			if source.Config.DefaultEnabled {
				newUser.BackendScopes = append(newUser.BackendScopes, users.BackendScope{
					Path:  source.Path,
					Scope: source.Config.DefaultUserScope,
				})
			}
		}

		if asAdmin {
			logger.Infof("Creating user as admin: %s\n", username)
		} else {
			logger.Infof("Creating non-admin user: %s\n", username)
		}
		newUser.Permissions = settings.ConvertPermissionsToUsers(settings.Config.UserDefaults.Account.Permissions)
		newUser.Permissions.Admin = asAdmin
		err = state.CreateUser(&newUser, password)
		if err != nil {
			return fmt.Errorf("could not create user: %v", err)
		}
		if dirErr := files.MakeUserDirs(&newUser, true); dirErr != nil {
			logger.Error(dirErr.Error())
		}
		fmt.Printf("successfully created user")
		return nil
	}
	if user.LoginMethod != users.LoginMethodPassword {
		return fmt.Errorf("user %s is not allowed to login with password authentication, cannot set password", username)
	}
	user.TOTPSecret = ""
	user.TOTPNonce = ""
	user.LoginMethod = users.LoginMethodPassword
	if asAdmin {
		user.Permissions.Admin = true
	}
	if user.Version == 0 {
		user.Version = users.CurrentUserMigrationVersion
	}
	err = state.UpdateUser(&user, password)
	if err != nil {
		return fmt.Errorf("could not update user: %v", err)
	}
	fmt.Printf("successfully updated user: %s\n", username)
	return nil
}

func promoteUser(username string) error {
	user, err := state.GetUserByUsername(username)
	if err != nil {
		return fmt.Errorf("user %s not found", username)
	}
	if user.Permissions.Admin {
		fmt.Printf("user %s is already an admin\n", username)
		return nil
	}
	user.Permissions.Admin = true
	if user.Version == 0 {
		user.Version = users.CurrentUserMigrationVersion
	}
	err = state.UpdateUser(&user, "", "permissions")
	if err != nil {
		return fmt.Errorf("could not promote user: %v", err)
	}
	fmt.Printf("successfully promoted user to admin: %s\n", username)
	return nil
}

func askQuestion(reader *bufio.Reader, prompt string, defaultValue string) string {
	fmt.Printf("%s (default: %s): ", prompt, defaultValue)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}
	return input
}

func askYesNoQuestion(reader *bufio.Reader, prompt string, defaultValue string) bool {
	for {
		answer := askQuestion(reader, prompt, defaultValue)
		answer = strings.ToLower(answer)
		if answer == "yes" || answer == "y" {
			return true
		}
		if answer == "no" || answer == "n" {
			return false
		}
		fmt.Println("Error: Invalid input. Please enter 'yes' or 'no'.")
	}
}

func createConfig(configpath string) {
	reader := bufio.NewReader(os.Stdin)
	config := settings.SetDefaults(false)
	config.Server.Sources = []*settings.Source{{Path: "./"}}

	if configpath == "" {
		configpath = "config.yaml"
	}

	fmt.Println("--- Starting Configuration Setup ---")
	realPath := ""
	for {
		config.Server.Sources[0].Path = askQuestion(reader, "What is the source filesystem path?", "./")
		absolutePath, err := filepath.Abs(config.Server.Sources[0].Path)
		if err == nil {
			var isDir bool
			realPath, isDir, _ = iteminfo.ResolveSymlinks(absolutePath)
			if realPath != "" && isDir {
				break
			}
		}
		fmt.Printf("Error: The path '%s' does not exist or isn't valid. Please try again.\n", config.Server.Sources[0].Path)
	}
	defaultSourceName := filepath.Base(realPath)
	sourceName := askQuestion(reader, "What should the first source name be?", defaultSourceName)
	if sourceName != defaultSourceName {
		config.Server.Sources[0].Name = sourceName
	}

	for {
		portStr := askQuestion(reader, "What port should the server listen on?", "80")
		port, err := strconv.Atoi(portStr)
		if err == nil && (port >= 1 && port <= 65535) {
			config.Http.Port = port
			break
		}
		fmt.Printf("Error: '%s' is not a valid port. Please enter a number between 1 and 65535.\n", portStr)
	}

	for {
		levels := askQuestion(reader, "What should the log levels be?", "info|warning|error")
		checkLevels := SplitByMultiple(levels)
		invalidOptions := []string{}
		for _, level := range checkLevels {
			if !(level == "info" || level == "warning" || level == "error" || level == "debug") {
				invalidOptions = append(invalidOptions, level)
			}
		}
		if len(invalidOptions) == 0 {
			break
		}
		fmt.Printf("Error: invalid options given '%s'. valid options: 'info|warning|error|debug'.\n", invalidOptions)
	}

	for {
		config.Server.DatabaseV2.Path = askQuestion(reader, "What should the file name and path be for the database?", "./database.db")
		if strings.HasSuffix(config.Server.DatabaseV2.Path, ".db") {
			break
		}
		fmt.Printf("Error: '%s' is not a valid path. Please enter a path to a file ending in .db", config.Server.DatabaseV2.Path)
	}
	config.Frontend.Name = askQuestion(reader, "What should the application brand name be?", "FileBrowser Quantum")
	config.Auth.AdminUsername = askQuestion(reader, "What should the default admin username be?", "admin")
	config.Auth.AdminPassword = askQuestion(reader, "What should the default admin password be?", "admin")

	modifyDefault := askYesNoQuestion(reader, "Should a new user be able to modify content by default?", "no")
	config.Server.Sources[0].Config.DefaultPermissions = settings.NormalizeSourceFilePermissions(users.SourceFilePermissions{
		View:     true,
		Download: true,
		Modify:   modifyDefault,
		Create:   modifyDefault,
		Delete:   modifyDefault,
	})
	config.UserDefaults.Account.Permissions.Share = askYesNoQuestion(reader, "Should a new user be able to create shares by default?", "no")

	fmt.Println("--- 	Configuration Complete 	---")

	yamlData, err := yaml.Marshal(&config)
	if err != nil {
		return
	}

	err = os.WriteFile(configpath, yamlData, 0644)
	if err != nil {
		return
	}
	if _, err := os.Stat(config.Server.DatabaseV2.Path); err == nil {
		response := askYesNoQuestion(reader, "Database specified already exists. Move database file to backup to start fresh?", "no")
		if !response {
			return
		}
		backupPath := config.Server.DatabaseV2.Path + ".bak"
		err = os.Rename(config.Server.DatabaseV2.Path, backupPath)
		if err != nil {
			fmt.Printf("Error moving database file to backup: %v\n", err)
		} else {
			fmt.Printf("Database file moved to backup: %s\n", backupPath)
		}
	}
}

func generateYaml() {
	generateConfig := os.Getenv("FILEBROWSER_GENERATE_CONFIG") == "true"
	if generateConfig || settings.Env.IsDevMode {
		logger.Info("Generating config.yaml")
		settings.GenerateYaml()
	}

	if generateConfig {
		os.Exit(0)
	}
}

func SplitByMultiple(str string) []string {
	delimiters := []rune{'|', ',', ' '}
	return strings.FieldsFunc(str, func(r rune) bool {
		for _, d := range delimiters {
			if r == d {
				return true
			}
		}
		return false
	})
}
