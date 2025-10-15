# SmallBiznis Control Plane

SmallBiznis Control Plane is the orchestration layer that brings together tenant provisioning, domain management, identity, and downstream integrations for the SmallBiznis platform. The services are written in Go, expose gRPC with an HTTP gateway, rely on Uber Fx for dependency injection, and ship with OpenTelemetry instrumentation so multi-tenant commerce workloads can scale reliably.

## Key Capabilities
- Manage the tenant lifecycle while generating default domains and credentials in a single transactional flow.
- Perform hostname verification via DNS lookups, row locking, and certificate status updates.
- Orchestrate downstream services (ledger, loyalty, voucher, rule engine) through Asynq queues for provisioning and event-driven work.
- Load configuration from local files, environment variables, or remote providers with Vault-backed secret injection.
- Deliver unified observability through OpenTelemetry, Pyroscope, structured logging, and integrations with PostgreSQL, Redis, Kafka, and Temporal.

## Repository Layout
| Path | Description |
| --- | --- |
| `cmd/<service>/` | Entry points for each service together with example `config.yaml` files. |
| `pkg/` | Shared packages for configuration, database access, gRPC/HTTP servers, logging, Redis, Snowflake IDs, and the task queue. |
| `services/` | Domain logic per module (tenant, domain, voucher, transaction, rule, etc.) including HTTP/gRPC gateways. |
| `docker-compose.yaml` | Compose stack for the control plane and supporting services. |
| `.env` | Sample environment variables for the observability stack (Tempo, Loki, Pyroscope, Grafana). |
| `flow-*.mmd` | Additional diagrams in Mermaid format. |

## System Requirements
- Go 1.25 or newer.
- Docker & Docker Compose (optional, for running the full stack).
- PostgreSQL and Redis instances (local or containerized).
- Access to Consul and Vault when using remote configuration and secret injection.

## Quick Setup
1. Clone the repository and switch to the workspace:
   ```bash
   git clone https://github.com/smallbiznis/smallbiznis-controlplane.git
   cd smallbiznis-controlplane
   ```
2. Copy the configuration file when creating a local override:
   ```bash
   cp cmd/controlplane/config.yaml cmd/controlplane/config.local.yaml
   ```
   Update database, Redis, and domain settings to match your environment. The application loads `config.yaml` by default.
3. Review `.env` if you plan to run `docker compose`; it provides shared settings for Tempo, Loki, Pyroscope, and Grafana.

## Running Services
### Go (development mode)
```bash
go run ./cmd/controlplane
```
This starts the gRPC server and HTTP gateway. Additional services such as `cmd/ledger`, `cmd/rule`, `cmd/loyalty`, or `cmd/voucher` can be launched separately when their integrations are required.

### Docker Compose
```bash
docker network create infra_default  # one-time setup if the network does not exist
docker compose up --build controlplane ledger rule loyalty voucher
```
Each container builds the Go binary defined by `SERVICE_PATH` and mounts its configuration from `cmd/<service>/config.yaml`. Ensure external dependencies (PostgreSQL, Redis, Tempo, etc.) are reachable on the same network.

## Configuration
- `cmd/controlplane/config.yaml` contains core settings such as `APP_ENV`, `ROOT_DOMAIN`, `DATABASE`, `REDIS`, `SESSION`, and downstream service endpoints.
- The `ACCESS_CONTROL` section uses Casbin model/policy files for authorization.
- Observability (`OTEL`, `PYROSCOPE`) and optional integrations (`MINIO`, `FLAGSMITH`, `TEMPORAL`) can be toggled as needed.
- To consume remote configuration, set `REMOTE_CONFIG_PROVIDER`, `REMOTE_CONFIG_ADDR`, and `REMOTE_CONFIG_PATH`, then wire `config.RemoteModule`. Vault injects secrets (Postgres, Redis, Flagsmith, AES key) into the configuration struct at startup.

## Observability & Integrations
- **OpenTelemetry**: configure `OTEL_EXPORTER_OTLP_ENDPOINT` or populate the `OTEL` section to stream traces and metrics.
- **Pyroscope**: set `PYROSCOPE.ADDR` to enable continuous profiling.
- **Asynq**: queues run on Redis (`pkg/task`). Handlers in voucher and provisioning services process background jobs.
- **Kafka & Temporal**: endpoints are configurable for event streaming and workflow coordination.
- **Health Check**: the HTTP gateway exposes `GET /healthz`.

## Development & Testing
- Run all unit tests:
  ```bash
  go test ./...
  ```
- Target a specific package:
  ```bash
  go test ./services/tenant -run TestTenantService
  ```
- Database models live under `services/*/model.go`; apply schema changes before testing against a real database.
- When adding new Asynq tasks, register handlers via `pkg/task` and confirm the relevant queue is provisioned.

## Contributing
- Create a feature branch from `main`.
- Include tests with every change and run `go test ./...` before submitting.
- Document configuration updates or required steps in the pull request description.

## License
This project is proprietary to SmallBiznis. Contact the maintainers for licensing or redistribution inquiries.
