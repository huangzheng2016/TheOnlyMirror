package config

import (
	"TheOnlyMirror/cert"
	"net"
	"net/url"
	"strconv"
	"strings"
)

func GetPort() string {
	return strconv.Itoa(config.Port)
}

func GetSources() []SourceSlice {
	return SourceSlices
}

func GetProxyHost() []*url.URL {
	return proxyHost
}

func NormalizeHost(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "[") {
		host, _, err := net.SplitHostPort(raw)
		if err == nil {
			return strings.Trim(host, "[]")
		}
		return strings.Trim(raw, "[]")
	}
	host, _, err := net.SplitHostPort(raw)
	if err == nil {
		return host
	}
	return raw
}

func GetHostAliasTarget(alias string) (*url.URL, bool) {
	targetURL, ok := hostAliasTargets[strings.ToLower(alias)]
	return targetURL, ok
}

func GetHostAliases() map[string]string {
	aliases := make(map[string]string, len(hostAliasTargets))
	for alias, targetURL := range hostAliasTargets {
		aliases[alias] = NormalizeHost(targetURL.Host)
	}
	return aliases
}

func GetAliasByUpstream(host string) (string, bool) {
	alias, ok := upstreamAliases[NormalizeHost(host)]
	return alias, ok
}

func GetTls() bool {
	return config.Tls
}
func GetTlsRedirect() bool {
	return config.TlsRedirect
}

func GetCert() (string, string) {
	if config.Crt != "" && config.Key != "" {
		return config.Crt, config.Key
	}
	cert.Generator_key()
	return "data/certificate.crt", "data/private.key"
}
