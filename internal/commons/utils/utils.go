package utils

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"gemini-web-to-api/internal/commons/models"

	"go.uber.org/zap"
)

// BuildPromptFromMessages constructs a unified prompt from messages
func BuildPromptFromMessages(messages []models.Message, systemPrompt string) string {
	var promptBuilder strings.Builder

	if systemPrompt != "" {
		promptBuilder.WriteString(fmt.Sprintf("System: %s\n\n", systemPrompt))
	}

	for _, msg := range messages {
		role := "User"
		if strings.EqualFold(msg.Role, "assistant") || strings.EqualFold(msg.Role, "model") {
			role = "Model"
		} else if strings.EqualFold(msg.Role, "system") {
			role = "System"
		}
		promptBuilder.WriteString(fmt.Sprintf("%s: %s\n", role, msg.GetText()))
	}

	return strings.TrimSpace(promptBuilder.String())
}

// ValidateMessages validates that messages array is not empty and not all empty
func ValidateMessages(messages []models.Message) error {
	if len(messages) == 0 {
		return fmt.Errorf("messages array cannot be empty")
	}

	allEmpty := true
	for _, msg := range messages {
		if strings.TrimSpace(msg.GetText()) != "" {
			allEmpty = false
			break
		}
	}

	if allEmpty {
		return fmt.Errorf("all messages have empty content")
	}

	return nil
}

// ValidateGenerationRequest validates common generation request parameters
func ValidateGenerationRequest(model string, maxTokens int, temperature float32) error {
	if maxTokens < 0 {
		return fmt.Errorf("max_tokens must be non-negative")
	}

	if temperature < 0 || temperature > 2 {
		return fmt.Errorf("temperature must be between 0 and 2")
	}

	return nil
}

// MarshalJSONSafely marshals JSON and logs errors instead of silently failing
func MarshalJSONSafely(log *zap.Logger, v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		log.Error("Failed to marshal JSON", zap.Error(err), zap.Any("value", v))
		return []byte("{}")
	}
	return data
}

// SendStreamChunk writes a JSON chunk to the stream writer with error handling
func SendStreamChunk(w *bufio.Writer, log *zap.Logger, chunk interface{}) error {
	data := MarshalJSONSafely(log, chunk)
	if _, err := w.Write(data); err != nil {
		log.Error("Failed to write chunk", zap.Error(err))
		return err
	}
	if _, err := w.Write([]byte("\n")); err != nil {
		log.Error("Failed to write newline", zap.Error(err))
		return err
	}
	if err := w.Flush(); err != nil {
		log.Error("Failed to flush writer", zap.Error(err))
		return err
	}
	return nil
}

// SendSSEChunk writes a Server-Sent Event chunk
func SendSSEChunk(w *bufio.Writer, log *zap.Logger, event string, chunk interface{}) error {
	data := MarshalJSONSafely(log, chunk)
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, string(data)); err != nil {
		log.Error("Failed to write SSE chunk", zap.Error(err))
		return err
	}
	if err := w.Flush(); err != nil {
		log.Error("Failed to flush SSE writer", zap.Error(err))
		return err
	}
	return nil
}

// SendSSEEvent writes a generic SSE data event by marshaling v as JSON.
// It returns false if writing fails (to signal the caller to stop streaming).
func SendSSEEvent(w *bufio.Writer, log *zap.Logger, v interface{}) bool {
	data := MarshalJSONSafely(log, v)
	if _, err := fmt.Fprintf(w, "data: %s\n\n", string(data)); err != nil {
		log.Error("Failed to write SSE event", zap.Error(err))
		return false
	}
	if err := w.Flush(); err != nil {
		log.Error("Failed to flush SSE event writer", zap.Error(err))
		return false
	}
	return true
}

// SplitResponseIntoChunks simulates streaming by splitting response into chunks
func SplitResponseIntoChunks(text string, delayMs int) []string {
	words := strings.Split(text, " ")
	var chunks []string
	for i, word := range words {
		content := word
		if i < len(words)-1 {
			content += " "
		}
		chunks = append(chunks, content)
	}
	return chunks
}

// SleepWithCancel sleeps for the specified duration or until context is cancelled
func SleepWithCancel(ctx context.Context, duration time.Duration) bool {
	select {
	case <-time.After(duration):
		return true
	case <-ctx.Done():
		return false
	}
}

// ErrorToResponse converts an error to a standardized error response
func ErrorToResponse(err error, errorType string) models.ErrorResponse {
	return models.ErrorResponse{
		Error: models.Error{
			Message: err.Error(),
			Type:    errorType,
		},
	}
}

// StripCodeFence removes markdown code fences from a string, supporting json and JSON labels.
func StripCodeFence(text string) string {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimPrefix(trimmed, "json")
	trimmed = strings.TrimPrefix(trimmed, "JSON")
	trimmed = strings.TrimSpace(trimmed)
	if idx := strings.LastIndex(trimmed, "```"); idx >= 0 {
		trimmed = strings.TrimSpace(trimmed[:idx])
	}
	return trimmed
}

// NormalizeToolInputMap fixes common LLM formatting mistakes before tool inputs
// are handed back to strict clients such as Claude Code.
func NormalizeToolInputMap(input map[string]interface{}) map[string]interface{} {
	if input == nil {
		return map[string]interface{}{}
	}

	normalized, ok := normalizeToolInputValue(input).(map[string]interface{})
	if !ok {
		return input
	}
	return normalized
}

// NormalizeToolArgumentsJSON normalizes JSON object arguments while preserving
// the original payload if it cannot be decoded.
func NormalizeToolArgumentsJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}

	var value interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return raw
	}

	data, err := json.Marshal(normalizeToolInputValue(value))
	if err != nil {
		return raw
	}
	return data
}

func normalizeToolInputValue(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(v))
		for key, child := range v {
			out[key] = normalizeToolInputChild(key, child)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(v))
		for i, child := range v {
			out[i] = normalizeToolInputValue(child)
		}
		return out
	default:
		return value
	}
}

func normalizeToolInputChild(key string, value interface{}) interface{} {
	if s, ok := value.(string); ok && isURLLikeKey(key) {
		return unwrapMarkdownURL(s)
	}
	return normalizeToolInputValue(value)
}

func isURLLikeKey(key string) bool {
	lower := strings.ToLower(strings.TrimSpace(key))
	return lower == "url" || lower == "uri" || strings.HasSuffix(lower, "_url") || strings.HasSuffix(lower, "_uri")
}

func unwrapMarkdownURL(value string) string {
	trimmed := strings.TrimSpace(value)
	linkStart := strings.Index(trimmed, "](")
	if !strings.HasPrefix(trimmed, "[") || linkStart < 0 || !strings.HasSuffix(trimmed, ")") {
		return value
	}

	candidate := strings.TrimSpace(trimmed[linkStart+2 : len(trimmed)-1])
	if !isHTTPURL(candidate) {
		return value
	}
	return candidate
}

func isHTTPURL(value string) bool {
	parsed, err := url.ParseRequestURI(value)
	if err != nil {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}
