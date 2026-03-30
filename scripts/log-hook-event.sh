#!/bin/bash
# log-hook-event.sh - Log Claude Code hook events to DuckDB
#
# Usage: echo '{"session_id":"...","tool_name":"Skill",...}' | ./log-hook-event.sh <event_type>
#
# event_type: skill_invoke | agent_invoke | tool_use | tool_failure |
#             command_use | subagent_start | subagent_stop |
#             session_start | session_end | stop | stop_failure
#
# Environment: $CLAUDE_PROJECT_DIR is set by Claude Code for all hooks.

set -euo pipefail

EVENT_TYPE="${1:-unknown}"

# Resolve DB path: prefer CLAUDE_PROJECT_DIR (available in all hooks), fallback to script location
if [ -n "${CLAUDE_PROJECT_DIR:-}" ]; then
  DB_PATH="$CLAUDE_PROJECT_DIR/data/duckdb/claude_hooks.db"
else
  SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
  DB_PATH="$(cd "$SCRIPT_DIR/.." && pwd)/data/duckdb/claude_hooks.db"
fi
LOCK_FILE="${DB_PATH}.lock"

# Ensure DB directory exists
mkdir -p "$(dirname "$DB_PATH")"

# Read stdin (hook JSON input from Claude Code)
INPUT_JSON=$(cat)

# --- Extract common fields (present in all events per official docs) ---
HOOK_EVENT_NAME=$(echo "$INPUT_JSON" | jq -r '.hook_event_name // empty')
SESSION_ID=$(echo "$INPUT_JSON" | jq -r '.session_id // empty')
CWD=$(echo "$INPUT_JSON" | jq -r '.cwd // empty')
TRANSCRIPT_PATH=$(echo "$INPUT_JSON" | jq -r '.transcript_path // empty')
PERMISSION_MODE=$(echo "$INPUT_JSON" | jq -r '.permission_mode // empty')
PROJECT=$(basename "${CWD:-unknown}" 2>/dev/null || echo "unknown")

# --- Extract event-specific fields ---
TOOL_NAME=""
TOOL_USE_ID=""
TOOL_INPUT_JSON=""
TOOL_RESPONSE_JSON=""
SKILL_NAME=""
SKILL_ARGS=""
AGENT_ID=""
AGENT_TYPE=""
AGENT_TASK=""
AGENT_DESCRIPTION=""
COMMAND_NAME=""
PROMPT_TEXT=""
SESSION_SOURCE=""
SESSION_END_REASON=""
MODEL=""
SUCCESS=""
ERROR=""
IS_INTERRUPT=""
STOP_HOOK_ACTIVE=""
LAST_ASSISTANT_MSG=""

case "$EVENT_TYPE" in
  skill_invoke)
    # PreToolUse with matcher=Skill
    TOOL_NAME=$(echo "$INPUT_JSON" | jq -r '.tool_name // empty')
    TOOL_USE_ID=$(echo "$INPUT_JSON" | jq -r '.tool_use_id // empty')
    TOOL_INPUT_JSON=$(echo "$INPUT_JSON" | jq -c '.tool_input // empty' | head -c 4096)
    SKILL_NAME=$(echo "$INPUT_JSON" | jq -r '.tool_input.skill // empty')
    SKILL_ARGS=$(echo "$INPUT_JSON" | jq -r '.tool_input.args // empty')
    ;;
  agent_invoke)
    # PreToolUse with matcher=Agent
    TOOL_NAME=$(echo "$INPUT_JSON" | jq -r '.tool_name // empty')
    TOOL_USE_ID=$(echo "$INPUT_JSON" | jq -r '.tool_use_id // empty')
    TOOL_INPUT_JSON=$(echo "$INPUT_JSON" | jq -c '.tool_input // empty' | head -c 4096)
    AGENT_TYPE=$(echo "$INPUT_JSON" | jq -r '.tool_input.subagent_type // empty')
    AGENT_TASK=$(echo "$INPUT_JSON" | jq -r '.tool_input.prompt // empty' | head -c 4096)
    AGENT_DESCRIPTION=$(echo "$INPUT_JSON" | jq -r '.tool_input.description // empty')
    ;;
  tool_use)
    # PostToolUse (all tools)
    TOOL_NAME=$(echo "$INPUT_JSON" | jq -r '.tool_name // empty')
    TOOL_USE_ID=$(echo "$INPUT_JSON" | jq -r '.tool_use_id // empty')
    TOOL_INPUT_JSON=$(echo "$INPUT_JSON" | jq -c '.tool_input // empty' | head -c 4096)
    TOOL_RESPONSE_JSON=$(echo "$INPUT_JSON" | jq -c '.tool_response // empty' | head -c 4096)
    SUCCESS="true"
    ;;
  tool_failure)
    # PostToolUseFailure
    TOOL_NAME=$(echo "$INPUT_JSON" | jq -r '.tool_name // empty')
    TOOL_USE_ID=$(echo "$INPUT_JSON" | jq -r '.tool_use_id // empty')
    TOOL_INPUT_JSON=$(echo "$INPUT_JSON" | jq -c '.tool_input // empty' | head -c 4096)
    ERROR=$(echo "$INPUT_JSON" | jq -r '.error // empty' | head -c 4096)
    IS_INTERRUPT=$(echo "$INPUT_JSON" | jq -r '.is_interrupt // empty')
    SUCCESS="false"
    ;;
  command_use)
    # UserPromptSubmit (fires on every prompt, no matcher support)
    PROMPT_TEXT=$(echo "$INPUT_JSON" | jq -r '.prompt // empty' | head -c 4096)
    # Extract command name only if prompt starts with /
    COMMAND_NAME=$(echo "$PROMPT_TEXT" | grep -oE '^/[a-zA-Z0-9_:.-]+' || echo "")
    ;;
  subagent_start)
    # SubagentStart
    AGENT_ID=$(echo "$INPUT_JSON" | jq -r '.agent_id // empty')
    AGENT_TYPE=$(echo "$INPUT_JSON" | jq -r '.agent_type // empty')
    ;;
  subagent_stop)
    # SubagentStop
    AGENT_ID=$(echo "$INPUT_JSON" | jq -r '.agent_id // empty')
    AGENT_TYPE=$(echo "$INPUT_JSON" | jq -r '.agent_type // empty')
    LAST_ASSISTANT_MSG=$(echo "$INPUT_JSON" | jq -r '.last_assistant_message // empty' | head -c 4096)
    ;;
  session_start)
    # SessionStart
    SESSION_SOURCE=$(echo "$INPUT_JSON" | jq -r '.source // empty')
    MODEL=$(echo "$INPUT_JSON" | jq -r '.model // empty')
    ;;
  session_end)
    # SessionEnd
    SESSION_END_REASON=$(echo "$INPUT_JSON" | jq -r '.reason // empty')
    ;;
  stop)
    # Stop (no matcher support)
    STOP_HOOK_ACTIVE=$(echo "$INPUT_JSON" | jq -r '.stop_hook_active // empty')
    LAST_ASSISTANT_MSG=$(echo "$INPUT_JSON" | jq -r '.last_assistant_message // empty' | head -c 4096)
    # Prevent recursive loop: if stop_hook_active is true, just exit
    if [ "$STOP_HOOK_ACTIVE" = "true" ]; then
      exit 0
    fi
    ;;
  stop_failure)
    # StopFailure
    ERROR=$(echo "$INPUT_JSON" | jq -r '.error // empty')
    # StopFailure uses error_details for detailed info
    ERROR_DETAILS=$(echo "$INPUT_JSON" | jq -r '.error_details // empty')
    if [ -n "$ERROR_DETAILS" ]; then
      ERROR="${ERROR}: ${ERROR_DETAILS}"
    fi
    ;;
