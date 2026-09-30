package sharedefaults

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
)

func TestMergeEditableUpdate_preservesOmittedFields(t *testing.T) {
	existing := share.ShareEditable{
		FrontendShareInfo: share.FrontendShareInfo{
			AllowModify: true,
			AllowDelete: false,
			ShareType:   "normal",
		},
	}
	patch := []byte(`{"hash":"abc","allowDelete":true}`)
	merged, err := MergeEditableUpdate(existing, patch)
	if err != nil {
		t.Fatal(err)
	}
	if !merged.AllowModify {
		t.Fatal("expected allowModify preserved from existing share")
	}
	if !merged.AllowDelete {
		t.Fatal("expected allowDelete updated from patch")
	}
	if merged.ShareType != "normal" {
		t.Fatalf("shareType=%q want normal", merged.ShareType)
	}
}
