package analytics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/version"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

const (
	schemaVersion               = "1"
	eventTypeDeploymentSnapshot = "deployment_snapshot"
	userAgent                   = "filebrowser-analytics"
	xApp                        = "filebrowser"
)

type envelope struct {
	SchemaVersion  string          `json:"schema_version"`
	EventType      string          `json:"event_type"`
	InstallationID string          `json:"installation_id"`
	ID             string          `json:"id"`
	Payload        json.RawMessage `json:"payload"`
}

type snapshotPayload struct {
	App          appPayload          `json:"app"`
	System       systemPayload       `json:"system"`
	Counts       countsPayload       `json:"counts"`
	Auth         authPayload         `json:"auth"`
	Integrations integrationsPayload `json:"integrations"`
	Features     featuresPayload     `json:"features"`
	Indexing     indexingPayload     `json:"indexing"`
	Storage      storagePayload      `json:"storage"`
}

type appPayload struct {
	Version   string `json:"version"`
	CommitSHA string `json:"commit_sha"`
}

type systemPayload struct {
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	CPUCores      int    `json:"cpu_cores"`
	MemoryRSSMB   int    `json:"memory_rss_mb"`
	MemoryTotalMB int    `json:"memory_total_mb"`
	Runtime       string `json:"runtime"`
}

type countsPayload struct {
	SourcesEnabled   int `json:"sources_enabled"`
	SourcesReadOnly  int `json:"sources_read_only"`
	SourcesPrivate   int `json:"sources_private"`
	UsersTotal       int `json:"users_total"`
	AccessRulesTotal int `json:"access_rules_total"`
	SharesActive     int `json:"shares_active"`
	CustomThemes     int `json:"custom_themes"`
}

type authPayload struct {
	NoAuth              bool `json:"noauth"`
	PasswordEnabled     bool `json:"password_enabled"`
	PasswordSignup      bool `json:"password_signup"`
	PasswordEnforcedOTP bool `json:"password_enforced_otp"`
	OidcEnabled         bool `json:"oidc_enabled"`
	LdapEnabled         bool `json:"ldap_enabled"`
	JwtEnabled          bool `json:"jwt_enabled"`
	ProxyEnabled        bool `json:"proxy_enabled"`
	PasskeyEnabled      bool `json:"passkey_enabled"`
}

type integrationsPayload struct {
	OnlyOfficeConfigured bool `json:"onlyoffice_configured"`
	OnlyOfficeViewOnly   bool `json:"onlyoffice_view_only"`
	MediaEnabled         bool `json:"media_enabled"`
	HardwareAcceleration bool `json:"hardware_acceleration"`
}

type featuresPayload struct {
	DisablePreviews        bool `json:"disable_previews"`
	DisableWebDAV          bool `json:"disable_webdav"`
	DisableUpdateCheck     bool `json:"disable_update_check"`
	ActivityLoggingEnabled bool `json:"activity_logging_enabled"`
	TLSEnabled             bool `json:"tls_enabled"`
	RateLimitDisabled      bool `json:"rate_limit_disabled"`
	IndexWALMode           bool `json:"index_wal_mode"`
	CacheDirCleanup        bool `json:"cache_dir_cleanup"`
}

type indexingPayload struct {
	AvgQuickScanSeconds int `json:"avg_quick_scan_seconds"`
	AvgFullScanSeconds  int `json:"avg_full_scan_seconds"`
	TotalFiles          int `json:"total_files"`
	TotalDirs           int `json:"total_dirs"`
	SourcesIndexed      int `json:"sources_indexed"`
}

type storagePayload struct {
	MainDBBytes  int64 `json:"main_db_bytes"`
	IndexDBBytes int64 `json:"index_db_bytes"`
}

func Enabled() bool {
	return state.IsAnalyticsEnabled()
}

