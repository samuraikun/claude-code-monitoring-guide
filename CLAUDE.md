# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This repository is a monitoring guide for Claude Code telemetry. It provides a Docker Compose-based observability stack that collects, stores, and visualizes telemetry data from Claude Code sessions.

## Stack Management

```bash
# Start the monitoring stack
docker compose up -d

# Stop the stack
docker compose down

# View logs from the OTEL Collector
docker logs otel-collector

# Check container status
docker compose ps
```

## Architecture

```
Claude Code
  │  (OTLP gRPC / localhost:4317)
  ▼
OpenTelemetry Collector  (otel-collector-config.yaml)
  ├─ metrics ──▶ Prometheus (:9090)
  ├─ logs    ──▶ Loki (:3100)
  └─ traces  ──▶ Tempo (:3200)
                    ▲
                Grafana (:3000) — auto-provisioned dashboards
```

**Key data flow**: Claude Code attaches a `prompt_id` (UUID v4) to every event, enabling end-to-end tracing from `user_prompt → api_request → tool_result`.

## Telemetry Configuration

Telemetry is auto-enabled when running `claude` in this repo via `.claude/settings.json`. The critical env vars are:

- `CLAUDE_CODE_ENABLE_TELEMETRY=1` — enables telemetry export
- `OTEL_LOGS_EXPORTER=otlp` — required for Loki event collection (tool_result, api_error, etc.)
- `OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE=cumulative` — required for correct Prometheus values; without this, delta values break `sum()` and `increase()` queries

To enable globally, copy the same config to `~/.claude/settings.json`.

## Grafana Dashboards

Access Grafana at `http://localhost:3000` (admin/admin). Dashboards are auto-provisioned from `grafana/dashboards/`:

| Dashboard | UID | Purpose |
|---|---|---|
| Prompt Timeline | `claude-code-prompt-timeline` | Trace all events for a single `prompt_id` |
| Session Timeline | `claude-code-session-timeline` | Drill down into a specific session |
| Developer Efficiency | `claude-code-dev-efficiency` | Tool usage, costs, cache hit rates, Fast Mode, model cost efficiency |
| Anomaly & Health | `claude-code-anomaly-health` | Failure rates, loop detection, P99 latency, rate limits |
| Working Dashboard | `claude-code-working` | Basic cost/token/session metrics, DAU, cache hit rate |
| ROI & Productivity | `claude-code-roi-productivity` | Cost per commit/PR/LOC, cache efficiency, user productivity |
| Adoption & Usage Patterns | `claude-code-adoption-usage` | DAU/WAU/MAU, Fast Mode adoption, terminal/IDE distribution |

## Verifying Data Collection

```bash
# Check Loki is receiving data
curl -s http://localhost:3100/loki/api/v1/labels | jq .

# Check Prometheus targets
# Open: http://localhost:9090/targets

# Test OTEL Collector is reachable
curl -v http://localhost:4317

# Test telemetry with console output (no stack needed)
OTEL_METRICS_EXPORTER=console OTEL_METRIC_EXPORT_INTERVAL=1000 claude -p "test"
```

## Key LogQL Queries (Loki)

```logql
# All Claude Code events
{service_name="claude-code"} | json | drop __error__, __error_details__

# Events for a specific prompt
{service_name="claude-code"} | json | drop __error__, __error_details__ | prompt_id=`<prompt-id>`

# Tool failures only
{service_name="claude-code"} | json | drop __error__, __error_details__ | event_name=`tool_result` | success=`false`
```

## Event Schema

| Event | Key Fields |
|---|---|
| `user_prompt` | `prompt_id`, `session_id`, `prompt_length` |
| `api_request` | `model`, `cost_usd`, `duration_ms`, `input_tokens`, `output_tokens`, `cache_read_tokens`, `cache_creation_tokens` |
| `tool_result` | `tool_name`, `success`, `duration_ms`, `tool_result_size_bytes` |
