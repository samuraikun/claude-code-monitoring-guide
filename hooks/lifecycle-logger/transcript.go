package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
)

// TokenUsage holds aggregated token counts from a session transcript.
type TokenUsage struct {
	APICalls             int `json:"api_calls"`
	InputTokens          int `json:"input_tokens"`
	OutputTokens         int `json:"output_tokens"`
	CacheCreationTokens  int `json:"cache_creation_input_tokens"`
	CacheReadTokens      int `json:"cache_read_input_tokens"`
	TotalTokens          int `json:"total_tokens"`
}

// transcriptEntry represents a single line in a transcript JSONL file.
type transcriptEntry struct {
	Type    string `json:"type"`
	Message struct {
		Usage *struct {
			InputTokens             int `json:"input_tokens"`
			OutputTokens            int `json:"output_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// parseTranscriptUsage searches for the transcript file matching sessionID
// under ~/.claude/projects/ and sums token usage from assistant messages.
// Returns nil if the file is not found or on any error.
func parseTranscriptUsage(sessionID string) *TokenUsage {
	if sessionID == "" {
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	pattern := filepath.Join(home, ".claude", "projects", "*", sessionID+".jsonl")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return nil
	}

	return parseTranscriptFile(matches[0])
}

// parseTranscriptFile reads a transcript JSONL file and sums token usage.
func parseTranscriptFile(path string) *TokenUsage {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	usage := &TokenUsage{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry transcriptEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		if entry.Type != "assistant" || entry.Message.Usage == nil {
			continue
		}

		u := entry.Message.Usage
		usage.APICalls++
		usage.InputTokens += u.InputTokens
		usage.OutputTokens += u.OutputTokens
		usage.CacheCreationTokens += u.CacheCreationInputTokens
		usage.CacheReadTokens += u.CacheReadInputTokens
	}

	if usage.APICalls == 0 {
		return nil
	}

	usage.TotalTokens = usage.InputTokens + usage.OutputTokens +
		usage.CacheCreationTokens + usage.CacheReadTokens
	return usage
}
