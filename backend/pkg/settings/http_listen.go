package settings

import (
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

// HTTPListenAddr returns a TCP listen address suitable for net.Listen.
// IPv6 addresses are bracketed via net.JoinHostPort (e.g. [::1]:8080).
func HTTPListenAddr(listen string, port int) string {
	if listen == "" {
		listen = "0.0.0.0"
	}
	return net.JoinHostPort(listen, strconv.Itoa(port))
}

// ValidateHttpConfig checks HTTP settings that would otherwise fatal during setup.
func ValidateHttpConfig(h Http) error {
	if h.Socket != "" && (h.TLSCert != "" || h.TLSKey != "") {
		return fmt.Errorf("http.socket cannot be used with tlsCert or tlsKey")
	}
	for _, entry := range h.TrustedProxies {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if _, err := netip.ParsePrefix(entry); err != nil {
			if _, addrErr := netip.ParseAddr(entry); addrErr != nil {
				return fmt.Errorf("http.trustedProxies entry %q is not a valid IP address or CIDR range", entry)
			}
		}
	}
	return nil
}
