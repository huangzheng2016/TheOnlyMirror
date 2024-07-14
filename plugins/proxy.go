package plugins

import (
	"golang.org/x/net/http2"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func HandlerProxy(w http.ResponseWriter, r *http.Request, targetUrl *url.URL) {
	proxy := httputil.NewSingleHostReverseProxy(targetUrl)
	if r.ProtoMajor == 2 {
		proxy.Transport = &http2.Transport{}
	}
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = targetUrl.Scheme
		req.URL.Host = targetUrl.Host
		req.URL.Path = targetUrl.Path
		req.Host = targetUrl.Host
	}
	proxy.ServeHTTP(w, r)
}
