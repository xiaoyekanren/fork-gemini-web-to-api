package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Gemini GeminiConfig `yaml:"gemini"`
	Server ServerConfig `yaml:"server"`
}

type GeminiConfig struct {
	Secure1PSID     string `yaml:"__Secure-1PSID"`
	Secure1PSIDTS   string `yaml:"__Secure-1PSIDTS"`
	Secure1PSIDCC   string `yaml:"__Secure-1PSIDCC"`
	RefreshInterval int    `yaml:"refresh_interval"` // in minutes
}

type ServerConfig struct {
	Port string `yaml:"port"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	
	// Set defaults
	if cfg.Server.Port == "" {
		cfg.Server.Port = "3000"
	}

	return &cfg, nil
}
