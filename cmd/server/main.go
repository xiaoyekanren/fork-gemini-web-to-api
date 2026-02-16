package main

import (
	"context"

	"gemini-web-to-api/internal/config"
	"gemini-web-to-api/internal/handlers"
	"gemini-web-to-api/internal/providers"
	"gemini-web-to-api/internal/providers/gemini"
	"gemini-web-to-api/internal/server"
	"gemini-web-to-api/pkg/logger"

	_ "gemini-web-to-api/cmd/swag/docs"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

// @title AI Bridges API
// @version 1.0
// @description 🚀 High-performance WebAI-to-API gateway. Seamlessly bridge Google Gemini into standardized OpenAI, Anthropic (Claude), and Google Native REST APIs.
// @host localhost:4981
// @BasePath /
func main() {
	fx.New(
		fx.Provide(
			config.New,
			func(cfg *config.Config) (*zap.Logger, error) {
				return logger.New(cfg.LogLevel)
			},
			providers.NewProviderManager,
			gemini.NewClient,
			handlers.NewGeminiHandler,
			handlers.NewOpenAIHandler,
			handlers.NewClaudeHandler,
		),
		fx.Invoke(
			server.New,
		),
		fx.Invoke(func(pm *providers.ProviderManager, c *gemini.Client, log *zap.Logger) {
			pm.Register("gemini", c)
			// Initialize all providers (non-blocking, logs warnings on failure)
			pm.InitAllProviders(context.Background())
			// Select Gemini as the provider
			if err := pm.SelectProvider("gemini"); err != nil {
				log.Error("Failed to select Gemini provider", zap.Error(err))
			} else {
				log.Debug("Gemini provider selected")
			}
		}),
		fx.NopLogger, 
	).Run()
}
