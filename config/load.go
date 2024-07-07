package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type Config struct {
	Port    int               `json:"port"`
	Tls     bool              `json:"tls"`
	Crt     string            `json:"crt"`
	Key     string            `json:"key"`
	Sources map[string]Source `json:"sources"`
}

type Source struct {
	Type    string   `json:"type"`
	UA      string   `json:"ua"`
	Path    string   `json:"path"`
	Mirrors []string `json:"mirrors"`
}

var config Config

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
	return nil
}
