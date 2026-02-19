package main

import (
	"TheOnlyMirror/config"
	"encoding/json"
	"net/http"
	"sort"
)

type apiSource struct {
	Key      string `json:"key"`
	Priority int    `json:"priority"`
	UA       string `json:"ua"`
	Path     string `json:"path"`
	Prefix   string `json:"prefix"`
	Mirror   string `json:"mirror"`
}

type apiProxy struct {
	Scheme string `json:"scheme"`
	Host   string `json:"host"`
}

type apiHostAlias struct {
	Alias    string `json:"alias"`
	Upstream string `json:"upstream"`
}

type apiServicesResponse struct {
	Sources     []apiSource    `json:"sources"`
	Proxy       []apiProxy     `json:"proxy"`
	HostAliases []apiHostAlias `json:"hostAliases"`
	RequestHost string         `json:"requestHost,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, "json encode failed", http.StatusInternalServerError)
	}
}

func buildServicesResponse(requestHost string) apiServicesResponse {
	sourcesConfig := config.GetSources()
	sources := make([]apiSource, 0, len(sourcesConfig))
	for _, source := range sourcesConfig {
		sources = append(sources, apiSource{
			Key:      source.Key,
			Priority: source.Sources.Priority,
			UA:       source.Sources.UA,
			Path:     source.Sources.Path,
			Prefix:   source.Sources.Prefix,
			Mirror:   source.Sources.Mirror,
		})
	}
	sort.Slice(sources, func(i, j int) bool {
		return sources[i].Key < sources[j].Key
	})

	proxyConfig := config.GetProxyHost()
	proxy := make([]apiProxy, 0, len(proxyConfig))
	for _, item := range proxyConfig {
		proxy = append(proxy, apiProxy{
			Scheme: item.Scheme,
			Host:   item.Host,
		})
	}
	sort.Slice(proxy, func(i, j int) bool {
		if proxy[i].Host == proxy[j].Host {
			return proxy[i].Scheme < proxy[j].Scheme
		}
		return proxy[i].Host < proxy[j].Host
	})

	aliasesConfig := config.GetHostAliases()
	aliases := make([]apiHostAlias, 0, len(aliasesConfig))
	for alias, upstream := range aliasesConfig {
		aliases = append(aliases, apiHostAlias{
			Alias:    alias,
			Upstream: upstream,
		})
	}
	sort.Slice(aliases, func(i, j int) bool {
		return aliases[i].Alias < aliases[j].Alias
	})

	return apiServicesResponse{
		Sources:     sources,
		Proxy:       proxy,
		HostAliases: aliases,
		RequestHost: requestHost,
	}
}

func handleAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	switch r.URL.Path {
	case "/api/services":
		writeJSON(w, http.StatusOK, buildServicesResponse(r.Host))
		return
	default:
		http.NotFound(w, r)
		return
	}
}
