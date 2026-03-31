package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseTranscriptFile(t *testing.T) {
	dir := t.TempDir()
	transcriptPath := filepath.Join(dir, "test-session.jsonl")

	entries := []map[string]any{
		{"type": "user", "message": map[string]any{"content": "hello"}},
		{
			"type": "assistant",
			"message": map[string]any{
				"usage": map[string]any{
					"input_tokens":                  100,
					"output_tokens":                 50,
					"cache_creation_input_tokens":   10,
					"cache_read_input_tokens":       20,
				},
			},
		},
		{
			"type": "assistant",
			"message": map[string]any{
				"usage": map[string]any{
					"input_tokens":                  200,
					"output_tokens":                 30,
					"cache_creation_input_tokens":   5,
					"cache_read_input_tokens":       15,
				},
			},
		},
	}

	f, err := os.Create(transcriptPath)
	if err != nil {
		t.Fatal(err)
	}
	enc := json.NewEncoder(f)
	for _, e := range entries {
		enc.Encode(e)
	}
	f.Close()

	usage := parseTranscriptFile(transcriptPath)
	if usage == nil {
		t.Fatal("expected non-nil usage")
	}

	if usage.APICalls != 2 {
		t.Errorf("expected 2 api_calls, got %d", usage.APICalls)
	}
	if usage.InputTokens != 300 {
		t.Errorf("expected 300 input_tokens, got %d", usage.InputTokens)
	}
	if usage.OutputTokens != 80 {
		t.Errorf("expected 80 output_tokens, got %d", usage.OutputTokens)
	}
	if usage.CacheCreationTokens != 15 {
		t.Errorf("expected 15 cache_creation_tokens, got %d", usage.CacheCreationTokens)
	}
	if usage.CacheReadTokens != 35 {
		t.Errorf("expected 35 cache_read_tokens, got %d", usage.CacheReadTokens)
	}
	if usage.TotalTokens != 430 {
		t.Errorf("expected 430 total_tokens, got %d", usage.TotalTokens)
	}
}

func TestParseTranscriptFile_NoAssistant(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "no-assistant.jsonl")
	f, _ := os.Create(path)
	json.NewEncoder(f).Encode(map[string]any{"type": "user", "message": map[string]any{"content": "hello"}})
	f.Close()

	usage := parseTranscriptFile(path)
	if usage != nil {
		t.Error("expected nil usage when no assistant entries")
	}
}

func TestParseTranscriptFile_Missing(t *testing.T) {
	usage := parseTranscriptFile("/nonexistent/path.jsonl")
	if usage != nil {
		t.Error("expected nil usage for missing file")
	}
}

func TestParseTranscriptUsage_EmptySessionID(t *testing.T) {
	usage := parseTranscriptUsage("")
	if usage != nil {
		t.Error("expected nil usage for empty session ID")
	}
}
