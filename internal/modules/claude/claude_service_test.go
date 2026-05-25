package claude

import (
	"encoding/json"
	"strings"
	"testing"

	"gemini-web-to-api/internal/commons/models"
	"gemini-web-to-api/internal/modules/claude/dto"
)

func TestBuildClaudePromptFromMessagesIncludesToolUseAndResult(t *testing.T) {
	messages := []models.Message{
		{
			Role: "assistant",
			Content: json.RawMessage(`[
				{"type":"text","text":"I will inspect the file."},
				{"type":"tool_use","id":"toolu_1","name":"Read","input":{"file_path":"/tmp/app.go"}}
			]`),
		},
		{
			Role: "user",
			Content: json.RawMessage(`[
				{"type":"tool_result","tool_use_id":"toolu_1","content":"package main\nfunc main() {}"}
			]`),
		},
	}

	got := buildClaudePromptFromMessages(messages, "Be precise.")

	for _, want := range []string{
		"System: Be precise.",
		"Assistant:",
		"I will inspect the file.",
		"Tool use requested:",
		"id: toolu_1",
		"name: Read",
		`"file_path":"/tmp/app.go"`,
		"User:",
		"Tool result:",
		"tool_use_id: toolu_1",
		"package main",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("prompt missing %q:\n%s", want, got)
		}
	}
}

func TestBuildClaudePromptFromMessagesIncludesNestedToolResultTextBlocks(t *testing.T) {
	messages := []models.Message{
		{
			Role: "user",
			Content: json.RawMessage(`[
				{
					"type":"tool_result",
					"tool_use_id":"toolu_2",
					"content":[{"type":"text","text":"first line"},{"type":"text","text":"second line"}]
				}
			]`),
		},
	}

	got := buildClaudePromptFromMessages(messages, "")

	for _, want := range []string{"toolu_2", "first line", "second line"} {
		if !strings.Contains(got, want) {
			t.Fatalf("prompt missing %q:\n%s", want, got)
		}
	}
}

func TestResolveClaudeToolChoiceModes(t *testing.T) {
	tools := []dto.Tool{{Name: "Read"}, {Name: "Bash"}}

	tests := []struct {
		name        string
		choice      *dto.ToolChoice
		wantMode    string
		wantForced  string
		allowsTools bool
		requires    bool
	}{
		{name: "default", wantMode: "auto", allowsTools: true},
		{name: "auto", choice: &dto.ToolChoice{Type: "auto"}, wantMode: "auto", allowsTools: true},
		{name: "none", choice: &dto.ToolChoice{Type: "none"}, wantMode: "none", allowsTools: false},
		{name: "any", choice: &dto.ToolChoice{Type: "any"}, wantMode: "any", allowsTools: true, requires: true},
		{name: "tool", choice: &dto.ToolChoice{Type: "tool", Name: "Read"}, wantMode: "tool", wantForced: "Read", allowsTools: true, requires: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveClaudeToolChoice(dto.MessageRequest{
				Tools:      tools,
				ToolChoice: tt.choice,
			})
			if err != nil {
				t.Fatalf("resolveClaudeToolChoice returned error: %v", err)
			}
			if got.mode != tt.wantMode {
				t.Fatalf("mode = %q, want %q", got.mode, tt.wantMode)
			}
			if got.forcedName != tt.wantForced {
				t.Fatalf("forcedName = %q, want %q", got.forcedName, tt.wantForced)
			}
			if got.allowsTools() != tt.allowsTools {
				t.Fatalf("allowsTools = %v, want %v", got.allowsTools(), tt.allowsTools)
			}
			if got.requiresTool() != tt.requires {
				t.Fatalf("requiresTool = %v, want %v", got.requiresTool(), tt.requires)
			}
		})
	}
}

func TestResolveClaudeToolChoiceRejectsInvalidForcedTool(t *testing.T) {
	_, err := resolveClaudeToolChoice(dto.MessageRequest{
		Tools:      []dto.Tool{{Name: "Read"}},
		ToolChoice: &dto.ToolChoice{Type: "tool", Name: "Write"},
	})
	if err == nil {
		t.Fatal("expected unknown forced tool error")
	}
}

func TestBuildToolBridgePromptIncludesToolChoiceRules(t *testing.T) {
	req := dto.MessageRequest{
		Tools: []dto.Tool{{Name: "Read"}, {Name: "Bash"}},
	}
	service := &ClaudeService{}

	anyPrompt := service.buildToolBridgePrompt(req, "User:\ninspect", claudeToolChoice{mode: "any"})
	if !strings.Contains(anyPrompt, "tool_choice is any") || !strings.Contains(anyPrompt, `MUST return status "tool_use"`) {
		t.Fatalf("any tool_choice rule missing:\n%s", anyPrompt)
	}

	toolPrompt := service.buildToolBridgePrompt(req, "User:\ninspect", claudeToolChoice{mode: "tool", forcedName: "Read"})
	if !strings.Contains(toolPrompt, "tool_choice is tool") || !strings.Contains(toolPrompt, "exactly one tool call named Read") {
		t.Fatalf("forced tool_choice rule missing:\n%s", toolPrompt)
	}
}

func TestValidateClaudeToolUsesRejectsForcedMismatch(t *testing.T) {
	_, err := validateClaudeToolUses(
		dto.MessageRequest{Tools: []dto.Tool{{Name: "Read"}, {Name: "Bash"}}},
		claudeToolChoice{mode: "tool", forcedName: "Read"},
		[]dto.ConfigContent{{Type: "tool_use", Name: "Bash"}},
	)
	if err == nil {
		t.Fatal("expected forced tool mismatch error")
	}
}
