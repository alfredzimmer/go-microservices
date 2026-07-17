# Go Microservices

A Go based microservices architecture for e-commerce application.

It includes services for account management, product catalog and order processing. Interservice communications are handled by gRPC. GraphQL serves as the API gateway for the entire microservices. 

```
  

                          ┌──────────────────────┐
   client (port 8000) ──▶ │   GraphQL gateway    │  gqlgen, /playground
                          └───┬──────┬──────┬────┘
                        gRPC  │      │      │  gRPC
                     ┌────────┘      │      └─────────┐
                     ▼               ▼                ▼
               ┌──────────┐   ┌──────────┐    ┌──────────────┐
               │ account  │   │ catalog  │    │    order     │
               └────┬─────┘   └────┬─────┘    └──┬───────┬───┘
                    ▼              ▼             ▼       │ gRPC calls back to
               PostgreSQL    Elasticsearch   PostgreSQL  │ account + catalog
                                                         ▼ (validate account,
                                                            fetch product prices)
```


## How to run

```bash
make certs        # generate the local CA + per-service TLS certificates
docker-compose up -d
```

Then go to `https://localhost:8000/playground`. The gateway serves HTTPS with a
certificate signed by the local CA in `certs/ca.crt`; your browser will warn
unless you import that CA (or click through the warning).

## API documentation

The GraphQL schema is the API documentation: every type, field and argument is
described in [`graphql/schema.graphql`](graphql/schema.graphql), and those
descriptions surface in the playground's **Docs** panel (and any
introspection-based tooling). Explore and run operations there — the e2e suite
in [`e2e/e2e_test.go`](e2e/e2e_test.go) also doubles as a set of working,
tested example queries and mutations.

## Transport security

All inter-service traffic is encrypted with **mutual TLS**: every service holds
an identity certificate signed by a shared local CA (`make certs`) and both
sides of every gRPC connection verify each other. Plaintext clients are
rejected at the handshake. Certificate paths are configured with
`TLS_CERT_FILE`/`TLS_KEY_FILE`/`TLS_CA_FILE`; when unset (in-process tests),
services fall back to plaintext and log a warning.

## Observability

### Distributed tracing

All services are instrumented with OpenTelemetry. Every GraphQL request produces
a single trace that follows the request through the gateway and across the gRPC
hops into the account, catalog and order services, and spans are exported to
Jaeger (started automatically by docker-compose).

- Open the Jaeger UI at `http://localhost:16686`
- Pick a service (e.g. `graphql`) and click **Find Traces**
- Open a trace to see the full request tree with per-hop timings

The OTLP endpoint is configured per service with the standard
`OTEL_EXPORTER_OTLP_ENDPOINT` environment variable.

### Structured logging

Services log JSON to stdout via `log/slog`, tagged with the service name. Logs
written inside a request (`slog.ErrorContext(ctx, ...)`) also carry the
`trace_id`/`span_id` of the active trace, so a log line can be looked up
directly in Jaeger:

```bash
docker-compose logs -f order
```

```json
{"time":"...","level":"ERROR","msg":"Error getting account","service":"order","accountId":"...","error":"...","trace_id":"4bf92f3577b34da6a3ce929d0e0e4736","span_id":"00f067aa0ba902b7"}
```
