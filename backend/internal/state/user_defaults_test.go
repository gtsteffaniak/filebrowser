package state

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestEffectiveUserDefaults_returnsUniversalTemplate(t *testing.T) {
	userDefaultsMu.Lock()
	userDefaultsDefault = settings.UserDefaults{
		Listing: settings.UserDefaultsListing{
			QuickDownload: true,
			ShowHidden:    true,
		},
	}
	userDefaultsMu.Unlock()

	effective := EffectiveUserDefaults()
	if !effective.Listing.ShowHidden {
		t.Fatalf("expected ShowHidden true")
	}
	if !effective.Listing.QuickDownload {
		t.Fatalf("expected QuickDownload true")
	}
}
