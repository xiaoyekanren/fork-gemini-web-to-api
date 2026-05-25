package claude

import (
	"bufio"
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"

	"gemini-web-to-api/internal/modules/claude/dto"

	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

func TestSendClaudeSSEEventWritesAnthropicEventName(t *testing.T) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	index := 1

	ok := sendClaudeSSEEvent(writer, zap.NewNop(), dto.StreamEvent{
		Type:  "content_block_delta",
		Index: &index,
	})
	if !ok {
		t.Fatal("expected SSE write to succeed")
	}

	got := buf.String()
	for _, want := range []string{
		"event: content_block_delta\n",
		`data: {"type":"content_block_delta","index":1}`,
		"\n\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("SSE output missing %q:\n%s", want, got)
		}
	}
}

func TestSendClaudeSSEEventKeepsZeroIndexAndEmptyBlockFields(t *testing.T) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	index := 0
	text := ""

	ok := sendClaudeSSEEvent(writer, zap.NewNop(), dto.StreamEvent{
		Type:  "content_block_start",
		Index: &index,
		ContentBlock: &dto.StreamContentBlock{
			Type: "text",
			Text: &text,
		},
	})
	if !ok {
		t.Fatal("expected SSE write to succeed")
	}

	got := buf.String()
	for _, want := range []string{
		"event: content_block_start\n",
		`"index":0`,
		`"content_block":{"type":"text","text":""}`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("SSE output missing %q:\n%s", want, got)
		}
	}
}

func TestSendClaudeSSEEventWritesEmptyToolInput(t *testing.T) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	index := 0
	input := map[string]interface{}{}

	ok := sendClaudeSSEEvent(writer, zap.NewNop(), dto.StreamEvent{
		Type:  "content_block_start",
		Index: &index,
		ContentBlock: &dto.StreamContentBlock{
			Type:  "tool_use",
			ID:    "toolu_1",
			Name:  "Read",
			Input: &input,
		},
	})
	if !ok {
		t.Fatal("expected SSE write to succeed")
	}

	got := buf.String()
	for _, want := range []string{
		`"index":0`,
		`"content_block":{"type":"tool_use","id":"toolu_1","name":"Read","input":{}}`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("SSE output missing %q:\n%s", want, got)
		}
	}
}

func TestHandleMessagesInvalidBodyDoesNotPanic(t *testing.T) {
	app := fiber.New()
	controller := NewClaudeController(nil)
	app.Post("/messages", controller.HandleMessages)

	req := httptest.NewRequest("POST", "/messages", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, fiber.StatusBadRequest)
	}
}
