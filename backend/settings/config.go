package settings

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
)

var GlobalConfiguration Settings

func Initialize() {
	// Open and read the YAML file
	yamlFile, err := os.Open("filebrowser.yaml")
	if err != nil {
		log.Println("Error opening config file: ", err)
		log.Println("Using default config only")
		// Get the current directory
		dir, err := os.Getwd()
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		// Use the filepath package to join the directory and file names
		err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				fmt.Println("Error:", err)
				return err
			}
			// Check if it's a regular file (not a directory)
			if !info.IsDir() {
				fmt.Println(path)
			}
			return nil
		})
		setDefaults()
		return
	}
	defer yamlFile.Close()

	stat, err := yamlFile.Stat()
	if err != nil {
		log.Fatalf("Error getting file information: %s", err.Error())
	}

	yamlData := make([]byte, stat.Size())
	_, err = yamlFile.Read(yamlData)
	if err != nil {
		log.Fatalf("Error reading YAML data: %v", err)
	}
	// Unmarshal the YAML data into the Settings struct
	err = yaml.Unmarshal(yamlData, &GlobalConfiguration)
	if err != nil {
		log.Fatalf("Error unmarshaling YAML data: %v", err)
	}
	// Now you have the Settings struct with values from the YAML file
	// You can access the values like: defaultSettings.Key, defaultSettings.Server.Port, etc.
}

func setDefaults() {
	GlobalConfiguration = Settings{
		Signup: true,
		Server: Server{
			IndexingInterval:   5,
			Port:               8080,
			NumImageProcessors: 1,
			BaseURL:            "",
		},
		Auth: Auth{
			Method: "password",
			Recaptcha: Recaptcha{
				Host: "",
			},
		},
	}
}
