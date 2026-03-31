package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"
)

// HookInput represents the JSON received from Claude Code hooks on stdin.
type HookInput map[string]any

// LifecycleEvent is a single event written to events.jsonl.
type LifecycleEvent struct {
	EventType       string  `json:"event_type"`
	EventTimestamp  string  `json:"event_timestamp"`
	SessionID       string  `json:"session_id"`
	Source          *string `json:"source,omitempty"`
	Model           *string `json:"model,omitempty"`
	Cwd             *string `json:"cwd,omitempty"`
	PromptText      *string `json:"prompt_text,omitempty"`
	DetectedCommand *string `json:"detected_command,omitempty"`
	SkillName       *string `json:"skill_name,omitempty"`
	SkillArgs       *string `json:"skill_args,omitempty"`
	AgentID         *string `json:"agent_id,omitempty"`
	AgentType       *string `json:"agent_type,omitempty"`
	SubagentModel   *string `json:"subagent_model,omitempty"`
	AgentPrompt     *string `json:"agent_prompt,omitempty"`
	TranscriptPath  *string `json:"transcript_path,omitempty"`
	LastMessage     *string `json:"last_message,omitempty"`
	// New fields
	ToolName     *string `json:"tool_name,omitempty"`
	ToolInput    *string `json:"tool_input,omitempty"`
	ToolResponse *string `json:"tool_response,omitempty"`
	TokenUsage   *string `json:"token_usage,omitempty"`
}

func main() {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil || len(raw) == 0 {
		return
	}

	var input HookInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return
	}

	eventName, _ := input["hook_event_name"].(string)
	if eventName == "" {
		return
	}

	var event *LifecycleEvent
	switch eventName {
	case "SessionStart":
		event = handleSessionStart(input)
	case "SessionEnd":
		event = handleSessionEnd(input)
	case "UserPromptSubmit":
		event = handleUserPromptSubmit(input)
	case "PreToolUse":
		event = handlePreToolUse(input)
	case "PostToolUse":
		event = handlePostToolUse(input)
	case "SubagentStart":
		event = handleSubagentStart(input)
	case "SubagentStop":
		event = handleSubagentStop(input)
	case "Stop":
		event = handleStop(input)
	default:
		return
	}

	if event == nil {
		return
	}

	writeEvent(event)
}

func handleSessionStart(input HookInput) *LifecycleEvent {
	return &LifecycleEvent{
		EventType:      "session_start",
		EventTimestamp: now(),
		SessionID:      getString(input, "session_id"),
		Source:         getStringPtr(input, "source"),
		Model:          getStringPtr(input, "model"),
		Cwd:            getStringPtr(input, "cwd"),
	}
}

func handleSessionEnd(input HookInput) *LifecycleEvent {
	return &LifecycleEvent{
		EventType:      "session_end",
		EventTimestamp: now(),
		SessionID:      getString(input, "session_id"),
		Source:         getStringPtr(input, "source"),
	}
}

func handleUserPromptSubmit(input HookInput) *LifecycleEvent {
	prompt := getString(input, "prompt")
	ev := &LifecycleEvent{
		EventType:      "user_prompt",
		EventTimestamp: now(),
		SessionID:      getString(input, "session_id"),
		PromptText:     strPtr(truncateStr(prompt, 500)),
	}

	// Detect slash commands (e.g., /commit, /review)
	if len(prompt) > 1 && prompt[0] == '/' && isAlpha(prompt[1]) {
		cmd := prompt
		for i, c := range prompt[1:] {
			if c == ' ' || c == '\n' {
				cmd = prompt[:i+1]
				break
			}
		}
		ev.DetectedCommand = strPtr(cmd)
	}

	return ev
}