func buildSnapshot(publishable bool) ([]byte, error) {
	versionLabel, err := snapshotVersion(publishable)
	if err != nil {
		return nil, err
	}

	installationID, err := state.GetOrCreateInstallationID()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	eventID := fmt.Sprintf("%s#%s#%s", installationID, eventTypeDeploymentSnapshot, now.Format("2006-01-02"))

	payload, err := collectPayload(versionLabel)
	if err != nil {
		return nil, err
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	event := envelope{
		SchemaVersion:  schemaVersion,
		EventType:      eventTypeDeploymentSnapshot,
		InstallationID: installationID,
		ID:             eventID,
		Payload:        payloadJSON,
	}
	return json.Marshal(event)
}

func collectPayload(versionLabel string) (snapshotPayload, error) {
	users, err := state.GetAllUsers()
	if err != nil {
		return snapshotPayload{}, err
	}
	shares, err := state.GetAllShares()
	if err != nil {
		return snapshotPayload{}, err
	}
	accessRules, err := state.CountAccessRules()
	if err != nil {
		return snapshotPayload{}, err
	}

	sourceCounts := countSources()
	indexStats := collectIndexStats()

	commitSHA := strings.ToLower(strings.TrimSpace(version.CommitSHA))
	if commitSHA == "untracked" {
		commitSHA = ""
	}

	return snapshotPayload{
		App: appPayload{
			Version:   versionLabel,
			CommitSHA: commitSHA,
		},
		System: systemPayload{
			OS:            mapSystemOS(runtime.GOOS),
			Arch:          mapSystemArch(runtime.GOARCH),
			CPUCores:      runtime.NumCPU(),
			MemoryRSSMB:   memoryRSSMB(),
			MemoryTotalMB: systemTotalMemoryMB(),
			Runtime:       detectRuntime(),
		},
		Counts: countsPayload{
			SourcesEnabled:   sourceCounts.enabled,
			SourcesReadOnly:  sourceCounts.readOnly,
			SourcesPrivate:   sourceCounts.private,
			UsersTotal:       len(users),
			AccessRulesTotal: accessRules,
			SharesActive:     len(shares),
			CustomThemes:     len(settings.Config.Frontend.Styling.CustomThemes),
		},
		Auth: authPayload{
			NoAuth:              settings.Config.Auth.Methods.NoAuth,
			PasswordEnabled:     settings.Config.Auth.Methods.PasswordAuth.Enabled,
			PasswordSignup:      settings.Config.Auth.Methods.PasswordAuth.Signup,
			PasswordEnforcedOTP: settings.Config.Auth.Methods.PasswordAuth.EnforcedOtp,
			OidcEnabled:         settings.Config.Auth.Methods.OidcAuth.Enabled,
			LdapEnabled:         settings.Config.Auth.Methods.LdapAuth.Enabled,
			JwtEnabled:          settings.Config.Auth.Methods.JwtAuth.Enabled,
			ProxyEnabled:        settings.Config.Auth.Methods.ProxyAuth.Enabled,
			PasskeyEnabled:      settings.Config.Auth.Methods.PasskeyAuth.Enabled,
		},
		Integrations: integrationsPayload{
			OnlyOfficeConfigured: strings.TrimSpace(settings.Config.Integrations.OnlyOffice.Url) != "",
			OnlyOfficeViewOnly:   settings.Config.Integrations.OnlyOffice.ViewOnly,
			MediaEnabled:         settings.MediaEnabled(),
			HardwareAcceleration: settings.Config.Integrations.Media.HardwareAcceleration,
		},
		Features: featuresPayload{
			DisablePreviews:        settings.Config.Server.DisablePreviews,
			DisableWebDAV:          settings.Config.Server.DisableWebDAV,
			DisableUpdateCheck:     settings.Config.Server.DisableUpdateCheck,
			ActivityLoggingEnabled: !settings.Config.Server.DatabaseV2.Activity.Disabled,
			TLSEnabled:             strings.TrimSpace(settings.Config.Server.TLSCert) != "" && strings.TrimSpace(settings.Config.Server.TLSKey) != "",
			RateLimitDisabled:      settings.Config.Http.DisableRateLimit,
			IndexWALMode:           settings.Config.Server.IndexSqlConfig.WalMode,
			CacheDirCleanup:        settings.Config.Server.CacheDirCleanup,
		},
		Indexing: indexStats,
		Storage: storagePayload{
			MainDBBytes:  fileSize(settings.Config.Server.DatabaseV2.Path),
			IndexDBBytes: fileSize(filepath.Join(settings.Config.Server.CacheDir, "sql", "index_all.db")),
		},
	}, nil
}

func snapshotVersion(publishable bool) (string, error) {
	versionLabel := strings.TrimSpace(version.Version)
	if versionLabel == "" {
		return "", fmt.Errorf("version not available")
	}
	if publishable && (versionLabel == "untracked" || versionLabel == "testing") {
		return "", fmt.Errorf("version not reportable")
	}
	return versionLabel, nil
}

type sourceCountSummary struct {
	enabled  int
	readOnly int
	private  int
}

func countSources() sourceCountSummary {
	summary := sourceCountSummary{}
	for _, src := range settings.Config.Server.Sources {
		if src == nil {
			continue
		}
		if !src.Config.Disabled {
			summary.enabled++
		}
		if src.Config.ReadOnly {
			summary.readOnly++
		}
		if src.Config.Private {
			summary.private++
		}
	}
	return summary
}

func collectIndexStats() indexingPayload {
	stats := indexingPayload{}
	var quickTotal int
	var fullTotal int

	for _, src := range settings.Config.Server.Sources {
		if src == nil || src.Config.Disabled {
			continue
		}
		info, err := indexing.GetIndexInfo(src.Name, false)
		if err != nil {
			continue
		}
		stats.SourcesIndexed++
		stats.TotalFiles += int(info.NumFiles)
		stats.TotalDirs += int(info.NumDirs)
		quickTotal += info.QuickScanTime
		fullTotal += info.FullScanTime
	}

	if stats.SourcesIndexed > 0 {
		stats.AvgQuickScanSeconds = quickTotal / stats.SourcesIndexed
		stats.AvgFullScanSeconds = fullTotal / stats.SourcesIndexed
	}
	return stats
}

func detectRuntime() string {
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return "kubernetes"
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return "docker"
	}
	return "native"
}

func mapSystemOS(goos string) string {
	switch goos {
	case "linux", "windows", "darwin":
		return goos
	default:
		return "other"
	}
}

func mapSystemArch(goarch string) string {
	switch goarch {
	case "amd64", "arm64", "386", "arm":
		return goarch
	default:
		return "other"
	}
}

func memoryRSSMB() int {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return int(mem.Sys / (1024 * 1024))
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}
