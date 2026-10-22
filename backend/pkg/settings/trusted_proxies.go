package settings

import (
	"net"
	"net/netip"
	"strings"
	"sync"
)

var (
	trustedProxiesMu    sync.Mutex
	trustedProxiesKey   string
	trustedProxiesCache []netip.Prefix
)

// TrustedProxyPrefixes parses Config.Http.TrustedProxies into CIDR prefixes,
// caching the result until the configured list changes. Entries may be CIDR
// ranges or bare IPs; unparsable entries are skipped (ValidateHttpConfig
// rejects them at startup).
func TrustedProxyPrefixes() []netip.Prefix {
	key := strings.Join(Config.Http.TrustedProxies, "\x1e")
	trustedProxiesMu.Lock()
	defer trustedProxiesMu.Unlock()
	if key == trustedProxiesKey {
		return trustedProxiesCache
	}
	var prefixes []netip.Prefix
	for _, entry := range Config.Http.TrustedProxies {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if prefix, err := netip.ParsePrefix(entry); err == nil {
			prefixes = append(prefixes, prefix.Masked())
			continue
		}
		if addr, err := netip.ParseAddr(entry); err == nil {
			prefixes = append(prefixes, netip.PrefixFrom(addr, addr.BitLen()))
		}
	}
	trustedProxiesKey = key
	trustedProxiesCache = prefixes
	return prefixes
}

// ipMatchesPrefixes reports whether ip falls inside any of the given prefixes.
// Entries that are not valid IPs (optionally with a port) never match.
func ipMatchesPrefixes(ip string, prefixes []netip.Prefix) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		if host, _, splitErr := net.SplitHostPort(ip); splitErr == nil {
			addr, err = netip.ParseAddr(host)
		}
	}
	if err != nil {
		return false
	}
	for _, prefix := range prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// TrustedProxyClientIP resolves the originating client IP from an
// X-Forwarded-For header value.
//
// When http.trustedProxies is empty, the rightmost entry is returned: that is
// the peer recorded by the single trusted reverse proxy, which is correct for
// a one-proxy deployment and ignores spoofed client-supplied XFF entries.
//
// When trustedProxies is configured, the chain is walked right-to-left past
// every entry that belongs to a trusted proxy; the first untrusted entry is
// the client. This keeps client identity correct in multi-hop deployments
// (e.g. CDN -> reverse proxy -> filebrowser), where the rightmost entry would
// otherwise be the edge proxy shared by all clients.
func TrustedProxyClientIP(xff string) string {
	parts := strings.Split(xff, ",")
	last := -1
	for i := len(parts) - 1; i >= 0; i-- {
		if strings.TrimSpace(parts[i]) != "" {
			last = i
			break
		}
	}
	if last < 0 {
		return ""
	}
	prefixes := TrustedProxyPrefixes()
	if len(prefixes) == 0 {
		return strings.TrimSpace(parts[last])
	}
	for i := last; i >= 0; i-- {
		ip := strings.TrimSpace(parts[i])
		if ip == "" {
			continue
		}
		if !ipMatchesPrefixes(ip, prefixes) {
			return ip
		}
	}
	// Every entry is a trusted proxy; the leftmost is closest to the client.
	for _, part := range parts {
		if ip := strings.TrimSpace(part); ip != "" {
			return ip
		}
	}
	return ""
}
