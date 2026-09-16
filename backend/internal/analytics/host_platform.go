package analytics

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	hostPlatformUnknown      = "unknown"
	hostPlatformGenericLinux = "generic-linux"
	hostPlatformUnraid       = "unraid"
	hostPlatformTrueNAS      = "truenas"
	hostPlatformSynology     = "synology"
	hostPlatformProxmox      = "proxmox"
	hostPlatformWindows      = "windows"
	hostPlatformMacOS        = "macos"
)

var hostPlatformEnv = "FILEBROWSER_HOST_PLATFORM"

// Roots where a Docker image may mount the host filesystem.
var hostRootPrefixes = []string{"/host", "/rootfs", "/mnt/host", ""}

// nativeHostRootPrefix is the scan root for non-container host detection. Tests may override.
var nativeHostRootPrefix = ""

func detectHostPlatform(containerRuntime string) string {
	if platform := normalizeHostPlatform(os.Getenv(hostPlatformEnv)); platform != "" {
		return platform
	}

	switch runtime.GOOS {
	case "windows":
		return hostPlatformWindows
	case "darwin":
		return hostPlatformMacOS
	case "linux":
		return detectLinuxHostPlatform(containerRuntime)
	case "freebsd":
		return detectFreeBSDHostPlatform(containerRuntime)
	default:
		return hostPlatformUnknown
	}
}

func hostScanPrefixes(containerRuntime string) ([]string, bool) {
	inContainer := containerRuntime == "docker" || containerRuntime == "kubernetes"
	if !inContainer {
		return []string{nativeHostRootPrefix}, false
	}
	return hostRootPrefixes, true
}

func detectLinuxHostPlatform(containerRuntime string) string {
	prefixes, inContainer := hostScanPrefixes(containerRuntime)

	for _, prefix := range prefixes {
		if platform := detectLinuxMarkers(prefix); platform != "" {
			return platform
		}
	}

	for _, prefix := range prefixes {
		if platform := classifyOSRelease(readOSRelease(prefix)); platform != "" {
			if platform != hostPlatformGenericLinux || !inContainer || prefix != "" {
				return platform
			}
		}
	}

	if inContainer {
		return hostPlatformUnknown
	}

	if classifyOSRelease(readOSRelease(nativeHostRootPrefix)) == hostPlatformGenericLinux {
		return hostPlatformGenericLinux
	}

	return hostPlatformUnknown
}

func detectFreeBSDHostPlatform(containerRuntime string) string {
	prefixes, inContainer := hostScanPrefixes(containerRuntime)

	for _, prefix := range prefixes {
		if platform := classifyOSRelease(readOSRelease(prefix)); platform != "" {
			if platform != hostPlatformGenericLinux {
				return platform
			}
		}
		if platform := classifyFreeBSDVersion(readFile(hostPath(prefix, "etc/version"))); platform != "" {
			return platform
		}
	}

	if inContainer {
		return hostPlatformUnknown
	}

	return hostPlatformUnknown
}

func classifyFreeBSDVersion(content string) string {
	lower := strings.ToLower(strings.TrimSpace(content))
	switch {
	case strings.Contains(lower, "truenas"), strings.Contains(lower, "freenas"):
		return hostPlatformTrueNAS
	default:
		return ""
	}
}

func detectLinuxMarkers(rootPrefix string) string {
	markers := []struct {
		path     string
		platform string
	}{
		{"etc/unraid-version", hostPlatformUnraid},
		{"etc/synoinfo.conf", hostPlatformSynology},
		{"etc/pve/.version", hostPlatformProxmox},
	}

	for _, marker := range markers {
		if fileExists(hostPath(rootPrefix, marker.path)) {
			return marker.platform
		}
	}
	return ""
}

func readOSRelease(rootPrefix string) string {
	return readFile(hostPath(rootPrefix, "etc/os-release"))
}

func hostPath(rootPrefix, rel string) string {
	if rootPrefix == "" {
		return filepath.Join(string(os.PathSeparator), rel)
	}
	return filepath.Join(rootPrefix, rel)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func parseOSRelease(content string) map[string]string {
	values := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"`)
	}
	return values
}

func classifyOSRelease(content string) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}

	vars := parseOSRelease(content)
	id := strings.ToLower(vars["ID"])
	idLike := strings.ToLower(vars["ID_LIKE"])
	pretty := strings.ToLower(vars["PRETTY_NAME"])
	name := strings.ToLower(vars["NAME"])
	combined := strings.Join([]string{id, idLike, pretty, name}, " ")

	switch {
	case strings.Contains(combined, "unraid"):
		return hostPlatformUnraid
	case strings.Contains(combined, "truenas"), strings.Contains(combined, "freenas"):
		return hostPlatformTrueNAS
	case strings.Contains(combined, "synology"):
		return hostPlatformSynology
	case strings.Contains(combined, "proxmox"):
		return hostPlatformProxmox
	case id != "":
		return hostPlatformGenericLinux
	default:
		return ""
	}
}

func normalizeHostPlatform(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "":
		return ""
	case "mac", "macos", "darwin", "osx":
		return hostPlatformMacOS
	case "win", "windows":
		return hostPlatformWindows
	case "linux", "generic-linux", "generic_linux":
		return hostPlatformGenericLinux
	case hostPlatformUnknown, hostPlatformUnraid, hostPlatformTrueNAS, hostPlatformSynology, hostPlatformProxmox:
		return value
	default:
		return value
	}
}
