package settings

import (
	"io/ioutil"
	"log"
	"os"

	"github.com/goccy/go-yaml"
)

var GlobalConfiguration Settings

func Initialize() {
	// Open and read the YAML file
	yamlFile, err := os.Open("filebrowser.yml")
	if err != nil {
		log.Fatalf("Error opening YAML file: %v", err)
	}
	defer yamlFile.Close()

	yamlData, err := ioutil.ReadAll(yamlFile)
	if err != nil {
		log.Fatalf("Error reading YAML data: %v", err)
	}
	log.Println(GlobalConfiguration)
	setDefaults()
	log.Println(GlobalConfiguration)

	// Unmarshal the YAML data into the Settings struct
	err = yaml.Unmarshal(yamlData, &GlobalConfiguration)
	if err != nil {
		log.Fatalf("Error unmarshaling YAML data: %v", err)
	}
	log.Println(GlobalConfiguration)

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
			BaseURL:            "/",
		},
		Auth: Auth{
			Method: "noauth",
			Recaptcha: Recaptcha{
				Host: "",
			},
		},
	}
}