func handlePreToolUse(input HookInput) *LifecycleEvent {
	toolName := getString(input, "tool_name")
	toolInput, _ := input["tool_input"].(map[string]any)

	switch toolName {
	case "Skill":
		skillName, _ := toolInput["skill"].(string)
		skillArgs, _ := toolInput["args"].(string)
		return &LifecycleEvent{
			EventType:      "skill_invoke",
			EventTimestamp: now(),
			SessionID:      getString(input, "session_id"),
			SkillName:      strPtr(skillName),
			SkillArgs:      strPtr(skillArgs),
		}
	case "Agent":
		agentType, _ := toolInput["subagent_type"].(string)
		if agentType == "" {
			agentType = "general-purpose"
		}
		subModel, _ := toolInput["model"].(string)
		agentPrompt, _ := toolInput["prompt"].(string)
		return &LifecycleEvent{
			EventType:      "agent_invoke",
			EventTimestamp: now(),
			SessionID:      getString(input, "session_id"),
			AgentType:      strPtr(agentType),
			SubagentModel:  strPtr(subModel),
			AgentPrompt:    strPtr(truncateStr(agentPrompt, 500)),
		}
	default:
		return nil
	}
}

func handlePostToolUse(input HookInput) *LifecycleEvent {
	toolName := getString(input, "tool_name")

	ev := &LifecycleEvent{
		EventType:      "tool_use",
		EventTimestamp: now(),
		SessionID:      getString(input, "session_id"),
		ToolName:       strPtr(toolName),
	}

	// Truncate and store tool_input
	if toolInput, ok := input["tool_input"]; ok && toolInput != nil {
		truncated := truncateJSON(toolInput)
		ev.ToolInput = &truncated
	}

	// Truncate and store tool_response
	if toolResp, ok := input["tool_response"]; ok && toolResp != nil {
		truncated := truncateJSON(toolResp)
		ev.ToolResponse = &truncated
	}

	return ev
}

func handleSubagentStart(input HookInput) *LifecycleEvent {
	return &LifecycleEvent{
		EventType:      "agent_start",
		EventTimestamp: now(),
		SessionID:      getString(input, "session_id"),
		AgentID:        getStringPtr(input, "agent_id"),
		AgentType:      getStringPtr(input, "agent_type"),
	}
}

func handleSubagentStop(input HookInput) *LifecycleEvent {
	return &LifecycleEvent{
		EventType:      "agent_stop",
		EventTimestamp: now(),
		SessionID:      getString(input, "session_id"),
		AgentID:        getStringPtr(input, "agent_id"),
		AgentType:      getStringPtr(input, "agent_type"),
		TranscriptPath: getStringPtr(input, "agent_transcript_path"),
	}
}

func handleStop(input HookInput) *LifecycleEvent {
	sessionID := getString(input, "session_id")
	lastMsg := getString(input, "last_assistant_message")

	ev := &LifecycleEvent{
		EventType:      "stop",
		EventTimestamp: now(),
		SessionID:      sessionID,
		LastMessage:    strPtr(truncateStr(lastMsg, 500)),
	}

	// Parse transcript for token usage
	if usage := parseTranscriptUsage(sessionID); usage != nil {
		usageJSON, err := json.Marshal(usage)
		if err == nil {
			s := string(usageJSON)
			ev.TokenUsage = &s
		}
	}

	return ev
}

// writeEvent appends a JSON line to events.jsonl.
func writeEvent(event *LifecycleEvent) {
	projectDir := os.Getenv("CLAUDE_PROJECT_DIR")
	if projectDir == "" {
		// Fallback: derive from this binary's location (hooks/lifecycle-logger -> project root)
		exe, err := os.Executable()
		if err != nil {
			return
		}
		projectDir = filepath.Dir(filepath.Dir(exe))
	}

	dataDir := filepath.Join(projectDir, "data", "lifecycle")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return
	}

	jsonlFile := filepath.Join(dataDir, "events.jsonl")
	line, err := json.Marshal(event)
	if err != nil {
		return
	}
	line = append(line, '\n')

	f, err := os.OpenFile(jsonlFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	f.Write(line)
}

// Helper functions

func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func getString(m HookInput, key string) string {
	v, _ := m[key].(string)
	return v
}

func getStringPtr(m HookInput, key string) *string {
	v, ok := m[key].(string)
	if !ok || v == "" {
		return nil
	}
	return &v
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

func isAlpha(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// FormatEventJSON exports event as JSON bytes for testing.
func FormatEventJSON(event *LifecycleEvent) ([]byte, error) {
	return json.Marshal(event)
}
