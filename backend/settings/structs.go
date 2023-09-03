package settings

import (
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
	Commands         map[string][]string `json:"commands"`
	Shell            []string            `json:"shell"`
	Rules            []rules.Rule        `json:"rules"`
	Server           Server              `json:"server"`
	Auth             Auth                `json:"auth"`
	Frontend         Frontend            `json:"frontend"`
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
	Theme                 string `json:"theme"`
	Color                 string `json:"color"`
}

// UserDefaults is a type that holds the default values
// for some fields on User.
type UserDefaults struct {
	Scope       string `json:"scope"`
	Locale      string `json:"locale"`
	ViewMode    string `json:"viewMode"`
	SingleClick bool   `json:"singleClick"`
	Sorting     struct {
		By  string `json:"by"`
		Asc bool   `json:"asc"`
	} `json:"sorting"`
	Perm struct {
		Admin    bool `json:"admin"`
		Execute  bool `json:"execute"`
		Create   bool `json:"create"`
		Rename   bool `json:"rename"`
		Modify   bool `json:"modify"`
		Delete   bool `json:"delete"`
		Share    bool `json:"share"`
		Download bool `json:"download"`
	} `json:"perm"`
	Commands     []string `json:"commands"`
	HideDotfiles bool     `json:"hideDotfiles"`
	DateFormat   bool     `json:"dateFormat"`
}
