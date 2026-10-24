# Changelog

All notable user-facing changes are documented here. This project follows
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.1.1] - 2026-10-24

Security release. Upgrading is recommended for every 2.x installation.

### Security

- Release binaries and container images are built with Go 1.27.3, which fixes
  GO-2026-6809 (CVE-2026-111943): `crypto/x509` now refuses known compromised
  intermediate certificates. govulncheck found the affected certificate
  verification path reachable in 2.1.0 through the TLS-enabled checks.
- The container build stage uses `golang:1.27.3-alpine`, pinned by digest.
- Go module dependencies are updated to their latest releases, including the
  AWS SDK for Go v2, `rabbitmq/amqp091-go` 1.16.0, and `klauspost/compress`
  1.20.3. govulncheck reports no reachable vulnerabilities.

### Changed

- `example/docker-compose.yml` uses currently supported service images:
  MySQL 8.4, PostgreSQL 18, Redis 8, RabbitMQ 4, OpenSearch 3.9.0,
  MongoDB 8.0, and curl 8.23.0. Mailpit replaces the unmaintained MailHog,
  and `mendhak/http-https-echo` replaces the unmaintained HTTPS echo server.
- Static analysis uses gosec 2.30.0. A weekly workflow re-runs govulncheck,
  and Dependabot now also tracks the Docker base image.

## [2.1.0] - 2026-10-14

### Added

- `dns` check type that verifies a hostname resolves, configured with `host` or
  `address` (a `host:port` address has its port ignored).
- Check registry: check types register through `checks.RegisterCheckType`, and
  validation and execution both read the registry. Plugin packages are activated
  with a blank import at compile time; there is no runtime plugin loading.

### Changed

- Every check type, including registered plugins, now runs with panic
  containment, the configured deadline, and secret redaction applied by the
  runner rather than per type.

## [2.0.0] - 2026-10-14

This is a major release because the secure defaults below can make existing
1.x configurations fail after upgrading.

### Upgrading from 1.x

- MySQL, PostgreSQL, Redis, and RabbitMQ checks that send credentials now
  require verified TLS by default. If a service only accepts plaintext, set
  `tls_mode` to the protocol's explicit plaintext or skip-verify mode (see
  [docs/CHECKS.md](docs/CHECKS.md)). PostgreSQL now defaults to
  `verify-full` instead of `require`; set `tls_mode: require` explicitly to keep
  the old encrypted-but-unverified behavior.
- HTTP checks no longer follow redirects. Point each check at the final URL,
  or expect a 3xx response to count as healthy (200-399).
- An HTTP check with `tls_mode: required` and an `http://` URL is now a
  configuration error.
- Anonymous RabbitMQ checks may still fall back to TCP reachability; checks
  with credentials no longer do.
- The Go module path is now `github.com/LanceSandino/is-it-up/v2`.

### Changed

- Credential-bearing MySQL, PostgreSQL, Redis, and RabbitMQ checks now default
  to verified TLS. Plaintext and certificate-skipping behavior require an
  explicit compatibility mode.
- RabbitMQ TCP fallback is limited to anonymous checks, PostgreSQL connection
  timeouts follow the check deadline, and database DSNs are built from escaped
  components.
- HTTP checks no longer follow redirects and reject `tls_mode: required` with a
  plaintext URL.

### Security

- Secret provenance is preserved through resolution and all secret-derived
  fields are redacted from validation errors, peer failures, runtime results,
  debug output, and resolved-configuration output.
- MySQL and PostgreSQL malformed-handshake panics are contained per check so an
  untrusted peer cannot terminate the process.
- HTTP and Elasticsearch response bodies are limited to 1 MiB, SMTP replies to
  64 KiB, and RabbitMQ frame size to 128 KiB.
- SMTP STARTTLS modes now fail closed when the server does not advertise the
  upgrade.
- DocumentDB URI options remain intact when TLS overrides are applied, and AWS
  Secrets Manager version stages are bound to requests and cache identities.
- Console output escapes untrusted terminal control bytes.
- The Linux installer passes GitHub authorization to curl over standard input
  and removes the token from curl's process environment.
- Release publication rejects a tag that does not target the current `main`
  commit, and the packaged systemd service and Docker examples apply memory,
  task/process, and file descriptor ceilings.

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

[2.1.1]: https://github.com/LanceSandino/is-it-up/compare/v2.1.0...v2.1.1
[2.1.0]: https://github.com/LanceSandino/is-it-up/compare/v2.0.0...v2.1.0
[2.0.0]: https://github.com/LanceSandino/is-it-up/compare/v1.0.0...v2.0.0
[1.0.0]: https://github.com/LanceSandino/is-it-up/releases/tag/v1.0.0
