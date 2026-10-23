# manifest-maestro

[![Build](https://github.com/Roshick/manifest-maestro/actions/workflows/build.yaml/badge.svg)](https://github.com/Roshick/manifest-maestro/actions/workflows/build.yaml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE.md)

An HTTP service that fetches Helm charts and Kustomizations from Git repositories, Helm chart repositories and OCI
registries, and renders them into Kubernetes manifests. Remote artifacts are cached, and the API is described by an
OpenAPI spec.

It is built for CI pipelines, previews and policy checks that need fast, reproducible manifest rendering without
running `helm`/`kustomize` binaries and managing credentials in every job.

## Features

- Render Helm charts from HTTP(S) chart repositories, OCI registries or paths in Git repositories
- Render Kustomizations from paths in Git repositories, optionally injecting additional manifests
- Merge Helm values from structured values, value files, `--set`-style and `--set-string`-style values
- Resolve chart dependencies, fetching missing sub-charts from their repositories
- Per-host Helm getter configuration with basic auth (`HELM_HOST_PROVIDERS`)
- Private GitHub repositories via GitHub App authentication
- Caching of Git repositories, Helm repository indexes and charts, in memory or in Redis (shared across replicas,
  with automatic fallback to memory when Redis is unavailable)
- Health checks, Prometheus metrics, OpenTelemetry tracing, pprof and structured logging

## Quick start

Requirements: Go (version in [`go.mod`](go.mod); `mise install` sets it up via [`mise.toml`](mise.toml)), Docker
optional.

```bash
git clone https://github.com/Roshick/manifest-maestro.git
cd manifest-maestro
go build -o manifest-maestro .
HELM_HOST_PROVIDERS='{"prometheus-community.github.io":[{"type":"https"}],"github.com":[{"type":"https"}]}' \
  ./manifest-maestro
```

A `.env` file in the working directory is loaded on startup if present (it is git-ignored).

With Docker, either use the published image `ghcr.io/roshick/manifest-maestro` or build it locally:

```bash
docker build -t manifest-maestro .
docker run --rm -p 8080:8080 manifest-maestro
```

Then open the Swagger UI at http://localhost:8080/swagger-ui/index.html.

## API

All actions are `POST` requests with a JSON body. The OpenAPI spec is served at `GET /swagger-ui/api/openapi.yaml`
and published as the Go module [`manifest-maestro-api`](https://github.com/Roshick/manifest-maestro-api), which also
provides a generated Go client.

| Endpoint | Description |
|---|---|
| `POST /rest/api/v1/helm/actions/render-chart` | Render a Helm chart |
| `POST /rest/api/v1/helm/actions/get-chart-metadata` | Return a chart's default values |
| `POST /rest/api/v1/kustomize/actions/render-kustomization` | Render a Kustomization |
| `POST /rest/api/v1/helm/actions/list-charts` | Not implemented yet (returns 500) |
| `POST /rest/api/v1/helm/actions/list-chart-versions` | Not implemented yet (returns 500) |

### Chart references

A Helm chart `reference` is one of:

- Helm chart repository (HTTP(S) or OCI): `{"repositoryURL": "...", "chartName": "...", "chartVersion": "..."}`;
  `chartVersion` is optional for HTTP(S) chart repositories, where it defaults to the latest version.
- Git repository path: `{"repositoryURL": "https://github.com/<owner>/<repo>", "reference": "<branch|tag|commit>", "path": "charts/app"}`;
  `path` is optional and defaults to the repository root.

Kustomizations always use a Git repository path reference.

### Examples

Render a chart from a chart repository:

```bash
curl -s -X POST localhost:8080/rest/api/v1/helm/actions/render-chart \
  -H 'Content-Type: application/json' \
  -d '{
    "reference": {
      "repositoryURL": "https://prometheus-community.github.io/helm-charts",
      "chartName": "prometheus-node-exporter",
      "chartVersion": "4.47.0"
    },
    "parameters": {
      "releaseName": "node-exporter",
      "namespace": "monitoring",
      "valuesFlat": ["service.port=9101"]
    }
  }' | jq '.manifests[].content.kind'
```

Get the default values of a chart stored in a Git repository:

```bash
curl -s -X POST localhost:8080/rest/api/v1/helm/actions/get-chart-metadata \
  -H 'Content-Type: application/json' \
  -d '{"reference": {"repositoryURL": "https://github.com/org/repo", "reference": "main", "path": "charts/app"}}' \
  | jq '.defaultValues'
```

Render a Kustomization and inject an extra manifest:

```bash
curl -s -X POST localhost:8080/rest/api/v1/kustomize/actions/render-kustomization \
  -H 'Content-Type: application/json' \
  -d '{
    "reference": {"repositoryURL": "https://github.com/org/repo", "reference": "main", "path": "deploy/overlays/dev"},
    "parameters": {
      "manifestInjections": [{
        "fileName": "extra.yaml",
        "manifests": [{"content": {"apiVersion": "v1", "kind": "ConfigMap", "metadata": {"name": "injected"}, "data": {"key": "value"}}}]
      }]
    }
  }' | jq '.manifests | length'
```

Responses contain `manifests` (each with `content` and, where known, `source`); `render-chart` additionally returns
`metadata` with the merged values, chart metadata and the Kubernetes/Helm versions used.

### Helm render parameters

| Parameter | Description |
|---|---|
| `releaseName`, `namespace` | Default to `HELM_DEFAULT_RELEASE_NAME` / `HELM_DEFAULT_KUBERNETES_NAMESPACE` |
| `kubeVersion`, `apiVersions` | Capabilities exposed to templates; `apiVersions` defaults to `HELM_DEFAULT_KUBERNETES_API_VERSIONS` |
| `complexValues` | Structured values object |
| `valueFiles` | Value files relative to the chart; missing files fail unless `ignoreMissingValueFiles` is `true` |
| `values`, `valuesFlat` | Like `helm --set` (map of `key: value`, or list of `key=value`) |
| `stringValues`, `stringValuesFlat` | Like `helm --set-string` |
| `includeCRDs`, `includeHooks` | Include CRDs / hook manifests in the output (default `true`) |

Values are merged in this order, later sources overriding earlier ones: `complexValues`, `valueFiles` (in the given
order), `values`/`valuesFlat`, `stringValues`/`stringValuesFlat`. Chart defaults apply underneath all of them.

### Errors

Errors are returned as JSON with an appropriate HTTP status, a `title` (e.g. `Helm repository chart not found`,
`Git repository reference not found`, `Malformed body`) and usually a `detail`:

```json
{"title": "Helm repository provider missing", "detail": "missing provider for 'charts.example.com' with scheme 'https'"}
```

## Configuration

All configuration is read from environment variables.

### Server and Helm

| Variable | Default | Description |
|---|---|---|
| `APPLICATION_NAME` | `manifest-maestro` | Used in logs and traces |
| `SERVER_ADDRESS` | *(all interfaces)* | Listen address |
| `SERVER_PRIMARY_PORT` | `8080` | Listen port |
| `HELM_DEFAULT_RELEASE_NAME` | `RELEASE-NAME` | Release name if none is given |
| `HELM_DEFAULT_KUBERNETES_NAMESPACE` | `default` | Namespace if none is given |
| `HELM_DEFAULT_KUBERNETES_API_VERSIONS` | `[]` | JSON array of API versions if none are given |
| `HELM_HOST_PROVIDERS` | `{}` | Helm getters per host, see below |

### Authentication

| Variable | Description |
|---|---|
| `AUTH_BASIC_USERNAME`, `AUTH_BASIC_PASSWORD` | Enable basic auth for `/rest/api/v1/*` and `/debug/*`. Set both or neither. Without them the API is unauthenticated and the profiler is disabled. |
| `GITHUB_APP_ID`, `GITHUB_APP_INSTALLATION_ID`, `GITHUB_APP_PRIVATE_KEY` | GitHub App used to access Git repositories; the private key is a PEM-encoded PKCS#1 RSA key. Required for Git references. |

### Helm host providers

Charts can only be fetched from hosts listed in `HELM_HOST_PROVIDERS`; other hosts fail with
`Helm repository provider missing`. The value is a JSON object mapping a host name to a list of providers:

| Field | Description |
|---|---|
| `type` | Required. `http`/`https` for chart repositories, `oci` for OCI registries |
| `schemes` | URL schemes handled by the provider. Defaults to `["http", "https"]` or `["oci"]` |
| `basicAuth` | Optional `username`/`password`, or `usernameEnvVar`/`passwordEnvVar` naming environment variables to read them from. Ignored if either resolves to an empty string. |

```json
{
  "charts.example.com": [
    {"type": "https", "basicAuth": {"usernameEnvVar": "HELM_USER", "passwordEnvVar": "HELM_PASS"}}
  ],
  "registry.example.com": [
    {"type": "oci", "basicAuth": {"usernameEnvVar": "OCI_USER", "passwordEnvVar": "OCI_PASS"}}
  ]
}
```

Invalid JSON or unknown types make startup fail.

### Caching

| Variable | Default | Description |
|---|---|---|
| `SYNCHRONIZATION_METHOD` | `MEMORY` | `MEMORY` (per instance) or `REDIS` (shared across instances) |
| `SYNCHRONIZATION_REDIS_URL` | | Redis address as `host:port`, e.g. `localhost:6379` |
| `SYNCHRONIZATION_REDIS_PASSWORD` | | Redis password, if required |

### Logging and telemetry

| Variable | Default | Description |
|---|---|---|
| `LOG_STYLE` | `PLAIN` | `PLAIN` or `JSON` |
| `LOG_LEVEL` | `INFO` | `TRACE`, `DEBUG`, `INFO`, `WARN`, `ERROR`, `FATAL`, `PANIC`, `SILENT` |
| `LOG_ATTRIBUTE_KEY_MAPPINGS` | ECS-style (`time` → `@timestamp`, `level` → `log.level`, ...) | JSON object renaming log attribute keys |
| `LOG_TIME_TRANSFORMER` | `UTC` | `UTC`, or `ZERO` to replace timestamps with the zero time (useful for tests) |

Traces are exported via OTLP/gRPC and configured with the standard `OTEL_*` variables
(e.g. `OTEL_EXPORTER_OTLP_ENDPOINT`).

## Caching

| Cache | Key | TTL |
|---|---|---|
| Git repositories | repository URL + commit hash | 15 min |
| Helm repository indexes | repository URL | 15 min |
| Helm charts (HTTP) | chart URL + digest | 15 min |
| Helm charts (OCI) | full OCI reference including tag | 5 min |

Git references are resolved to a commit hash on every request, so new commits are picked up immediately. A repository
index can be up to 15 minutes old, so newly published chart versions may take that long to appear. There is no manual
invalidation.

With `REDIS`, Redis is connected lazily and every call is limited to 3 seconds. If Redis is unreachable or failing,
the service logs a warning, uses an in-memory cache with the same TTLs, and retries Redis every 30 seconds. Startup and
requests therefore never depend on Redis being available.

## Operations

| Endpoint | Description |
|---|---|
| `GET /health/liveness`, `GET /health/readiness` | Health checks |
| `GET /metrics` | Prometheus metrics |
| `GET /debug/pprof/` | Go profiler; requires basic auth to be configured |
| `GET /swagger-ui/index.html` | Swagger UI |

The server shuts down gracefully on `SIGTERM`/`SIGINT`. Requests time out after 120 seconds; request bodies are
limited to 32 MiB.

## Security

- Without `AUTH_BASIC_*` the REST API is unauthenticated; only expose it to trusted clients.
- Git repository URLs must point to github.com (`https://github.com/<owner>/<repo>`, `git@github.com:<owner>/<repo>`
  or `ssh://git@github.com/<owner>/<repo>`), so the GitHub App token is never sent to other hosts.
- Kustomizations may only reference files inside the cloned repository; remote resources (HTTP(S), SSH, Git URLs) are
  rejected.
- Helm chart repository and registry credentials are only used for the hosts they are configured for.
- No CORS headers are sent, so browsers cannot read API responses cross-origin.
- The container image is based on `scratch` and currently runs as root.

## Development

```bash
go build ./...
go vet ./...
go test ./...
```

Project layout:

| Path | Contents |
|---|---|
| `main.go` | Entry point; loads `.env` and runs the application |
| `internal/wiring` | Builds the dependency graph and runs the server |
| `internal/config` | Environment configuration |
| `internal/client` | HTTP and GitHub client factory |
| `internal/repository` | Remote access: Git (`git`), Helm repositories and registries (`helmremote`) |
| `internal/service/cache` | Caches for Git repositories, Helm indexes and charts |
| `internal/service/helm`, `internal/service/kustomize` | Chart/Kustomization providers and renderers |
| `internal/resilientcache` | Redis cache wrapper with in-memory fallback |
| `internal/web` | Router, middlewares and controllers |
| `pkg/filesystem`, `pkg/targz` | In-memory filesystem and tar.gz helpers |
| `test/mock` | Mocks for caches, clock, Git and Helm remotes |
| `test/perf` | Replay-based performance tests, see below |

For a local Redis, run `docker compose up -d cache` and set `SYNCHRONIZATION_METHOD=REDIS` and
`SYNCHRONIZATION_REDIS_URL=localhost:6379`.

Performance tests replay recorded requests against the handler stack with in-memory remotes. Scenarios are placed
in `test/resources/perf/<name>` (git-ignored); the format is documented in
[`test/perf/perf_test.go`](test/perf/perf_test.go).

```bash
go test -tags perf ./test/perf/ -run TestPerf -v -count=1
```

API changes are made in [`manifest-maestro-api`](https://github.com/Roshick/manifest-maestro-api); bump its version
in `go.mod` afterwards.

## Contributing

Use [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, ...), since releases are created
from them by semantic-release. Add tests for behavior changes and make sure `go test ./...` and `go vet ./...` pass.

## License

[MIT](LICENSE.md)
