# Aurora OSS — Helm Deployment Guide

Deploy Aurora OSS AI gateway on Kubernetes with provider credentials, optional Redis, Prometheus metrics, Ingress, Gateway API, autoscaling, and production-safe pod defaults.

---

## What You Need

| Requirement | Minimum | Notes |
|---|---|---|
| **Kubernetes cluster** | 1.24+ | Any K8s: minikube, EKS, AKS, GKE, k3s, etc. |
| **Helm** | 3.x | Package manager for Kubernetes |
| **Docker image** | `aurorahq/aurora:latest` | Published on Docker Hub |
| **At least one provider API key** | — | OpenAI, Anthropic, Gemini, Groq, xAI, Z.ai, Oracle, or vLLM |

---

## How Configuration Works

Aurora uses a **3-layer config chain**:

```
code defaults  →  config.yaml  →  environment variables (win)
```

- **Code defaults**: Baked into the Go binary (port 8080, SQLite at `data/aurora-oss.db`, etc.)
- **config.yaml**: Ships inside the Docker image at `/app/configs/config.yaml` using `${VAR:-default}` syntax throughout
- **Env vars**: Set via Helm (ConfigMap, Secret, or `env` tags) — always override YAML values

Most settings can be overridden via environment variables without rebuilding the image.

---

## Quick Install

### 1. Single Provider (Groq — No Redis, No Auth)

Suitable for local development:

```bash
helm install aurora ./helm \
  --namespace aurora --create-namespace \
  --set image.repository=aurorahq/aurora \
  --set image.tag=latest \
  --set providers.groq.apiKey="gsk_your_key_here" \
  --set providers.groq.enabled=true \
  --set redis.enabled=false \
  --set auth.masterKey=""
```

### 2. Multiple Providers with Redis

Production-ready setup with auth:

```bash
helm install aurora ./helm \
  --namespace aurora --create-namespace \
  --set image.repository=aurorahq/aurora \
  --set image.tag=latest \
  --set providers.openai.apiKey="sk-..." \
  --set providers.openai.enabled=true \
  --set providers.anthropic.apiKey="sk-ant-..." \
  --set providers.anthropic.enabled=true \
  --set auth.masterKey="your-secure-master-key" \
  --set logging.enabled=true \
  --set metrics.enabled=true \
  --set guardrails.enabled=true \
  --set tokenSaver.enabled=true \
  --set redis.enabled=true
```

### 3. Using Existing Secrets (GitOps)

```bash
# Create the secret first
kubectl create secret generic llm-keys \
  --namespace aurora \
  --from-literal=OPENAI_API_KEY=sk-... \
  --from-literal=ANTHROPIC_API_KEY=sk-ant-...

# Install referencing the secret
helm install aurora ./helm \
  --namespace aurora --create-namespace \
  --set image.repository=aurorahq/aurora \
  --set image.tag=latest \
  --set providers.existingSecret=llm-keys \
  --set providers.openai.enabled=true \
  --set providers.anthropic.enabled=true \
  --set auth.masterKey="your-key" \
  --set logging.enabled=true \
  --set metrics.enabled=true \
  --set guardrails.enabled=true \
  --set redis.enabled=true
```

---

## Providers

### Supported Providers

| Provider | Enable Flag | API Key Value | Base URL (optional) |
|---|---|---|---|
| **OpenAI** | `providers.openai.enabled` | `providers.openai.apiKey` | `providers.openai.baseUrl` |
| **Anthropic** | `providers.anthropic.enabled` | `providers.anthropic.apiKey` | `providers.anthropic.baseUrl` |
| **Gemini** | `providers.gemini.enabled` | `providers.gemini.apiKey` | — |
| **Groq** | `providers.groq.enabled` | `providers.groq.apiKey` | `providers.groq.baseUrl` |
| **xAI (Grok)** | `providers.xai.enabled` | `providers.xai.apiKey` | `providers.xai.baseUrl` |
| **Z.ai** | `providers.zai.enabled` | `providers.zai.apiKey` | `providers.zai.baseUrl` |
| **Oracle** | `providers.oracle.enabled` | `providers.oracle.apiKey` | `providers.oracle.baseUrl` (required) |
| **vLLM** | `providers.vllm.enabled` | `providers.vllm.apiKey` (optional) | `providers.vllm.baseUrl` (required) |

### Example: Oracle

```bash
helm install aurora ./helm \
  --set providers.oracle.enabled=true \
  --set providers.oracle.apiKey="..." \
  --set providers.oracle.baseUrl="https://inference.generativeai.us-chicago-1.oci.oraclecloud.com/20231130/actions/v1"
```

### Example: Keyless vLLM (internal cluster)

```bash
helm install aurora ./helm \
  --set providers.vllm.enabled=true \
  --set providers.vllm.baseUrl="http://vllm.default.svc.cluster.local:8000/v1"
```

