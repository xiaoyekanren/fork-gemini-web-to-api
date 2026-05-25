package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gemini-web-to-api/internal/commons/models"
	common "gemini-web-to-api/internal/commons/utils"
	"gemini-web-to-api/internal/modules/claude/dto"
	"gemini-web-to-api/internal/modules/providers"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type claudeContentBlock struct {
	Type      string                 `json:"type"`
	Text      string                 `json:"text,omitempty"`
	ID        string                 `json:"id,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Input     map[string]interface{} `json:"input,omitempty"`
	ToolUseID string                 `json:"tool_use_id,omitempty"`
	Content   json.RawMessage        `json:"content,omitempty"`
	IsError   bool                   `json:"is_error,omitempty"`
}

type claudeToolChoice struct {
	mode       string
	forcedName string
}

type ClaudeService struct {
	client *providers.Client
	log    *zap.Logger
}

func NewClaudeService(client *providers.Client, log *zap.Logger) *ClaudeService {
	return &ClaudeService{
		client: client,
		log:    log,
	}
}

func (s *ClaudeService) ListModels() []providers.ModelInfo {
	return s.client.ListModels()
}

func (s *ClaudeService) GenerateMessage(ctx context.Context, req dto.MessageRequest) (*dto.MessageResponse, error) {
	// Logic: Validate
	if err := validateClaudeMessages(req.Messages); err != nil {
		return nil, err
	}

	// Logic: Build Prompt
	prompt := buildClaudePromptFromMessages(req.Messages, dto.GetSystemText(req.System))
	if prompt == "" {
		return nil, fmt.Errorf("no valid content in messages")
	}

	toolChoice, err := resolveClaudeToolChoice(req)
	if err != nil {
		return nil, err
	}

	hasTools := len(req.Tools) > 0 && toolChoice.allowsTools()
	if hasTools {
		prompt = s.buildToolBridgePrompt(req, prompt, toolChoice)
	}

	opts := []providers.GenerateOption{}

	// Logic: Call Provider
	response, err := s.client.GenerateContent(ctx, prompt, opts...)
	if err != nil {
		return nil, err
	}

	// Logic: Construct Response
	msgID := fmt.Sprintf("msg_%s", uuid.New().String())
	resContent := []dto.ConfigContent{}
	stopReason := "end_turn"

	if hasTools {
		toolUses, text := s.parseToolBridgeOutput(req, response.Text)
		if len(toolUses) > 0 {
			toolUses, err = validateClaudeToolUses(req, toolChoice, toolUses)
			if err != nil {
				return nil, err
			}
			for _, tu := range toolUses {
				resContent = append(resContent, tu)
			}
			stopReason = "tool_use"
		} else if toolChoice.requiresTool() {
			return nil, fmt.Errorf("tool_choice %q requires a tool_use response, but model returned text", toolChoice.mode)
		} else {
			resContent = append(resContent, dto.ConfigContent{Type: "text", Text: text})
		}
	} else {
		resContent = append(resContent, dto.ConfigContent{Type: "text", Text: response.Text})
	}

	return &dto.MessageResponse{
		ID:         msgID,
		Type:       "message",
		Role:       "assistant",
		Model:      req.Model,
		Content:    resContent,
		StopReason: stopReason,
		Usage: models.Usage{
			InputTokens:  len(prompt) / 4,
			OutputTokens: len(response.Text) / 4,
		},
	}, nil
}

// GenerateMessageStream handles the logic of transforming a message generation into a stream of Claude events.
func (s *ClaudeService) GenerateMessageStream(ctx context.Context, req dto.MessageRequest, onEvent func(dto.StreamEvent) bool) error {
	response, err := s.GenerateMessage(ctx, req)
	if err != nil {
		return err
	}

	// message_start
	if !onEvent(dto.StreamEvent{
		Type: "message_start",
		Message: &dto.MessageResponse{
			ID:    response.ID,
			Type:  "message",
			Role:  "assistant",
			Model: req.Model,
			Usage: response.Usage,
		},
	}) {
		return nil
	}

	for i, content := range response.Content {
		// content_block_start
		startEv := dto.StreamEvent{
			Type:         "content_block_start",
			Index:        i,
			ContentBlock: &dto.ConfigContent{Type: content.Type},
		}
		if content.Type == "tool_use" {
			startEv.ContentBlock.ID = content.ID
			startEv.ContentBlock.Name = content.Name
		}
		if !onEvent(startEv) {
			return nil
		}

		if content.Type == "text" {
			chunks := common.SplitResponseIntoChunks(content.Text, 30)
			for _, chunk := range chunks {
				if !onEvent(dto.StreamEvent{
					Type:  "content_block_delta",
					Index: i,
					DeltaField: &models.Delta{
						Type: "text_delta",
						Text: chunk,
					},
				}) {
					return nil
				}
				if !common.SleepWithCancel(ctx, 30*time.Millisecond) {
					return nil
				}
			}
		} else if content.Type == "tool_use" {
			inputJSON, err := json.Marshal(content.Input)
			if err != nil {
				s.log.Error("Failed to marshal tool input", zap.Error(err))
				return fmt.Errorf("failed to marshal tool input: %w", err)
			}
			if !onEvent(dto.StreamEvent{
				Type:  "content_block_delta",
				Index: i,
				DeltaField: &models.Delta{
					Type:        "input_json_delta",
					PartialJSON: string(inputJSON),
				},
			}) {
				return nil
			}
		}

		// content_block_stop
		if !onEvent(dto.StreamEvent{
			Type:  "content_block_stop",
			Index: i,
		}) {
			return nil
		}
	}

	// message_delta
	if !onEvent(dto.StreamEvent{
		Type: "message_delta",
		DeltaField: &models.Delta{
			StopReason: response.StopReason,
		},
	}) {
		return nil
	}

	// message_stop
	onEvent(dto.StreamEvent{Type: "message_stop"})
	return nil
}

func (s *ClaudeService) buildToolBridgePrompt(req dto.MessageRequest, basePrompt string, toolChoice claudeToolChoice) string {
	var b strings.Builder
	b.WriteString("You are a Claude-compatible assistant running behind a bridge that supports tool use.\n")
	b.WriteString("You MUST respond with JSON only. Do not output markdown code fences.\n")
	b.WriteString("Output schema:\n")
	b.WriteString("{\"status\":\"tool_use\",\"tool_calls\":[{\"id\":\"<unique_id>\",\"name\":\"<tool_name>\",\"input\":{}}]} OR {\"status\":\"text\",\"content\":\"<assistant_text>\"}\n")
	b.WriteString("Rules:\n")
	b.WriteString("- Use only tool names listed below.\n")
	b.WriteString("- input must be valid JSON object.\n")
	b.WriteString("- Tool input values must be plain JSON values, not Markdown.\n")
	b.WriteString("- For URL fields, use the raw URL string only, never [text](url).\n")

	switch toolChoice.mode {
	case "any":
		b.WriteString("- tool_choice is any: you MUST return status \"tool_use\" with at least one tool call. Do not return status \"text\".\n")
	case "tool":
		b.WriteString("- tool_choice is tool: you MUST return status \"tool_use\" with exactly one tool call named ")
		b.WriteString(toolChoice.forcedName)
		b.WriteString(". Do not call any other tool and do not return status \"text\".\n")
	default:
		b.WriteString("- tool_choice is auto: call a tool only when needed; otherwise return status \"text\".\n")
	}

	b.WriteString("Available tools:\n")
	for _, t := range req.Tools {
		b.WriteString("- name: ")
		b.WriteString(t.Name)
		if t.Description != "" {
			b.WriteString(" | description: ")
			b.WriteString(t.Description)
		}
		if len(t.InputSchema) > 0 {
			b.WriteString(" | input_schema: ")
			b.Write(t.InputSchema)
		}
		b.WriteString("\n")
	}

	b.WriteString("\nConversation:\n")
	b.WriteString(basePrompt)
	return b.String()
}

func (s *ClaudeService) parseToolBridgeOutput(req dto.MessageRequest, text string) ([]dto.ConfigContent, string) {
	cleaned := common.StripCodeFence(text)
	if cleaned == "" {
		return nil, ""
	}

	var payload struct {
		Status    string `json:"status"`
		ToolCalls []struct {
			ID    string                 `json:"id"`
			Name  string                 `json:"name"`
			Input map[string]interface{} `json:"input"`
		} `json:"tool_calls"`
		Content string `json:"content"`
	}

	if err := json.Unmarshal([]byte(cleaned), &payload); err != nil {
		return nil, text
	}

	if payload.Status == "tool_use" && len(payload.ToolCalls) > 0 {
		uses := make([]dto.ConfigContent, 0, len(payload.ToolCalls))
		for _, tc := range payload.ToolCalls {
			id := tc.ID
			if id == "" {
				id = fmt.Sprintf("toolu_%s", uuid.New().String())
			}
			uses = append(uses, dto.ConfigContent{
				Type:  "tool_use",
				ID:    id,
				Name:  tc.Name,
				Input: common.NormalizeToolInputMap(tc.Input),
			})
		}
		return uses, ""
	}

	return nil, payload.Content
}

func resolveClaudeToolChoice(req dto.MessageRequest) (claudeToolChoice, error) {
	choice := claudeToolChoice{mode: "auto"}
	if req.ToolChoice == nil {
		return choice, nil
	}

	mode := strings.ToLower(strings.TrimSpace(req.ToolChoice.Type))
	if mode == "" {
		mode = "auto"
	}

	switch mode {
	case "auto", "none", "any":
		choice.mode = mode
	case "tool":
		forcedName := strings.TrimSpace(req.ToolChoice.Name)
		if forcedName == "" {
			return choice, fmt.Errorf("tool_choice type %q requires a tool name", mode)
		}
		if !claudeToolExists(req.Tools, forcedName) {
			return choice, fmt.Errorf("tool_choice requested unknown tool %q", forcedName)
		}
		choice.mode = mode
		choice.forcedName = forcedName
	default:
		choice.mode = "auto"
	}

	if choice.requiresTool() && len(req.Tools) == 0 {
		return choice, fmt.Errorf("tool_choice %q requires at least one tool", choice.mode)
	}

	return choice, nil
}

func (c claudeToolChoice) allowsTools() bool {
	return c.mode != "none"
}

func (c claudeToolChoice) requiresTool() bool {
	return c.mode == "any" || c.mode == "tool"
}

func validateClaudeToolUses(req dto.MessageRequest, choice claudeToolChoice, toolUses []dto.ConfigContent) ([]dto.ConfigContent, error) {
	available := make(map[string]struct{}, len(req.Tools))
	for _, tool := range req.Tools {
		name := strings.TrimSpace(tool.Name)
		if name != "" {
			available[name] = struct{}{}
		}
	}

	for _, toolUse := range toolUses {
		name := strings.TrimSpace(toolUse.Name)
		if _, ok := available[name]; !ok {
			return nil, fmt.Errorf("model requested unknown tool %q", name)
		}
		if choice.mode == "tool" && name != choice.forcedName {
			return nil, fmt.Errorf("tool_choice requires tool %q, but model requested %q", choice.forcedName, name)
		}
	}

	if choice.mode == "tool" && len(toolUses) != 1 {
		return nil, fmt.Errorf("tool_choice requires exactly one tool call named %q, got %d", choice.forcedName, len(toolUses))
	}

	return toolUses, nil
}

func claudeToolExists(tools []dto.Tool, name string) bool {
	for _, tool := range tools {
		if strings.TrimSpace(tool.Name) == name {
			return true
		}
	}
	return false
}

func validateClaudeMessages(messages []models.Message) error {
	if len(messages) == 0 {
		return fmt.Errorf("messages array cannot be empty")
	}
	if buildClaudePromptFromMessages(messages, "") == "" {
		return fmt.Errorf("all messages have empty content")
	}
	return nil
}

func buildClaudePromptFromMessages(messages []models.Message, systemPrompt string) string {
	var b strings.Builder

	if strings.TrimSpace(systemPrompt) != "" {
		b.WriteString("System: ")
		b.WriteString(strings.TrimSpace(systemPrompt))
		b.WriteString("\n\n")
	}

	for _, msg := range messages {
		rendered := renderClaudeMessageContent(msg)
		if rendered == "" {
			continue
		}
		b.WriteString(claudePromptRole(msg.Role))
		b.WriteString(":\n")
		b.WriteString(rendered)
		b.WriteString("\n\n")
	}

	return strings.TrimSpace(b.String())
}

func claudePromptRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "assistant", "model":
		return "Assistant"
	case "system":
		return "System"
	default:
		return "User"
	}
}

func renderClaudeMessageContent(msg models.Message) string {
	if len(msg.Content) == 0 || string(msg.Content) == "null" {
		return ""
	}

	var text string
	if err := json.Unmarshal(msg.Content, &text); err == nil {
		return strings.TrimSpace(text)
	}

	var blocks []claudeContentBlock
	if err := json.Unmarshal(msg.Content, &blocks); err != nil {
		return strings.TrimSpace(msg.GetText())
	}

	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if rendered := renderClaudeContentBlock(block); rendered != "" {
			parts = append(parts, rendered)
		}
	}
	return strings.Join(parts, "\n")
}

func renderClaudeContentBlock(block claudeContentBlock) string {
	switch block.Type {
	case "text":
		return strings.TrimSpace(block.Text)
	case "tool_use":
		input := "{}"
		if len(block.Input) > 0 {
			if data, err := json.Marshal(common.NormalizeToolInputMap(block.Input)); err == nil {
				input = string(data)
			}
		}
		return fmt.Sprintf("Tool use requested:\nid: %s\nname: %s\ninput: %s",
			strings.TrimSpace(block.ID),
			strings.TrimSpace(block.Name),
			input,
		)
	case "tool_result":
		content := renderClaudeToolResultContent(block.Content)
		if block.IsError {
			content = "ERROR: " + content
		}
		return fmt.Sprintf("Tool result:\ntool_use_id: %s\ncontent: %s",
			strings.TrimSpace(block.ToolUseID),
			strings.TrimSpace(content),
		)
	default:
		return renderUnknownClaudeBlock(block)
	}
}

func renderClaudeToolResultContent(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}

	var blocks []claudeContentBlock
	if err := json.Unmarshal(raw, &blocks); err == nil {
		parts := make([]string, 0, len(blocks))
		for _, block := range blocks {
			if rendered := renderClaudeContentBlock(block); rendered != "" {
				parts = append(parts, rendered)
			}
		}
		return strings.Join(parts, "\n")
	}

	var value interface{}
	if err := json.Unmarshal(raw, &value); err == nil {
		if data, err := json.Marshal(value); err == nil {
			return string(data)
		}
	}

	return string(raw)
}

func renderUnknownClaudeBlock(block claudeContentBlock) string {
	data, err := json.Marshal(block)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("Unsupported Claude content block: %s", string(data))
}
