package main

import (
	"encoding/json"
	"testing"
)

func TestHandleSessionStart(t *testing.T) {
	input := HookInput{
		"hook_event_name": "SessionStart",
		"session_id":      "sess-123",
		"source":          "startup",
		"model":           "claude-opus-4-6",
		"cwd":             "/workspace",
	}

	ev := handleSessionStart(input)
	if ev.EventType != "session_start" {
		t.Errorf("expected event_type=session_start, got %q", ev.EventType)
	}
	if ev.SessionID != "sess-123" {
		t.Errorf("expected session_id=sess-123, got %q", ev.SessionID)
	}
	if ev.Model == nil || *ev.Model != "claude-opus-4-6" {
		t.Error("expected model=claude-opus-4-6")
	}
	if ev.Cwd == nil || *ev.Cwd != "/workspace" {
		t.Error("expected cwd=/workspace")
	}
}

func TestHandleSessionEnd(t *testing.T) {
	input := HookInput{
		"hook_event_name": "SessionEnd",
		"session_id":      "sess-123",
		"source":          "prompt_input_exit",
	}

	ev := handleSessionEnd(input)
	if ev.EventType != "session_end" {
		t.Errorf("expected event_type=session_end, got %q", ev.EventType)
	}
	if ev.Source == nil || *ev.Source != "prompt_input_exit" {
		t.Error("expected source=prompt_input_exit")
	}
}

func TestHandleUserPromptSubmit(t *testing.T) {
	input := HookInput{
		"hook_event_name": "UserPromptSubmit",
		"session_id":      "sess-123",
		"prompt":          "fix the login bug",
	}

	ev := handleUserPromptSubmit(input)
	if ev.EventType != "user_prompt" {
		t.Errorf("expected event_type=user_prompt, got %q", ev.EventType)
	}
	if ev.PromptText == nil || *ev.PromptText != "fix the login bug" {
		t.Error("expected prompt_text='fix the login bug'")
	}
	if ev.DetectedCommand != nil {
		t.Error("expected no detected_command for non-slash prompt")
	}
}

func TestHandleUserPromptSubmit_SlashCommand(t *testing.T) {
	input := HookInput{
		"hook_event_name": "UserPromptSubmit",
		"session_id":      "sess-123",
		"prompt":          "/commit fix login flow",
	}

	ev := handleUserPromptSubmit(input)
	if ev.DetectedCommand == nil || *ev.DetectedCommand != "/commit" {
		t.Errorf("expected detected_command=/commit, got %v", ev.DetectedCommand)
	}
}

func TestHandlePreToolUse_Skill(t *testing.T) {
	input := HookInput{
		"hook_event_name": "PreToolUse",
		"session_id":      "sess-123",
		"tool_name":       "Skill",
		"tool_input":      map[string]any{"skill": "commit", "args": "-m test"},
	}

	ev := handlePreToolUse(input)
	if ev.EventType != "skill_invoke" {
		t.Errorf("expected event_type=skill_invoke, got %q", ev.EventType)
	}
	if ev.SkillName == nil || *ev.SkillName != "commit" {
		t.Error("expected skill_name=commit")
	}
	if ev.SkillArgs == nil || *ev.SkillArgs != "-m test" {
		t.Error("expected skill_args='-m test'")
	}
}

func TestHandlePreToolUse_Agent(t *testing.T) {
	input := HookInput{
		"hook_event_name": "PreToolUse",
		"session_id":      "sess-123",
		"tool_name":       "Agent",
		"tool_input":      map[string]any{"subagent_type": "Explore", "model": "sonnet", "prompt": "search for auth"},
	}

	ev := handlePreToolUse(input)
	if ev.EventType != "agent_invoke" {
		t.Errorf("expected event_type=agent_invoke, got %q", ev.EventType)
	}
	if ev.AgentType == nil || *ev.AgentType != "Explore" {
		t.Error("expected agent_type=Explore")
	}
	if ev.SubagentModel == nil || *ev.SubagentModel != "sonnet" {
		t.Error("expected subagent_model=sonnet")
	}
}

