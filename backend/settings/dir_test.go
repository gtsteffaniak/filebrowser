package settings

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/rules"
)

func TestSettings_MakeUserDir(t *testing.T) {
	type fields struct {
		Key              []byte
		Signup           bool
		CreateUserDir    bool
		UserHomeBasePath string
		Commands         map[string][]string
		Shell            []string
		AdminUsername    string
		AdminPassword    string
		Rules            []rules.Rule
		Server           Server
		Auth             Auth
		Frontend         Frontend
		Users            []UserDefaults
		UserDefaults     UserDefaults
	}
	type args struct {
		username   string
		userScope  string
		serverRoot string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Settings{
				Key:              tt.fields.Key,
				Signup:           tt.fields.Signup,
				CreateUserDir:    tt.fields.CreateUserDir,
				UserHomeBasePath: tt.fields.UserHomeBasePath,
				Commands:         tt.fields.Commands,
				Shell:            tt.fields.Shell,
				AdminUsername:    tt.fields.AdminUsername,
				AdminPassword:    tt.fields.AdminPassword,
				Rules:            tt.fields.Rules,
				Server:           tt.fields.Server,
				Auth:             tt.fields.Auth,
				Frontend:         tt.fields.Frontend,
				Users:            tt.fields.Users,
				UserDefaults:     tt.fields.UserDefaults,
			}
			got, err := s.MakeUserDir(tt.args.username, tt.args.userScope, tt.args.serverRoot)
			if (err != nil) != tt.wantErr {
				t.Errorf("Settings.MakeUserDir() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Settings.MakeUserDir() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_cleanUsername(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cleanUsername(tt.args.s); got != tt.want {
				t.Errorf("cleanUsername() = %v, want %v", got, tt.want)
			}
		})
	}
}
