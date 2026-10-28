package settings

import (
	"fmt"
	"net"
	"net/netip"
	"strings"
	"sync"
)

const ipv4MappedPrefixBits = 96

// normalizeTrustedProxyPrefix converts IPv4-mapped IPv6 prefixes to IPv4 and
// masks the result. Mapped prefixes must include the full ::ffff:0:0/96 anchor.
func normalizeTrustedProxyPrefix(prefix netip.Prefix) (netip.Prefix, error) {
	if prefix.Addr().Is4In6() {
		if prefix.Bits() < ipv4MappedPrefixBits {
			return netip.Prefix{}, fmt.Errorf("IPv4-mapped prefix must be at least /%d", ipv4MappedPrefixBits)
		}
		v4Bits := prefix.Bits() - ipv4MappedPrefixBits
		if v4Bits > 32 {
			v4Bits = 32
		}
		return netip.PrefixFrom(prefix.Addr().Unmap(), v4Bits).Masked(), nil
	}
	return prefix.Masked(), nil
}

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
			normalized, normErr := normalizeTrustedProxyPrefix(prefix)
			if normErr != nil {
				continue
			}
			prefixes = append(prefixes, normalized)
			continue
		}
		if addr, err := netip.ParseAddr(entry); err == nil {
			if addr.Is4In6() {
				addr = addr.Unmap()
			}
			prefixes = append(prefixes, netip.PrefixFrom(addr, addr.BitLen()))
		}
	}
	trustedProxiesKey = key
	trustedProxiesCache = prefixes
	return prefixes
}

// parseHopAddr parses an IP (optionally with a port, as in host:port) into a
// canonical address. IPv4-mapped IPv6 addresses are unmapped to plain IPv4 so
// they compare correctly against IPv4 prefixes.
func parseHopAddr(hop string) (netip.Addr, bool) {
	hop = strings.TrimSpace(hop)
	addr, err := netip.ParseAddr(hop)
	if err != nil {
		host, _, splitErr := net.SplitHostPort(hop)
		if splitErr != nil {
			return netip.Addr{}, false
		}
		addr, err = netip.ParseAddr(host)
		if err != nil {
			return netip.Addr{}, false
		}
	}
	return addr.Unmap(), true
}

// addrMatchesPrefixes reports whether addr falls inside any of the prefixes.
func addrMatchesPrefixes(addr netip.Addr, prefixes []netip.Prefix) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// ipMatchesPrefixes reports whether ip falls inside any of the given prefixes.
// Entries that are not valid IPs (optionally with a port) never match.
func ipMatchesPrefixes(ip string, prefixes []netip.Prefix) bool {
	addr, ok := parseHopAddr(ip)
	if !ok {
		return false
	}
	return addrMatchesPrefixes(addr, prefixes)
}

// IsTrustedProxyPeer reports whether a socket peer address (RemoteAddr, either
// host:port or a bare IP) belongs to a configured trusted proxy. Forwarded
// client headers must only be honored when this returns true; otherwise any
// client that can reach the listener directly could spoof its address.
func IsTrustedProxyPeer(remoteAddr string) bool {
	return ipMatchesPrefixes(remoteAddr, TrustedProxyPrefixes())
}

// canonicalHopIP returns the bare, unmapped IP for a forwarded hop, dropping
// any source port. It returns "" when the hop is not a valid IP.
func canonicalHopIP(hop string) string {
	addr, ok := parseHopAddr(hop)
	if !ok {
		return ""
	}
	return addr.String()
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
		return canonicalHopIP(parts[last])
	}
	for i := last; i >= 0; i-- {
		ip := strings.TrimSpace(parts[i])
		if ip == "" {
			continue
		}
		addr, ok := parseHopAddr(ip)
		if !ok {
			return ""
		}
		if !addrMatchesPrefixes(addr, prefixes) {
			return addr.String()
		}
	}
	// Every entry is a trusted proxy; the leftmost is closest to the client.
	for _, part := range parts {
		if ip := strings.TrimSpace(part); ip != "" {
			return canonicalHopIP(ip)
		}
	}
	return ""
}
