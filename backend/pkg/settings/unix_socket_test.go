package settings

import "testing"

func TestParseSocketFileModeDefaults(t *testing.T) {
	mode, err := parseSocketFileMode("")
	if err != nil {
		t.Fatal(err)
	}
	if mode != 0o660 {
		t.Fatalf("got %o, want 0660", mode)
	}
}

func TestParseSocketFileModeCustom(t *testing.T) {
	mode, err := parseSocketFileMode("0600")
	if err != nil {
		t.Fatal(err)
	}
	if mode != 0o600 {
		t.Fatalf("got %o, want 0600", mode)
	}
}

func TestValidateHttpConfigRejectsBadSocketFileMode(t *testing.T) {
	err := ValidateHttpConfig(Http{SocketFileMode: "not-octal"})
	if err == nil {
		t.Fatal("expected validation error for socketFileMode")
	}
}

func TestUnixPeerAllowed(t *testing.T) {
	Config.Http.UnixSocketTrustedUIDs = []int{10}
	Config.Http.UnixSocketTrustedGIDs = []int{20}
	t.Cleanup(func() {
		Config.Http.UnixSocketTrustedUIDs = nil
		Config.Http.UnixSocketTrustedGIDs = nil
	})
	if !unixPeerAllowed(10, 999) {
		t.Fatal("expected UID match")
	}
	if !unixPeerAllowed(999, 20) {
		t.Fatal("expected GID match")
	}
	if unixPeerAllowed(1, 1) {
		t.Fatal("expected no match")
	}
	Config.Http.UnixSocketTrustedUIDs = nil
	Config.Http.UnixSocketTrustedGIDs = nil
	if unixPeerAllowed(0, 0) {
		t.Fatal("empty allowlists must deny")
	}
}
