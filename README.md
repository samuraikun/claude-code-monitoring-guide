# Claude Code Monitoring Guide

A guide for collecting and visualizing telemetry data from Claude Code to monitor costs, efficiency, and anomalies.

[日本語](README_JA.md)

## Architecture

```
Claude Code
  │  (OTLP gRPC / localhost:4317)
  ▼
OpenTelemetry Collector
  ├─ metrics ──▶ Prometheus (:9090)
  ├─ logs    ──▶ Loki (:3100)
  └─ traces  ──▶ Tempo (:3200)
                    ▲
                Grafana (:3000) ─ unified visualization across all data sources
```

Claude Code attaches a `prompt.id` (UUID v4) to every event. This ID is the key to tracing the entire processing flow from `user_prompt → api_request → tool_result`.

## Prerequisites

- Docker / Docker Compose
- Claude Code installed

## Setup

### 1. Start the Monitoring Stack

```bash
git clone https://github.com/samuraikun/claude-code-monitoring-guide
cd claude-code-monitoring-guide
docker compose up -d
```

Services started:

| Service | Port | Role |
|---|---|---|
| OpenTelemetry Collector | 4317 (gRPC), 4318 (HTTP) | Receives and forwards telemetry |
| Prometheus | 9090 | Metrics storage |
| Loki | 3100 | Log/event storage |
| Tempo | 3200 | Trace storage |
| Grafana | 3000 | Visualization (admin/admin) |

### 2. Configure Claude Code Telemetry

When you launch `claude` in this repository, telemetry is automatically enabled via `.claude/settings.json`.

```json
{
  "env": {
    "CLAUDE_CODE_ENABLE_TELEMETRY": "1",
    "OTEL_METRICS_EXPORTER": "otlp",
    "OTEL_LOGS_EXPORTER": "otlp",
    "OTEL_EXPORTER_OTLP_PROTOCOL": "grpc",
    "OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4317",
    "OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE": "cumulative"
  }
}
```

To use this in other repositories, add the same configuration to `~/.claude/settings.json` (global settings).

### 3. Verify

```bash
# Start Claude Code and do some work
claude

# Check if data is arriving at Loki
curl -s http://localhost:3100/loki/api/v1/labels | jq .

# View logs in Grafana: http://localhost:3000
# Explore → Loki → {service_name="claude-code"}
```

## Grafana Dashboards

After starting Grafana, navigate to `http://localhost:3000` and the following dashboards will be automatically provisioned in the **Claude Code** folder.

---

### Prompt Timeline

**UID**: `claude-code-prompt-timeline`

A trace-equivalent view showing all events for a single prompt in chronological order.

**How to use**:
1. Copy a `prompt_id` from the "Recent Prompts" panel
2. Paste it into the "Prompt ID" input at the top
3. Review the `user_prompt → api_request → tool_result` flow

**Panels**:

| Panel | Type | Description |
|---|---|---|
| Recent Prompts | Logs | List of recent prompts. Copy prompt_id from here |
| Prompt Event Timeline | Logs | All events for the selected prompt_id in chronological order |
| API Requests for Prompt | Table | Model, cost, token count, latency |
| Tool Executions for Prompt | Table | Tool name, success/failure, duration, result size |

---

### Developer Efficiency

**UID**: `claude-code-dev-efficiency`

Dashboard for understanding tool usage patterns, costs, and cache efficiency.

**Panels**:

| Panel | Type | Description |
|---|---|---|
| Tool Usage Distribution | Bar chart | Total executions per tool. High Bash ratio may indicate vague prompts |
| Bash vs Total Tool Executions | Time series | Ratio of Bash to total tool executions over time |
| Top 10 Sessions by Cost | Table | Highest-cost sessions (USD) |
| Top 10 Prompts by Turn Count | Table | Highest turn-count prompts. Over 10 may indicate loops or inefficiency |
| Cache Performance | Time series | Cache Read (green) vs Cache Creation (orange). More green = more efficient |
| Prompt Activity Over Time | Time series | Number of prompts over time (usage frequency patterns) |

---

### Anomaly & Health

**UID**: `claude-code-anomaly-health`

Dashboard for anomaly detection and health monitoring.

**Panels**:

| Panel | Type | Description |
|---|---|---|
| Tool Failure Rate | Time series | Tool failure count over time. Consecutive failures are a sign of needed intervention |
| API Request Duration (p50/p95) | Time series | p95 over 30s may indicate API delays or stuck processes |
| Loop Detection — High Turn Count | Table | Top 20 prompts by turn count. Over 10 requires review |
| Large Tool Results (> 100KB) | Logs | Detects unintended large data reads |
| Tool Failure Count by Tool Name | Table | Failure count per tool |
| Max Tool Result Size by Tool | Table | Maximum output size per tool |

---

### Working Dashboard

General-purpose dashboard showing basic metrics such as cost, tokens, and session count.

## File Structure

```
.
├── docker-compose.yml              # Monitoring stack definition
├── otel-collector-config.yaml      # OTEL Collector configuration
├── prometheus.yml                  # Prometheus scrape configuration
├── tempo.yaml                      # Tempo configuration
├── .claude/
│   └── settings.json               # Claude Code telemetry settings (auto-enabled)
├── grafana/
│   ├── dashboards/
│   │   ├── prompt-timeline.json    # Prompt Timeline dashboard
│   │   ├── developer-efficiency.json # Developer Efficiency dashboard
│   │   ├── anomaly-health.json     # Anomaly & Health dashboard
│   │   └── working-dashboard.json  # Basic metrics dashboard
│   └── provisioning/
│       ├── dashboards/
│       │   └── dashboards.yaml     # Dashboard auto-provisioning config
│       └── datasources/
│           └── datasources.yaml    # Data source config (Prometheus/Loki/Tempo)
├── claude_code_roi_full.md         # Detailed ROI measurement guide
├── troubleshooting.md              # Troubleshooting
└── report-generation-prompt.md    # Automated report generation prompt template
```

## Key Metrics and Log Events

Claude Code sends the following events via OTLP:

| Event | Description |
|---|---|
| `user_prompt` | Prompt submission. Includes `prompt_id` / `session_id` / `prompt_length` |
| `api_request` | API request. Includes `model` / `cost_usd` / `duration_ms` / `input_tokens` / `output_tokens` / `cache_read_tokens` / `cache_creation_tokens` |
| `tool_result` | Tool execution result. Includes `tool_name` / `success` / `duration_ms` / `tool_result_size_bytes` |

Basic Loki queries:

```logql
# View all events
{service_name="claude-code"} | json | drop __error__, __error_details__

# Trace events for a specific prompt_id
{service_name="claude-code"} | json | drop __error__, __error_details__ | prompt_id=`<your-prompt-id>`

# Filter only tool failures
{service_name="claude-code"} | json | drop __error__, __error_details__ | event_name=`tool_result` | success=`false`
```

## Troubleshooting

See [troubleshooting.md](troubleshooting.md) for details.

**Common issues**:

- **No data arriving at Loki**: Check that `OTEL_LOGS_EXPORTER=otlp` is set
- **Inaccurate Prometheus values**: `OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE=cumulative` is required
- **Telemetry not enabled**: Run `docker compose ps` to verify containers are running and `curl http://localhost:4317` to check the port is open

## Contributing

If you have improvements or additional use cases based on your implementation experience, please open an Issue or PR.

Original guide by [Kashyap Coimbatore Murali](https://www.linkedin.com/in/kashyap-murali/)
