package config

import (
	"TheOnlyMirror/cert"
	"net/url"
	"strconv"
)

func GetPort() string {
	return strconv.Itoa(config.Port)
}

func GetSources() map[string]Source {
	return config.Sources
}
func GetProxy() []string {
	return config.Proxy
}

func GetProxyHost() []*url.URL {
	return proxyHost
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
