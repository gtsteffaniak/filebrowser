package icons

import "testing"

func TestShareRootFromStartURL(t *testing.T) {
	baseURL := "/testing/"
	startURL := "/testing/public/share/abc123XYZ-_/"
	shareRoot, normalizedStart, ok := ShareRootFromStartURL(baseURL, startURL)
	if !ok {
		t.Fatal("expected valid share start URL")
	}
	if shareRoot != "/testing/public/share/abc123XYZ-_/" {
		t.Fatalf("unexpected share root: %q", shareRoot)
	}
	if normalizedStart != startURL {
		t.Fatalf("unexpected normalized start: %q", normalizedStart)
	}
}

func TestShareRootFromStartURLRejectsTraversal(t *testing.T) {
	baseURL := "/testing/"
	startURL := "/testing/public/share/abc123XYZ-_/../secret/"
	_, _, ok := ShareRootFromStartURL(baseURL, startURL)
	if ok {
		t.Fatal("expected traversal path to be rejected")
	}
}

func TestShareHashFromHTTPPath(t *testing.T) {
	baseURL := "/testing/"
	paths := []string{
		"/testing/public/share/hash1234567890abcdef/",
		"/public/share/hash1234567890abcdef/files",
		"/share/hash1234567890abcdef/",
	}
	for _, path := range paths {
		hash := ShareHashFromHTTPPath(path, baseURL)
		if hash != "hash1234567890abcdef" {
			t.Fatalf("path %q: got hash %q", path, hash)
		}
	}
}

func TestShareHashFromHTTPPathRejectsEmbeddedPrefix(t *testing.T) {
	baseURL := "/testing/"
	path := "/evil/route/share/hash1234567890abcdef/"
	hash := ShareHashFromHTTPPath(path, baseURL)
	if hash != "" {
		t.Fatalf("expected empty hash for non-prefixed path, got %q", hash)
	}
}

func TestManifestForShare(t *testing.T) {
	CachedManifest = PWAManifest{
		Name:      "FileBrowser Quantum",
		ShortName: "FBQ",
		StartURL:  "/testing/",
		Scope:     "/testing/",
		ID:        "/testing/",
	}

	manifest, ok := ManifestForShare(
		"/testing/",
		"/testing/public/share/hash1234567890abcdef/",
		"My Share",
		"Shared files",
	)
	if !ok {
		t.Fatal("expected share manifest")
	}
	if manifest.StartURL != "/testing/public/share/hash1234567890abcdef/" {
		t.Fatalf("unexpected start_url: %q", manifest.StartURL)
	}
	if manifest.Scope != "/testing/public/share/hash1234567890abcdef/" {
		t.Fatalf("unexpected scope: %q", manifest.Scope)
	}
	if manifest.Name != "My Share" {
		t.Fatalf("unexpected name: %q", manifest.Name)
	}
}
