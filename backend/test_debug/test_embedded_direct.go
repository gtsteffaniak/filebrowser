package main

import (
	"fmt"
	"os"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
)

func main() {
	// Read the config.generated.yaml file from the dist directory
	embeddedContent, err := os.ReadFile("../http/dist/config.generated.yaml")
	if err != nil {
		fmt.Printf("Error reading config.generated.yaml: %v\n", err)
		os.Exit(1)
	}

	// Initialize with test config
	settings.Initialize("../test_config.yaml")
	currentConfig := &settings.Config

	fmt.Println("=== Testing GenerateConfigYamlWithEmbedded directly ===")

	// Call the embedded method directly
	yamlOutput, err := settings.GenerateConfigYamlWithEmbedded(currentConfig, false, false, false, string(embeddedContent))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Generated YAML (first 1000 chars):")
	output := yamlOutput
	if len(output) > 1000 {
		output = output[:1000] + "..."
	}
	fmt.Println(output)
}
