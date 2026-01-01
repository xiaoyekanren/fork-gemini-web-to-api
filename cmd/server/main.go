package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"ai-bridges/internal/config"
	"ai-bridges/internal/providers/gemini"
	"ai-bridges/internal/server"
	"ai-bridges/pkg/logger"

	_ "ai-bridges/docs" // Import generated docs

	"go.uber.org/zap"
)

// @title AI Bridges API
// @version 1.0
// @description WebAI-to-API service for Go - Convert web-based AI services to REST APIs
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:3000
// @BasePath /
func main() {
	// Initialize logger
	logger.Init()
	defer logger.Sync()

	logger.Info("🚀 AI Bridges - WebAI-to-API Service")
	logger.Info("Cookies are managed via config.yml and auto-rotation")

	// Load configuration
	cfg, err := config.Load("config.yml")
	if err != nil {
		logger.Fatal("Failed to load config", zap.Error(err))
	}

	// Initialize Gemini Client
	client := gemini.NewClient(
		cfg.Gemini.Secure1PSID,
		cfg.Gemini.Secure1PSIDTS,
		cfg.Gemini.Secure1PSIDCC,
		cfg.Gemini.RefreshInterval,
	)

	// Context for initialization
	ctx := context.Background()
	if err := client.Init(ctx); err != nil {
		logger.Warn("Gemini client initialization warning (check cookies/config)", zap.Error(err))
		// We allow starting even if init failed, might recover later via cache or periodic refresh?
		// User wants "auto refresh... all for me". If Init fails (bad cookies), auto refresh might also fail unless cache exists.
		// However, stopping the server might be too harsh if user just needs to update config.
	}
	defer client.Close()

	// Create server
	srv, err := server.New(client)
	if err != nil {
		logger.Fatal("Failed to create server", zap.Error(err))
	}

	// Setup graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-quit
		logger.Info("Received shutdown signal")
		if err := srv.Shutdown(); err != nil {
			logger.Error("Error during shutdown", zap.Error(err))
		}
		os.Exit(0)
	}()

	// Start server
	if err := srv.Start(":" + cfg.Server.Port); err != nil {
		logger.Fatal("Server error", zap.Error(err))
	}
}
