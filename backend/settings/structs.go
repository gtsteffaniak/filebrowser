package settings

import (
	"github.com/gtsteffaniak/filebrowser/rules"
	"github.com/gtsteffaniak/filebrowser/users"
)

type Settings struct {
	Key              []byte              `json:"key"`
	Signup           bool                `json:"signup"`
	CreateUserDir    bool                `json:"createUserDir"`
	UserHomeBasePath string              `json:"userHomeBasePath"`
	Commands         map[string][]string `json:"commands"`
	Shell            []string            `json:"shell"`
	AdminUsername    string              `json:"adminUsername"`
	AdminPassword    string              `json:"adminPassword"`
	Rules            []rules.Rule        `json:"rules"`
	Server           Server              `json:"server"`
	Auth             Auth                `json:"auth"`
	Frontend         Frontend            `json:"frontend"`
	Users            []UserDefaults      `json:"users,omitempty"`
	UserDefaults     UserDefaults        `json:"userDefaults"`
}

type Auth struct {
	Recaptcha Recaptcha `json:"recaptcha"`
	Header    string    `json:"header"`
	Method    string    `json:"method"`
	Command   string    `json:"command"`
	Signup    bool      `json:"signup"`
	Shell     string    `json:"shell"`
}

type Recaptcha struct {
	Host   string `json:"host"`
	Key    string `json:"key"`
	Secret string `json:"secret"`
}

type Server struct {
	IndexingInterval      uint32 `json:"indexingInterval"`
	NumImageProcessors    int    `json:"numImageProcessors"`
	Socket                string `json:"socket"`
	TLSKey                string `json:"tlsKey"`
	TLSCert               string `json:"tlsCert"`
	EnableThumbnails      bool   `json:"enableThumbnails"`
	ResizePreview         bool   `json:"resizePreview"`
	EnableExec            bool   `json:"enableExec"`
	TypeDetectionByHeader bool   `json:"typeDetectionByHeader"`
	AuthHook              string `json:"authHook"`
	Port                  int    `json:"port"`
	BaseURL               string `json:"baseURL"`
	Address               string `json:"address"`
	Log                   string `json:"log"`
	Database              string `json:"database"`
	Root                  string `json:"root"`
}

type Frontend struct {
	Name                  string `json:"name"`
	DisableExternal       bool   `json:"disableExternal"`
	DisableUsedPercentage bool   `json:"disableUsedPercentage"`
	Files                 string `json:"files"`
	Color                 string `json:"color"`
}

// UserDefaults is a type that holds the default values
// for some fields on User.
type UserDefaults struct {
<<<<<<< HEAD
	LockPassword bool         `json:"lockPassword"`
	Scope        string       `json:"scope"`
	Locale       string       `json:"locale"`
	ViewMode     string       `json:"viewMode"`
	SingleClick  bool         `json:"singleClick"`
	Rules        []rules.Rule `json:"rules"`
	Sorting      struct {
=======
	DarkMode        bool         `json:"darkMode"`
	LockPassword    bool         `json:"lockPassword"`
	DisableSettings bool         `json:"disableSettings,omitempty"`
	Scope           string       `json:"scope"`
	Locale          string       `json:"locale"`
	ViewMode        string       `json:"viewMode"`
	SingleClick     bool         `json:"singleClick"`
	Rules           []rules.Rule `json:"rules"`
	Sorting         struct {
>>>>>>> v0.2.1
		By  string `json:"by"`
		Asc bool   `json:"asc"`
	} `json:"sorting"`
	Perm         users.Permissions `json:"perm"`
	Permissions  users.Permissions `json:"permissions"`
<<<<<<< HEAD
	Commands     []string          `json:"commands,omitemptys"`
=======
	Commands     []string          `json:"commands,omitempty"`
>>>>>>> v0.2.1
	HideDotfiles bool              `json:"hideDotfiles"`
	DateFormat   bool              `json:"dateFormat"`
}
