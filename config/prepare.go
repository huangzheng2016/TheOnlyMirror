package config

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

type SourceSlice struct {
	Key     string
	Sources Source
}

var SourceSlices []SourceSlice

func validateUpstream(raw string, field string) (*url.URL, error) {
	targetURL, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%s parse failed: %w", field, err)
	}
	if targetURL.Host == "" {
		return nil, fmt.Errorf("%s host is empty", field)
	}
	if targetURL.Scheme != "http" && targetURL.Scheme != "https" {
		return nil, fmt.Errorf("%s scheme must be http/https: %s", field, targetURL.Scheme)
	}
	return targetURL, nil
}

func normalizePath(path string) string {
	if path == "" {
		return path
	}
	if strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}

func validateSourceConfig(key string, source Source) (Source, error) {
	if source.Template || len(source.Targets) > 0 || source.BaseName != "" {
		return source, fmt.Errorf("sources.%s contains template-only fields", key)
	}

	if strings.TrimSpace(source.Mirror) == "" {
		return source, fmt.Errorf("sources.%s.mirror is required", key)
	}
	if strings.TrimSpace(source.UA) == "" && strings.TrimSpace(source.Path) == "" {
		return source, fmt.Errorf("sources.%s requires at least one matcher: ua/path", key)
	}

	mirrorURL, err := validateUpstream(source.Mirror, "sources."+key+".mirror")
	if err != nil {
		return source, err
	}
	source.Mirror = mirrorURL.String()
	source.Path = normalizePath(source.Path)
	source.Prefix = normalizePath(source.Prefix)

	for index, replace := range source.Replaces {
		replaceType := strings.ToLower(strings.TrimSpace(replace.Type))
		if replaceType == "" {
			replaceType = "body"
		}
		if replaceType == "header" && strings.TrimSpace(replace.Header) == "" {
			return source, fmt.Errorf("sources.%s.replaces[%d].header is required when type=header", key, index)
		}
		if strings.TrimSpace(replace.Src) == "" {
			return source, fmt.Errorf("sources.%s.replaces[%d].src cannot be empty", key, index)
		}
		source.Replaces[index].Type = replaceType
	}

	return source, nil
}

func prepareConfig() error {
	SourceSlices = SourceSlices[:0]
	proxyHost = proxyHost[:0]
	hostAliasTargets = map[string]*url.URL{}
	upstreamAliases = map[string]string{}

	for key, source := range config.Sources {
		validated, err := validateSourceConfig(key, source)
		if err != nil {
			return err
		}
		SourceSlices = append(SourceSlices, SourceSlice{Key: key, Sources: validated})
	}
	sort.Slice(SourceSlices, func(i, j int) bool {
		return SourceSlices[i].Sources.Priority > SourceSlices[j].Sources.Priority
	})
	for index, proxy := range config.Proxy {
		targetURL, err := validateUpstream(proxy, fmt.Sprintf("proxy[%d]", index))
		if err != nil {
			return err
		}
		proxyHost = append(proxyHost, targetURL)
	}

	proxyLookup := map[string]*url.URL{}
	for _, proxy := range proxyHost {
		proxyLookup[NormalizeHost(proxy.Host)] = proxy
	}

	for alias, upstreamHostRaw := range config.HostAliases {
		alias = strings.ToLower(strings.TrimSpace(alias))
		if alias == "" || strings.Contains(alias, ".") {
			return fmt.Errorf("host_aliases key is invalid: %s", alias)
		}

		upstreamHost := NormalizeHost(upstreamHostRaw)
		if upstreamHost == "" {
			return fmt.Errorf("host_aliases.%s is empty", alias)
		}
		if strings.Contains(upstreamHost, "/") {
			return fmt.Errorf("host_aliases.%s must be host only, got: %s", alias, upstreamHostRaw)
		}

		proxyURL, ok := proxyLookup[upstreamHost]
		if !ok {
			return fmt.Errorf("host_aliases.%s target %s must exist in proxy whitelist", alias, upstreamHost)
		}

		targetURL := *proxyURL
		hostAliasTargets[alias] = &targetURL
		upstreamAliases[NormalizeHost(proxyURL.Host)] = alias
	}

	return nil
}