esac

# Truncate raw_json to 4096 chars
RAW_JSON=$(echo "$INPUT_JSON" | head -c 4096)

# --- Helper: escape single quotes for SQL ---
sql_escape() {
  echo "$1" | sed "s/'/''/g"
}

# --- Helper: format SQL value (NULL or quoted string) ---
sql_val() {
  local val="$1"
  if [ -z "$val" ]; then
    echo "NULL"
  else
    echo "'$(sql_escape "$val")'"
  fi
}

sql_bool() {
  local val="$1"
  if [ -z "$val" ]; then
    echo "NULL"
  elif [ "$val" = "true" ]; then
    echo "true"
  elif [ "$val" = "false" ]; then
    echo "false"
  else
    echo "NULL"
  fi
}

# --- Insert into DuckDB with file lock for concurrency ---
(
  flock -w 10 200 || { echo "Failed to acquire lock" >&2; exit 1; }

  duckdb "$DB_PATH" <<SQL
INSERT INTO lifecycle_events (
  hook_event_name, event_type,
  session_id, cwd, project, transcript_path,
  tool_name, tool_use_id, tool_input_json, tool_response_json,
  skill_name, skill_args,
  agent_id, agent_type, agent_task, agent_description,
  command_name, prompt_text,
  session_source, session_end_reason, model,
  success, error, is_interrupt,
  stop_hook_active, last_assistant_message,
  permission_mode, raw_json
) VALUES (
  $(sql_val "$HOOK_EVENT_NAME"), $(sql_val "$EVENT_TYPE"),
  $(sql_val "$SESSION_ID"), $(sql_val "$CWD"), $(sql_val "$PROJECT"), $(sql_val "$TRANSCRIPT_PATH"),
  $(sql_val "$TOOL_NAME"), $(sql_val "$TOOL_USE_ID"), $(sql_val "$TOOL_INPUT_JSON"), $(sql_val "$TOOL_RESPONSE_JSON"),
  $(sql_val "$SKILL_NAME"), $(sql_val "$SKILL_ARGS"),
  $(sql_val "$AGENT_ID"), $(sql_val "$AGENT_TYPE"), $(sql_val "$AGENT_TASK"), $(sql_val "$AGENT_DESCRIPTION"),
  $(sql_val "$COMMAND_NAME"), $(sql_val "$PROMPT_TEXT"),
  $(sql_val "$SESSION_SOURCE"), $(sql_val "$SESSION_END_REASON"), $(sql_val "$MODEL"),
  $(sql_bool "$SUCCESS"), $(sql_val "$ERROR"), $(sql_bool "$IS_INTERRUPT"),
  $(sql_bool "$STOP_HOOK_ACTIVE"), $(sql_val "$LAST_ASSISTANT_MSG"),
  $(sql_val "$PERMISSION_MODE"), $(sql_val "$RAW_JSON")
);
SQL
) 200>"$LOCK_FILE"

exit 0
