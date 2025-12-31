package handlers

import (
	"ai-bridges/internal/models"
	"ai-bridges/internal/providers/gemini"

	"github.com/gofiber/fiber/v2"
)

// HandleGeminiChat handles simple chat requests with optional cookies
// @Summary Simple Gemini chat endpoint
// @Description Send a message to Gemini. Cookies can be provided or auto-loaded from browser
// @Tags Gemini Chat
// @Accept json
// @Produce json
// @Param request body models.SimpleChatRequest true "Chat request"
// @Success 200 {object} map[string]interface{} "response message"
// @Failure 400 {object} map[string]interface{} "error message"
// @Failure 401 {object} map[string]interface{} "authentication error"
// @Failure 500 {object} map[string]interface{} "generation error"
// @Router /gemini/chat [post]
func HandleGeminiChat(c *fiber.Ctx) error {
	var req models.SimpleChatRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Message == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Message cannot be empty",
		})
	}

	// Resolve cookies (from request or browser)
	secure1PSID, secure1PSIDTS := resolveCookies(
		req.Cookies.Secure1PSID,
		req.Cookies.Secure1PSIDTS,
	)

	if secure1PSID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Missing __Secure-1PSID cookie and could not find it in browser",
		})
	}

	// Initialize Gemini Client
	client := gemini.NewClient(secure1PSID, secure1PSIDTS)
	if err := client.Init(); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Failed to authenticate with Gemini: " + err.Error(),
		})
	}

	// Generate Content
	response, err := client.GenerateContent(req.Message)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Generate content failed: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"response": response,
	})
}
