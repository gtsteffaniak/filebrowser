package main

import (
	"fmt"
	"os"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
)

func main() {
	// Initialize with test config
	settings.Initialize("../test_config.yaml")
	currentConfig := &settings.Config

	fmt.Println("=== Testing both methods ===")

	// Test the main method (which tries source first, then embedded fallback)
	fmt.Println("\n1. GenerateConfigYaml (source code first, embedded fallback):")
	yamlOutput1, err := settings.GenerateConfigYaml(currentConfig, false, false, false)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		lines := yamlOutput1
		if len(lines) > 500 {
			lines = lines[:500] + "..."
		}
		fmt.Println(lines)

		// Check if minSearchLength is present
		if contains := yamlOutput1; contains != "" {
			if yamlOutput1[0:30] != "" {
				fmt.Printf("Contains minSearchLength: %v\n", contains != "" && contains[0:30] != "")
			}
		}
	}

	// Test the embedded method directly
	fmt.Println("\n2. GenerateConfigYamlWithEmbedded (forced embedded method):")
	embeddedContent, err := os.ReadFile("../http/dist/config.generated.yaml")
	if err != nil {
		fmt.Printf("Error reading embedded: %v\n", err)
		return
	}

	yamlOutput2, err := settings.GenerateConfigYamlWithEmbedded(currentConfig, false, false, false, string(embeddedContent))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		lines := yamlOutput2
		if len(lines) > 500 {
			lines = lines[:500] + "..."
		}
		fmt.Println(lines)
	}
}
