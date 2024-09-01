package plugins

import (
	"TheOnlyMirror/config"
	"golang.org/x/net/http2"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
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
	proxy.ModifyResponse = func(resp *http.Response) error {
		proxyHost := config.GetProxyHost()
		location := resp.Header.Get("location")
		for _, proxy := range proxyHost {
			if strings.Contains(location, proxy.Host) {
				location = strings.Replace(location, proxy.Host, r.Host+"/"+proxy.Host, -1)
				break
			}
		}
		resp.Header.Set("location", location)
		return nil
	}
	proxy.ServeHTTP(w, r)
}
