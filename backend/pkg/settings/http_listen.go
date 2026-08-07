package settings

import (
	"fmt"
	"net"
	"strconv"
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
	return nil
}
