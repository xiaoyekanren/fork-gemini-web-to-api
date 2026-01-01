package config

import (
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Gemini GeminiConfig `yaml:"gemini"`
	Server ServerConfig `yaml:"server"`
}

type GeminiConfig struct {
	Secure1PSID     string `yaml:"GEMINI_1PSID"`
	Secure1PSIDTS   string `yaml:"GEMINI_1PSIDTS"`
	Secure1PSIDCC   string `yaml:"GEMINI_1PSIDCC"`
	RefreshInterval int    `yaml:"GEMINI_REFRESH_INTERVAL"`
}

type ServerConfig struct {
	Port string `yaml:"PORT"`
}

func Load(path string) (*Config, error) {
	var cfg Config
	
	// 1. Load from YAML file if it exists
	data, err := os.ReadFile(path)
	if err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	// 2. Environment variable overrides (Precedence over YAML)
	override := func(envName string, target *string) {
		if val := os.Getenv(envName); val != "" {
			*target = val
		}
	}

	override("GEMINI_1PSID", &cfg.Gemini.Secure1PSID)
	override("GEMINI_1PSIDTS", &cfg.Gemini.Secure1PSIDTS)
	override("GEMINI_1PSIDCC", &cfg.Gemini.Secure1PSIDCC)
	override("PORT", &cfg.Server.Port)

	if refresh := os.Getenv("GEMINI_REFRESH_INTERVAL"); refresh != "" {
		if val, err := strconv.Atoi(refresh); err == nil {
			cfg.Gemini.RefreshInterval = val
		}
	}

	// 3. Set final defaults for missing values
	if cfg.Server.Port == "" {
		cfg.Server.Port = "3000"
	}
	if cfg.Gemini.RefreshInterval <= 0 {
		cfg.Gemini.RefreshInterval = 5 // Default to 5 minutes
	}

	return &cfg, nil
}
