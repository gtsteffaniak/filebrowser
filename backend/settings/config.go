package settings

import (
	"log"
	"os"

	"github.com/goccy/go-yaml"
)

var GlobalConfiguration Settings
var configYml = "filebrowser.yaml"

func Initialize() {
	yamlData := loadConfigFile()
	GlobalConfiguration = setDefaults()
	err := yaml.Unmarshal(yamlData, &GlobalConfiguration)
	if err != nil {
		log.Fatalf("Error unmarshaling YAML data: %v", err)
	}
}

func loadConfigFile() []byte {
	// Open and read the YAML file
	yamlFile, err := os.Open(configYml)
	if err != nil {
		log.Printf("Error opening config file: %v\nUsing default config only", err)
		setDefaults()
		return []byte{}
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
	return yamlData
}

func setDefaults() Settings {
	return Settings{
		Signup: true,
		Server: Server{
			IndexingInterval:   5,
			Port:               8080,
			NumImageProcessors: 4,
			BaseURL:            "",
		},
		Auth: Auth{
			Method: "password",
			Recaptcha: Recaptcha{
				Host: "",
			},
		},
		UserDefaults: UserDefaults{
			HideDotfiles: true,
		},
	}
}
