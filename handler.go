package main

import (
	"TheOnlyMirror/config"
	"TheOnlyMirror/plugins"
	"log"
	"net/http"
	"net/url"
	"strings"
)

func endsWithSlash(path string) string {
	if !strings.HasSuffix(path, "/") {
		return path + "/"
	}
	return path
}

func inPath(path string, target string) bool {
	if path == "" || path == target || strings.HasPrefix(target, endsWithSlash(path)) {
		return true
	}
	return false
}

func inUA(ua string, target string) bool {
	if ua == "" {
		return true
	}
	return strings.HasPrefix(strings.ToLower(target), strings.ToLower(ua))
}

func handler(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	ua := r.UserAgent()
	path := r.URL.Path
	sources := config.GetSources()
	log.Println(host, ua, path)
	for _, s := range sources {
		if inPath(s.Sources.Path, path) && inUA(s.Sources.UA, ua) {
			log.Println("Match source " + s.Key)
			switch s.Sources.Type {
			default:
				plugins.HandlerReverse(w, r, s.Sources)
			}
			return
		}
	}
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimPrefix(path, "http:/")
	path = strings.TrimPrefix(path, "https:/")
	path = strings.TrimPrefix(path, "/")

	proxyHost := config.GetProxyHost()
	targetUrl, err := url.Parse("https://" + path)
	if err == nil {
		for _, proxy := range proxyHost {
			if proxy.Host == targetUrl.Host {
				log.Println("Match proxy " + proxy.Host)
				targetUrl.Scheme = proxy.Scheme
				plugins.HandlerProxy(w, r, targetUrl)
				return
			}
		}
	}
	http.NotFound(w, r)
}
