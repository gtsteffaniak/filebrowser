package toolaccess

import "github.com/gtsteffaniak/filebrowser/backend/internal/database/users"

// CatalogIDs returns the canonical ordered list of tool identifiers.
func CatalogIDs() []users.ToolID {
	return append([]users.ToolID(nil), users.AllToolIDs...)
}

// IsKnownTool reports whether toolID is in the catalog.
func IsKnownTool(toolID users.ToolID) bool {
	return toolID.Valid()
}
