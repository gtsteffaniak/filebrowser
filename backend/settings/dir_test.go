package settings

import (
	"testing"
)

func TestSettings_MakeUserDirs(t *testing.T) {
	type fields struct {
		Signup           bool
		CreateUserDir    bool
		UserHomeBasePath string
		Shell            []string
		AdminUsername    string
		AdminPassword    string
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
				Shell:        tt.fields.Shell,
				Server:       tt.fields.Server,
				Auth:         tt.fields.Auth,
				Frontend:     tt.fields.Frontend,
				Users:        tt.fields.Users,
				UserDefaults: tt.fields.UserDefaults,
			}
			got, err := s.MakeUserDirs(tt.args.username, tt.args.userScope, tt.args.serverRoot)
			if (err != nil) != tt.wantErr {
				t.Errorf("Settings.MakeUserDirs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Settings.MakeUserDirs() = %v, want %v", got, tt.want)
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
