package dto

import (
	"encoding/json"
	"strings"

	models "gemini-web-to-api/internal/commons/models"
)

// MessageRequest represents the specialized Claude request body
type MessageRequest struct {
	Model      string           `json:"model"`
	MaxTokens  int              `json:"max_tokens"`
	Messages   []models.Message `json:"messages"`
	System     json.RawMessage  `json:"system,omitempty"` // string or [{type,text}]
	Stream     bool             `json:"stream,omitempty"`
	Tools      []Tool           `json:"tools,omitempty"`
	ToolChoice *ToolChoice      `json:"tool_choice,omitempty"`
}

// Tool represents a tool available to the model
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema" swagignore:"true"`
}

// ToolChoice represents how the model should use tools
type ToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// MessageResponse represents the non-streaming response body
type MessageResponse struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Role       string          `json:"role"`
	Model      string          `json:"model"`
	Content    []ConfigContent `json:"content"`
	StopReason string          `json:"stop_reason"`
	Usage      models.Usage    `json:"usage"`
}

// ConfigContent represents the content block in a response
type ConfigContent struct {
	Type  string                 `json:"type"`
	Text  string                 `json:"text,omitempty"`
	ID    string                 `json:"id,omitempty"`
	Name  string                 `json:"name,omitempty"`
	Input map[string]interface{} `json:"input,omitempty"`
}

// StreamMessage is the message shape emitted by Anthropic-compatible streams.
type StreamMessage struct {
	ID           string          `json:"id"`
	Type         string          `json:"type"`
	Role         string          `json:"role"`
	Model        string          `json:"model"`
	Content      []ConfigContent `json:"content"`
	StopReason   *string         `json:"stop_reason"`
	StopSequence *string         `json:"stop_sequence"`
	Usage        models.Usage    `json:"usage"`
}

// StreamContentBlock preserves required empty fields in content_block_start events.
type StreamContentBlock struct {
	Type  string                  `json:"type"`
	Text  *string                 `json:"text,omitempty"`
	ID    string                  `json:"id,omitempty"`
	Name  string                  `json:"name,omitempty"`
	Input *map[string]interface{} `json:"input,omitempty"`
}

// StreamEvent represents a streaming event
type StreamEvent struct {
	Type         string              `json:"type"`
	Message      *StreamMessage      `json:"message,omitempty"`
	Index        *int                `json:"index,omitempty"`
	ContentBlock *StreamContentBlock `json:"content_block,omitempty"`
	DeltaField   *models.Delta       `json:"delta,omitempty"`
	StopReason   string              `json:"stop_reason,omitempty"`
	UsageField   *models.Usage       `json:"usage,omitempty"`
	Error        *Error              `json:"error,omitempty"`
}

type Error struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// GetSystemText extracts text from System field (handles both string and array format)
func GetSystemText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var texts []string
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				texts = append(texts, b.Text)
			}
		}
		return strings.Join(texts, "\n")
	}
	return ""
}
