package models

// Request Structures matching Google Generative API
type Part struct {
	Text string `json:"text"`
}

type Content struct {
	Parts []Part `json:"parts"`
	Role  string `json:"role,omitempty"`
}

type GoogleGenerativeRequest struct {
	Contents []Content `json:"contents"`
}

// Simple chat request for our custom endpoint
type SimpleChatRequest struct {
	Message string `json:"message"`
	Cookies struct {
		Secure1PSID   string `json:"__Secure-1PSID"`
		Secure1PSIDTS string `json:"__Secure-1PSIDTS"`
	} `json:"cookies"`
}

// OpenAI Chat Completion Request
type OpenAIRequestMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIRequest struct {
	Model    string                 `json:"model"`
	Messages []OpenAIRequestMessage `json:"messages"`
	Stream   bool                   `json:"stream"`
}

// Translate Request (mimicking the Python GeminiRequest)
type TranslateRequest struct {
	Model   string   `json:"model"`
	Message string   `json:"message"`
	Files   []string `json:"files,omitempty"`
}
