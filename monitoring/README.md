# Monitoring & Observability

Aurora Gateway ships with built-in Prometheus metrics and optional telemetry exports
for external observability platforms. This directory contains Grafana dashboard
provisioning config; the root `prometheus.yml` provides the Prometheus scrape config.

---

## Prometheus (OSS, fully supported)

### Enable metrics

```yaml
# config.yaml
metrics:
  enabled: true         # env: METRICS_ENABLED
  endpoint: "/metrics"  # env: METRICS_ENDPOINT (default: /metrics)
```

Or via env var: `METRICS_ENABLED=true`.

### Scrape config

The root `prometheus.yml` is already configured to scrape the gateway at
`aurora:8080/metrics`. Start Prometheus with:

```bash
docker run -d --net=host -p 9090:9090 \
  -v ./prometheus.yml:/etc/prometheus/prometheus.yml \
  prom/prometheus
```

Or use `docker compose --profile infra up -d` (starts Prometheus + Grafana together).

### Available metrics

| Metric | Type | Labels |
|---|---|---|
| `aurora_requests_total` | Counter | `provider`, `model`, `endpoint`, `status_code`, `status_type`, `stream` |
| `aurora_request_duration_seconds` | Histogram | `provider`, `model`, `endpoint`, `stream` |
| `aurora_requests_in_flight` | Gauge | `provider`, `endpoint`, `stream` |
| `aurora_gateway_phase_duration_seconds` | Histogram | `endpoint`, `phase`, `status`, `stream` |
| `aurora_response_snapshot_store_failures_total` | Counter | `provider`, `provider_name`, `operation` |

### Useful queries

```promql
# Request rate by provider
sum(rate(aurora_requests_total[5m])) by (provider)

# Error rate %
sum(rate(aurora_requests_total{status_type="error"}[5m]))
/
sum(rate(aurora_requests_total[5m])) * 100

# P95 latency by provider
histogram_quantile(0.95,
  sum(rate(aurora_request_duration_seconds_bucket[5m])) by (le, provider))

# In-flight requests
sum(aurora_requests_in_flight) by (provider)
```

### Helm / Kubernetes

When `metrics.enabled && metrics.serviceMonitor.enabled` in Helm values, a Prometheus
Operator ServiceMonitor is created automatically (see `helm/templates/servicemonitor.yaml`).

---

## Grafana (OSS, fully supported)

### Start with Docker Compose

```bash
docker compose --profile infra up -d
```

Grafana is available at `http://localhost:3000` (default: admin / admin).

### What's pre-provisioned

| Resource | File | Description |
|---|---|---|
| Prometheus datasource | `monitoring/grafana/datasources/prometheus.yml` | Points at `http://prometheus:9090` |
| Aurora dashboard | `monitoring/grafana/dashboards/aurora-gateway.json` | 5 panels (request rate, errors, latency, in-flight, by model) |

### Customizing

- **Add panels**: Edit the JSON dashboard file or create new ones in `monitoring/grafana/dashboards/`.
- **Add datasources**: Add YAML files to `monitoring/grafana/datasources/`.
- **Environment**: Set `GRAFANA_USER`, `GRAFANA_PASSWORD`, `GRAFANA_HOST_PORT` via `.env`.

---

## Observability Exports (Enterprise only)

Aurora Gateway can periodically export telemetry snapshots to external platforms.
This feature is **not available in the OSS build** — enabling it returns an error:

```
observability_exports.enabled is not available in OSS build
```

The `internal/telemetry/exporter.go` implements a 30-second heartbeat loop that sends
a JSON payload with gateway status metrics to a configurable HTTP endpoint. It is **not**
a full OpenTelemetry SDK integration — it's a lightweight status reporter.

### Configuration reference (Enterprise builds only)

```yaml
observability_exports:
  enabled: false                        # env: OBSERVABILITY_EXPORTS_ENABLED
  destination: "otlp"                   # otlp | datadog | webhook | s3
  endpoint: "http://otel-collector:4318" # export URL
  format: "json"                        # json | otlp_json | datadog_json | s3_json
  auth_mode: "none"                     # none | bearer | api_key | aws_iam
  sample_rate: 1.0                      # 0.0–1.0 (FNV-hash sampling)
  redact_headers: true                  # strip request headers from payload
  labels:
    environment: production
    region: us-east-1
```

### Destination details

| Destination | Auth | Headers | Format |
|---|---|---|---|
| `otlp` | Bearer token (`AURORA_OBSERVABILITY_EXPORT_TOKEN`) | `X-Aurora-Export-Destination: otlp` | `json` / `otlp_json` |
| `datadog` | API key (`DATADOG_API_KEY`) | `DD-API-KEY`, `X-Aurora-Export-Destination: datadog` | `json` / `datadog_json` |
| `webhook` | Bearer token | `X-Aurora-Export-Destination: webhook` | `json` |
| `s3` | `aws_iam` (implicit from env) or `api_key` | `X-Aurora-Export-Destination: s3` | `json` / `s3_json` (HTTP PUT) |

### How it works

1. At startup, the gateway validates the export config.
2. A goroutine loops every 30s, constructs a JSON `Payload` with timestamp, destination,
   labels, and a `Metrics` map containing `{"exporter": "aurora", "status": "heartbeat"}`.
3. The payload is sampled using FNV-1a hash: `hash(nodeID + timestamp) % 1000 < sample_rate * 1000`.
4. The HTTP request is sent with configurable auth headers.
5. Status is tracked in-memory and exposed through the admin dashboard at
   `Settings → Observability Exports`.

---

## Audit Logging (OSS, fully supported)

Requests can be logged to SQLite, PostgreSQL, or MongoDB for audit trails.

```yaml
logging:
  enabled: false                        # env: LOGGING_ENABLED
  log_bodies: false                     # env: LOGGING_LOG_BODIES
  log_headers: false                    # env: LOGGING_LOG_HEADERS
  buffer_size: 1000                     # env: LOGGING_BUFFER_SIZE
  flush_interval: 5                     # env: LOGGING_FLUSH_INTERVAL
  retention_days: 90                    # env: LOGGING_RETENTION_DAYS
  only_model_interactions: true         # skip /health, /metrics, /admin
```

---

## pprof (OSS, fully supported)

Go runtime profiling is available when enabled:

```yaml
server:
  pprof_enabled: true                   # env: PPROF_ENABLED
```

Endpoints at `/debug/pprof/{profile,heap,goroutine,threadcreate,block,mutex}`.

---

## Not supported

- **Langfuse** — Not integrated. No imports, config, or code references exist.
- **Full OpenTelemetry SDK** — The `observability_exports` feature sends JSON heartbeats
  only, not OTel spans/traces via the SDK. True distributed tracing would require
  the `go.opentelemetry.io/otel` SDK.
