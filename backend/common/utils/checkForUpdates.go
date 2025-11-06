package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/version"
	"github.com/gtsteffaniak/go-logger/logger"
	"golang.org/x/mod/semver"
)

var updateAvailableUrl = ""

type Tag struct {
	Name string `json:"name"`
}

type updateInfo struct {
	LatestVersion  string
	CurrentVersion string
	ReleaseNotes   string
	ReleaseUrl     string
}

func CheckForUpdates() (updateInfo, error) {
	// --- Configuration ---
	repoOwner := "gtsteffaniak"
	repoName := "filebrowser"
	currentVersion := version.Version
	isDevMode := os.Getenv("FILEBROWSER_DEVMODE") == "true"
	if currentVersion == "untracked" || currentVersion == "testing" || currentVersion == "" || isDevMode {
		return updateInfo{}, nil
	}
	splitVersion := strings.Split(currentVersion, "-")
	versionCategory := "stable"
	if len(splitVersion) > 1 {
		versionCategory = splitVersion[1]
	}
	githubApiUrl := fmt.Sprintf("https://api.github.com/repos/%s/%s/tags", repoOwner, repoName)
	// Make the GET request
	resp, err := http.Get(githubApiUrl)
	if err != nil {
		return updateInfo{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return updateInfo{}, fmt.Errorf("failed to fetch tags from GitHub API")
	}
	// Decode the JSON response
	var tags []Tag
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return updateInfo{}, err
	}
	// Find the latest version greater than the current one that matches the version category
	var NewVersion string
	for _, tag := range tags {
		// Check if the version is valid and the prerelease contains the version category
		if semver.IsValid(tag.Name) && strings.Contains(semver.Prerelease(tag.Name), versionCategory) {
			// Check if this tag is greater than the current version
			// and also greater than any other version we've found so far
			if semver.Compare(tag.Name, currentVersion) > 0 {
				if NewVersion == "" || semver.Compare(tag.Name, NewVersion) > 0 {
					NewVersion = tag.Name
				}
			}
		}
	}
	if NewVersion == "" {
		// No newer version found, return empty updateInfo
		return updateInfo{}, fmt.Errorf("no newer version found for %s", currentVersion)
	}

	updateAvailableUrl = fmt.Sprintf("https://github.com/%s/%s/releases/tag/%s", repoOwner, repoName, NewVersion)
	return updateInfo{
		LatestVersion:  NewVersion,
		CurrentVersion: currentVersion,
		ReleaseNotes:   updateAvailableUrl,
		ReleaseUrl:     updateAvailableUrl,
	}, nil
}

func GetUpdateAvailableUrl() string {
	return updateAvailableUrl
}

// starts a background process to check for updates periodically.
func StartCheckForUpdates() {
	// Create a ticker that fires every 24 hours.
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	// Start a loop that waits for the ticker to fire.
	for range ticker.C {
		_, err := CheckForUpdates()
		if err != nil {
			// In a real application, you might want more sophisticated logging.
			logger.Debug("update check failed:", err)
			continue // Don't stop the loop, just wait for the next tick.
		}
		// Update the global variable with the result of the check.
		// This will be an empty string if no newer version was found.
	}
}
