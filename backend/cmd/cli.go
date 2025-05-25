package cmd

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/common/version"
	"github.com/gtsteffaniak/filebrowser/backend/database/storage"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
	"gopkg.in/yaml.v2"
)

var (
	configPath string
)

// return bool to indicate if the program should continue running
func runCLI() bool {

	generateYaml()

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
		case "setup":
			createConfig(configPath)
			return false
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
					logger.Infof("Creating user as admin: %s\n", username)
				} else {
					logger.Infof("Creating non-admin user: %s\n", username)
				}
				err = storage.CreateUser(newUser, asAdmin)
				if err != nil {
					logger.Errorf("could not create user: %v", err)
				}
				return false
			}
			user.Password = password
			if asAdmin {
				user.Permissions.Admin = true
			}
			err = store.Users.Save(user, true, false)
			if err != nil {
				logger.Errorf("could not update user: %v", err)
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

// Config holds the configuration values gathered from the user.
type Config struct {
	SourcePath          string
	SourceName          string
	BrandName           string
	Port                int
	AdminUser           string
	AdminPass           string
	CanDefaultShare     bool
	CanUserModify       bool
	CanUserCreateShares bool
	DatabasePath        string
}

// UserDefaults defines default settings for new users.
type UserDefaults struct {
	Permissions users.Permissions `yaml:"permissions"`
}

// Frontend defines settings related to the web interface.
type Frontend struct {
	Name string `yaml:"name,omitempty"`
	// Other fields for Frontend, e.g., Theme string `yaml:"theme,omitempty"`
}

// Source defines a directory to be served.
type Source struct {
	Name string `yaml:"name,omitempty"`
	Path string `yaml:"path"`
}

// Server defines server-specific configurations.
type Server struct {
	Port     int      `yaml:"port"`
	Database string   `yaml:"database"`
	Sources  []Source `yaml:"sources"`
}

type Auth struct {
	AdminUsername string `yaml:"adminUsername"`
	AdminPassword string `yaml:"adminPassword"`
}

// Settings is the top-level configuration structure.
type Settings struct {
	Server       Server       `yaml:"server"`
	Frontend     Frontend     `yaml:"frontend,omitempty"`
	Auth         Auth         `yaml:"auth"`
	UserDefaults UserDefaults `yaml:"userDefaults"`
}

// askQuestion displays a prompt and reads a line of input from the user.
// It returns the defaultValue if the user's input is empty.
func askQuestion(reader *bufio.Reader, prompt string, defaultValue string) string {
	fmt.Printf("%s (default: %s): ", prompt, defaultValue)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}
	return input
}

// askYesNoQuestion prompts the user with a yes/no question and returns a boolean.
// It retries until valid input ("yes", "y", "no", "n") is received.
// The defaultValue must be "yes" or "no".
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

// createConfig orchestrates the configuration process by asking the user a series of questions.
func createConfig(configpath string) {
	// check if config file exists
	if _, err := os.Stat("config.yaml"); err == nil {
		fmt.Println("Config file 'config.yaml' already exists, skipping setup.")
		return
	}
	reader := bufio.NewReader(os.Stdin)
	var config Config

	fmt.Println("--- Starting Configuration Setup ---")
	realPath := ""
	// 1. Ask for the source filesystem path (with validation)
	for {
		config.SourcePath = askQuestion(reader, "What is the source filesystem path?", "./")
		// Convert relative path to absolute path
		absolutePath, err := filepath.Abs(config.SourcePath)
		if err == nil {
			var isDir bool
			// Resolve symlinks and get the real path
			realPath, isDir, _ = utils.ResolveSymlinks(absolutePath)
			if realPath != "" && isDir {
				break // Valid path found, exit loop
			}
		}
		fmt.Printf("Error: The path '%s' does not exist or isn't valid. Please try again.\n", config.SourcePath)
	}
	// 2. Ask for the source name
	defaultSourceName := filepath.Base(realPath)
	config.SourceName = askQuestion(reader, "What should the first source name be?", defaultSourceName)
	if config.SourceName == defaultSourceName {
		config.SourceName = ""
	}

	// 3. Ask for server port (with validation)
	for {
		portStr := askQuestion(reader, "What port should the server listen on?", "80")
		port, err := strconv.Atoi(portStr)
		if err == nil && (port >= 1 && port <= 65535) {
			config.Port = port
			break // Port is valid, exit loop
		}
		fmt.Printf("Error: '%s' is not a valid port. Please enter a number between 1 and 65535.\n", portStr)
	}

	for {
		config.DatabasePath = askQuestion(reader, "What should the file name and path be for the database?", "./database.db")
		if strings.HasSuffix(config.DatabasePath, ".db") {
			break // Valid path found, exit loop
		}
		fmt.Printf("Error: '%s' is not a valid path. Please enter a path to a file ending in .db", config.DatabasePath)
	}
	// 4. Ask for the application brand name
	config.BrandName = askQuestion(reader, "What should the application brand name be?", "FileBrowser Quantum")

	// 5. Ask for admin username and password
	config.AdminUser = askQuestion(reader, "What should the default admin username be?", "admin")
	config.AdminPass = askQuestion(reader, "What should the default admin password be?", "admin")

	// 6. Ask boolean (Yes/No) questions using the helper
	config.CanUserModify = askYesNoQuestion(reader, "Should a new user be able to modify content by default?", "no")
	config.CanUserCreateShares = askYesNoQuestion(reader, "Should a new user be able to create shares by default?", "no")

	fmt.Println("---    Configuration Complete    ---")
	// save config file yaml from settings.Settings struct
	writeConfig := Settings{
		Server: Server{
			Port:     config.Port,
			Database: config.DatabasePath,
			Sources: []Source{
				{
					Name: config.SourceName,
					Path: config.SourcePath,
				},
			},
		},
		Frontend: Frontend{
			Name: config.BrandName,
		},
		UserDefaults: UserDefaults{
			Permissions: users.Permissions{
				Share:  config.CanDefaultShare,
				Modify: config.CanUserModify,
			},
		},
	}
	// marshall yaml and write to file
	// Marshal the struct to YAML bytes
	yamlData, err := yaml.Marshal(&writeConfig)
	if err != nil {
		return
	}

	// Write the YAML data to the file
	err = os.WriteFile("config.yaml", yamlData, 0644) // 0644 provides read/write for owner, read for others
	if err != nil {
		return
	}

}

func generateYaml() {
	if os.Getenv("FILEBROWSER_GENERATE_CONFIG") != "" {
		logger.Info("Generating config.yaml")
		settings.GenerateYaml()
		os.Exit(0)
	}
}
