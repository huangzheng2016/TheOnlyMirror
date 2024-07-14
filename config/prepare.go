package config

import (
	"net/url"
	"sort"
)

type SourceSlice struct {
	Key     string
	Sources Source
}

var SourceSlices []SourceSlice

func prepareConfig() {
	for key, source := range config.Sources {
		SourceSlices = append(SourceSlices, SourceSlice{Key: key, Sources: source})
	}
	sort.Slice(SourceSlices, func(i, j int) bool {
		return SourceSlices[i].Sources.Priority > SourceSlices[j].Sources.Priority
	})
	for _, proxy := range config.Proxy {
		targetUrl, _ := url.Parse(proxy)
		proxyHost = append(proxyHost, targetUrl)
	}
}
