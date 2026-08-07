package settings

import (
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// UserProfile is the nested profile document stored in SQLite user_data.profile
// and aligned with admin UserDefaults sections.
type UserProfile struct {
	Sidebar     UserDefaultsSidebar    `json:"sidebar,omitempty"`
	Listing     UserDefaultsListing    `json:"listing,omitempty"`
	Preview     UserDefaultsPreview    `json:"preview,omitempty"`
	FileViewer  UserDefaultsFileViewer `json:"fileViewer,omitempty"`
	Search      UserDefaultsSearch     `json:"search,omitempty"`
	UI          UserDefaultsUI         `json:"ui,omitempty"`
	FileLoading users.FileLoading      `json:"fileLoading,omitempty"`
	Account     UserDefaultsAccount    `json:"account,omitempty"`
}

// ProfileStorageVersion is the user.Version value after nested profile JSON is persisted.
const ProfileStorageVersion = 5

// ProfileFromUserDefaults extracts the profile sections from a defaults template.
func ProfileFromUserDefaults(d UserDefaults) UserProfile {
	return UserProfile{
		Sidebar:     d.Sidebar,
		Listing:     d.Listing,
		Preview:     d.Preview,
		FileViewer:  d.FileViewer,
		Search:      d.Search,
		UI:          d.UI,
		FileLoading: d.FileLoading,
		Account:     d.Account,
	}
}
