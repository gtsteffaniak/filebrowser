package access

import "github.com/gtsteffaniak/filebrowser/backend/internal/utils"

// ruleKey returns the canonical storage key for access rules (directory form with trailing slash).
func ruleKey(path utils.IndexPath) string {
	return path.RuleKey()
}

// checkPath normalizes an index path for permission checks (directory form for hierarchy walk).
func checkPath(path utils.IndexPath) utils.IndexPath {
	return path.AsDirectory()
}
