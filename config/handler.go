package config

import (
	"TheOnlyMirror/cert"
	"strconv"
)

func GetPort() string {
	return strconv.Itoa(config.Port)
}

func GetSources() map[string]Source {
	return config.Sources
}

func GetTls() bool {
	return config.Tls
}

func GetCert() (string, string) {
	if config.Crt != "" && config.Key != "" {
		return config.Crt, config.Key
	}
	cert.Generator_key()
	return "data/certificate.crt", "data/private.key"
}
