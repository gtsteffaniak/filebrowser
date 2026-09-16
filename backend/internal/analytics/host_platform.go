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

// Roots where a Docker image may mount the host filesystem.
var hostRootPrefixes = []string{"/host", "/rootfs", "/mnt/host", ""}

// nativeHostRootPrefix is the scan root for non-container host detection. Tests may override.
var nativeHostRootPrefix = ""

func detectHostPlatform(containerRuntime string) string {
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

const (
	deploymentRuntimeKubernetes = "kubernetes"
	deploymentRuntimeDocker       = "docker"
	deploymentRuntimeLinux        = "linux"
	deploymentRuntimeWindows      = "windows"
	deploymentRuntimeMacOS        = "macos"
	deploymentRuntimeUnknown      = "unknown"
)

// detectDeploymentRuntime reports how the instance is deployed: detected NAS/hypervisor
// product when possible, otherwise kubernetes, docker, or native OS (linux, windows, macos).
func detectDeploymentRuntime() string {
	return deploymentRuntimeFrom(detectRuntime())
}

func deploymentRuntimeFrom(containerRuntime string) string {
	platform := detectHostPlatform(containerRuntime)
	if isProductDeploymentRuntime(platform) {
		return platform
	}

	switch containerRuntime {
	case "kubernetes":
		return deploymentRuntimeKubernetes
	case "docker":
		return deploymentRuntimeDocker
	}

	switch runtime.GOOS {
	case "windows":
		return deploymentRuntimeWindows
	case "darwin":
		return deploymentRuntimeMacOS
	case "linux":
		return deploymentRuntimeLinux
	default:
		return deploymentRuntimeUnknown
	}
}

func isProductDeploymentRuntime(platform string) bool {
	switch platform {
	case hostPlatformUnraid, hostPlatformTrueNAS, hostPlatformSynology, hostPlatformProxmox,
		hostPlatformWindows, hostPlatformMacOS:
		return true
	default:
		return false
	}
}
