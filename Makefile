.PHONY: all build build-oss build-editions package-editions run run-oss clean tidy test test-race test-dashboard test-e2e test-integration test-contract test-all lint lint-fix record-api swagger docs-openapi install-tools perf-check perf-bench perf-server bench-load bench-report infra image ui-install ui-dev ui-build ui-test ui-lint build-react reset-data reset-data-dry-run

all: build

# Get version info
VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT ?= $(shell git rev-parse --short HEAD)
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
DOCS_API_SERVERS ?= https://aurora.example.com,http://localhost:8080
LOG_LEVEL ?= debug
SWAGGER_ENABLED ?= true

# Linker flags to inject version info
LDFLAGS := -X "aurora/internal/version.Version=$(VERSION)" \
           -X "aurora/internal/version.Commit=$(COMMIT)" \
           -X "aurora/internal/version.Date=$(DATE)"

install-tools:
	@command -v golangci-lint > /dev/null 2>&1 || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10)
	@command -v pre-commit > /dev/null 2>&1 || (echo "Installing pre-commit..." && pip install pre-commit==4.5.1)
	@echo "All tools are ready"

build:
	go build -ldflags '$(LDFLAGS)' -o bin/aurora ./apps/aurora

build-oss:
	go build -ldflags '$(LDFLAGS) -X "aurora/internal/version.Version=oss"' -o bin/aurora-oss ./apps/aurora

build-editions: ui-build build-oss
	@echo "Built Aurora OSS artifact."

package-editions:
	powershell -ExecutionPolicy Bypass -File scripts/release/build-editions.ps1 -Package

# Run the application
run:
	LOG_LEVEL=$(LOG_LEVEL) SWAGGER_ENABLED=$(SWAGGER_ENABLED) go run -tags=swagger ./apps/aurora

run-oss:
	AURORA_CONFIG_PATH=configs/editions/oss.example.yaml LOG_LEVEL=$(LOG_LEVEL) go run ./apps/aurora

# Clean build artifacts
clean:
	rm -rf bin/

# Tidy dependencies
tidy:
	go mod tidy

# Docker Compose: Redis, PostgreSQL, MongoDB, Adminer (no app image build)
infra:
	docker compose up -d

# Docker Compose: full stack (Aurora + Prometheus; builds app image when needed)
image:
	docker compose --profile app up -d

# Run unit tests only
test:
	go test ./apps/... ./internal/... ./configuration/... -v

# Run unit tests with race detection and coverage
test-race:
	go test -v -race -coverprofile=coverage.out ./apps/... ./internal/... ./configuration/...

