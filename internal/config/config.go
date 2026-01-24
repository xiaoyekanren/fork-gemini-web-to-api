package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Providers ProvidersConfig
	Gemini    GeminiConfig
	Claude    ClaudeConfig
	OpenAI    OpenAIConfig
	Server    ServerConfig
}

type ProvidersConfig struct {
	ProviderType string
}

type GeminiConfig struct {
	Secure1PSID     string
	Secure1PSIDTS   string
	Secure1PSIDCC   string
	RefreshInterval int
	Cookies         string
}

type ClaudeConfig struct {
	APIKey  string
	Model   string
	Cookies string
}

type OpenAIConfig struct {
	APIKey  string
	Model   string
	Cookies string
}

type ServerConfig struct {
	Port string
}

const (
	defaultServerPort            = "3000"
	defaultGeminiRefreshInterval = 5
	defaultProviderType          = "gemini"
)

func New() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	var cfg Config

	// Provider Type
	cfg.Providers.ProviderType = getEnv("PROVIDER_TYPE", defaultProviderType)

	// Server
	cfg.Server.Port = getEnv("PORT", defaultServerPort)

	// Gemini
	cfg.Gemini.Secure1PSID = os.Getenv("GEMINI_1PSID")
	cfg.Gemini.Secure1PSIDTS = os.Getenv("GEMINI_1PSIDTS")
	cfg.Gemini.Secure1PSIDCC = os.Getenv("GEMINI_1PSIDCC")
	cfg.Gemini.Cookies = os.Getenv("GEMINI_COOKIES")
	cfg.Gemini.RefreshInterval = getEnvInt("GEMINI_REFRESH_INTERVAL", defaultGeminiRefreshInterval)

	// Claude
	cfg.Claude.APIKey = os.Getenv("CLAUDE_API_KEY")
	cfg.Claude.Model = os.Getenv("CLAUDE_MODEL")
	cfg.Claude.Cookies = os.Getenv("CLAUDE_COOKIES")

	// OpenAI
	cfg.OpenAI.APIKey = os.Getenv("OPENAI_API_KEY")
	cfg.OpenAI.Model = os.Getenv("OPENAI_MODEL")
	cfg.OpenAI.Cookies = os.Getenv("OPENAI_COOKIES")

	return &cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}


