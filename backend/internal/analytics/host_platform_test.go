package analytics

import "testing"

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
