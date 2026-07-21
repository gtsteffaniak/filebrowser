package settings

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func TestProxyConfigUserDefaults(t *testing.T) {
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	proxyConfig := "../../../_docker/src/proxy/backend/config.yaml"
	if err := loadConfigWithDefaults(proxyConfig, true); err != nil {
		t.Fatal(err)
	}
	if !Config.UserDefaults.Account.Permissions.Share {
		t.Fatalf("expected account.permissions.share true, got false")
	}
	u := &users.User{FrontendUser: users.FrontendUser{Username: "demo-127.0.0.1", LoginMethod: users.LoginMethodProxy}}
	ApplyUserDefaults(u)
	if !u.Permissions.Share {
		t.Fatalf("ApplyUserDefaults Share=false, want true")
	}
}
