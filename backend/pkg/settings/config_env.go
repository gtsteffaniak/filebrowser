package settings

import "os"

// expandConfigEnv substitutes $VAR and ${VAR} in config YAML using the process environment.
// Applied to the combined config document before YAML decode.
func expandConfigEnv(yamlContent []byte) []byte {
	if len(yamlContent) == 0 {
		return yamlContent
	}
	return []byte(os.ExpandEnv(string(yamlContent)))
}
