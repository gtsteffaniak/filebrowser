package settings

import "testing"

func TestMergeUserDefaultsPatchJSON_listingOverridePreservesDefaultPreview(t *testing.T) {
	base := UserDefaults{
		Listing: UserDefaultsListing{
			QuickDownload: true,
			ShowHidden:    false,
		},
		Preview: UserDefaultsPreview{
			Image: boolPtr(true),
		},
	}
	patchJSON := []byte(`{"listing":{"showHidden":true}}`)
	merged, err := MergeUserDefaultsPatchJSON(base, patchJSON)
	if err != nil {
		t.Fatalf("MergeUserDefaultsPatchJSON: %v", err)
	}
	if !merged.Listing.ShowHidden {
		t.Fatalf("expected ShowHidden true after merge")
	}
	if !merged.Listing.QuickDownload {
		t.Fatalf("expected QuickDownload preserved from base")
	}
	if merged.Preview.Image == nil || !*merged.Preview.Image {
		t.Fatalf("expected preview.image preserved from base")
	}
}
