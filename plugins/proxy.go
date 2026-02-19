package plugins

import (
	"TheOnlyMirror/config"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

func renderLocationPath(locationURL *url.URL) string {
	location := locationURL.Path
	if !strings.HasPrefix(location, "/") {
		location = "/" + location
	}
	if locationURL.RawQuery != "" {
		location += "?" + locationURL.RawQuery
	}
	if locationURL.Fragment != "" {
		location += "#" + locationURL.Fragment
	}
	return location
}

func aliasHostFromRequestHost(requestHost string, alias string) string {
	if alias == "" {
		return requestHost
	}
	if idx := strings.Index(requestHost, "."); idx > 0 && idx+1 < len(requestHost) {
		return alias + requestHost[idx:]
	}
	return alias
}

func isRedirectStatus(code int) bool {
	switch code {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	default:
		return false
	}
}

func requestScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func isProxyUpstreamHost(host string) bool {
	for _, upstream := range config.GetProxyHost() {
		if config.NormalizeHost(upstream.Host) == host {
			return true
		}
	}
	return false
}

func rewriteLocation(location string, r *http.Request, targetURL *url.URL, aliasRequestHost string) (string, bool) {
	locationURL, err := url.Parse(location)
	if err != nil {
		return "", false
	}

	// Relative redirects should stay in current alias host mode, but must be prefixed in path-proxy mode.
	if !locationURL.IsAbs() {
		if aliasRequestHost != "" {
			return location, true
		}
		if targetURL == nil || targetURL.Host == "" {
			return "", false
		}
		locationPath := renderLocationPath(locationURL)
		rewrite := "/" + config.NormalizeHost(targetURL.Host) + "/" + strings.TrimPrefix(locationPath, "/")
		return rewrite, true
	}

	upstreamHost := config.NormalizeHost(locationURL.Host)
	if upstreamHost == "" {
		return "", false
	}
	if !isProxyUpstreamHost(upstreamHost) {
		return "", false
	}

	if aliasRequestHost != "" {
		redirectHost := aliasRequestHost
		if alias, ok := config.GetAliasByUpstream(upstreamHost); ok {
			redirectHost = aliasHostFromRequestHost(aliasRequestHost, alias)
		}
		redirectURL := &url.URL{
			Scheme:   requestScheme(r),
			Host:     redirectHost,
			Path:     locationURL.Path,
			RawQuery: locationURL.RawQuery,
			Fragment: locationURL.Fragment,
		}
		if redirectURL.Path == "" {
			redirectURL.Path = "/"
		}
		return redirectURL.String(), true
	}

	locationPath := renderLocationPath(locationURL)
	rewrite := "/" + upstreamHost + "/" + strings.TrimPrefix(locationPath, "/")
	return rewrite, true
}

func HandlerProxy(w http.ResponseWriter, r *http.Request, targetUrl *url.URL, aliasRequestHost string) {
	proxy := httputil.NewSingleHostReverseProxy(targetUrl)
	proxy.Transport = upstreamTransport()
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = targetUrl.Scheme
		req.URL.Host = targetUrl.Host
		req.URL.Path = targetUrl.Path
		if req.URL.Path == "" {
			req.URL.Path = "/"
		}
		req.URL.RawPath = targetUrl.RawPath
		req.URL.RawQuery = targetUrl.RawQuery
		req.Host = targetUrl.Host
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		if !isRedirectStatus(resp.StatusCode) {
			return nil
		}

		location := resp.Header.Get("Location")
		if location == "" {
			return nil
		}

		rewrite, ok := rewriteLocation(location, r, targetUrl, aliasRequestHost)
		if ok {
			resp.Header.Set("Location", rewrite)
		}
		return nil
	}
	proxy.ServeHTTP(w, r)
}
