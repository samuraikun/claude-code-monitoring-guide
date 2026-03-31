package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTruncateJSON_SmallValue(t *testing.T) {
	value := map[string]string{"key": "small"}
	result := truncateJSON(value)

	var parsed map[string]string
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if parsed["key"] != "small" {
		t.Errorf("expected key=small, got %q", parsed["key"])
	}
}

func TestTruncateJSON_LargeValue(t *testing.T) {
	value := map[string]string{"data": strings.Repeat("x", 20000)}
	result := truncateJSON(value)

	if len(result) > maxFieldBytes {
		t.Errorf("truncated result exceeds maxFieldBytes: %d > %d", len(result), maxFieldBytes)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to unmarshal truncated result: %v", err)
	}
	if parsed["_truncated"] != true {
		t.Error("expected _truncated=true")
	}
	if _, ok := parsed["_original_size"]; !ok {
		t.Error("expected _original_size field")
	}
	if _, ok := parsed["content"]; !ok {
		t.Error("expected content field")
	}
}

func TestTruncateJSON_ExactBoundary(t *testing.T) {
	// Create a value that's exactly at the boundary
	value := strings.Repeat("a", maxFieldBytes-2) // -2 for JSON quotes
	result := truncateJSON(value)
	if len(result) <= maxFieldBytes {
		// Should pass through without truncation
		var parsed string
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
	}
}

func TestTruncateJSON_NilValue(t *testing.T) {
	result := truncateJSON(nil)
	if result != "null" {
		t.Errorf("expected null, got %q", result)
	}
}

func TestTruncateRawJSON_Small(t *testing.T) {
	raw := `{"key": "value"}`
	result := truncateRawJSON(raw)
	if result != raw {
		t.Errorf("expected passthrough, got %q", result)
	}
}

func TestTruncateRawJSON_Large(t *testing.T) {
	raw := `{"data":"` + strings.Repeat("x", 20000) + `"}`
	result := truncateRawJSON(raw)
	if len(result) > maxFieldBytes {
		t.Errorf("truncated result exceeds maxFieldBytes: %d > %d", len(result), maxFieldBytes)
	}
}
