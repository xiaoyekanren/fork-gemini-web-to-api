package gemini

import (
	"context"
	"encoding/json"
	"fmt"

	"ai-bridges/internal/providers"
)

// ChatSession implements providers.ChatSession for Gemini
type ChatSession struct {
	client   *Client
	model    string
	metadata *providers.SessionMetadata
	history  []providers.Message
}

// SendMessage sends a message in the chat session
func (s *ChatSession) SendMessage(ctx context.Context, message string, options ...providers.GenerateOption) (*providers.Response, error) {
	s.client.reqMu.Lock()
	defer s.client.reqMu.Unlock()
	
	if s.client.at == "" {
		return nil, fmt.Errorf("client not initialized")
	}

	// Build conversation context
	inner := []interface{}{
		[]interface{}{message},
		nil,
		s.buildMetadata(),
	}

	innerJSON, _ := json.Marshal(inner)
	outer := []interface{}{nil, string(innerJSON)}
	outerJSON, _ := json.Marshal(outer)

	formData := map[string]string{
		"at":    s.client.at,
		"f.req": string(outerJSON),
	}

	resp, err := s.client.httpClient.R().
		SetFormData(formData).
		SetQueryParam("at", s.client.at).
		Post(EndpointGenerate)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("chat failed with status: %d", resp.StatusCode)
	}

	response, err := s.client.parseResponse(resp.String())
	if err != nil {
		return nil, err
	}

	// Update session metadata
	if response.Metadata != nil {
		if cid, ok := response.Metadata["cid"].(string); ok && cid != "" {
			if s.metadata == nil {
				s.metadata = &providers.SessionMetadata{}
			}
			s.metadata.ConversationID = cid
		}
		if rid, ok := response.Metadata["rid"].(string); ok && rid != "" {
			if s.metadata == nil {
				s.metadata = &providers.SessionMetadata{}
			}
			s.metadata.ResponseID = rid
		}
		if rcid, ok := response.Metadata["rcid"].(string); ok && rcid != "" {
			if s.metadata == nil {
				s.metadata = &providers.SessionMetadata{}
			}
			s.metadata.ChoiceID = rcid
		}
	}

	// Update history
	s.history = append(s.history, providers.Message{
		Role:    "user",
		Content: message,
	})
	s.history = append(s.history, providers.Message{
		Role:    "model",
		Content: response.Text,
	})

	return response, nil
}

// GetMetadata returns session metadata
func (s *ChatSession) GetMetadata() *providers.SessionMetadata {
	if s.metadata == nil {
		return &providers.SessionMetadata{
			Model: s.model,
		}
	}
	s.metadata.Model = s.model
	return s.metadata
}

// GetHistory returns conversation history
func (s *ChatSession) GetHistory() []providers.Message {
	return s.history
}

// Clear clears the conversation history
func (s *ChatSession) Clear() {
	s.history = []providers.Message{}
	s.metadata = nil
}

// buildMetadata builds metadata array for API request
func (s *ChatSession) buildMetadata() []interface{} {
	if s.metadata == nil {
		return []interface{}{nil, nil, nil}
	}

	return []interface{}{
		s.metadata.ConversationID,
		s.metadata.ResponseID,
		s.metadata.ChoiceID,
	}
}
