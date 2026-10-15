# is-it-up

`is-it-up` is a small, one-shot health-check CLI for Linux. It checks network
services, databases, queues, mail servers, and search clusters, then writes
structured JSON or readable console results. Run it from a shell, CI, cron,
Docker, or the included systemd timer.

Release binaries and containers support Linux amd64 and arm64. The project is
licensed under the [MIT License](LICENSE).

## Supported checks

| Type | What it verifies | Important options |
| --- | --- | --- |
| `http` | HTTP/HTTPS GET; status 200–399 is healthy | `path`, `tls_mode` |
| `tcp` | A TCP socket can be opened | `address` or `host`/`port` |
| `mysql` | Connect, ping, and `SELECT 1` | `user`, `password`, `database`, `tls_mode` |
| `postgres` | Connect, ping, and `SELECT 1` | `user`, `password`, `database`, `tls_mode` |
| `redis` | AUTH when configured, then PING; optional INFO parsing | `password`, `redis_info`, `tls_mode` |
| `elasticsearch`, `opensearch` | HTTP 200 and cluster health; green/yellow is healthy | `path` |
| `rabbitmq` | AMQP connect and channel open; anonymous checks may fall back to TCP | `user`, `password`, `vhost`, `tls_mode` |
| `smtp` | SMTP handshake, optional authentication, and selected encryption | `user`, `password`, `smtp_encryption` |
| `mongo`, `documentdb` | MongoDB-compatible connection and primary ping | `user`, `password`, `database`, `tls_mode` |

Checks are non-destructive. See the complete [check reference](docs/CHECKS.md)
for accepted fields, success criteria, TLS modes, defaults, and examples.

## Install on Linux

The installer supports amd64 and arm64, downloads the requested GitHub Release,
verifies its SHA-256 checksum, rejects unsafe archive entries and symbolic-link
payloads, and replaces the executable.

The recommended installation lets you inspect the script before using `sudo`:

```bash
curl -fsSLo /tmp/install-is-it-up.sh \
  https://raw.githubusercontent.com/LanceSandino/is-it-up/main/scripts/install.sh
less /tmp/install-is-it-up.sh
sudo /bin/bash /tmp/install-is-it-up.sh --version 2.0.0
is-it-up --version
```

Install the latest release in one command when you have already reviewed the
script and trust this repository:

```bash
curl -fsSL https://raw.githubusercontent.com/LanceSandino/is-it-up/main/scripts/install.sh | sudo /bin/bash
```

Installer options:

```text
--version VERSION       Install a pinned semantic version; default is latest
--install-dir PATH      Install directory; default is /usr/local/bin
--service               Install and enable the systemd service and timer
--interval DURATION     Timer interval such as 30s, 5min, 1h, or 1d
--help                  Show all options
```

### Install as a systemd timer

```bash
sudo /bin/bash /tmp/install-is-it-up.sh \
  --version 2.0.0 --service --interval 5min
sudoedit /etc/is-it-up/config.yaml
sudo systemctl start is-it-up.service
sudo systemctl status is-it-up.service
sudo systemctl status is-it-up.timer
```

The installer creates a non-login `is-it-up` user, preserves an existing
configuration, and enables a hardened oneshot service plus timer. The service
uses `--fail-on-unhealthy`, so an unhealthy result is visible to systemd.

Inspect recent runs with:

```bash
sudo journalctl -u is-it-up.service --since today
sudo systemctl list-timers is-it-up.timer
```

AWS region or credential environment variables may be placed in
`/etc/is-it-up/environment`. Prefer an instance, task, or workload IAM role over
long-lived credentials. Restrict that file to `root:is-it-up` with mode `0640`.

### Upgrade or reinstall

Rerun the installer. Existing `/etc/is-it-up/config.yaml` content is preserved:

```bash
sudo /bin/bash /tmp/install-is-it-up.sh --version 2.0.0
```

Omit `--version` to follow the latest GitHub Release.

### Uninstall

```bash
sudo systemctl disable --now is-it-up.timer
sudo rm -f /etc/systemd/system/is-it-up.timer \
  /etc/systemd/system/is-it-up.service /usr/local/bin/is-it-up
sudo systemctl daemon-reload
```

Configuration is intentionally retained in `/etc/is-it-up`. Remove it and the
`is-it-up` system account separately only when you no longer need that data.

## Configure and run

Create a YAML file with one or more checks:

