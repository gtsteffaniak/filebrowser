package settings

import (
	"github.com/gtsteffaniak/filebrowser/files"
	"github.com/gtsteffaniak/filebrowser/rules"
	"github.com/gtsteffaniak/filebrowser/users"
)

// Apply applies the default options to a user.
func (d *UserDefaults) Apply(u *users.User) {
	u.Scope = d.Scope
	u.Locale = d.Locale
	u.ViewMode = d.ViewMode
	u.SingleClick = d.SingleClick
	u.Perm = d.Perm
	u.Sorting = d.Sorting
	u.Commands = d.Commands
	u.HideDotfiles = d.HideDotfiles
	u.DateFormat = d.DateFormat
}

type Settings struct {
	Key              []byte              `json:"key"`
	Signup           bool                `json:"signup"`
	CreateUserDir    bool                `json:"createUserDir"`
	UserHomeBasePath string              `json:"userHomeBasePath"`
	Defaults         UserDefaults        `json:"defaults"`
	Commands         map[string][]string `json:"commands"`
	Shell            []string            `json:"shell"`
	Rules            []rules.Rule        `json:"rules"`
	Server           Server              `json:"server"`
	AuthMethod       string              `json:"authMethod"`
	Auth             struct {
		Header  string `json:"header"`
		Method  string `json:"method"`
		Command string `json:"command"`
		Signup  bool   `json:"signup"`
		Shell   string `json:"shell"`
	} `json:"auth"`

	Branding Branding `json:"branding"`

	UserDefaults UserDefaults `json:"userDefaults"`
}

type Server struct {
	Socket                string `json:"socket"`
	TLSKey                string `json:"tlsKey"`
	TLSCert               string `json:"tlsCert"`
	EnableThumbnails      bool   `json:"enableThumbnails"`
	ResizePreview         bool   `json:"resizePreview"`
	EnableExec            bool   `json:"enableExec"`
	TypeDetectionByHeader bool   `json:"typeDetectionByHeader"`
	AuthHook              string `json:"authHook"`
	Port                  string `json:"port"`
	BaseURL               string `json:"baseURL"`
	Address               string `json:"address"`
	Log                   string `json:"log"`
	Database              string `json:"database"`
	Root                  string `json:"root"`
	EnablePreviewResize   bool   `json:"disable-preview-resize"`
}

type Branding struct {
	Name                  string `json:"name"`
	DisableExternal       bool   `json:"disableExternal"`
	DisableUsedPercentage bool   `json:"disableUsedPercentage"`
	Files                 string `json:"files"`
	Theme                 string `json:"theme"`
	Color                 string `json:"color"`
}

// UserDefaults is a type that holds the default values
// for some fields on User.
type UserDefaults struct {
	Scope        string            `json:"scope"`
	Locale       string            `json:"locale"`
	ViewMode     users.ViewMode    `json:"viewMode"`
	SingleClick  bool              `json:"singleClick"`
	Sorting      files.Sorting     `json:"sorting"`
	Perm         users.Permissions `json:"perm"`
	Commands     []string          `json:"commands"`
	HideDotfiles bool              `json:"hideDotfiles"`
	DateFormat   bool              `json:"dateFormat"`
}

//{
//	"server":{
//	   "port":8080,
//	   "baseURL":"",
//	   "address":"",
//	   "log":"stdout",
//	   "database":"./database.db",
//	   "root":"/srv",
//	   "disable-thumbnails":false,
//	   "disable-preview-resize":false,
//	   "disable-exec":false,
//	   "disable-type-detection-by-header":false
//	},
//	"auth":{
//	   "header":"",
//	   "method":"",
//	   "command":"",
//	   "signup":false,
//	   "shell":""
//	},
//	"branding":{
//	   "name":"",
//	   "color":"",
//	   "files":"",
//	   "disableExternal":"",
//	   "disableUsedPercentage":""
//	},
//	"permissions":{
//	   "Admin":false,
//	   "Execute":true,
//	   "Create":true,
//	   "Rename":true,
//	   "Modify":true,
//	   "Delete":true,
//	   "Share":true,
//	   "Download":true
//	},
//	"commands":{},
//	"shell":{},
//	"rules":{}
// }
