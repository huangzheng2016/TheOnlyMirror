package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
)

type Config struct {
	Port        int               `json:"port"`
	Tls         bool              `json:"tls"`
	TlsRedirect bool              `json:"tls_redirect"`
	Crt         string            `json:"crt"`
	Key         string            `json:"key"`
	Sources     map[string]Source `json:"sources"`
	Proxy       []string          `json:"proxy"`
}

type Source struct {
	Priority int       `json:"priority"`
	Type     string    `json:"type"`
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
	prepareConfig()
	return nil
}
