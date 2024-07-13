package config

import (
	"net/url"
	"sort"
)

type SourceSlice struct {
	Index   int
	Sources Source
}

func prepareConfig() {
	var sourceSlice []SourceSlice
	i := 0
	for _, source := range config.Sources {
		sourceSlice = append(sourceSlice, SourceSlice{Index: i, Sources: source})
		i++
	}
	sort.Slice(sourceSlice, func(i, j int) bool {
		return sourceSlice[i].Sources.Priority > sourceSlice[j].Sources.Priority
	})
	i = 0
	for key, _ := range config.Sources {
		for _, sourceS := range sourceSlice {
			if i == sourceS.Index {
				config.Sources[key] = sourceS.Sources
				break
			}
		}
		i++
	}
	for _, proxy := range config.Proxy {
		targetUrl, _ := url.Parse(proxy)
		proxyHost = append(proxyHost, targetUrl)
	}
}
