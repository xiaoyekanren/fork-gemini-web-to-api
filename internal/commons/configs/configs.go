package configs

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Gemini    GeminiConfig
	Claude    ClaudeConfig
	OpenAI    OpenAIConfig
	Server    ServerConfig
	RateLimit RateLimitConfig
	Auth      AuthConfig
	LogLevel  string
}

type AuthConfig struct {
	APIKey string
}

type RateLimitConfig struct {
	Enabled     bool
	WindowMs    int
	MaxRequests int
}

type GeminiConfig struct {
	Secure1PSID   string
	Secure1PSIDCC string
	MaxRetries    int
	Cookies       string
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
	Host string
}

const (
	defaultServerPort       = "4981"
	defaultGeminiMaxRetries = 3
	defaultLogLevel         = "info"
)

func New() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	var cfg Config

	// Server
	cfg.Server.Port = getEnv("PORT", defaultServerPort)
	cfg.Server.Host = getEnv("HOST", "")

	// General
	cfg.LogLevel = getEnv("LOG_LEVEL", defaultLogLevel)

	// Auth
	cfg.Auth.APIKey = os.Getenv("API_KEY")

	// Rate Limit
	cfg.RateLimit.Enabled = getEnvBool("RATE_LIMIT_ENABLED", false)
	cfg.RateLimit.WindowMs = getEnvInt("RATE_LIMIT_WINDOW_MS", 60000)
	cfg.RateLimit.MaxRequests = getEnvInt("RATE_LIMIT_MAX_REQUESTS", 10)

	// Gemini
	cfg.Gemini.Secure1PSID = os.Getenv("GEMINI_1PSID")
	cfg.Gemini.Secure1PSIDCC = os.Getenv("GEMINI_1PSIDCC")
	cfg.Gemini.Cookies = os.Getenv("GEMINI_COOKIES")
	cfg.Gemini.MaxRetries = getEnvInt("GEMINI_MAX_RETRIES", defaultGeminiMaxRetries)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate checks if the configuration has required values
func (c *Config) Validate() error {
	var missingVars []string

	// Gemini Web auth now relies on a browser cookie set. GEMINI_1PSID is
	// still accepted as a compatibility shortcut.
	if c.Gemini.Cookies == "" && c.Gemini.Secure1PSID == "" {
		missingVars = append(missingVars, "GEMINI_COOKIES or GEMINI_1PSID")
	}

	// Check Server port is valid
	if c.Server.Port == "" {
		c.Server.Port = defaultServerPort
	}

	if _, err := strconv.Atoi(c.Server.Port); err != nil {
		return fmt.Errorf("invalid PORT value: %q (must be a number)", c.Server.Port)
	}

	if len(missingVars) > 0 {
		return fmt.Errorf("missing required environment variables: %v. Please set them before running the application", missingVars)
	}

	return nil
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

func getEnvBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}
