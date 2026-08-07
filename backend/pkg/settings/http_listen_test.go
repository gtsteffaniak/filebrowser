package settings

import (
	"net"
	"testing"
)

func TestHTTPListenAddr(t *testing.T) {
	const testPort = 38473
	tests := []struct {
		name   string
		listen string
		want   string
	}{
		{name: "ipv4 wildcard", listen: "0.0.0.0", want: "0.0.0.0:38473"},
		{name: "ipv4 loopback", listen: "127.0.0.1", want: "127.0.0.1:38473"},
		{name: "ipv6 loopback", listen: "::1", want: "[::1]:38473"},
		{name: "ipv6 unspecified", listen: "::", want: "[::]:38473"},
		{name: "empty listen defaults", listen: "", want: "0.0.0.0:38473"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := HTTPListenAddr(tc.listen, testPort)
			if got != tc.want {
				t.Fatalf("HTTPListenAddr(%q, %d) = %q, want %q", tc.listen, testPort, got, tc.want)
			}
			ln, err := net.Listen("tcp", got)
			if err != nil {
				t.Fatalf("net.Listen(%q): %v", got, err)
			}
			_ = ln.Close()
		})
	}
}

func TestValidateHttpConfig_rejectsSocketWithTLS(t *testing.T) {
	err := ValidateHttpConfig(Http{
		Socket:  "/var/run/filebrowser.sock",
		TLSCert: "/etc/ssl/cert.pem",
	})
	if err == nil {
		t.Fatal("expected error when socket is combined with tlsCert")
	}

	err = ValidateHttpConfig(Http{
		Socket: "/var/run/filebrowser.sock",
		TLSKey: "/etc/ssl/key.pem",
	})
	if err == nil {
		t.Fatal("expected error when socket is combined with tlsKey")
	}

	if err := ValidateHttpConfig(Http{Socket: "/var/run/filebrowser.sock"}); err != nil {
		t.Fatalf("socket-only config should be valid: %v", err)
	}
	if err := ValidateHttpConfig(Http{TLSCert: "cert.pem", TLSKey: "key.pem"}); err != nil {
		t.Fatalf("tls-only config should be valid: %v", err)
	}
}
