package utils

import (
	"encoding/json"
	"testing"
)

func TestNormalizeToolInputMapUnwrapsMarkdownURL(t *testing.T) {
	input := map[string]interface{}{
		"url":    "[https://example.com/a?id=1](https://example.com/a?id=1)",
		"prompt": "keep [this](not-a-url) as text",
		"nested": map[string]interface{}{
			"source_url": "[docs](https://example.com/docs)",
		},
	}

	got := NormalizeToolInputMap(input)

	if got["url"] != "https://example.com/a?id=1" {
		t.Fatalf("url was not normalized: %#v", got["url"])
	}
	if got["prompt"] != input["prompt"] {
		t.Fatalf("non-url text was changed: %#v", got["prompt"])
	}

	nested, ok := got["nested"].(map[string]interface{})
	if !ok {
		t.Fatalf("nested value has unexpected type: %T", got["nested"])
	}
	if nested["source_url"] != "https://example.com/docs" {
		t.Fatalf("nested URL was not normalized: %#v", nested["source_url"])
	}
}

func TestNormalizeToolArgumentsJSONPreservesInvalidJSON(t *testing.T) {
	raw := json.RawMessage(`{"url":"[link](https://example.com)"}`)
	got := NormalizeToolArgumentsJSON(raw)

	var decoded map[string]string
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("normalized JSON did not decode: %v", err)
	}
	if decoded["url"] != "https://example.com" {
		t.Fatalf("url was not normalized: %#v", decoded["url"])
	}

	invalid := json.RawMessage(`not-json`)
	if string(NormalizeToolArgumentsJSON(invalid)) != string(invalid) {
		t.Fatal("invalid JSON should be preserved")
	}
}
