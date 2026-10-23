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

func TestValidateHttpConfigRejectsShortMappedPrefix(t *testing.T) {
	err := ValidateHttpConfig(Http{TrustedProxies: []string{"::ffff:198.51.100.0/80"}})
	if err == nil {
		t.Fatal("expected validation error for short IPv4-mapped prefix")
	}
}
