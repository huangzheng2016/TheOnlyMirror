package config

import (
	"net/url"
	"sort"
)

type SourceSlice struct {
	Key     string
	Sources Source
}

func prepareConfig() {
	var sourceSlice []SourceSlice
	for key, source := range config.Sources {
		sourceSlice = append(sourceSlice, SourceSlice{Key: key, Sources: source})
	}
	sort.Slice(sourceSlice, func(i, j int) bool {
		return sourceSlice[i].Sources.Priority > sourceSlice[j].Sources.Priority
	})
	Sources := make(map[string]Source)
	for _, sourceS := range sourceSlice {
		Sources[sourceS.Key] = sourceS.Sources
	}
	config.Sources = Sources
	for _, proxy := range config.Proxy {
		targetUrl, _ := url.Parse(proxy)
		proxyHost = append(proxyHost, targetUrl)
	}
}
