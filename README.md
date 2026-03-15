# Claude Code Monitoring Guide

Claude Code のテレメトリデータを収集・可視化し、コスト・効率・異常を監視するためのガイドです。

## アーキテクチャ

```
Claude Code
  │  (OTLP gRPC / localhost:4317)
  ▼
OpenTelemetry Collector
  ├─ metrics ──▶ Prometheus (:9090)
  ├─ logs    ──▶ Loki (:3100)
  └─ traces  ──▶ Tempo (:3200)
                    ▲
                Grafana (:3000) ─ 全データソースを横断可視化
```

Claude Code は `prompt.id`（UUID v4）をすべてのイベントに付与して送信します。この ID をキーに `user_prompt → api_request → tool_result` の処理フロー全体をトレースできます。

## 前提条件

- Docker / Docker Compose
- Claude Code がインストール済み

## セットアップ

### 1. 監視スタックの起動

```bash
git clone https://github.com/samuraikun/claude-code-monitoring-guide
cd claude-code-monitoring-guide
docker compose up -d
```

起動するサービス:

| サービス | ポート | 役割 |
|---|---|---|
| OpenTelemetry Collector | 4317 (gRPC), 4318 (HTTP) | テレメトリ受信・転送 |
| Prometheus | 9090 | メトリクス保存 |
| Loki | 3100 | ログ/イベント保存 |
| Tempo | 3200 | トレース保存 |
| Grafana | 3000 | 可視化 (admin/admin) |

### 2. Claude Code のテレメトリ設定

このリポジトリで `claude` を起動すると `.claude/settings.json` により自動でテレメトリが有効になります。

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

他のリポジトリで使う場合は `~/.claude/settings.json`（グローバル設定）に同じ内容を追記します。

### 3. 動作確認

```bash
# Claude Code を起動して何か作業する
claude

# Loki にデータが届いているか確認
curl -s http://localhost:3100/loki/api/v1/labels | jq .

# Grafana でログを確認: http://localhost:3000
# Explore → Loki → {service_name="claude-code"}
```

## Grafana ダッシュボード

Grafana 起動後、`http://localhost:3000` にアクセスすると **Claude Code** フォルダに以下のダッシュボードが自動プロビジョニングされています。

---

### Prompt Timeline

**UID**: `claude-code-prompt-timeline`

1つのプロンプト処理の全イベントを時系列で確認する、トレース相当のビューです。

**使い方**:
1. "Recent Prompts" パネルで調査したい `prompt_id` をコピー
2. 画面上部の "Prompt ID" 入力欄に貼り付け
3. `user_prompt → api_request → tool_result` の流れを確認

**パネル構成**:

| パネル | タイプ | 内容 |
|---|---|---|
| Recent Prompts | Logs | 直近のプロンプト一覧。prompt_id をここからコピー |
| Prompt Event Timeline | Logs | 選択した prompt_id の全イベントを時系列表示 |
| API Requests for Prompt | Table | モデル・コスト・トークン数・レイテンシ |
| Tool Executions for Prompt | Table | ツール名・成否・実行時間・結果サイズ |

---

### Developer Efficiency

**UID**: `claude-code-dev-efficiency`

ツール使用パターン・コスト・キャッシュ効率を把握するダッシュボードです。

**パネル構成**:

| パネル | タイプ | 内容 |
|---|---|---|
| Tool Usage Distribution | Bar chart | ツール別の総実行回数。Bash 比率が高い場合はプロンプトの曖昧さを疑う |
| Bash vs Total Tool Executions | Time series | Bash と全ツール実行の比率推移 |
| Top 10 Sessions by Cost | Table | コスト上位セッション（USD） |
| Top 10 Prompts by Turn Count | Table | ターン数上位プロンプト。10回超はループや非効率の可能性 |
| Cache Performance | Time series | Cache Read（緑）と Cache Creation（橙）の比率。緑が多いほど効率的 |
| Prompt Activity Over Time | Time series | 時系列でのプロンプト数（使用頻度のパターン把握） |