### Existing Secret Format

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: llm-keys
type: Opaque
stringData:
  OPENAI_API_KEY: "sk-..."
  ANTHROPIC_API_KEY: "sk-ant-..."
  GEMINI_API_KEY: "..."
  GROQ_API_KEY: "..."
  XAI_API_KEY: "..."
  ZAI_API_KEY: "..."
  ORACLE_API_KEY: "..."
  VLLM_API_KEY: "..."
```

Oracle also requires `providers.oracle.baseUrl`. vLLM requires `providers.vllm.baseUrl`; `providers.vllm.apiKey` is only needed if the upstream vLLM server requires one.

---

## Storage

### Default: SQLite (Ephemeral)

By default, Aurora uses SQLite at `/cache/aurora-oss.db` (emptyDir). **Data does NOT persist across pod restarts.** Suitable for development.

### Redis (Persistent, Production)

Enable the bundled Redis subchart (Bitnami):

```bash
helm install aurora ./helm \
  --set redis.enabled=true \
  --set redis.architecture=standalone \
  --set redis.auth.enabled=false \
  --set redis.master.persistence.enabled=true
```

Use an external Redis:

```bash
helm install aurora ./helm \
  --set redis.enabled=false \
  --set cache.redis.url="redis://my-redis:6379"
```

### Persistent Volume for SQLite

```bash
helm install aurora ./helm \
  --set redis.enabled=false \
  --set extraVolumes[0].name=cache \
  --set extraVolumes[0].persistentVolumeClaim.claimName=aurora-cache \
  --set extraVolumeMounts[0].name=cache \
  --set extraVolumeMounts[0].mountPath=/cache
```

---

## Custom Configuration

### Override Individual Settings via Env Vars

Many settings are overridable via environment variables set in the Helm deployment:

| Env Variable | Default | What It Controls |
|---|---|---|
| `SQLITE_PATH` | `data/aurora-oss.db` | SQLite storage path |
| `MODEL_LIST_LOCAL_PATH` | `data/models.local.json` | Local model list path |
| `MODEL_LIST_USER_OVERRIDES_PATH` | `data/user_pricing.yaml` | User pricing overrides |
| `MODEL_LIST_URL` | _(empty)_ | Remote model list URL |
| `MODEL_REFRESH_INTERVAL` | `3600` | Model cache refresh (seconds) |
| `METRICS_ENABLED` | `false` | Enable /metrics endpoint |
| `GUARDRAILS_ENABLED` | `false` | Enable content guardrails |
| `TOKEN_SAVER_ENABLED` | `false` | Enable token optimization |
| `LOGGING_ENABLED` | `false` | Enable request logging |
| `LOG_FORMAT` | _(auto)_ | Log format: `json` or `text` |
| `ADMIN_ENDPOINTS` | `true` | Enable admin API/dashboard |
| `USAGE_ENABLED` | `true` | Enable usage tracking |
| `COMBOS_ENABLED` | `true` | Enable model combos |
| `EDITION` | `oss` | Edition identifier |

### Mount a Custom config.yaml

For full control, mount your own `config.yaml` via ConfigMap:

```bash
# Create config.yaml file then install
helm install aurora ./helm \
  --set-file config.content=my-config.yaml
```

Or reference an existing ConfigMap:

```bash
helm install aurora ./helm \
  --set config.existingConfigMap=my-aurora-config
```

The custom config overrides the image default. Env vars from Helm still win.

---

## Networking

### ClusterIP (Default)

```bash
kubectl port-forward -n aurora svc/aurora 8080:8080
curl http://localhost:8080/health
```

### Ingress (NGINX + cert-manager)

```yaml
ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: aurora.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: aurora-tls
      hosts:
        - aurora.example.com
```

### Gateway API

```yaml
gateway:
  enabled: true
  parentRef:
    name: shared-gateway
    namespace: gateway-system
  hostnames:
    - aurora.example.com
```

---

## Security

### Authentication

Set `auth.masterKey` to enable API key authentication:

```bash
--set auth.masterKey="your-strong-master-key"
```

Or use an existing secret:

```bash
--set auth.existingSecret=my-auth-secret \
--set auth.existingSecretKey=master-key
```

If neither is set, the gateway runs **without authentication** (unsafe mode warning in logs).

### Pod Security

The default `securityContext` enforces:

- Read-only root filesystem (writable `/cache` emptyDir for SQLite/data)
- Non-root user (UID 65532, matching distroless nonroot image)
- Dropped all Linux capabilities
- No privilege escalation

These are production-safe defaults. To disable read-only rootfs:

```yaml
securityContext:
  readOnlyRootFilesystem: false
