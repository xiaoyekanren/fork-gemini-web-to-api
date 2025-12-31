package handlers

import (
	"strings"

	"ai-bridges/internal/models"
	"ai-bridges/internal/providers/gemini"
	"ai-bridges/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

// HandleGoogleGenerativeGenerate handles Google Generative API compatible requests
// @Summary Generate content using Google Generative AI format
// @Description Accepts requests in Google Generative AI format and returns responses compatible with the official API
// @Tags Google Generative AI
// @Accept json
// @Produce json
// @Param model path string true "Model name (e.g., gemini-1.5-flash)"
// @Param request body models.GoogleGenerativeRequest true "Generation request"
// @Success 200 {object} models.GoogleGenerativeResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 503 {object} models.ErrorResponse
// @Router /v1beta/models/{model} [post]
func HandleGoogleGenerativeGenerate(c *fiber.Ctx) error {
	var req models.GoogleGenerativeRequest
	if err := c.BodyParser(&req); err != nil {
		return sendError(c, fiber.StatusBadRequest, "Invalid request body", "INVALID_ARGUMENT")
	}

	// Extract prompt from contents
	var promptBuilder strings.Builder
	for _, content := range req.Contents {
		for _, part := range content.Parts {
			promptBuilder.WriteString(part.Text)
		}
	}
	prompt := promptBuilder.String()

	if prompt == "" {
		return sendError(c, fiber.StatusBadRequest, "Prompt cannot be empty", "INVALID_ARGUMENT")
	}

	// Authenticate using local browser cookies
	secure1PSID, secure1PSIDTS := resolveCookies("", "")
	if secure1PSID == "" {
		return sendError(c, fiber.StatusServiceUnavailable, "No valid cookies found to authenticate with Gemini Web.", "UNAVAILABLE")
	}

	client := gemini.NewClient(secure1PSID, secure1PSIDTS)
	if err := client.Init(); err != nil {
		return sendError(c, fiber.StatusUnauthorized, "Gemini initialization failed: "+err.Error(), "UNAUTHENTICATED")
	}

	// Generate
	responseText, err := client.GenerateContent(prompt)
	if err != nil {
		return sendError(c, fiber.StatusInternalServerError, "Generation failed: "+err.Error(), "INTERNAL")
	}

	// Construct Response matching Google Generative API format
	response := models.GoogleGenerativeResponse{
		Candidates: []models.Candidate{
			{
				Content: models.Content{
					Parts: []models.Part{
						{Text: responseText},
					},
					Role: "model",
				},
				FinishReason: "STOP",
				Index:        0,
				SafetyRatings: []models.SafetyRating{
					{Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Probability: "NEGLIGIBLE"},
					{Category: "HARM_CATEGORY_HATE_SPEECH", Probability: "NEGLIGIBLE"},
					{Category: "HARM_CATEGORY_HARASSMENT", Probability: "NEGLIGIBLE"},
					{Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Probability: "NEGLIGIBLE"},
				},
			},
		},
		PromptFeedback: models.PromptFeedback{
			SafetyRatings: []models.SafetyRating{
				{Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Probability: "NEGLIGIBLE"},
				{Category: "HARM_CATEGORY_HATE_SPEECH", Probability: "NEGLIGIBLE"},
				{Category: "HARM_CATEGORY_HARASSMENT", Probability: "NEGLIGIBLE"},
				{Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Probability: "NEGLIGIBLE"},
			},
		},
	}

	return c.JSON(response)
}

func resolveCookies(psid, psidts string) (string, string) {
	if psid != "" {
		return psid, psidts
	}
	// Try auto-load from browser
	browserCookies, err := utils.GetGeminiCookies()
	if err == nil {
		return browserCookies["__Secure-1PSID"], browserCookies["__Secure-1PSIDTS"]
	}
	return "", ""
}

func sendError(c *fiber.Ctx, statusCode int, message string, status string) error {
	return c.Status(statusCode).JSON(models.ErrorResponse{
		Error: struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Status  string `json:"status"`
		}{
			Code:    statusCode,
			Message: message,
			Status:  status,
		},
	})
}
