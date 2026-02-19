package main

import (
	"TheOnlyMirror/config"
	"TheOnlyMirror/plugins"
	"fmt"
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
	return path == "" || path == target || strings.HasPrefix(target, endsWithSlash(path))
}

func inUA(ua string, target string) bool {
	if ua == "" {
		return true
	}
	return strings.HasPrefix(strings.ToLower(target), strings.ToLower(ua))
}

func parseProxyTarget(r *http.Request) (*url.URL, error) {
	target := strings.TrimPrefix(r.URL.Path, "/")
	if target == "" {
		return nil, fmt.Errorf("empty proxy target")
	}

	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		target = "https://" + target
	}

	targetURL, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	if targetURL.Host == "" {
		return nil, fmt.Errorf("proxy target host is empty")
	}

	// Preserve user query when original target does not provide one.
	if targetURL.RawQuery == "" && r.URL.RawQuery != "" {
		targetURL.RawQuery = r.URL.RawQuery
	}

	return targetURL, nil
}

func extractAlias(host string) (string, bool) {
	if !strings.Contains(host, ".") {
		return "", false
	}
	idx := strings.Index(host, ".")
	if idx <= 0 {
		return "", false
	}
	return host[:idx], true
}

func matchHostAlias(r *http.Request) (*url.URL, string, bool) {
	host := config.NormalizeHost(r.Host)
	if host == "" {
		return nil, "", false
	}

	alias, ok := extractAlias(host)
	if !ok {
		return nil, "", false
	}

	targetBase, ok := config.GetHostAliasTarget(alias)
	if !ok {
		return nil, "", false
	}

	targetURL := *targetBase
	targetURL.Path = r.URL.Path
	targetURL.RawPath = r.URL.RawPath
	targetURL.RawQuery = r.URL.RawQuery
	if targetURL.Path == "" {
		targetURL.Path = "/"
	}

	return &targetURL, host, true
}

func matchSource(path string, ua string) (*config.Source, string, bool) {
	sources := config.GetSources()
	for _, s := range sources {
		if inPath(s.Sources.Path, path) && inUA(s.Sources.UA, ua) {
			return &s.Sources, s.Key, true
		}
	}
	return nil, "", false
}

func handler(w http.ResponseWriter, r *http.Request) {
	host := config.NormalizeHost(r.Host)
	ua := r.UserAgent()
	path := r.URL.Path
	log.Println(host, ua, path)

	if strings.HasPrefix(path, "/api/") {
		handleAPI(w, r)
		return
	}
	if serveFrontend(w, r) {
		return
	}

	if targetURL, aliasHost, ok := matchHostAlias(r); ok {
		log.Println("Match host alias " + aliasHost)
		plugins.HandlerProxy(w, r, targetURL, aliasHost)
		return
	}

	if source, sourceKey, ok := matchSource(path, ua); ok {
		log.Println("Match source " + sourceKey)
		plugins.HandlerReverse(w, r, *source)
		return
	}

	proxyHost := config.GetProxyHost()
	targetURL, err := parseProxyTarget(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	for _, proxy := range proxyHost {
		if config.NormalizeHost(proxy.Host) == config.NormalizeHost(targetURL.Host) {
			log.Println("Match proxy " + proxy.Host)
			targetURL.Scheme = proxy.Scheme
			targetURL.Host = proxy.Host
			plugins.HandlerProxy(w, r, targetURL, "")
			return
		}
	}
	http.NotFound(w, r)
}
