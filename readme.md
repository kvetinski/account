# Account Service

Internal account service with layered architecture (`grpc adapter -> service -> repository -> domain`).

## Runbook

### Local (Docker Compose)
1. `make run-observability`
2. `make test-integration`
3. Open:
   - Grafana: `http://localhost:3000` (`admin/admin`)
   - Jaeger: `http://localhost:16686`
4. `make down`

### Local (Kubernetes + kind)
1. `make k8s-up`
2. `kubectl -n account get pods`
3. In separate terminals:
   - `make k8s-pf-grafana`
   - `make k8s-pf-jaeger`
4. Open:
   - Grafana: `http://localhost:3000`
   - Jaeger: `http://localhost:16686`
5. `make k8s-down-all`

## Docker Compose Useful Commands:
- Start with Docker: `make run`
- Start with tracing (app + otel-collector + jaeger): `make run-tracing`
- Start with metrics (app + prometheus + grafana + cadvisor): `make run-metrics`
- Start with tracing + metrics together: `make run-observability`
- Stop and remove containers/volumes: `make down`
- Run tests locally: `make test`
- Run integration tests only (requires Postgres running): `make test-integration`
- Run integration full flow (up -> test -> down): `make test-up-int-down`

## Kubernetes Useful Commands:
- `make k8s-kind-create`: create local `kind` cluster named `account`.
- `make k8s-load-image`: build `account:local` image and load it into the `kind` cluster.
- `make k8s-apply`: apply all manifests from `k8s/` via kustomize.
- `make k8s-refresh`: re-apply manifests and restart all workloads in namespace `account`.
- `make k8s-migrate`: recreate migration job, run DB migrations, and stream migration logs.
- `make k8s-pf-account`: port-forward account gRPC service to `localhost:8080`.
- `make k8s-pf-grafana`: port-forward Grafana UI to `localhost:3000`.
- `make k8s-pf-prometheus`: port-forward Prometheus UI to `localhost:9090`.
- `make k8s-pf-jaeger`: port-forward Jaeger UI to `localhost:16686`.
- `make k8s-pf-cadvisor`: port-forward cAdvisor to `localhost:8081`.

## Functionality
- Create account by unique `phone` with generated unique `nick`
- Get account by `id`
- Update account nick
- Delete account (soft delete)

## Architecture
- Diagram: `docs/architecture.md`
- Components:
  - `account` gRPC app
  - `postgres` storage
  - `otel-collector` + `jaeger` for tracing
  - `prometheus` + `grafana` + `cadvisor` for metrics

## Account Model
- `id` (UUID)
- `nick` (unique, generated on create)
- `phone` (unique)
- `created_at`
- `updated_at`
- `deleted_at`

## gRPC API
- `account.v1.AccountService/CreateAccount`
- `account.v1.AccountService/GetAccount`
- `account.v1.AccountService/UpdateNick`
- `account.v1.AccountService/DeleteAccount`
- Proto: `proto/account/v1/account.proto`
- Regenerate stubs: `make proto`

## Nick Rules
- Must start with `@`
- Allowed chars after `@`: letters, digits, `_`
- Length: 3..31 total characters

## Phone Rules
- E.164 format: `+` and 8..15 digits total after `+`
- Example: `+15551234567`

## Ports
- gRPC server: `:8080`
- Metrics endpoint: `:9091/metrics`

## Tracing
- Jaeger UI: `http://localhost:16686`
- Service name: `account-service`
- Collector config: `deploy/otel-collector.yaml`
- App exports OTLP traces to `otel-collector:4317` when `OTEL_ENABLED=true`

## Metrics
- App metrics endpoint: `http://localhost:9091/metrics`
- Prometheus UI: `http://localhost:9090`
- Grafana UI: `http://localhost:3000` (`admin` / `admin`)
- cAdvisor metrics source: `http://localhost:8081/metrics`
- Auto-provisioned dashboard: `Account / Account Service Observability`
- Includes gRPC, DB query, DB pool, Go runtime/process, and container metrics.

### Useful PromQL
- gRPC RPS by method:
`sum(rate(account_grpc_requests_total[1m])) by (method)`
- gRPC p95 latency by method:
`histogram_quantile(0.95, sum(rate(account_grpc_request_duration_seconds_bucket[5m])) by (le, method))`
- DB QPS by method:
`sum(rate(account_db_queries_total[1m])) by (method)`
- DB p95 by method:
`histogram_quantile(0.95, sum(rate(account_db_query_duration_seconds_bucket[5m])) by (le, method))`
- DB pool open/in-use/idle:
`max by (pod) (account_db_pool_open_connections)`
`max by (pod) (account_db_pool_in_use_connections)`
`max by (pod) (account_db_pool_idle_connections)`
- DB pool wait count per second:
`rate(account_db_pool_wait_count_total[1m])`
- DB pool avg wait duration (ms):
`1000 * rate(account_db_pool_wait_duration_seconds_total[1m]) / clamp_min(rate(account_db_pool_wait_count_total[1m]), 1e-9)`
- Go CPU seconds:
`rate(process_cpu_seconds_total[1m])`
- Go memory RSS:
`process_resident_memory_bytes`
- Container CPU by container:
`sum(rate(container_cpu_usage_seconds_total[1m])) by (name)`
- Container memory working set:
`sum(container_memory_working_set_bytes) by (name)`

## Integration Tests
- File: `test/account_grpc_int_test.go`
- Tests perform real gRPC calls against a test gRPC server backed by Postgres.
- DB connection env:
  - `TEST_POSTGRES_URI` (default: `postgres://account:account@localhost:5432/account?sslmode=disable`)

## Kubernetes (Local)
- Manifests: `k8s/`
- Quick start guide: `k8s/README.md`

## License

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
