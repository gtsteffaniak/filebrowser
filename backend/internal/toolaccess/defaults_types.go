package toolaccess

import "github.com/gtsteffaniak/filebrowser/backend/internal/database/users"

// ToolAccessDefaultItem is one admin-configured tool with default/enforced flags.
type ToolAccessDefaultItem struct {
	ToolID   users.ToolID `json:"toolId"`
	Enabled  bool         `json:"enabled"`
	Enforced bool         `json:"enforced"`
}

// ToolAccessDefaultsDocument is persisted in SQLite settings.
type ToolAccessDefaultsDocument struct {
	Items []ToolAccessDefaultItem `json:"items"`
}
