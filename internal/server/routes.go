package server

import (
	"ai-bridges/internal/handlers"

	"github.com/gofiber/fiber/v2"
)

// RegisterRoutes registers all application routes
func RegisterRoutes(app *fiber.App) {
	// API v1 group
	api := app.Group("/api")
	v1 := api.Group("/v1")

	// Google Generative AI compatible endpoints
	// Matches: POST /api/v1/v1beta/models/{model}
	v1beta := v1.Group("/v1beta")
	v1beta.Post("/models/:model", handlers.HandleGoogleGenerativeGenerate)

	// Custom endpoints for testing and utilities
	v1.Post("/gemini/chat", handlers.HandleGeminiChat)

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
			"service": "ai-bridges",
		})
	})
}
