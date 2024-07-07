package plugins

import (
	"TheOnlyMirror/config"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func HandlerReverse(w http.ResponseWriter, r *http.Request, source config.Source) {
	for _, MirrorUrl := range source.Mirrors {
		targetUrl, _ := url.Parse(MirrorUrl)
		proxy := httputil.NewSingleHostReverseProxy(targetUrl)
		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.URL.Scheme = targetUrl.Scheme
			req.URL.Host = targetUrl.Host
			req.Host = targetUrl.Host
		}
		proxy.ServeHTTP(w, r)
		break
	}
}