```yaml
checks:
  - name: web
    type: http
    address: https://example.com
    path: /health
    timeout_ms: 5000
    severity: critical

  - name: database
    type: postgres
    address: database.example.com:5432
    user: healthcheck
    password: sm://is-it-up/config#postgres.password
    database: app
    tls_mode: verify-full
    timeout_ms: 10000
    severity: critical

  - name: cache
    type: redis
    address: cache.example.com:6379
    password: sm://is-it-up/config#redis.password
    tls_mode: required
    redis_info: true
    timeout_ms: 3000
    severity: warning
```

Every check requires a unique `name`, a supported `type`, and either `address`
or `host`/`port`. `timeout_ms` accepts 1000 through 300000 milliseconds and is
enforced as the complete check deadline. `severity` accepts `info`, `warning`,
or `critical`.

Run once with readable output:

```bash
is-it-up --config config.yaml --pretty
```

Run for automation and return exit status 1 if any check is unhealthy:

```bash
is-it-up --config config.yaml --fail-on-unhealthy
```

Useful options:

| Option | Behavior |
| --- | --- |
| `-c`, `--config PATH` | Required YAML configuration path |
| `-l`, `--log-level LEVEL` | `debug`, `info`, `warn`, or `error` |
| `--log-format FORMAT` | `json` or `console` |
| `--debug`, `--verbose` | Enable debug logging |
| `--pretty` | Select console output |
| `--dump-resolved-config` | Resolve references, print a redacted configuration, and exit |
| `--fail-on-unhealthy` | Exit 1 when at least one check is unhealthy |
| `-v`, `--version` | Print the embedded release version |
| `-h`, `--help` | Print complete CLI help |

Exit status behavior:

- `0`: execution completed; all checks were healthy, or unhealthy checks were
  reported without `--fail-on-unhealthy`.
- `1`: an unhealthy check with `--fail-on-unhealthy`, or an output/runtime error.
- `2`: invalid arguments, configuration, or secret resolution.

By default, results are JSON for log collection. `--pretty` selects console
output. Debug mode never disables secret redaction.

Complete configurations are available in
[example/config.example.yaml](example/config.example.yaml) and
[example/aws-secret.example.yaml](example/aws-secret.example.yaml).

## AWS Secrets Manager

Supported string fields can use either a literal value or an AWS Secrets Manager
reference:

```text
sm://SECRET_ID#JSON.KEY?region=REGION&stage=STAGE
```

Mapping syntax, ARN handling, JSON key extraction, IAM permissions, credential
selection, and troubleshooting are covered in the
[secrets guide](docs/SECRETS.md). Resolved values live only for the current run;
logs and resolved-configuration output remain redacted.

## Docker

Run the published image as a non-root, read-only one-shot container:

```bash
docker run --rm --read-only --network host \
  --user 65532:65532 --cap-drop ALL --memory 128m --pids-limit 64 \
  -v "$PWD/config.yaml:/config/config.yaml:ro" \
  ghcr.io/lancesandino/is-it-up:2.0.0 \
  --config /config/config.yaml --fail-on-unhealthy
```

Linux host networking is convenient for services bound to localhost. Prefer an
ordinary Docker network and service DNS names for containerized dependencies. A
hardened Compose example is available at
[example/docker-compose.is-it-up.yml](example/docker-compose.is-it-up.yml).

Build locally:

```bash
docker build --tag is-it-up:local .
docker run --rm is-it-up:local --version
```

## Build and test

Development requires Go 1.27.2 or newer.

```bash
make test             # format, vet, race detector, tests, and >=85% coverage
make static-analysis  # go vet and pinned gosec
make security-scan    # pinned govulncheck
make validate         # parse and validate both shipped YAML examples
make build            # Linux amd64 and arm64 binaries
make package          # release archives and SHA256SUMS
make test-install     # isolated installer integration test
```

All shell entry points use `/bin/bash`. Docker is required only for container
builds and the optional local integration stack under `example/`.

## Releases and support

`VERSION` and [CHANGELOG.md](CHANGELOG.md) define the release. A `vX.Y.Z` tag
runs the release workflow, repeats quality and vulnerability gates, publishes
Linux archives with `SHA256SUMS`, and publishes multi-architecture GHCR images.
GitHub Actions are pinned to immutable commit SHAs with least-privilege
permissions.

See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidance and
[SECURITY.md](SECURITY.md) for private vulnerability reporting.
