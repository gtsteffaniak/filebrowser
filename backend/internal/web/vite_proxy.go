package web

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

const defaultViteDevUpstream = "http://127.0.0.1:5173"

var viteDevUpstream = defaultViteDevUpstream

func viteProxyHandler() http.Handler {
	target, err := url.Parse(viteDevUpstream)
	if err != nil {
		panic(err)
	}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.Header.Del("Cookie")
			pr.Out.Header.Del("Authorization")
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			http.Error(w, "Vite dev server is not running. Start local development with: make dev\n\n"+err.Error(), http.StatusBadGateway)
		},
	}
	return proxy
}
