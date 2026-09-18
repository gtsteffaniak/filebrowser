package usersidebar

import "github.com/gtsteffaniak/filebrowser/backend/internal/database/users"

// SidebarLinkDefaultItem is one admin-configured sidebar link with default/enforced flags.
type SidebarLinkDefaultItem struct {
	Link     users.SidebarLink `json:"link"`
	Enabled  bool              `json:"enabled"`
	Enforced bool              `json:"enforced"`
}

// SidebarLinkDefaultsDocument is persisted in SQLite settings.
type SidebarLinkDefaultsDocument struct {
	Items []SidebarLinkDefaultItem `json:"items"`
}