```

### Secrets

- **Provider API keys**: Set via `providers.*.apiKey` or `providers.existingSecret`
- **Master key**: Set via `auth.masterKey` or `auth.existingSecret`
- Never commit keys in `values.yaml` — use `--set` or existing secrets

---

## Operations

### Install

```bash
helm install aurora ./helm --namespace aurora --create-namespace
```

### Upgrade

```bash
helm upgrade aurora ./helm --namespace aurora -f my-values.yaml
```

### Uninstall

```bash
helm uninstall aurora --namespace aurora
```

### Check Status

```bash
# Pods
kubectl get pods -n aurora

# Logs
kubectl logs -n aurora -l app.kubernetes.io/name=aurora

# Health
kubectl port-forward -n aurora svc/aurora 8080:8080 &
curl http://localhost:8080/health
```

### Verify Chat Completion

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"groq/llama-3.3-70b-versatile","messages":[{"role":"user","content":"Hello!"}]}'
```

### Dry Run

```bash
helm template aurora ./helm --namespace aurora -f my-values.yaml
```

### Validate

```bash
helm lint ./helm
```

---

## All Chart Values

| Value | Default | Description |
|---|---|---|
| `replicaCount` | `2` | Pod replicas when autoscaling disabled |
| `image.repository` | `aurorahq/aurora` | Container image repository |
| `image.tag` | chart appVersion | Container image tag |
| `image.pullPolicy` | `IfNotPresent` | Image pull policy |
| `server.port` | `8080` | Aurora HTTP port |
| `server.basePath` | `/` | URL path prefix |
| `server.bodySizeLimit` | `10M` | Max request body size |
| `auth.masterKey` | `""` | Master API key (empty = no auth) |
| `auth.existingSecret` | `""` | Existing K8s secret for master key |
| `auth.existingSecretKey` | `"master-key"` | Key in existing secret |
| `providers.existingSecret` | `""` | Existing K8s secret for provider keys |
| `providers.*.enabled` | `false` | Enable specific provider |
| `providers.*.apiKey` | `""` | Provider API key |
| `providers.*.baseUrl` | `""` | Custom provider base URL |
| `cache.redis.url` | `""` | External Redis URL |
| `cache.redis.keyModels` | `"aurora:models"` | Redis models key prefix |
| `cache.redis.ttlModels` | `86400` | Models cache TTL (seconds) |
| `cache.redis.keyResponses` | `"aurora:response:"` | Response cache key prefix |
| `cache.redis.ttlResponses` | `3600` | Response cache TTL (seconds) |
| `redis.enabled` | `true` | Deploy Bitnami Redis subchart |
| `redis.architecture` | `standalone` | Redis: standalone or replication |
| `redis.auth.enabled` | `false` | Disable Redis auth by default |
| `metrics.enabled` | `true` | Enable /metrics endpoint |
| `metrics.endpoint` | `"/metrics"` | Metrics path |
| `metrics.serviceMonitor.enabled` | `false` | Create Prometheus ServiceMonitor |
| `ingress.enabled` | `false` | Create Ingress resource |
| `gateway.enabled` | `false` | Create Gateway API HTTPRoute |
| `autoscaling.enabled` | `false` | Enable HPA |
| `autoscaling.minReplicas` | `2` | Min HPA replicas |
| `autoscaling.maxReplicas` | `10` | Max HPA replicas |
| `podDisruptionBudget.enabled` | `true` | Create PDB |
| `podDisruptionBudget.minAvailable` | `1` | Min available pods |
| `resources.requests.cpu` | `100m` | CPU request |
| `resources.requests.memory` | `128Mi` | Memory request |
| `resources.limits.cpu` | `1000m` | CPU limit |
| `resources.limits.memory` | `512Mi` | Memory limit |
| `service.type` | `ClusterIP` | K8s service type |
| `service.port` | `8080` | Service port |
| `podSecurityContext.runAsUser` | `65532` | Pod user (distroless nonroot) |
| `podSecurityContext.fsGroup` | `65532` | Pod fs group |
| `securityContext.readOnlyRootFilesystem` | `true` | Read-only rootfs |
| `config.content` | `""` | Custom config.yaml content |
| `config.existingConfigMap` | `""` | Existing config ConfigMap |
| `logging.format` | `""` | Log format: json or text |
| `nodeSelector` | `{}` | Node selector |
| `tolerations` | `[]` | Pod tolerations |
| `affinity` | `{}` | Pod affinity rules |

---

## Links

- **Source code**: [github.com/aurorallm/aurora](https://github.com/aurorallm/aurora)
- **Docker images**: [hub.docker.com/r/aurorahq/aurora](https://hub.docker.com/r/aurorahq/aurora)
- **npm package**: [npmjs.com/package/iaurora](https://www.npmjs.com/package/iaurora)
- **Issue tracker**: [github.com/aurorallm/aurora/issues](https://github.com/aurorallm/aurora/issues)
