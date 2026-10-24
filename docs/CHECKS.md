# Health Check Reference

This document provides comprehensive documentation for all supported health check types in the is-it-up tool.

## Table of Contents

- [Overview](#overview)
- [Common Configuration](#common-configuration)
- [Check Types](#check-types)
  - [HTTP Checks](#http-checks)
  - [TCP Checks](#tcp-checks)
  - [DNS Checks](#dns-checks)
  - [MySQL Database Checks](#mysql-database-checks)
  - [PostgreSQL Database Checks](#postgresql-database-checks)
  - [Redis Cache Checks](#redis-cache-checks)
  - [DocumentDB / MongoDB Checks](#documentdb--mongodb-checks)
  - [Elasticsearch/OpenSearch Checks](#elasticsearch--opensearch-checks)
  - [RabbitMQ Message Queue Checks](#rabbitmq-message-queue-checks)
  - [SMTP Mail Server Checks](#smtp-mail-server-checks)
- [Advanced Examples](#advanced-examples)
- [Troubleshooting](#troubleshooting)

---

## Overview

Health checks are designed to be **non-destructive** and **lightweight**. Each check type implements specific protocols to verify service availability and basic functionality.

### Design Principles
- **Read-only operations**: No data modification
- **Fast execution**: Optimized for frequent polling (1-5 minute intervals)
- **Detailed reporting**: Structured results with timing and error information
- **Secret integration**: Support for dynamic credential resolution

---

## Common Configuration

All check types support these common fields:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | ✓ | Unique identifier for the check |
| `type` | string | ✓ | Check type (http, tcp, mysql, postgres, redis, documentdb/mongo, elasticsearch/opensearch, rabbitmq, smtp) |
| `severity` | string | | Severity level: info, warning, critical |
| `timeout_ms` | integer | | Complete check deadline in milliseconds (1000-300000) |

### Default Timeouts
- HTTP: 4 seconds
- TCP: 3 seconds
- MySQL: 4 seconds
- PostgreSQL: 4 seconds
- Redis: 3 seconds
- Elasticsearch: 4 seconds
- RabbitMQ: 4 seconds
- SMTP: 10 seconds
- MongoDB/DocumentDB: 4 seconds

An explicit `timeout_ms` replaces the protocol default and covers the complete
check, including authentication, optional commands, and fallback behavior.

### Secret Support
Any string field can reference secrets using:
- **Compact URI**: `sm://secret-name#key`
- **Mapping form**: See [Secrets Documentation](SECRETS.md)

**Working Examples**: See `example/aws-secret.example.yaml` which uses:
- `sm://is-it-up/config` — Credentials and configuration values
- `sm://is-it-up/web-app-address` — Simple endpoint values

---

## Check Types

### HTTP Checks

**Purpose**: Verify web service availability and response status

**Method**: HTTP GET request with status code validation

**Configuration**:
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `address` | string | ✓ | Full URL with protocol (http:// or https://) |
| `path` | string | | Path to append to address |
| `tls_mode` | string | | Canonical TLS mode for checks that support TLS. Allowed values vary by check type (see below). Example: skip-verify, required, disable, preferred, verify-full |

**Important**: The `address` field must include the protocol scheme. For HTTPS URLs, TLS validation defaults to `required` (secure). Use `tls_mode: skip-verify` for self-signed certificates. Setting `tls_mode: required` with an `http://` address is rejected before any request is sent. Redirects are returned as the checked response and are never followed automatically, so a check cannot cross to another authority. Response bodies are limited to 1 MiB.

**Success Criteria**: HTTP status codes 200-399
**Failure Criteria**: HTTP status codes 400+, connection errors, timeouts

**Examples**:

```yaml
# HTTP endpoint (no TLS configuration needed)
- name: web-frontend
  type: http
  address: http://api.example.com
  path: /health
  timeout_ms: 5000
  severity: critical

# HTTPS endpoint with default secure TLS
- name: secure-api
  type: http
  address: https://api.example.com
  path: /health
  severity: critical

# HTTPS with self-signed certificate
- name: internal-api
  type: http
  address: https://internal.example.com
  path: /health
  tls_mode: skip-verify
  severity: warning

# HTTPS with secrets
- name: secure-api
  type: http
  address: sm://api-config#base_url
  path: /v1/health
  severity: critical
```

**Result Details**:
```
http GET address=https://api.example.com/health
```

---

### TCP Checks

**Purpose**: Verify network connectivity and port availability

**Method**: TCP socket connection attempt

**Configuration**:
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `address` | string | ✓ | Host:port combination |

**Success Criteria**: Successful TCP connection establishment
**Failure Criteria**: Connection refused, timeout, DNS resolution failure

**Examples**:

```yaml
# SSH service availability
- name: ssh-access
  type: tcp
  address: server.example.com:22
  timeout_ms: 2000
  severity: warning

# Database port check
- name: db-connectivity
  type: tcp
  address: localhost:5432
  severity: critical

# Load balancer health
- name: load-balancer
  type: tcp
  address: lb.example.com:80
  severity: critical
```

**Result Details**:
```
tcp connect to server.example.com:22
```

---

### DNS Checks

**Purpose**: Verify that a hostname resolves to at least one address.

**Method**: Resolve the name with Go's system-aware resolver, which honors
`/etc/hosts` and `/etc/resolv.conf`. No connection is opened to the resolved
addresses.

**Configuration**:
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `host` or `address` | string | ✓ | Hostname to resolve (supports secrets). A `host:port` address is accepted and the port is ignored. URLs are rejected. |

**Success Criteria**: The resolver returns at least one address.
**Failure Criteria**: Resolution error, an invalid hostname, or the check deadline expiring.

```yaml
- name: public-dns
  type: dns
  host: example.com
  timeout_ms: 3000
  severity: warning
```

Details list the resolved addresses, for example
`dns lookup host=example.com addresses=93.184.215.14,2606:2800:21f:cb07:6820:80da:af6b:8b2c`.

---

### MySQL Database Checks

**Purpose**: Verify database connectivity and basic query execution

**Method**: Direct MySQL database connection with ping and SELECT 1 query

**Configuration**:
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `address` | string | ✓ | Host:port for MySQL server |
| `user` | string | ✓ | Database username (supports secrets) |
| `password` | string | | Database password (supports secrets) |
| `database` | string | | Database name (no database selected by default) |
| `tls_mode` | string | | TLS mode: required (verified), skip-verify, preferred, or disable |

When a password is configured, TLS defaults to `required` and verifies the
server certificate and hostname. `skip-verify`, `preferred`, and `disable` are
explicit compatibility modes and should not be used for production credentials.
User names containing MySQL DSN delimiters are rejected rather than interpreted
as connection parameters.

**Success Criteria**: Successful database connection, ping, AND SELECT 1 execution
**Failure Criteria**: Connection failure, authentication failure, ping failure, or query failure

**Result**: Generates **single result** with direct database connectivity status

**Examples**:

```yaml
# Production database
- name: primary-db
  type: mysql
  address: rds.us-east-1.amazonaws.com:3306
  user: healthcheck
  password: sm://db-credentials#mysql_password
  tls_mode: required
  timeout_ms: 10000
  severity: critical

# Local development database
- name: local-mysql
  type: mysql
  address: localhost:3306
  user: root
  password: development_pass
  tls_mode: disable
  severity: warning

# Read replica with secrets
- name: read-replica
  type: mysql
  address: sm://db-config#replica_host
  user: sm://db-credentials#readonly_user
  password: sm://db-credentials#readonly_password
  severity: warning
```

**Result Details**:
```
mysql SELECT 1 on rds.us-east-1.amazonaws.com:3306 as user=healthcheck password=REDACTED
```

---

### PostgreSQL Database Checks

**Purpose**: Verify database connectivity and basic query execution

**Method**: Direct PostgreSQL database connection with ping and SELECT 1 query

**Configuration**:
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `address` | string | ✓ | Host:port for PostgreSQL server |
| `user` | string | ✓ | Database username (supports secrets) |
| `password` | string | | Database password (supports secrets) |
| `database` | string | | Database name (default: postgres) |
| `tls_mode` | string | | TLS mode: disable, require, verify-ca, or verify-full |

PostgreSQL defaults to `verify-full`, including certificate and hostname
verification. `require` encrypts the connection but does not verify peer
identity, so use it only as an explicit compatibility mode. The check derives a
driver connection timeout from the complete check deadline.

**Success Criteria**: Successful database connection, ping, AND SELECT 1 execution
**Failure Criteria**: Connection failure, authentication failure, ping failure, or query failure

**Result**: Generates **single result** with direct database connectivity status

**Examples**:

```yaml
# Production database
- name: primary-db
  type: postgres
  address: postgres.us-east-1.rds.amazonaws.com:5432
  user: healthcheck
  password: sm://db-credentials#postgres_password
  database: appdb
  tls_mode: verify-full
  timeout_ms: 10000
  severity: critical

# Local development database
- name: local-postgres
  type: postgres
  address: localhost:5432
  user: postgres
  password: development_pass
  database: appdb
  tls_mode: disable
  severity: warning

# Read replica with secrets
- name: read-replica
  type: postgres
  address: sm://db-config#replica_host
  user: sm://db-credentials#readonly_user
  password: sm://db-credentials#readonly_password
  database: appdb
  severity: warning
```

**Result Details**:
```
postgres SELECT 1 on postgres.us-east-1.rds.amazonaws.com:5432 as user=healthcheck password=REDACTED sslmode=verify-full
```

---

### DocumentDB / MongoDB Checks

**Purpose**: Verify MongoDB-compatible (DocumentDB) connectivity and basic ping.

**Method**: Connect using a MongoDB URI and issue a ping command (non-destructive).

**Configuration**:
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `address` | string | ✓ | Host:port or full MongoDB URI (mongodb://host:27017 or mongodb+srv://...) |
| `user` | string | | Username for authentication (supports secrets) |
| `password` | string | | Password for authentication (supports secrets) |
| `database` | string | | Database to connect to (default: admin) |
| `tls_mode` | string | | Generic TLS mode mapping used by this check: disable, required, skip-verify |

**Success Criteria**: Successful connection and `ping` command (driver-level ping).
**Failure Criteria**: Connection or authentication failure, ping failure, or timeout.

**Examples**:

```yaml
- name: local-documentdb
  type: documentdb
  address: localhost:27017
  user: root
  password: examplepass
  timeout_ms: 4000
  severity: critical

# Using a full MongoDB URI
- name: cloud-mongo
  type: mongo
  address: "mongodb+srv://<username>:<password>@cluster0.example.mongodb.net/admin"
  timeout_ms: 4000
  severity: critical

# Using a secret-stored full URI
- name: cloud-mongo-secret
  type: mongo
  address: sm://is-it-up/config#mongo_uri
  timeout_ms: 4000
  severity: critical
```

This check is intentionally simple: it verifies connectivity and basic liveness via the MongoDB driver's Ping operation. For advanced TLS or certificate configuration, provide a full URI with appropriate parameters or use secrets-backed files referenced by your environment and CLI invocation.

### Redis Cache Checks

**Purpose**: Verify Redis connectivity and optional performance metrics

**Method**: Redis PING command + optional INFO command

**Configuration**:
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `address` | string | ✓ | Host:port for Redis server |
| `password` | string | | Redis AUTH password (supports secrets) |
| `redis_info` | boolean | | Enable INFO command for metrics |
| `tls_mode` | string | | required (verified), skip-verify, or disable |

Credential-bearing checks default to verified TLS. `tls_mode: disable` is
required for an authenticated plaintext endpoint; `skip-verify` encrypts but
does not authenticate the server and is intended only for controlled testing.

**Success Criteria**: Successful PING response (and INFO if enabled)
**Failure Criteria**: Connection failure, AUTH failure, command timeout

**Examples**:

```yaml
# ElastiCache cluster with metrics
- name: cache-cluster
  type: redis
  address: elasticache.abcd.cache.amazonaws.com:6379
  password: sm://cache-secrets#redis_auth_token
  tls_mode: required
  redis_info: true
  timeout_ms: 3000
  severity: critical

# Simple connectivity check
- name: redis-session
  type: redis
  address: redis.internal:6379
  redis_info: false
  severity: warning

# Local development Redis
- name: local-redis
  type: redis
  address: localhost:6379
  tls_mode: disable
  redis_info: true
  severity: info
```

**Result Details**:
```yaml
# Basic PING
pong

# With redis_info: true
pong; info: role=master used_memory=908528
```

**INFO Metrics Parsed**:
- `role`: master/slave replication role
- `used_memory`: current memory usage in bytes

---

### Elasticsearch / OpenSearch Checks

**Purpose**: Verify search cluster health and availability

**Method**: HTTP GET to cluster health endpoint with status parsing

**Configuration**:
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `address` | string | ✓ | Full HTTP URL to cluster |
| `path` | string | | Health endpoint path (default: _cluster/health) |

**Success Criteria**: HTTP 200 response with cluster status green/yellow
**Failure Criteria**: HTTP errors, red cluster status, connection failure, or a response larger than 1 MiB

**Supported Types**: `elasticsearch` (alias: `opensearch`)

**Examples**:

```yaml
# AWS OpenSearch domain
- name: search-cluster
  type: elasticsearch
  address: https://search-prod.us-east-1.es.amazonaws.com
  path: _cluster/health
  timeout_ms: 4000
  severity: critical

# Self-hosted Elasticsearch
- name: logs-cluster
  type: elasticsearch
  address: http://elasticsearch.internal:9200
  severity: warning

# Custom health endpoint
- name: search-api
  type: elasticsearch
  address: http://search.example.com:9200
  path: _custom/health
  severity: critical
```

**Result Details**:
```yaml
# Successful check
cluster green

# Health endpoint details
elasticsearch GET url=https://search-prod.us-east-1.es.amazonaws.com/_cluster/health result=cluster green
```

---

### RabbitMQ Message Queue Checks

**Purpose**: Verify message queue connectivity and channel operations

**Method**: AMQP connection + channel creation; anonymous checks may use a TCP fallback

**Configuration**:
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `address` | string | ✓ | Host:port for RabbitMQ server |
| `user` | string | | AMQP username (supports secrets) |
| `password` | string | | AMQP password (supports secrets) |
| `vhost` | string | | Virtual host (default: "/") |
| `tls_mode` | string | | required (verified), skip-verify, or disable |

**Behavior**:
- Credential-bearing checks default to verified TLS. Plaintext requires
  `tls_mode: disable`; `skip-verify` is an explicit testing-only mode.
- The check attempts an AMQP connection and channel open first. Only anonymous
  checks may fall back to a TCP connection. Authentication or channel failures
  never become a healthy result through TCP fallback.

**Examples**:

```yaml
# Production AMQP cluster
- name: message-broker
  type: rabbitmq
  address: rmq-cluster.example.com:5672
  user: healthcheck
  password: sm://mq-secrets#rabbitmq_password
  tls_mode: required
  vhost: /production
  timeout_ms: 3000
  severity: critical
```

**Result Details**:
```yaml
# AMQP method
amqp connect to rmq-cluster.example.com:5672 vhost=/production

# TCP method
tcp connect to localhost:5672
```

---

### SMTP Mail Server Checks

**Purpose**: Verify email server connectivity, authentication, and encryption

**Method**: Direct SMTP protocol test with optional authentication and encryption

**Configuration**:
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `address` | string | ✓ | Host:port for SMTP server |
| `user` | string | | SMTP username (supports secrets) |
| `password` | string | | SMTP password (supports secrets) |
| `smtp_encryption` | string | | Encryption: tls, starttls, ssl, smtps, none, plain |

**Encryption Modes**:
- **tls/starttls**: Connect plain, then upgrade with STARTTLS (port 587)
- **ssl/smtps**: Direct SSL/TLS connection (port 465)
- **none/plain**: No encryption (port 587, not recommended for production)

**Success Criteria**: SMTP protocol handshake AND optional authentication success
**Failure Criteria**: Connection, protocol, encryption, authentication, or oversized server reply failure

`starttls` and `tls` require the server to advertise and complete STARTTLS;
there is no plaintext fallback. SMTP protocol replies are limited to 64 KiB.

**Examples**:

```yaml
# AWS SES with authentication
- name: ses-smtp
  type: smtp
  host: email-smtp.us-east-1.amazonaws.com
  port: "587"
  user: sm://email-config#aws_smtp_user
  password: sm://email-config#aws_smtp_password
  smtp_encryption: starttls
  timeout_ms: 10000
  severity: critical

# Local development (no auth)
- name: local-smtp
  type: smtp
  address: localhost:1025
  smtp_encryption: none
  severity: info
```

**Result Details**:
```yaml
# Successful authentication
smtp auth ok (starttls)

# Connection without authentication
smtp connect ok (none)

# Authentication failure
smtp check 'ses-smtp' auth failed: 535 Authentication failed
```

**Common SMTP Configurations**:

| Provider | Host | Port | Encryption | Notes |
|----------|------|------|------------|-------|
| AWS SES | email-smtp.{region}.amazonaws.com | 587 | starttls | Requires IAM SMTP credentials |
| Office 365 | smtp.office365.com | 587 | starttls | Requires app password |
| Gmail | smtp.gmail.com | 587/465 | starttls/ssl | Requires app password |
| Mailpit | localhost | 1025 | none | Development only |

---

## Advanced Examples

### Multi-Environment Configuration

```yaml
# Production services
checks:
  # Web tier
  - name: api-gateway
    type: http
    address: https://api.prod.example.com
    path: /health
    timeout_ms: 5000
    severity: critical

  - name: web-app
    type: http
    address: https://app.prod.example.com
    path: /status
    timeout_ms: 8000
    severity: critical

  # Data tier
  - name: primary-database
    type: mysql
    address: sm://db-config#primary_host
    user: sm://db-credentials#health_user
    password: sm://db-credentials#health_password
    tls_mode: required
    timeout_ms: 10000
    severity: critical

  - name: read-replica
    type: mysql
    address: sm://db-config#replica_host
    user: sm://db-credentials#readonly_user
    password: sm://db-credentials#readonly_password
    tls_mode: required
    severity: warning

  # Cache tier
  - name: redis-sessions
    type: redis
    address: sm://cache-config#sessions_host
    password: sm://cache-credentials#sessions_password
    redis_info: true
    severity: critical

  - name: redis-cache
    type: redis
    address: sm://cache-config#cache_host
    password: sm://cache-credentials#cache_password
    redis_info: false
    severity: warning

  # Search tier
  - name: search-primary
    type: elasticsearch
    address: sm://search-config#primary_url
    timeout_ms: 6000
    severity: critical

  - name: search-analytics
    type: elasticsearch
    address: sm://search-config#analytics_url
    path: _cluster/health?level=indices
    severity: warning

  # Message queue
  - name: event-queue
    type: rabbitmq
    address: sm://mq-config#cluster_host
    user: sm://mq-credentials#app_user
    password: sm://mq-credentials#app_password
    vhost: /events
    timeout_ms: 4000
    severity: critical

  # Email service
  - name: transactional-email
    type: smtp
    address: sm://email-config#smtp_host
    user: sm://email-credentials#smtp_user
    password: sm://email-credentials#smtp_password
    smtp_encryption: sm://email-config#smtp_encryption
    timeout_ms: 10000
    severity: warning
```

### Staged Rollout Configuration

```yaml
# Canary deployment health checks
checks:
  # Check both versions during rollout
  - name: app-stable
    type: http
    address: http://app-stable.internal:8080
    path: /health
    severity: critical

  - name: app-canary
    type: http
    address: http://app-canary.internal:8080
    path: /health
    severity: warning  # Lower severity during testing

  # Shared services (full criticality)
  - name: shared-database
    type: mysql
    address: db.internal:3306
    user: health_user
    password: sm://shared-secrets#db_password
    severity: critical
```

---

## Troubleshooting

### Common Issues

**Connection Timeouts**:
```yaml
# Increase timeout for slow services
- name: slow-service
  type: http
  address: http://legacy-system.example.com
  timeout_ms: 15000  # 15 seconds
```

**TLS/SSL Issues**:
```yaml
# Skip certificate validation for internal services
- name: internal-api
  type: mysql
  address: internal-db.example.com:3306
  tls_mode: skip-verify
```

**Protocol Scheme Issues**:
```yaml
# ❌ WRONG: Missing protocol scheme
- name: web-service
  type: http
  address: sm://endpoints#web_host  # Contains: "localhost:8080"

# ✅ CORRECT: Full URL with protocol
- name: web-service
  type: http
  address: sm://endpoints#web_url   # Contains: "http://localhost:8080"
```

**Authentication Failures**:
```yaml
# Ensure secrets are properly formatted
- name: authenticated-service
  type: redis
  address: secure-redis.example.com:6379
  password: sm://redis-secrets#auth_token  # Check secret key name
```

### Debug Mode

Run with debug logging to see detailed execution:
```bash
is-it-up -c config.yaml --log-level debug --log-format console
```

### Configuration Validation

Use dump mode to verify configuration parsing:
```bash
is-it-up -c config.yaml --dump-resolved-config
```

### Secret Resolution Issues

Enable debug logging while keeping resolved secret values redacted:
```bash
is-it-up -c config.yaml --dump-resolved-config --debug
```

---

For more information, see:
- [Architecture Overview](ARCHITECTURE.md)
- [Secrets Configuration](SECRETS.md)
- [Working Examples](../example/aws-secret.example.yaml)
- Main [README](../README.md)
