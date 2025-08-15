package share

import "sync"

type CommonShare struct {
	//AllowEdit           bool   `json:"allowEdit,omitempty"`
	DisablingFileViewer bool   `json:"disableFileViewer,omitempty"`
	DownloadsLimit      int    `json:"downloadsLimit,omitempty"`
	ShareTheme          string `json:"shareTheme,omitempty"`
	DisableAnonymous    bool   `json:"disableAnonymous,omitempty"`
	//AllowUpload         bool   `json:"allowUpload,omitempty"`
	MaxBandwidth        int      `json:"maxBandwidth,omitempty"`
	DisableThumbnails   bool     `json:"disableThumbnails,omitempty"`
	KeepAfterExpiration bool     `json:"keepAfterExpiration,omitempty"`
	AllowedUsernames    []string `json:"allowedUsernames,omitempty"`
}
type CreateBody struct {
	CommonShare
	Hash       string `json:"hash,omitempty"`
	SourceName string `json:"sourceName,omitempty"`
	Password   string `json:"password"`
	Expires    string `json:"expires"`
	Unit       string `json:"unit"`
}

// Link is the information needed to build a shareable link.
type Link struct {
	CommonShare
	Mu           sync.Mutex `json:"-"`
	Downloads    int        `json:"-"`
	Hash         string     `json:"hash" storm:"id,index"`
	Path         string     `json:"path" storm:"index"`
	Source       string     `json:"source" storm:"index"`
	UserID       uint       `json:"userID"`
	Expire       int64      `json:"expire"`
	PasswordHash string     `json:"password_hash,omitempty"`
	// Token is a random value that will only be set when PasswordHash is set. It is
	// URL-Safe and is used to download links in password-protected shares via a
	// query arg.
	Token string `json:"token,omitempty"`
}