func TestHandlePreToolUse_Unknown(t *testing.T) {
	input := HookInput{
		"hook_event_name": "PreToolUse",
		"session_id":      "sess-123",
		"tool_name":       "Read",
		"tool_input":      map[string]any{"file_path": "/tmp/test.txt"},
	}

	ev := handlePreToolUse(input)
	if ev != nil {
		t.Error("expected nil for non-Skill/Agent PreToolUse")
	}
}

func TestHandlePostToolUse_Bash(t *testing.T) {
	input := HookInput{
		"hook_event_name": "PostToolUse",
		"session_id":      "sess-123",
		"tool_name":       "Bash",
		"tool_input":      map[string]any{"command": "git status"},
		"tool_response":   "On branch main\nnothing to commit",
	}

	ev := handlePostToolUse(input)
	if ev.EventType != "tool_use" {
		t.Errorf("expected event_type=tool_use, got %q", ev.EventType)
	}
	if ev.ToolName == nil || *ev.ToolName != "Bash" {
		t.Error("expected tool_name=Bash")
	}
	if ev.ToolInput == nil {
		t.Error("expected tool_input to be set")
	}
	if ev.ToolResponse == nil {
		t.Error("expected tool_response to be set")
	}
}

func TestHandlePostToolUse_NilResponse(t *testing.T) {
	input := HookInput{
		"hook_event_name": "PostToolUse",
		"session_id":      "sess-123",
		"tool_name":       "Read",
		"tool_input":      map[string]any{"file_path": "/tmp/test.txt"},
	}

	ev := handlePostToolUse(input)
	if ev.ToolResponse != nil {
		t.Error("expected tool_response=nil when not provided")
	}
}

func TestHandleSubagentStart(t *testing.T) {
	input := HookInput{
		"hook_event_name": "SubagentStart",
		"session_id":      "sess-123",
		"agent_id":        "agent-001",
		"agent_type":      "Explore",
	}

	ev := handleSubagentStart(input)
	if ev.EventType != "agent_start" {
		t.Errorf("expected event_type=agent_start, got %q", ev.EventType)
	}
	if ev.AgentID == nil || *ev.AgentID != "agent-001" {
		t.Error("expected agent_id=agent-001")
	}
}

func TestHandleSubagentStop(t *testing.T) {
	input := HookInput{
		"hook_event_name": "SubagentStop",
		"session_id":      "sess-123",
		"agent_id":        "agent-001",
		"agent_type":      "Explore",
		"agent_transcript_path": "/tmp/transcript.jsonl",
	}

	ev := handleSubagentStop(input)
	if ev.EventType != "agent_stop" {
		t.Errorf("expected event_type=agent_stop, got %q", ev.EventType)
	}
	if ev.TranscriptPath == nil || *ev.TranscriptPath != "/tmp/transcript.jsonl" {
		t.Error("expected transcript_path=/tmp/transcript.jsonl")
	}
}

func TestHandleStop(t *testing.T) {
	input := HookInput{
		"hook_event_name":        "Stop",
		"session_id":             "sess-123",
		"last_assistant_message": "Done!",
	}

	ev := handleStop(input)
	if ev.EventType != "stop" {
		t.Errorf("expected event_type=stop, got %q", ev.EventType)
	}
	if ev.LastMessage == nil || *ev.LastMessage != "Done!" {
		t.Error("expected last_message='Done!'")
	}
	// token_usage may be nil if transcript not found — that's expected
}

func TestLifecycleEvent_JSON_OmitsEmpty(t *testing.T) {
	ev := &LifecycleEvent{
		EventType:      "session_start",
		EventTimestamp: "2026-03-30T00:00:00Z",
		SessionID:      "sess-123",
	}

	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]any
	json.Unmarshal(data, &parsed)

	// Fields with omitempty should not appear
	for _, key := range []string{"tool_name", "tool_input", "tool_response", "token_usage", "model", "cwd"} {
		if _, ok := parsed[key]; ok {
			t.Errorf("expected %q to be omitted from JSON", key)
		}
	}
}
