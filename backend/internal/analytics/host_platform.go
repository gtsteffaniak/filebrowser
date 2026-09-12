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
	default:
		return hostPlatformUnknown
	}
}

func detectLinuxHostPlatform(containerRuntime string) string {
	inContainer := containerRuntime == "docker" || containerRuntime == "kubernetes"

	for _, prefix := range hostRootPrefixes {
		if platform := detectLinuxMarkers(prefix); platform != "" {
			return platform
		}
	}

	for _, prefix := range hostRootPrefixes {
		if platform := classifyOSRelease(readOSRelease(prefix)); platform != "" {
			if platform != hostPlatformGenericLinux || !inContainer || prefix != "" {
				return platform
			}
		}
	}

	if inContainer {
		return hostPlatformUnknown
	}

	if classifyOSRelease(readOSRelease("")) == hostPlatformGenericLinux {
		return hostPlatformGenericLinux
	}

	return hostPlatformUnknown
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
