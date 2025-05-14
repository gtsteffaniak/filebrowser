package cmd

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/common/logger"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/version"
	"github.com/gtsteffaniak/filebrowser/backend/database/storage"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

var (
	configPath string
)

// return bool to indicate if the program should continue running
func runCLI() bool {
	// Global flags
	var help bool
	// Override the default usage output to use generalUsage()
	flag.Usage = generalUsage
	flag.StringVar(&configPath, "c", "", "Path to the config file, default: config.yaml")
	flag.BoolVar(&help, "h", false, "Get help about commands")

	if configPath == "" {
		configPath = os.Getenv("FILEBROWSER_CONFIG")
	}

	if configPath == "" {
		configPath = "config.yaml"
	}

	// Parse global flags (before subcommands)
	flag.Parse() // print generalUsage on error

	// Show help if requested
	if help {
		generalUsage()
		return false
	}

	// Create a new FlagSet for the 'set' subcommand
	setCmd := flag.NewFlagSet("set", flag.ExitOnError)
	var user, scope, dbConfig string
	var asAdmin bool

	setCmd.StringVar(&user, "u", "", "Comma-separated username and password: \"set -u <username>,<password>\"")
	setCmd.BoolVar(&asAdmin, "a", false, "Create user as admin user, used in combination with -u")
	setCmd.StringVar(&scope, "s", "", "Specify a user scope, otherwise default user config scope is used")
	setCmd.StringVar(&dbConfig, "c", "config.yaml", "Path to the config file, default: config.yaml")

	// Parse subcommand flags only if a subcommand is specified
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "set":
			err := setCmd.Parse(os.Args[2:])
			if err != nil {
				setCmd.PrintDefaults()
				os.Exit(1)
			}
			userInfo := strings.Split(user, ",")
			if len(userInfo) < 2 {
				fmt.Printf("not enough info to create user: \"set -u username,password\", only provided %v\n", userInfo)
				setCmd.PrintDefaults()
				os.Exit(1)
			}
			username := userInfo[0]
			password := userInfo[1]
			ok := getStore(dbConfig)
			if !ok {
				logger.Fatal("could not load db info")
			}
			user, err := store.Users.Get(username)
			if err != nil {
				newUser := users.User{
					Username: username,
					NonAdminEditable: users.NonAdminEditable{
						Password: password,
					},
				}
				for _, source := range settings.Config.Server.SourceMap {
					if source.Config.DefaultEnabled {
						newUser.Scopes = append(newUser.Scopes, users.SourceScope{
							Name:  source.Name,
							Scope: source.Config.DefaultUserScope,
						})
					}
				}

				// Create the user logic
				if asAdmin {
					logger.Info(fmt.Sprintf("Creating user as admin: %s\n", username))
				} else {
					logger.Info(fmt.Sprintf("Creating non-admin user: %s\n", username))
				}
				err = storage.CreateUser(newUser, asAdmin)
				if err != nil {
					logger.Error(fmt.Sprintf("could not create user: %v", err))
				}
				return false
			}
			user.Password = password
			if asAdmin {
				user.Permissions.Admin = true
			}
			err = store.Users.Save(user, true, false)
			if err != nil {
				logger.Error(fmt.Sprintf("could not update user: %v", err))
			}
			fmt.Printf("successfully updated user: %s\n", username)
			return false

		case "version":
			fmt.Printf(`FileBrowser Quantum - A modern web-based file manager
	Version        : %v
	Commit         : %v
	Release Info   : https://github.com/gtsteffaniak/filebrowser/releases/tag/%v
	`, version.Version, version.CommitSHA, version.Version)
			return false
		}
	}
	return true
}
