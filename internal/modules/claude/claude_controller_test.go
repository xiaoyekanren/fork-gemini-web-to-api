package claude

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"gemini-web-to-api/internal/modules/claude/dto"

	"go.uber.org/zap"
)

func TestSendClaudeSSEEventWritesAnthropicEventName(t *testing.T) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)

	ok := sendClaudeSSEEvent(writer, zap.NewNop(), dto.StreamEvent{
		Type:  "content_block_delta",
		Index: 1,
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
