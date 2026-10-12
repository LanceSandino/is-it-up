# Changelog

All notable user-facing changes are documented here. This project follows
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-10-12

### Added

- HTTP/HTTPS GET checks with TLS controls and 200-399 status validation, plus
  direct TCP reachability checks.
- MySQL and PostgreSQL checks that connect, ping, and run `SELECT 1`, with
  database selection and protocol-specific TLS modes.
- Redis AUTH/PING checks with optional INFO role and memory reporting.
- Elasticsearch and OpenSearch checks that accept green or yellow cluster
  health and reject red health.
- RabbitMQ AMQP connection and channel checks with TCP reachability fallback.
- SMTP checks for plain, STARTTLS, and implicit TLS connections with optional
  authentication.
- MongoDB and DocumentDB-compatible primary-ping checks with URI, database,
  credential, and TLS support.
- YAML configuration with per-check severity, enforced deadlines, address or
  host/port forms, and optional AWS Secrets Manager references.
- Structured JSON and human-readable console logging, redacted resolved-config
  inspection, debug logging, and an opt-in unhealthy exit status.
- Checksum-verified Linux installation, hardened systemd timer setup, a
  non-root scratch container, and amd64/arm64 release artifacts.
- Continuous integration for race-enabled tests, coverage, static analysis,
  vulnerability scanning, committed-secret scanning, installer integration,
  release packaging, and container builds.

### Changed

- Version metadata is injected into reproducible release builds.
- Health-check failures can optionally produce exit status 1 with
  `--fail-on-unhealthy`; the default remains log-oriented and returns success
  after a completed run.
- `timeout_ms` now governs the complete deadline for every supported protocol.
- `--verbose` is an alias for debug-level logging.

### Security

- Configuration dumps always redact resolved secrets, including in debug mode.
- Redis commands use RESP bulk strings so credential values cannot inject
  additional commands.
- Endpoint credentials are removed from diagnostic output.
- RabbitMQ, MongoDB, and Go text dependencies were updated to versions without
  known reachable vulnerabilities.
- Release builds use Go 1.27.2, including the current standard-library security
  fixes, and compatible analysis tools are pinned to their latest stable releases.

[1.0.0]: https://github.com/LanceSandino/is-it-up/releases/tag/v1.0.0
