package settings

import (
	"net/netip"
	"testing"
)

func resetTrustedProxyCache() {
	trustedProxiesMu.Lock()
	trustedProxiesKey = ""
	trustedProxiesCache = nil
	trustedProxiesMu.Unlock()
}

func TestNormalizeTrustedProxyPrefixIPv4Mapped(t *testing.T) {
	prefix, err := netip.ParsePrefix("::ffff:198.51.100.0/120")
	if err != nil {
		t.Fatal(err)
	}
	got, err := normalizeTrustedProxyPrefix(prefix)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if got.Addr().String() != "198.51.100.0" || got.Bits() != 24 {
		t.Fatalf("got %v, want 198.51.100.0/24", got)
	}
}

func TestNormalizeTrustedProxyPrefixRejectsShortMapped(t *testing.T) {
	prefix, err := netip.ParsePrefix("::ffff:198.51.100.0/80")
	if err != nil {
		t.Fatal(err)
	}
	_, err = normalizeTrustedProxyPrefix(prefix)
	if err == nil {
		t.Fatal("expected error for mapped prefix shorter than /96")
	}
}

func TestTrustedProxyPrefixesMatchesPlainIPv4Client(t *testing.T) {
	resetTrustedProxyCache()
	Config.Http.TrustedProxies = []string{"::ffff:198.51.100.0/120"}
	t.Cleanup(func() {
		Config.Http.TrustedProxies = nil
		resetTrustedProxyCache()
	})

	prefixes := TrustedProxyPrefixes()
	if len(prefixes) != 1 {
		t.Fatalf("prefixes len = %d, want 1", len(prefixes))
	}
	if !ipMatchesPrefixes("198.51.100.7", prefixes) {
		t.Fatal("plain IPv4 client should match normalized mapped prefix")
	}
}

func TestTrustedProxyClientIPRejectsMalformedHop(t *testing.T) {
	resetTrustedProxyCache()
	Config.Http.TrustedProxies = []string{"198.51.100.7"}
	t.Cleanup(func() {
		Config.Http.TrustedProxies = nil
		resetTrustedProxyCache()
	})

	got := TrustedProxyClientIP("203.0.113.50, not-an-ip, 198.51.100.7")
	if got != "" {
		t.Fatalf("malformed hop: got %q, want empty string", got)
	}
}

func TestIPMatchesPrefixesUnmapsMappedHop(t *testing.T) {
	resetTrustedProxyCache()
	Config.Http.TrustedProxies = []string{"198.51.100.0/24"}
	t.Cleanup(func() {
		Config.Http.TrustedProxies = nil
		resetTrustedProxyCache()
	})

	if !ipMatchesPrefixes("::ffff:198.51.100.7", TrustedProxyPrefixes()) {
		t.Fatal("IPv4-mapped hop should match plain IPv4 prefix")
	}
}

func TestTrustedProxyClientIPStripsHopPorts(t *testing.T) {
	resetTrustedProxyCache()
	Config.Http.TrustedProxies = []string{"198.51.100.0/24"}
	t.Cleanup(func() {
		Config.Http.TrustedProxies = nil
		resetTrustedProxyCache()
	})

	got := TrustedProxyClientIP("203.0.113.50:40000, 198.51.100.7:5555")
	if got != "203.0.113.50" {
		t.Fatalf("got %q, want 203.0.113.50 without port", got)
	}
}

func TestTrustedProxyClientIPCanonicalizesWithoutPrefixes(t *testing.T) {
	resetTrustedProxyCache()
	Config.Http.TrustedProxies = nil
	t.Cleanup(resetTrustedProxyCache)

	if got := TrustedProxyClientIP("203.0.113.50, 198.51.100.7:5555"); got != "198.51.100.7" {
		t.Fatalf("got %q, want 198.51.100.7", got)
	}
}

func TestIsTrustedProxyPeer(t *testing.T) {
	resetTrustedProxyCache()
	Config.Http.TrustedProxies = nil
	t.Cleanup(func() {
		Config.Http.TrustedProxies = nil
		resetTrustedProxyCache()
	})

	if IsTrustedProxyPeer("10.0.0.1:1234") {
		t.Fatal("no trusted proxies configured: peer must not be trusted")
	}

	Config.Http.TrustedProxies = []string{"10.0.0.0/8"}
	resetTrustedProxyCache()
	if !IsTrustedProxyPeer("10.0.0.1:1234") {
		t.Fatal("peer in configured range should be trusted")
	}
	if !IsTrustedProxyPeer("10.0.0.1") {
		t.Fatal("bare IP peer in configured range should be trusted")
	}
	if IsTrustedProxyPeer("192.0.2.1:1234") {
		t.Fatal("peer outside configured range must not be trusted")
	}
}

// With http.socket set the only listener is a Unix socket, whose peers have no IP.
func TestIsTrustedProxyPeerUnixSocket(t *testing.T) {
	resetTrustedProxyCache()
	Config.Http.TrustedProxies = nil
	Config.Http.Socket = "/tmp/filebrowser.sock"
	t.Cleanup(func() {
		Config.Http.Socket = ""
		resetTrustedProxyCache()
	})
	for _, peer := range []string{"", "@"} {
		if !IsTrustedProxyPeer(peer) {
			t.Fatalf("unix socket peer %q must be trusted", peer)
		}
	}
	if got := TrustedProxyClientIP("203.0.113.50"); got != "203.0.113.50" {
		t.Fatalf("got %q, want 203.0.113.50", got)
	}
}

func TestValidateHttpConfigRejectsShortMappedPrefix(t *testing.T) {
	err := ValidateHttpConfig(Http{TrustedProxies: []string{"::ffff:198.51.100.0/80"}})
	if err == nil {
		t.Fatal("expected validation error for short IPv4-mapped prefix")
	}
}