---

### Anomaly & Health

**UID**: `claude-code-anomaly-health`

異常検知と健全性監視のダッシュボードです。

**パネル構成**:

| パネル | タイプ | 内容 |
|---|---|---|
| Tool Failure Rate | Time series | ツール失敗数の推移。連続失敗は手動介入のサイン |
| API Request Duration (p50/p95) | Time series | p95 が 30 秒超は API 遅延またはスタックの疑い |
| Loop Detection — High Turn Count | Table | ターン数上位 20 プロンプト。10 回超は要確認 |
| Large Tool Results (> 100KB) | Logs | 意図しない大量データ読み出しの検知 |
| Tool Failure Count by Tool Name | Table | ツール別の失敗回数 |
| Max Tool Result Size by Tool | Table | ツール別の最大出力サイズ |

---

### Working Dashboard（元のダッシュボード）

コスト・トークン・セッション数などの基本メトリクスを表示する汎用ダッシュボードです。

## ファイル構成

```
.
├── docker-compose.yml              # 監視スタック定義
├── otel-collector-config.yaml      # OTEL Collector 設定
├── prometheus.yml                  # Prometheus スクレイプ設定
├── tempo.yaml                      # Tempo 設定
├── .claude/
│   └── settings.json               # Claude Code テレメトリ設定（自動有効化）
├── grafana/
│   ├── dashboards/
│   │   ├── prompt-timeline.json    # Prompt Timeline ダッシュボード
│   │   ├── developer-efficiency.json # Developer Efficiency ダッシュボード
│   │   ├── anomaly-health.json     # Anomaly & Health ダッシュボード
│   │   └── working-dashboard.json  # 基本メトリクスダッシュボード
│   └── provisioning/
│       ├── dashboards/
│       │   └── dashboards.yaml     # ダッシュボード自動プロビジョニング設定
│       └── datasources/
│           └── datasources.yaml    # データソース設定（Prometheus/Loki/Tempo）
├── claude_code_roi_full.md         # ROI 計測の詳細ガイド
├── troubleshooting.md              # トラブルシューティング
└── report-generation-prompt.md    # 自動レポート生成プロンプトテンプレート
```

## 主要メトリクスとログイベント

Claude Code は以下のイベントを OTLP で送信します：

| イベント | 用途 |
|---|---|
| `user_prompt` | プロンプト送信。`prompt_id` / `session_id` / `prompt_length` を含む |
| `api_request` | API リクエスト。`model` / `cost_usd` / `duration_ms` / `input_tokens` / `output_tokens` / `cache_read_tokens` / `cache_creation_tokens` を含む |
| `tool_result` | ツール実行結果。`tool_name` / `success` / `duration_ms` / `tool_result_size_bytes` を含む |

Loki での基本クエリ:

```logql
# 全イベントを確認
{service_name="claude-code"} | json | drop __error__, __error_details__

# 特定 prompt_id のイベントをトレース
{service_name="claude-code"} | json | drop __error__, __error_details__ | prompt_id=`<your-prompt-id>`

# ツール失敗のみ抽出
{service_name="claude-code"} | json | drop __error__, __error_details__ | event_name=`tool_result` | success=`false`
```

## トラブルシューティング

詳細は [troubleshooting.md](troubleshooting.md) を参照してください。

**よくある問題**:

- **Loki にデータが届かない**: `OTEL_LOGS_EXPORTER=otlp` が設定されているか確認
- **Prometheus の値が不正確**: `OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE=cumulative` が必要
- **テレメトリが有効にならない**: `docker compose ps` でコンテナが起動しているか確認し、`curl http://localhost:4317` でポートが開いているか確認

## Contributing

実装経験に基づいた改善や追加ユースケースがあれば、Issue / PR をお送りください。

Original guide by [Kashyap Coimbatore Murali](https://www.linkedin.com/in/kashyap-murali/)
