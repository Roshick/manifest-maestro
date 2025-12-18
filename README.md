# manifest-maestro

One service to fetch, compose, and render Kubernetes manifests from Helm charts & Kustomizations with caching, observability, and a clean JSON/OpenAPI interface.

## Status & Badges (placeholders)
- Build: ![build](https://img.shields.io/badge/build-passing-brightgreen)  
- Go Version: 1.25  
- Coverage: _coming soon_  
- License: MIT  
- Release: _coming soon_  
- Docker Image: _coming soon_  
- Go Report Card: _coming soon_  
- OpenAPI: `GET /swagger-ui/api/openapi.yaml`

## Motivation
Platform & infra teams often need consistent, fast, reproducible Kubernetes manifest generation for CI pipelines, previews, and policy checks. Combining Helm and Kustomize sources (including remote Git and OCI/image-backed Helm repositories) reliably is non‑trivial. `manifest-maestro` centralizes retrieval, dependency handling, value merging, rendering, and uniform error responses with optional Redis synchronization for horizontal scaling.

## Key Features
- Render Helm charts (from HTTP/HTTPS repositories, OCI registries, or Git paths)
- Render Kustomizations from Git repositories (with optional path scoping)
- Merge Helm values from multiple sources: complex values (structured), value files, flat and string values
- Inject arbitrary YAML manifests into Kustomize render pipeline
- Dependency resolution for Helm chart sub‑charts including remote fetch of missing dependencies
- Pluggable Helm getter providers via `HELM_HOST_PROVIDERS` env var (HTTP(S) + Basic Auth, OCI)
- Caching layers (Git repositories, Helm indexes, Helm chart tarballs) with time‑based TTLs
- Uniform JSON error model & OpenAPI documented API
- Health (readiness/liveness), metrics (Prometheus), profiling (`/debug/pprof`), and tracing (OpenTelemetry)
- GitHub App authentication for private Git repository access w/ smart pagination & rate limit metrics
- Vault secret retrieval to environment (optional)
- Structured logging (plain or JSON) with attribute renaming and UTC timestamp transformer

## Architecture Overview
Layered composition:
```
main.go → wiring.Application
  ├─ bootstrap: logging, vault, config, telemetry
  ├─ repositories: GitHubClient, Git, HelmRemote
  ├─ services: GitRepositoryCache, HelmIndexCache, HelmChartCache,
  │            HelmChartProvider, HelmChartRenderer,
  │            KustomizationProvider, KustomizationRenderer
  └─ web: chi Router + middlewares (CORS, request id, tracing, metrics, panic recovery)
        controllers: Health, Metrics, Profiler, Swagger, V1 (Helm/Kustomize actions)
```
Caching + provider flow (Helm chart path example):
```
V1Controller → ChartProvider → (GitRepositoryCache OR HelmChartCache + HelmIndexCache) → HelmRemote → Render (ChartRenderer)
```
Kustomize flow:
```
V1Controller → KustomizationProvider → GitRepositoryCache → KustomizationRenderer
```

## Folder Structure
- `main.go` – entrypoint (loads `.env`, runs application wiring)
- `internal/wiring` – dependency graph construction
- `internal/config` – environment driven configuration & parsing (application & logging)
- `internal/repository` – remote connectors (git, helmremote)
- `internal/service` – business logic: caching, Helm & Kustomize providers/renderers
- `internal/web` – server, controllers, middleware wiring
- `internal/utils` – helper utilities (pointers, deep merge, etc.)
- `pkg/filesystem`, `pkg/targz` – lightweight in‑memory filesystem + tar.gz compress/extract helpers
- `test/mock` – mocks for clock/git/helmremote
- `Dockerfile` – multi‑stage minimal container build (scratch)
- `compose.yaml` – Redis service for cache synchronization
- `LICENSE` – MIT

## Configuration (Environment Variables)
Application (from `ApplicationConfig`):
- `APPLICATION_NAME` (default: `manifest-maestro`)
- `SERVER_ADDRESS` (default empty → binds on all interfaces when combined with port)
- `SERVER_PRIMARY_PORT` (default: `8080`)
- `HELM_DEFAULT_RELEASE_NAME` (default: `RELEASE-NAME`)
- `HELM_DEFAULT_KUBERNETES_API_VERSIONS` (JSON array default: `[]`)
- `HELM_DEFAULT_KUBERNETES_NAMESPACE` (default: `default`)
- `HELM_HOST_PROVIDERS` – JSON object mapping hostnames to lists of Helm getter providers used when talking to that host.
  - Shape: `{ "<host>": [ { "type": "http" | "https" | "oci", "schemes": ["..."], "basicAuth": { ... } } ] }`.
  - `type` (required):
    - `"http"` or `"https"` → HTTP(S) chart repositories (defaults `schemes` to `["http","https"]` when omitted or empty).
    - `"oci"` → OCI registries (defaults `schemes` to `["oci"]` when omitted or empty).
  - `schemes` (optional): list of URL schemes this provider should handle for the given host (e.g. `["https"]`, `["oci"]`).
  - `basicAuth` (optional):
    - `username` / `password`: literal credentials.
    - `usernameEnvVar` / `passwordEnvVar`: names of environment variables whose values will be read at runtime.
    - If either username or password resolves to an empty string, no basic auth is configured for that provider.
  - Invalid JSON or unsupported `type` values will cause startup to fail with an `invalid HELM_HOST_PROVIDERS` error.
  - When unset or `{}`, only Helm's default providers are used (no host-specific overrides).

  Example: HTTP(S) chart repo with env-based basic auth
  ```json
  {
    "charts.example.com": [
      {
        "type": "http",
        "basicAuth": {
          "usernameEnvVar": "HELM_USER",
          "passwordEnvVar": "HELM_PASS"
        }
      }
    ]
  }
  ```

  Example: OCI registry with explicit schemes and inline credentials
  ```json
  {
    "oci-registry.example.com": [
      {
        "type": "oci",
        "schemes": ["oci"],
        "basicAuth": {
          "username": "robot",
          "password": "s3cr3t"
        }
      }
    ]
  }
  ```

  Example: single host with separate HTTP and OCI providers
  ```json
  {
    "artifacts.example.com": [
      {
        "type": "http",
        "schemes": ["https"],
        "basicAuth": {
          "usernameEnvVar": "HELM_HTTP_USER",
          "passwordEnvVar": "HELM_HTTP_PASS"
        }
      },
      {
        "type": "oci",
        "schemes": ["oci"],
        "basicAuth": {
          "usernameEnvVar": "HELM_OCI_USER",
          "passwordEnvVar": "HELM_OCI_PASS"
        }
      }
    ]
  }
  ```
- `GITHUB_APP_ID`, `GITHUB_APP_INSTALLATION_ID`, `GITHUB_APP_PRIVATE_KEY` (PEM for GitHub App auth)
- `SYNCHRONIZATION_METHOD` (`MEMORY` | `REDIS`)
- `SYNCHRONIZATION_REDIS_URL` (e.g. `redis://localhost:6379`)
- `SYNCHRONIZATION_REDIS_PASSWORD`

Logging (from `LoggingConfig`):
- `LOG_STYLE` (`PLAIN` | `JSON`)
- `LOG_LEVEL` (`DEBUG`, `INFO`, `WARN`, `ERROR`)
- `LOG_ATTRIBUTE_KEY_MAPPINGS` (JSON map; remaps slog keys → structured output keys)
- `LOG_TIME_TRANSFORMER` (`UTC` | `ZERO`)

Vault (via go-autumn-vault, optional): When enabled, secrets are fetched and added to env before application config parsing.

Telemetry (OTLP exporters) rely on standard OpenTelemetry env vars if set (e.g. `OTEL_EXPORTER_OTLP_ENDPOINT`).

## Quick Start
Prerequisites:
- Go >= 1.25
- Docker (optional)
- Redis (optional for REDIS synchronization)

Local run:
```bash
git clone https://github.com/Roshick/manifest-maestro.git
cd manifest-maestro
# optional: create .env
cat > .env <<'EOF'
SERVER_ADDRESS=0.0.0.0
SERVER_PRIMARY_PORT=8080
SYNCHRONIZATION_METHOD=MEMORY
LOG_STYLE=PLAIN
LOG_LEVEL=INFO
EOF

go build ./...
./main
```

Docker build & run:
```bash
docker build -t manifest-maestro:dev .
docker run --rm -p 8080:8080 manifest-maestro:dev
```

Docker Compose (Redis cache):
```bash
docker compose up -d
# set SYNCHRONIZATION_METHOD=REDIS and SYNCHRONIZATION_REDIS_URL=redis://:eYVX7EwVmmxKPCDmwMtyKVge8oLd2t81@localhost:6379
```

## API Usage Examples
Health:
```bash
curl -s localhost:8080/health/liveness | jq
curl -s localhost:8080/health/readiness | jq
```
OpenAPI & Swagger UI:
```bash
curl -s localhost:8080/swagger-ui/api/openapi.yaml
# Browser: http://localhost:8080/swagger-ui/index.html
```
Metrics (Prometheus):
```bash
curl -s localhost:8080/metrics | head
```
Render Helm chart from repository (HTTP):
```bash
curl -s -X POST localhost:8080/rest/api/v1/helm/actions/render-chart \
 -H 'Content-Type: application/json' \
 -d '{
  "reference": {"helmChartRepositoryChartReference": {"repositoryURL": "https://charts.bitnami.com/bitnami", "chartName": "nginx"}},
  "parameters": {"releaseName": "example", "namespace": "demo", "valuesFlat": ["service.type=ClusterIP"]}
 }' | jq '.manifests[0]'
```
Render Helm chart from Git path:
```bash
curl -s -X POST localhost:8080/rest/api/v1/helm/actions/get-chart-metadata \
 -H 'Content-Type: application/json' \
 -d '{
  "reference": {"gitRepositoryPathReference": {"repositoryURL": "https://github.com/org/repo.git", "reference": "main", "path": "charts/app"}}
 }'
```
Render Kustomization from Git path with manifest injection:
```bash
curl -s -X POST localhost:8080/rest/api/v1/kustomize/actions/render-kustomization \
 -H 'Content-Type: application/json' \
 -d '{
  "reference": {"gitRepositoryPathReference": {"repositoryURL": "https://github.com/org/repo.git", "reference": "main", "path": "deploy/overlays/dev"}},
  "parameters": {"manifestInjections": [{"fileName": "extra.yaml", "manifests": [{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"injected"},"data":{"key":"value"}}]}]}
 }' | jq '.manifests | length'
```
Error sample (chart not found): returns JSON:
```json
{"title":"Helm repository chart not found"}
```

## Endpoints Summary
- `GET /health/liveness`, `GET /health/readiness`
- `GET /metrics`
- `GET /debug/*` (pprof profiler)
- `GET /swagger-ui/*` (Swagger UI assets & OpenAPI spec)
- `POST /rest/api/v1/helm/actions/list-charts` (currently returns 500 – roadmap item)
- `POST /rest/api/v1/helm/actions/get-chart-metadata`
- `POST /rest/api/v1/helm/actions/render-chart`
- `POST /rest/api/v1/kustomize/actions/render-kustomization`

## Caching Strategy
- Git repositories: 5m TTL – keyed by `repositoryURL|commitHash`
- Helm repository indexes: 5m TTL – keyed by repository URL
- Helm charts (OCI): 5m TTL – keyed by fully qualified OCI reference (including version tag)
- Helm charts (HTTP): 15m TTL – keyed by `chartURL|digest`
Mechanism: abstraction from `go-autumn-synchronisation` offering in‑memory or Redis (select via `SYNCHRONIZATION_METHOD`). Invalidation: time‑based only (no manual purge API yet). Git commit resolution ensures immutability -> safe longer TTLs.

## Helm Value Merging Order
1. Structured `complexValues`
2. Value files (merged in provided order; later overrides earlier) – missing files error unless `ignoreMissingValueFiles=true`
3. Simple values & flattened pairs (`values`, `valuesFlat`) using Helm's precedence
4. String values (`stringValues`, `stringValuesFlat`) applied last
CRDs & hooks included by default; disable with `{"includeCRDs": false}` or `{"includeHooks": false}`.

## Security Considerations
- GitHub App private key loaded via `GITHUB_APP_PRIVATE_KEY` (ensure proper secret management, consider Vault integration)
- Vault integration can populate sensitive credentials before other config parsing
- CORS middleware currently permissive (review before exposing publicly)
- Input validation for request bodies (schema enforcement & malformed body handling)
- Remote code artifacts (Helm charts, Git repos) are fetched & executed only as data (no template execution outside Helm rendering). Review dependencies for supply chain integrity.
- Use Redis AUTH (`SYNCHRONIZATION_REDIS_PASSWORD`) when using Redis
- Run container as non‑root (enhancement pending – scratch base currently inherits root UID)

## Observability
- Metrics: Prometheus format at `/metrics` includes request metrics & GitHub rate limit gauges
- Tracing: OpenTelemetry spans via `otelchi` & `otelhttp` – configure OTLP endpoint with env vars
- Profiling: `/debug/pprof` endpoint for CPU, heap, goroutine analysis
- Structured logging: slog with customizable key mapping (e.g. `time` → `@timestamp`) and level control

## Development Guide
Install tool versions (optional) with `mise` if configured in `mise.toml`.
Run & iterate:
```bash
go build ./...
go test ./...
GOLOG_LOG_LEVEL=debug ./main
```
Suggested linters (add later): `golangci-lint run`.
Redis setup for integration testing:
```bash
docker compose up -d cache
export SYNCHRONIZATION_METHOD=REDIS
export SYNCHRONIZATION_REDIS_URL=redis://:eYVX7EwVmmxKPCDmwMtyKVge8oLd2t81@localhost:6379
./main
```
Update OpenAPI (external module `manifest-maestro-api`): bump version in `go.mod` after regenerating spec upstream.

## Testing
- Mocks under `test/mock/*` for deterministic unit tests (git, helmremote, clock)
- Focus service tests on provider/render behavior using injected caches
- Add integration tests for value file precedence and manifest injections (ToDo: contribute more examples)

## Performance Notes
- Caching reduces redundant remote fetches (commit hashes & chart digests are immutable)
- Helm dependency resolution may be CPU heavy for large charts; consider profiling if latency spikes
- Value merging performs deep map merges – watch for very large nested structures
- Potential future optimization: parallel fetch of Helm dependencies not already embedded

## Roadmap
- Implement `/helm/actions/list-charts`
- Manual cache invalidation endpoints
- Configurable TTLs via environment
- RBAC / API tokens & tighter CORS config
- Streaming responses for large manifest sets
- More comprehensive e2e tests & benchmarks

## Contributing
1. Fork & create feature branch (`feat/short-description`)
2. Follow Conventional Commits (`feat:`, `fix:`) for clarity
3. Add/adjust tests for any behavior change
4. Ensure `go test ./...` passes and no lint regressions
5. Submit PR with description & context (link related issues)

## License
MIT – see `LICENSE` file.

## Acknowledgements
- Helm project (chart rendering engine)
- Kustomize project
- Chi router & related middleware ecosystem
- OpenTelemetry maintainers
- go-git & supporting billy filesystem
- Prometheus client library
- GitHub API libraries & rate limit helpers
- `go-autumn-*` libraries powering logging, vault, web, synchronization

---
Questions or ideas? Open an issue or start a discussion.
