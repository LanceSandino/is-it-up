# Docker Development Environment

This workspace includes a docker-compose setup to spin up local services that
match the checks implemented in `pkg/checks` so you can test the tool end-to-end
on your machine.

## 📚 Related Documentation

- **[Health Check Reference](../docs/CHECKS.md)** — Complete guide to all supported check types
- **[Secrets Configuration](../docs/SECRETS.md)** — AWS Secrets Manager integration
- **[Architecture Overview](../docs/ARCHITECTURE.md)** — System design and components
- **[Main README](../README.md)** — Project overview and quick start

Which services are provided
- `http-app` - HTTP/HTTPS echo server with self-signed certificates (used by HTTP checks for both HTTP and HTTPS testing)
- `http-health` - Sidecar health probe container used to verify `http-app` readiness for compose-based tests
- `mysql` - MySQL server (used by TCP/MySQL checks), exposed on port 3306
- `postgres` - PostgreSQL server (used by TCP/Postgres checks), exposed on port 5432
- `rabbitmq` - RabbitMQ broker with the standard AMQP port (5672)
- `opensearch` - OpenSearch single-node (cluster health endpoint on 9200)
- `redis` - Redis with optional AUTH (used by PING/INFO checks), exposed on port 6379
- `mailpit` - Mailpit SMTP server for email testing (SMTP on 1025, Web UI on 8025)
- `mongo` - MongoDB (DocumentDB-compatible) server for the DocumentDB/Mongo checks, exposed on port 27017

Notes:
- Each service in the compose file includes a basic Docker healthcheck to help ensure readiness before running checks.

Quick start

1. Copy `.env.sample` to `.env` and edit credentials if needed.

2. Start the local stack from the repository root:

```bash
docker compose up -d
```

3. Wait until containers are healthy. Use `docker compose ps` and `docker compose logs -f <service>` to watch startup.

Run the `is-it-up` binary against the example configs

Build the binary first:

```bash
go build -o is-it-up ./cmd/is-it-up
```

Run with the literal config (no Secrets Manager):

```bash
./is-it-up -config example/config.example.yaml
```

Run with the AWS-backed example config (requires AWS credentials in env):

```bash
# export AWS_ACCESS_KEY_ID=... AWS_SECRET_ACCESS_KEY=... AWS_REGION=...
./is-it-up -config example/aws-secret.example.yaml
```

Docker-based run (optional)

If you want to run the compiled binary inside a container, build the image and
mount the example config in:

```bash
# from repo root
docker build -t is-it-up:local .

# Run with literal config mounted
docker run --rm -v "$(pwd)/example/config.example.yaml:/app/config.yaml" is-it-up:local \
  -config /app/config.yaml

# Run with AWS-backed example (forward AWS env vars as needed)
docker run --rm -v "$(pwd)/example/aws-secret.example.yaml:/app/config.yaml" \
  -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_REGION is-it-up:local \
  -config /app/config.yaml
```

## Configuration Examples

The example configurations demonstrate:

- **`config.example.yaml`** — Literal configuration (no secrets)
- **`aws-secret.example.yaml`** — AWS Secrets Manager integration

For detailed configuration syntax and examples, see the **[Health Check Reference](../docs/CHECKS.md)**.

For comprehensive secrets setup, including all supported formats and troubleshooting, see the **[Secrets Configuration Guide](../docs/SECRETS.md)**.

## Notes & troubleshooting
- If Secrets Manager resolution fails inside Docker, confirm AWS credentials are passed to the container correctly (environment vars or IAM role in more advanced setups).
- OpenSearch can be memory hungry; if it fails to start, increase Docker resources or reduce heap sizes in the compose environment.
- For additional troubleshooting guidance, see the [Secrets Configuration Guide](../docs/SECRETS.md#troubleshooting).
