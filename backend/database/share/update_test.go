package share

import "testing"

func TestApplyPostBodyUpdateCopiesShareFrontendFields(t *testing.T) {
	link := &Share{
		ShareFrontend: ShareFrontend{
			MaxBandwidth: 99,
		},
	}
	req := SharePostBody{
		ShareFrontend: ShareFrontend{
			MaxBandwidth:   512,
			DownloadsLimit: 3,
			HideFileExt:    ".tmp",
			FrontendShareInfo: FrontendShareInfo{
				ShareTheme: "dark",
				Title:      "updated",
			},
		},
	}

	ApplyPostBodyUpdate(link, &req, 12345)

	if link.MaxBandwidth != 512 {
		t.Fatalf("MaxBandwidth: got %d want 512", link.MaxBandwidth)
	}
	if link.DownloadsLimit != 3 {
		t.Fatalf("DownloadsLimit: got %d want 3", link.DownloadsLimit)
	}
	if link.HideFileExt != ".tmp" {
		t.Fatalf("HideFileExt: got %q want .tmp", link.HideFileExt)
	}
	if link.ShareTheme != "dark" || link.Title != "updated" {
		t.Fatalf("FrontendShareInfo not applied: %#v", link.FrontendShareInfo)
	}
	if link.Expire != 12345 {
		t.Fatalf("Expire: got %d want 12345", link.Expire)
	}
}