# Run dashboard JavaScript unit tests
test-dashboard:
	node --test internal/admin/dashboard/static/js/modules/*.test.cjs

# Run e2e tests (uses an in-process mock LLM server; no Docker required)
test-e2e:
	go test -v -tags=e2e ./test/end_to_end/...

# Run integration tests (requires Docker)
test-integration:
	go test -v -tags=integration -timeout=10m ./test/integration/...

# Run contract tests (validates API response structures against golden files)
test-contract:
	go test -v -tags=contract -timeout=5m ./test/contract/...

# Run all tests including dashboard, e2e, integration, and contract tests
test-all: test test-dashboard test-e2e test-integration test-contract

perf-check:
	@echo "Performance tests not available in OSS edition; skipping."
	@exit 0

perf-bench:
	go test -bench=. -benchmem ./test/performance/...

# Record API responses for contract tests
# Usage: OPENAI_API_KEY=sk-xxx make record-api
record-api:
	@echo "Recording OpenAI chat completion..."
	go run ./apps/record-api -provider=openai -endpoint=chat \
		-output=test/contract/testdata/openai/chat_completion.json
	@echo "Recording OpenAI models..."
	go run ./apps/record-api -provider=openai -endpoint=models \
		-output=test/contract/testdata/openai/models.json
	@echo "Done! Golden files saved to test/contract/testdata/"

swagger:
	go run github.com/swaggo/swag/v2/cmd/swag init --generalInfo main.go \
		--dir apps/aurora,internal \
		--output apps/aurora/docs \
		--outputTypes go \
		--parseDependency
	$(MAKE) docs-openapi

docs-openapi:
	@command -v node >/dev/null 2>&1 || { echo "node is required to build docs; install from https://nodejs.org" >&2; exit 1; }
	@command -v npx >/dev/null 2>&1 || { echo "npx is required; install npm (includes npx)" >&2; exit 1; }
	@tmp_dir=$$(mktemp -d); \
	trap 'rm -rf "$$tmp_dir"' EXIT; \
	go run github.com/swaggo/swag/v2/cmd/swag init --quiet --generalInfo main.go \
		--dir apps/aurora,internal \
		--output "$$tmp_dir" \
		--outputTypes json \
		--parseDependency; \
	npx -y swagger2openapi@7.0.8 --patch -o docs/openapi.json "$$tmp_dir/swagger.json"; \
	DOCS_API_SERVERS="$(DOCS_API_SERVERS)" node devtools/openapi-postprocess.mjs docs/openapi.json

# -------------------------------------------------------------------------
# Benchmarking targets
# -------------------------------------------------------------------------

# Build the standalone benchmark server binary
bench-server-build:
	go build -o bin/bench-server.exe ./apps/benchmark-server

# Start the benchmark server (standalone, listens on :9090 by default)
# Use SERVER_ARGS to pass flags:
#   make bench-server SERVER_ARGS="--port 9090 --latency 10 --error-rate 0.05"
bench-server: bench-server-build
	./bin/bench-server.exe $(SERVER_ARGS)

# Run the full Go benchmark suite (allocation + concurrency)
perf-bench-all:
	go test -bench=. -benchmem -benchtime=5s -count=3 ./test/performance/... | tee bench-results/perf-bench.txt

# Profile CPU during benchmarks
perf-cpu:
	go test -bench=BenchmarkGatewayConcurrent -benchmem -cpuprofile=bench-results/cpu.prof ./test/performance/...

# Profile memory during benchmarks
perf-mem:
	go test -bench=BenchmarkGatewayConcurrent -benchmem -memprofile=bench-results/mem.prof ./test/performance/...

# Generate pprof CPU flame graph (requires graphviz)
perf-flamegraph: perf-cpu
	go tool pprof -http=:6060 bench-results/cpu.prof

# Run all benchmarks and generate reports
bench-report: perf-bench-all
	@echo "Benchmark results saved to bench-results/perf-bench.txt"
	@echo "Run 'go tool pprof -http=:6060 bench-results/cpu.prof' for CPU profile"

# Load test via PowerShell script (requires bench-server running)
bench-load:
	powershell -ExecutionPolicy Bypass -File scripts/benchmarks/bench-load.ps1

# Full benchmark workflow: build server, run micro-benchmarks, run load test
bench-all: bench-server-build perf-bench-all
	@echo ""
	@echo "========================================"
	@echo " Next: start the bench server and run load tests"
	@echo "   make bench-server"
	@echo "   make bench-load"
	@echo "========================================"

# -------------------------------------------------------------------------
# oha h2c benchmark targets
# -------------------------------------------------------------------------

# Verify oha is installed (scoop install oha)
oha-check:
	@command -v oha > /dev/null 2>&1 || (echo "oha not found. Install: scoop install oha" && exit 1)

# Run oha benchmark against the running gateway (port 8080).
# Override OHA_URL, OHA_N, OHA_C for custom parameters.
#   make oha-bench OHA_N=100000 OHA_C=200 OHA_URL=http://localhost:8080/health
OHA_N ?= 100000
OHA_C ?= 200
OHA_URL ?= http://localhost:8080/health
OHA_FLAGS ?=

oha-bench: oha-check
	oha -n $(OHA_N) -c $(OHA_C) $(OHA_FLAGS) $(OHA_URL)

# Run oha against the bench server (port 9090).
oha-bench-server: bench-server-build
	@echo "Starting bench server in background..."
	oha -n $(OHA_N) -c $(OHA_C) $(OHA_FLAGS) http://localhost:9090/health

# Full oha benchmark: build aurora, start with h2c bench mode, run oha
bench-oha: build
	@echo "Starting Aurora in bench mode with h2c enabled..."
	@echo "Run in another terminal: make oha-bench OHA_URL=http://localhost:8080/health"
	@echo "Then run: AURORA_MINIMAL_BENCH_MODE=true AURORA_H2C_ENABLED=true ./bin/aurora.exe"

# -------------------------------------------------------------------------
# Stress testing targets (in-process load test suite)
# -------------------------------------------------------------------------

# Build the load test binary
bench-load-build:
	go build -o bin/bench-load.exe ./apps/benchmark-load

# Run full load test suite (pure gateway overhead, no mock latency)
# Use LEVELS, DURATION, MOCK_LATENCY to customize
#   make bench-load-run LEVELS="1,10,50,100,200,500" DURATION=10s MOCK_LATENCY=0
bench-load-run: bench-load-build
	./bin/bench-load.exe \
		$(if $(LEVELS),--levels $(LEVELS)) \
		$(if $(DURATION),--duration $(DURATION)) \
		$(if $(MOCK_LATENCY),--mock-latency $(MOCK_LATENCY)) \
		$(if $(AUTH),--auth) \
		$(if $(RESPONSE_TOKENS),--response-tokens $(RESPONSE_TOKENS))

# Bifrost-equivalent test: 60ms mock latency, 500 concurrency, 60s
bench-vs-bifrost: bench-load-build
	./bin/bench-load.exe --mock-latency 60 --levels 500 --duration 60s --response-tokens 200

# Quick smoke test (10s total)
bench-smoke: bench-load-build
	./bin/bench-load.exe --duration 5s --levels 1,10,50 --mock-latency 0

# Full stress test across all concurrency levels
bench-stress: bench-load-build
	./bin/bench-load.exe --duration 30s --levels 1,10,50,100,200,500,1000

# oha standalone benchmark — uses h2c via AURORA_H2C_ENABLED
#   make bench-oha-standalone CONCURRENCY="50,100,200,300"
#   make bench-oha-standalone ENDPOINT=/v1/chat/completions USE_CHAT=1
bench-oha-standalone:
	powershell -ExecutionPolicy Bypass -File scripts/benchmarks/scenarios/aurora-standalone.ps1
		$(if $(CONCURRENCY),-ConcurrencyLevels $(CONCURRENCY)) \
		$(if $(ENDPOINT),-Endpoint $(ENDPOINT)) \
		$(if $(USE_CHAT),-UseChatCompletion)

# Run linter
lint:
	golangci-lint run --build-tags=swagger,e2e,integration,contract ./apps/... ./configuration/... ./internal/... ./test/...

# Run linter with auto-fix
lint-fix:
	golangci-lint run --fix ./apps/... ./configuration/... ./internal/... ./test/...

# -------------------------------------------------------------------------
# React dashboard (dashboard-ui/ workspace)
# -------------------------------------------------------------------------
# These targets build the React admin UI. Output is written to
# internal/admin/dashboard/dist and embedded into the Go binary at
# compile time to serve /admin/dashboard.

# Install JS dependencies (requires Node 22+, pnpm 9+).
ui-install:
	cd dashboard-ui && pnpm install --frozen-lockfile || cd dashboard-ui && pnpm install

# Vite dev server on :5173 with API proxy to localhost:8080.
ui-dev:
	cd dashboard-ui && pnpm dev

# Production build → ../internal/admin/dashboard/dist
ui-build:
	cd dashboard-ui && pnpm build

# Vitest unit tests for dashboard-ui/.
ui-test:
	cd dashboard-ui && pnpm test

# Oxlint pass.
ui-lint:
	cd dashboard-ui && pnpm lint

# Build the Go binary AFTER refreshing the embedded React bundle. Use this
# instead of `make build` when you have unbuilt UI changes you want to ship.
build-react: ui-install ui-build build

# -------------------------------------------------------------------------
# Cloud data reset
# -------------------------------------------------------------------------
# Wipes Postgres tables, Redis keys, and the Qdrant semantic-cache
# collection used by the gateway. Reads connection info from the project-local
# .env and helper script in .aurora.local/. Pass DRY_RUN=1 to preview the plan.
#
#   make reset-data              # full reset (requires --confirm internally)
#   make reset-data DRY_RUN=1    # preview only
#   make reset-data TARGETS="--postgres --redis"   # selective
RESET_SCRIPT := python .aurora.local/reset_data.py

reset-data:
ifeq ($(DRY_RUN),1)
	$(RESET_SCRIPT) --dry-run $(TARGETS)
else
	$(RESET_SCRIPT) --confirm $(TARGETS)
endif

reset-data-dry-run:
	$(RESET_SCRIPT) --dry-run $(TARGETS)

