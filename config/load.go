package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	Port            int               `json:"port"`
	Tls             bool              `json:"tls"`
	TlsRedirect     bool              `json:"tls_redirect"`
	Crt             string            `json:"crt"`
	Key             string            `json:"key"`
	HostAliases     map[string]string `json:"host_aliases"`
	SourceTemplates map[string]Source `json:"source_templates"`
	Sources         map[string]Source `json:"sources"`
	Proxy           []string          `json:"proxy"`
}

type Source struct {
	Template bool      `json:"template"`
	BaseName string    `json:"base_name"`
	Targets  []string  `json:"targets"`
	Priority int       `json:"priority"`
	UA       string    `json:"ua"`
	Path     string    `json:"path"`
	Prefix   string    `json:"prefix"`
	Replaces []Replace `json:"replaces"`
	Mirror   string    `json:"mirror"`
}

type Replace struct {
	Type   string `json:"type"`
	Header string `json:"header"`
	Src    string `json:"src"`
	Dst    string `json:"dst"`
}

var config Config

var proxyHost []*url.URL
var hostAliasTargets map[string]*url.URL
var upstreamAliases map[string]string

func cloneAndSubstituteSource(source Source, target string) Source {
	expanded := source
	expanded.Template = false
	expanded.Targets = nil
	expanded.BaseName = ""
	expanded.UA = strings.ReplaceAll(expanded.UA, "{target}", target)
	expanded.Path = strings.ReplaceAll(expanded.Path, "{target}", target)
	expanded.Prefix = strings.ReplaceAll(expanded.Prefix, "{target}", target)
	expanded.Mirror = strings.ReplaceAll(expanded.Mirror, "{target}", target)

	expanded.Replaces = make([]Replace, 0, len(source.Replaces))
	for _, replace := range source.Replaces {
		expanded.Replaces = append(expanded.Replaces, Replace{
			Type:   strings.ReplaceAll(replace.Type, "{target}", target),
			Header: strings.ReplaceAll(replace.Header, "{target}", target),
			Src:    strings.ReplaceAll(replace.Src, "{target}", target),
			Dst:    strings.ReplaceAll(replace.Dst, "{target}", target),
		})
	}
	return expanded
}

func expandSourceTemplates() error {
	expandedSources := map[string]Source{}
	for key, source := range config.Sources {
		expandedSources[key] = source
	}

	for templateName, template := range config.SourceTemplates {
		if !template.Template {
			return fmt.Errorf("source_templates.%s must set template=true", templateName)
		}
		if template.BaseName == "" || !strings.Contains(template.BaseName, "{target}") {
			return fmt.Errorf("source_templates.%s.base_name must contain {target}", templateName)
		}
		if len(template.Targets) == 0 {
			return fmt.Errorf("source_templates.%s.targets is empty", templateName)
		}
		if strings.TrimSpace(template.Mirror) == "" {
			return fmt.Errorf("source_templates.%s.mirror is required", templateName)
		}
		if strings.TrimSpace(template.UA) == "" && strings.TrimSpace(template.Path) == "" {
			return fmt.Errorf("source_templates.%s requires at least one matcher: ua/path", templateName)
		}

		for _, target := range template.Targets {
			target = strings.TrimSpace(target)
			if target == "" {
				return fmt.Errorf("source_templates.%s.targets contains empty item", templateName)
			}
			sourceKey := strings.ReplaceAll(template.BaseName, "{target}", target)
			if _, exists := expandedSources[sourceKey]; exists {
				return fmt.Errorf("source key conflict while expanding template %s: %s", templateName, sourceKey)
			}
			expandedSources[sourceKey] = cloneAndSubstituteSource(template, target)
		}
	}

	config.Sources = expandedSources
	return nil
}

func Load() error {
	file, err := os.Open("config.json")
	if err != nil {
		return fmt.Errorf("failed to open config file: %v", err)
	}
	defer file.Close()
	bytes, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}
	err = json.Unmarshal(bytes, &config)
	if err != nil {
		return fmt.Errorf("failed to parse config file: %v", err)
	}
	if err = expandSourceTemplates(); err != nil {
		return err
	}
	if err = prepareConfig(); err != nil {
		return err
	}
	return nil
}
