package settings

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestInitialize(t *testing.T) {
	type args struct {
		configFile string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Initialize(tt.args.configFile)
		})
	}
}

func Test_setDefaults(t *testing.T) {
	tests := []struct {
		name string
		want Settings
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := setDefaults(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("setDefaults() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfigLoadChanged(t *testing.T) {
	defaultConfig := setDefaults()
	err := loadConfigWithDefaults("./validConfig.yaml")
	if err != nil {
		t.Fatalf("error loading config file: %v", err)
	}
	// Use go-cmp to compare the two structs
	if diff := cmp.Diff(defaultConfig, Config); diff == "" {
		t.Errorf("No change when there should have been (-want +got):\n%s", diff)
	}
}

func TestConfigLoadSpecificValues(t *testing.T) {
	defaultConfig := setDefaults()
	err := loadConfigWithDefaults("./validConfig.yaml")
	if err != nil {
		t.Fatalf("error loading config file: %v", err)
	}
	testCases := []struct {
		fieldName string
		globalVal interface{}
		newVal    interface{}
	}{
		{"Server.Database", Config.Server.Database, defaultConfig.Server.Database},
	}

	for _, tc := range testCases {
		if tc.globalVal == tc.newVal {
			t.Errorf("Differences should have been found:\nConfig.%s: %v \nSetConfig: %v \n", tc.fieldName, tc.globalVal, tc.newVal)
		}
	}
}

func TestInvalidConfig(t *testing.T) {
	configFile := "./invalidConfig.yaml"
	err := loadConfigWithDefaults(configFile)
	if err == nil {
		t.Fatalf("expected error loading config file %s, got nil", configFile)
	}
}

func TestValidationVarious(t *testing.T) {
	testCases := []struct {
		name       string
		configFile []byte
		searchErr  string
	}{
		{
			name: "valid minimal config",
			configFile: []byte(`
server:
  port: 8080
  sources:
    - path: "/data"
auth:
  tokenExpirationHours: 24
  methods:
    password:
      enabled: true
  signup: false
  key: ""
  adminUsername: "admin"
  adminPassword: "password"
frontend:
  name: "MyApp"
userDefaults:
  stickySidebar: false
  darkMode: false
  lockPassword: false
  locale: "en"
  viewMode: "grid"
  gallerySize: 5
  singleClick: false
  showHidden: false
  dateFormat: false
  themeColor: "#ffffff"
  quickDownload: false
integrations:
  office:
    url: "https://office.example.com"
    secret: "secret-key"
`),
			searchErr: "",
		},
		{
			name: "missing required server.sources",
			configFile: []byte(`
server:
  port: 8080
`),
			searchErr: "Field validation for 'Sources' failed on the 'required'",
		},
		{
			name: "missing onlyoffice.secret",
			configFile: []byte(`
server:
  port: 8080
  sources:
   - path: "/data"
integrations:
  office:
    url: "https://office.example.com"
`),
			searchErr: "Field validation for 'Secret' failed on the 'required'",
		},
		{
			name: "typo: Server with lowercase s",
			configFile: []byte(`
serverx:
  port: 8080
  sources:
    - path: "/data"
`),
			searchErr: "unknown field \"serverx\"",
		},
		{
			name: "typo in auth.methods.password",
			configFile: []byte(`
server:
  port: 8080
  sources:
    - path: "/data"
auth:
  tokenExpirationHours: 24
  methods:
    passwrd:
      enabled: true
  signup: false
  key: ""
  adminUsername: "admin"
  adminPassword: "password"
`),
			searchErr: "unknown field \"passwrd\"",
		},
		{
			name: "missing userDefaults",
			configFile: []byte(`
server:
  port: 8080
  sources:
    - path: "/data"
auth:
  tokenExpirationHours: 24
  methods:
    password:
      enabled: true
  signup: false
  key: ""
  adminUsername: "admin"
  adminPassword: "password"
frontend:
  name: "MyApp"
integrations:
  office:
    url: "https://office.example.com"
    secret: "secret-key"
`),
			searchErr: "Field validation for 'Locale' failed", // or related subfields
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateConfig(tc.configFile)
			if !(err == nil && tc.searchErr == "") {
				if err == nil || !strings.Contains(err.Error(), tc.searchErr) {
					t.Fatalf("testcase %v could not find error: '%v', got: %v", tc.name, tc.searchErr, err)
				}
			}
			if err != nil && tc.searchErr == "" {
				t.Fatalf("testcase %v should not have failed but got: %v", tc.name, err)
			}
			if err == nil && tc.searchErr != "" {
				t.Fatalf("testcase %v should have failed but got: %v", tc.name, err)
			}
		})
	}
}
