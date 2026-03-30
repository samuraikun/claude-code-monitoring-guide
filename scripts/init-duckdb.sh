#!/bin/bash
# init-duckdb.sh - Initialize DuckDB database for Claude Code hook event logging
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
DB_DIR="$REPO_ROOT/data/duckdb"
DB_PATH="$DB_DIR/claude_hooks.db"

# Ensure directory exists
mkdir -p "$DB_DIR"

# Check duckdb is installed
if ! command -v duckdb &> /dev/null; then
  echo "Error: duckdb CLI is not installed. Install with: brew install duckdb" >&2
  exit 1
fi

echo "Initializing DuckDB at $DB_PATH ..."

duckdb "$DB_PATH" <<'SQL'
CREATE SEQUENCE IF NOT EXISTS event_seq START 1;

CREATE TABLE IF NOT EXISTS lifecycle_events (
    id                    INTEGER DEFAULT (nextval('event_seq')),
    timestamp             TIMESTAMP DEFAULT current_timestamp,

    -- Event identification
    hook_event_name       VARCHAR NOT NULL,
    event_type            VARCHAR NOT NULL,

    -- Common fields (all events)
    session_id            VARCHAR,
    cwd                   VARCHAR,
    project               VARCHAR,
    transcript_path       VARCHAR,

    -- Tool fields (PreToolUse, PostToolUse, PostToolUseFailure)
    tool_name             VARCHAR,
    tool_use_id           VARCHAR,
    tool_input_json       VARCHAR,
    tool_response_json    VARCHAR,

    -- Skill fields (PreToolUse matcher=Skill)
    skill_name            VARCHAR,
    skill_args            VARCHAR,

    -- Agent fields (PreToolUse matcher=Agent, SubagentStart, SubagentStop)
    agent_id              VARCHAR,
    agent_type            VARCHAR,
    agent_task            VARCHAR,
    agent_description     VARCHAR,

    -- Command fields (UserPromptSubmit)
    command_name          VARCHAR,
    prompt_text           VARCHAR,

    -- Session fields (SessionStart, SessionEnd)
    session_source        VARCHAR,
    session_end_reason    VARCHAR,
    model                 VARCHAR,

    -- Result fields
    success               BOOLEAN,
    error                 VARCHAR,
    is_interrupt          BOOLEAN,

    -- Stop fields
    stop_hook_active      BOOLEAN,
    last_assistant_message VARCHAR,

    -- Metadata
    permission_mode       VARCHAR,
    raw_json              VARCHAR
);

CREATE INDEX IF NOT EXISTS idx_events_timestamp ON lifecycle_events(timestamp);
CREATE INDEX IF NOT EXISTS idx_events_type ON lifecycle_events(event_type);
CREATE INDEX IF NOT EXISTS idx_events_session ON lifecycle_events(session_id);
CREATE INDEX IF NOT EXISTS idx_events_hook ON lifecycle_events(hook_event_name);
CREATE INDEX IF NOT EXISTS idx_events_tool ON lifecycle_events(tool_name);
SQL

echo "DuckDB initialized successfully at $DB_PATH"
echo "Tables:"
duckdb "$DB_PATH" "SHOW TABLES;"
