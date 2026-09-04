package sharedefaults

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestApplyDefaultsToEditable_mergesClientOverDefaults(t *testing.T) {
	defaults := settings.ShareDefaults{
		ShareType:       "normal",
		AllowModify:     true,
		DisableDownload: true,
		Title:           "Default title",
	}
	editable := share.ShareEditable{
		FrontendShareInfo: share.FrontendShareInfo{
			ShareType: "upload",
			Title:     "Custom",
		},
	}
	ApplyDefaultsToEditable(&editable, defaults)
	if editable.ShareType != "upload" {
		t.Fatalf("client shareType should win, got %q", editable.ShareType)
	}
	if editable.Title != "Custom" {
		t.Fatalf("client title should win, got %q", editable.Title)
	}
	if !editable.AllowModify || !editable.DisableDownload {
		t.Fatal("expected defaults applied for unset fields")
	}
}

func TestValidateEditableNotEnforced_blocksMismatch(t *testing.T) {
	defaults := settings.ShareDefaults{
		AllowModify:  true,
		SidebarLinks: []users.SidebarLink{{Name: "Info", Category: "shareInfo", Target: "#"}},
	}
	enforced := settings.ShareDefaultsEnforcement{AllowModify: true, SidebarLinks: true}
	editable := share.ShareEditable{
		FrontendShareInfo: share.FrontendShareInfo{
			AllowModify:  false,
			SidebarLinks: []users.SidebarLink{{Name: "Other", Category: "custom", Target: "/x"}},
		},
	}
	if err := ValidateEditableNotEnforced(&editable, enforced, defaults); err == nil {
		t.Fatal("expected enforcement error")
	}
}
