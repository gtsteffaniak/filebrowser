package icons

import (
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
)

const maxManifestTextLen = 256

func isValidShareHash(hash string) bool {
	if len(hash) < 8 || len(hash) > 64 {
		return false
	}
	if strings.Contains(hash, "..") || strings.Contains(hash, "/") {
		return false
	}
	return true
}

// ShareRootFromStartURL validates a share PWA start URL and returns the share root
// used for manifest scope/id plus the normalized start URL.
func ShareRootFromStartURL(baseURL, startURL string) (shareRoot, normalizedStart string, ok bool) {
	if startURL == "" || !strings.HasPrefix(startURL, baseURL) {
		return "", "", false
	}

	rel := strings.TrimPrefix(startURL, baseURL)
	if !strings.HasPrefix(rel, "public/share/") {
		return "", "", false
	}

	rest := strings.TrimPrefix(rel, "public/share/")
	hash, subPath, _ := strings.Cut(rest, "/")
	if !isValidShareHash(hash) {
		return "", "", false
	}
	if subPath != "" {
		if _, err := utils.SanitizeUserPath(subPath); err != nil {
			return "", "", false
		}
	}

	shareRoot = baseURL + "public/share/" + hash + "/"
	normalizedStart = startURL
	if !strings.HasPrefix(normalizedStart, shareRoot) && normalizedStart != strings.TrimSuffix(shareRoot, "/") {
		return "", "", false
	}
	if normalizedStart == baseURL+"public/share/"+hash {
		normalizedStart = shareRoot
	}

	return shareRoot, normalizedStart, true
}

// ShareHashFromHTTPPath extracts a share hash from common FileBrowser share routes.
func ShareHashFromHTTPPath(path, baseURL string) string {
	prefixes := []string{
		strings.TrimSuffix(baseURL, "/") + "/public/share/",
		"/public/share/",
		"/share/",
	}
	for _, prefix := range prefixes {
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		rest := strings.TrimPrefix(path, prefix)
		hash, _, _ := strings.Cut(rest, "/")
		if isValidShareHash(hash) {
			return hash
		}
	}
	return ""
}

func sanitizeManifestText(value string) string {
	if len(value) > maxManifestTextLen {
		return value[:maxManifestTextLen]
	}
	return value
}

// ManifestForShare returns a manifest scoped to a single public share link.
func ManifestForShare(baseURL, startURL, name, description string) (PWAManifest, bool) {
	shareRoot, normalizedStart, ok := ShareRootFromStartURL(baseURL, startURL)
	if !ok {
		return PWAManifest{}, false
	}

	manifest := CachedManifest
	manifest.StartURL = normalizedStart
	manifest.Scope = shareRoot
	manifest.ID = shareRoot

	if name = sanitizeManifestText(strings.TrimSpace(name)); name != "" {
		manifest.Name = name
		shortName := name
		if len(shortName) > 12 {
			shortName = shortName[:12]
		}
		manifest.ShortName = shortName
	}
	if description = sanitizeManifestText(strings.TrimSpace(description)); description != "" {
		manifest.Description = description
	}

	return manifest, true
}
