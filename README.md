# SmallBiznis Control Plane

SmallBiznis Control Plane is the orchestration layer that stitches together tenant provisioning, domain management, identity, and downstream service integrations for the SmallBiznis platform. It is written in Go and built around gRPC/HTTP APIs, Uber Fx dependency injection, and OpenTelemetry instrumentation so the platform team can manage multi-tenant commerce workloads at scale.

## Features

- **Tenant lifecycle management** – create, list, and paginate tenants while automatically generating default domains and API credentials through a single transactional flow. 【F:services/tenant/service.go†L1-L149】
- **Domain verification workflow** – verify hostnames via DNS lookups, lock rows to avoid race conditions, and update certificate status in the control plane database. 【F:services/domain/service.go†L1-L134】
- **Configurable via Consul/Vault** – load configuration from local files or remote secrets stores (Consul KV + Vault) with automatic environment overrides. 【F:pkg/config/config.go†L1-L153】【F:pkg/config/config.go†L155-L236】
- **Instrumented infrastructure** – emits traces/metrics via OpenTelemetry, queues background jobs with Asynq, and supports Redis, PostgreSQL, Kafka, and Temporal integrations. 【F:cmd/controlplane/main.go†L1-L52】【F:pkg/config/config.go†L23-L129】

## Repository layout

| Path | Description |
| --- | --- |
| `cmd/<service>/` | Main entrypoints, Dockerfiles, and service-specific configuration. 【F:cmd/controlplane/main.go†L1-L52】|
| `pkg/` | Shared libraries for configuration, logging, database access, Redis, HTTP/gRPC servers, security, and utilities. 【F:pkg/config/config.go†L1-L236】|
| `services/` | Domain logic for tenants, domains, API keys, inventory, ledger, loyalty, provisioning, and more. 【F:services/tenant/service.go†L1-L149】【F:services/domain/service.go†L1-L134】|
| `docker-compose.yaml` | Multi-service development stack that builds the control plane and companion services with shared configuration volumes. 【F:docker-compose.yaml†L1-L38】|

## Getting started

### Prerequisites

- Go 1.25+
- Docker & Docker Compose (for running dependencies and optional services)
- PostgreSQL and Redis (local instances or via Docker)
- Access to Consul and Vault if you plan to use remote configuration

### Clone and configure

```bash
git clone https://github.com/<your-org>/smallbiznis-controlplane.git
cd smallbiznis-controlplane
cp cmd/controlplane/config.yaml cmd/controlplane/config.local.yaml # adjust values for your environment
```

> The control plane expects a reachable PostgreSQL instance, Redis, and (optionally) ancillary services such as Temporal, Kafka, MinIO, and Flagsmith. Review `cmd/controlplane/config.yaml` for sample values. 【F:cmd/controlplane/config.yaml†L1-L89】

### Run locally with Go

```bash
# Ensure required environment variables are exported (see configuration section below)
go run ./cmd/controlplane
```

This starts both the gRPC and HTTP servers defined in `server.ProvideGRPCServer` and `server.ProvideHTTPServer`, wiring dependencies through the Fx container. 【F:cmd/controlplane/main.go†L23-L48】

### Run with Docker Compose

The repository includes a `docker-compose.yaml` file that builds the control plane and its sibling services (ledger, rule, loyalty, inventory, voucher). Each container mounts the matching `cmd/<service>/config.yaml` file for runtime configuration.

```bash
docker compose up --build
```

All containers share the external `infra_default` network so they can talk to provisioned infrastructure. 【F:docker-compose.yaml†L1-L38】

## Configuration

Configuration is provided by Viper. By default, the control plane loads `config.yaml` from the working directory and overlays environment variables using the `FOO_BAR` naming convention for nested keys.

Key settings include:

- `ROOT_DOMAIN`: used to generate tenant-specific hostnames.
- `DATABASE`: database connection details and pooling configuration.
- `REDIS`: Redis instance for caching, queues, and session storage.
- `ACCESS_CONTROL`: Casbin model and policy adapters for authorization.
- `MINIO`, `FLAGSMITH`, `TEMPORAL`, `PYROSCOPE`: optional integrations.

When Vault is supplied to the Fx container, secrets such as database credentials and API keys are injected at startup. Remote configuration (Consul KV) is also supported by setting the `REMOTE_CONFIG_*` environment variables before boot. 【F:pkg/config/config.go†L23-L153】【F:pkg/config/config.go†L155-L236】

## Development workflow

### Database migrations

Migrations are not bundled, but the services rely on GORM models located in `services/*/model.go`. Apply schema changes before running the control plane.

### Testing

```bash
go test ./...
```

This runs unit tests across all services and shared packages. For deterministic results, ensure dependent services (e.g., PostgreSQL, Redis) are available or mock them appropriately. 【F:services/tenant/service_test.go†L1-L200】

## Observability

The control plane uses OpenTelemetry for tracing and metrics. Configure `OTEL_EXPORTER_OTLP_ENDPOINT` (or set the `OTEL` section in the config file) to emit telemetry to your collector. Pyroscope support is available through the `PYROSCOPE.ADDR` setting for continuous profiling. 【F:cmd/controlplane/main.go†L29-L44】【F:pkg/config/config.go†L35-L73】

## Contributing

1. Fork the repository and create a feature branch.
2. Make your changes along with tests.
3. Run `go test ./...` to ensure everything passes.
4. Submit a pull request with a detailed description of your changes and testing notes.

## License

This project is proprietary to SmallBiznis. Contact the maintainers for licensing information.
