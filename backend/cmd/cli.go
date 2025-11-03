package cmd

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/version"
	"github.com/gtsteffaniak/filebrowser/backend/database/storage"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/go-logger/logger"
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
		// backwards compatibility in docker, prefer config.yaml if it exists
		if configPath != "" {
			_, err := os.Stat(configPath)
			if err != nil {
				logger.Fatalf("config file %v does not exist, please create it or set the FILEBROWSER_CONFIG environment variable to a valid config file path", configPath)
			}
		} else {
			configPath = "config.yaml"
		}
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
	var user, dbConfig string
	var asAdmin bool

	setCmd.StringVar(&user, "u", "", "Comma-separated username and password: \"set -u <username>,<password>\"")
	setCmd.BoolVar(&asAdmin, "a", false, "Create a user as admin user, used in combination with -u")
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
			_ = getStore(dbConfig) // ignore bool check
			user, err := store.Users.Get(username)
			if err != nil {
				newUser := users.User{
					Username:    username,
					LoginMethod: users.LoginMethodPassword,
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
				newUser.Permissions = settings.Config.UserDefaults.Permissions
				newUser.Permissions.Admin = asAdmin
				err = storage.CreateUser(newUser)
				if err != nil {
					logger.Errorf("could not create user: %v", err)
				}
				return false
			}
			if user.LoginMethod != users.LoginMethodPassword {
				logger.Fatalf("user %s is not allowed to login with password authentication, cannot set password", username)
			}
			user.Password = password
			user.TOTPSecret = "" // reset TOTP secret if it exists
			user.TOTPNonce = ""  // reset TOTP nonce if it exists
			user.LoginMethod = users.LoginMethodPassword
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
	Version 	 : %v
	Commit 		 : %v
	Release Info 	 : https://github.com/gtsteffaniak/filebrowser/releases/tag/%v
	`, version.Version, version.CommitSHA, version.Version)
			return false
		}
	}
	return true
}

// UserDefaults defines default settings for new users.
type UserDefaults struct {
	Permissions users.Permissions `yaml:"permissions"`
}

// Frontend defines settings related to the web interface.
type Frontend struct {
	Name string `yaml:"name,omitempty"`
}

// Source defines a directory to be served.
type Source struct {
	Name string `yaml:"name,omitempty"`
	Path string `yaml:"path"`
}

// Server defines server-specific configurations.
type Server struct {
	Port     int         `yaml:"port"`
	Database string      `yaml:"database"`
	Sources  []Source    `yaml:"sources"`
	Logging  []LogConfig `json:"logging"`
}

type LogConfig struct {
	Levels    string `json:"levels"`    // separated list of log levels to enable. (eg. "info|warning|error|debug")
	ApiLevels string `json:"apiLevels"` // separated list of log levels to enable for the API. (eg. "info|warning|error")
	Output    string `json:"output"`    // output location. (eg. "stdout" or "path/to/file.log")
	NoColors  bool   `json:"noColors"`  // disable colors in the output
	Utc       bool   `json:"utc"`       // use UTC time in the output instead of local time
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
	config := Settings{
		Server: Server{
			Logging: []LogConfig{
				{
					Levels:    "info|warning|error",
					ApiLevels: "info|warning|error",
					Output:    "stdout",
					NoColors:  false,
					Utc:       false,
				},
			},
			Sources: []Source{
				{
					Path: "",
				},
			},
		},
	}

	fmt.Println("--- Starting Configuration Setup ---")
	realPath := ""
	// 1. Ask for the source filesystem path (with validation)
	for {
		config.Server.Sources[0].Path = askQuestion(reader, "What is the source filesystem path?", "./")
		// Convert relative path to absolute path
		absolutePath, err := filepath.Abs(config.Server.Sources[0].Path)
		if err == nil {
			var isDir bool
			// Resolve symlinks and get the real path
			realPath, isDir, _ = iteminfo.ResolveSymlinks(absolutePath)
			if realPath != "" && isDir {
				break // Valid path found, exit loop
			}
		}
		fmt.Printf("Error: The path '%s' does not exist or isn't valid. Please try again.\n", config.Server.Sources[0].Path)
	}
	// 2. Ask for the source name
	defaultSourceName := filepath.Base(realPath)
	sourceName := askQuestion(reader, "What should the first source name be?", defaultSourceName)
	if sourceName != defaultSourceName {
		config.Server.Sources[0].Name = sourceName
	}

	// 3. Ask for server port (with validation)
	for {
		portStr := askQuestion(reader, "What port should the server listen on?", "80")
		port, err := strconv.Atoi(portStr)
		if err == nil && (port >= 1 && port <= 65535) {
			config.Server.Port = port
			break // Port is valid, exit loop
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
		config.Server.Database = askQuestion(reader, "What should the file name and path be for the database?", "./database.db")
		if strings.HasSuffix(config.Server.Database, ".db") {
			break // Valid path found, exit loop
		}
		fmt.Printf("Error: '%s' is not a valid path. Please enter a path to a file ending in .db", config.Server.Database)
	}
	// 4. Ask for the application brand name
	config.Frontend.Name = askQuestion(reader, "What should the application brand name be?", "FileBrowser Quantum")

	// 5. Ask for admin username and password
	config.Auth.AdminUsername = askQuestion(reader, "What should the default admin username be?", "admin")
	config.Auth.AdminPassword = askQuestion(reader, "What should the default admin password be?", "admin")

	// 6. Ask boolean (Yes/No) questions using the helper
	config.UserDefaults.Permissions.Modify = askYesNoQuestion(reader, "Should a new user be able to modify content by default?", "no")
	config.UserDefaults.Permissions.Share = askYesNoQuestion(reader, "Should a new user be able to create shares by default?", "no")

	fmt.Println("--- 	Configuration Complete 	---")

	// marshall yaml and write to file
	// Marshal the struct to YAML bytes
	yamlData, err := yaml.Marshal(&config)
	if err != nil {
		return
	}

	// Write the YAML data to the file
	err = os.WriteFile("config.yaml", yamlData, 0644) // 0644 provides read/write for owner, read for others
	if err != nil {
		return
	}
	// cleanup database if it exists
	if _, err := os.Stat(config.Server.Database); err == nil {
		response := askYesNoQuestion(reader, "Database specified already exists. Move database file to backup to start fresh?", "no")
		if !response {
			return
		}
		// move database file to backup
		backupPath := config.Server.Database + ".bak"
		err = os.Rename(config.Server.Database, backupPath)
		if err != nil {
			fmt.Printf("Error moving database file to backup: %v\n", err)
		} else {
			fmt.Printf("Database file moved to backup: %s\n", backupPath)
		}
	}

}

func generateYaml() {
	generateConfig := os.Getenv("FILEBROWSER_GENERATE_CONFIG") == "true"
	if generateConfig || settings.Config.Env.IsDevMode {
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
