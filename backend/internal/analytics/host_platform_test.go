package analytics

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClassifyOSRelease(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name: "unraid",
			content: `NAME="Unraid OS"
ID=unraid
ID_LIKE="debian"`,
			want: hostPlatformUnraid,
		},
		{
			name: "truenas scale",
			content: `NAME="TrueNAS-SCALE"
ID=truenas-scale
ID_LIKE=debian`,
			want: hostPlatformTrueNAS,
		},
		{
			name: "freenas",
			content: `PRETTY_NAME="FreeNAS-11.3-U5"`,
			want: hostPlatformTrueNAS,
		},
		{
			name: "synology",
			content: `NAME="Synology"
ID=synology`,
			want: hostPlatformSynology,
		},
		{
			name: "proxmox",
			content: `PRETTY_NAME="Debian GNU/Linux 12 (bookworm)"
ID=debian
NAME="Proxmox VE"`,
			want: hostPlatformProxmox,
		},
		{
			name: "generic ubuntu",
			content: `NAME="Ubuntu"
ID=ubuntu
ID_LIKE=debian`,
			want: hostPlatformGenericLinux,
		},
		{
			name:    "empty",
			content: "",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyOSRelease(tt.content); got != tt.want {
				t.Fatalf("classifyOSRelease() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeHostPlatform(t *testing.T) {
	tests := map[string]string{
		"":                "",
		"  unraid  ":      hostPlatformUnraid,
		"TrueNAS":         hostPlatformTrueNAS,
		"macOS":           hostPlatformMacOS,
		"generic_linux":   hostPlatformGenericLinux,
		"custom-platform": "custom-platform",
	}

	for input, want := range tests {
		if got := normalizeHostPlatform(input); got != want {
			t.Fatalf("normalizeHostPlatform(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestDetectHostPlatformEnvOverride(t *testing.T) {
	t.Setenv(hostPlatformEnv, "unraid")
	if got := detectHostPlatform("docker"); got != hostPlatformUnraid {
		t.Fatalf("detectHostPlatform() = %q, want %q", got, hostPlatformUnraid)
	}
}

func withHostDetectionRoots(t *testing.T, roots []string, nativeRoot string) {
	t.Cleanup(func() {
		hostRootPrefixes = []string{"/host", "/rootfs", "/mnt/host", ""}
		nativeHostRootPrefix = ""
	})
	hostRootPrefixes = roots
	nativeHostRootPrefix = nativeRoot
}

func writeFixtureFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func TestDetectLinuxHostPlatformMarkerPrecedence(t *testing.T) {
	root := t.TempDir()
	hostMount := filepath.Join(root, "host")
	writeFixtureFile(t, filepath.Join(hostMount, "etc", "unraid-version"), "6.12.0")
	writeFixtureFile(t, filepath.Join(hostMount, "etc", "os-release"), `NAME="Ubuntu"
ID=ubuntu`)

	withHostDetectionRoots(t, []string{hostMount}, "")

	if got := detectLinuxHostPlatform("docker"); got != hostPlatformUnraid {
		t.Fatalf("detectLinuxHostPlatform() = %q, want %q", got, hostPlatformUnraid)
	}
}

func TestDetectLinuxHostPlatformMountedOSRelease(t *testing.T) {
	root := t.TempDir()
	hostMount := filepath.Join(root, "host")
	writeFixtureFile(t, filepath.Join(hostMount, "etc", "os-release"), `NAME="Synology"
ID=synology`)

	withHostDetectionRoots(t, []string{hostMount}, "")

	if got := detectLinuxHostPlatform("kubernetes"); got != hostPlatformSynology {
		t.Fatalf("detectLinuxHostPlatform() = %q, want %q", got, hostPlatformSynology)
	}
}

func TestDetectLinuxHostPlatformContainerUnknownFallback(t *testing.T) {
	root := t.TempDir()
	withHostDetectionRoots(t, []string{filepath.Join(root, "host")}, "")

	if got := detectLinuxHostPlatform("docker"); got != hostPlatformUnknown {
		t.Fatalf("detectLinuxHostPlatform() = %q, want %q", got, hostPlatformUnknown)
	}
}

func TestDetectLinuxHostPlatformNativeSkipsHostMounts(t *testing.T) {
	root := t.TempDir()
	hostOnly := filepath.Join(root, "host")
	nativeRoot := filepath.Join(root, "native")
	writeFixtureFile(t, filepath.Join(hostOnly, "etc", "unraid-version"), "6.12.0")
	writeFixtureFile(t, filepath.Join(nativeRoot, "etc", "os-release"), `NAME="Ubuntu"
ID=ubuntu`)

	withHostDetectionRoots(t, []string{hostOnly}, nativeRoot)

	if got := detectLinuxHostPlatform("native"); got != hostPlatformGenericLinux {
		t.Fatalf("detectLinuxHostPlatform() = %q, want %q", got, hostPlatformGenericLinux)
	}
}

func TestDetectFreeBSDHostPlatformTrueNASVersion(t *testing.T) {
	root := t.TempDir()
	hostMount := filepath.Join(root, "host")
	writeFixtureFile(t, filepath.Join(hostMount, "etc", "version"), "TrueNAS-13.0-U6.1")

	withHostDetectionRoots(t, []string{hostMount}, "")

	if got := detectFreeBSDHostPlatform("docker"); got != hostPlatformTrueNAS {
		t.Fatalf("detectFreeBSDHostPlatform() = %q, want %q", got, hostPlatformTrueNAS)
	}
}

func TestClassifyFreeBSDVersion(t *testing.T) {
	tests := map[string]string{
		"TrueNAS-13.0-U6.1": hostPlatformTrueNAS,
		"FreeNAS-11.3-U5":   hostPlatformTrueNAS,
		"13.2-RELEASE":      "",
	}

	for input, want := range tests {
		if got := classifyFreeBSDVersion(input); got != want {
			t.Fatalf("classifyFreeBSDVersion(%q) = %q, want %q", input, got, want)
		}
	}
}
