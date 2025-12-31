package openai

import (
	geminiProvider "ai-bridges/internal/providers/gemini"

	"github.com/gofiber/fiber/v2"
)

// RegisterRoutes registers OpenAI-compatible routes
func RegisterRoutes(router fiber.Router, client *geminiProvider.Client) {
	handler := NewHandler(client)

	// OpenAI-compatible endpoints
	v1Group := router.Group("/v1")
	{
		v1Group.Post("/chat/completions", handler.HandleChatCompletions)
	}
}
