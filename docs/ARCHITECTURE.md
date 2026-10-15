# Architecture Overview — is-it-up

This document describes the high-level structure, key design choices, and development patterns for the is-it-up health checking tool.

## Table of Contents

- [System Overview](#system-overview)
- [Core Packages](#core-packages)
- [Architecture Principles](#architecture-principles)
- [Data Flow](#data-flow)
- [Secret Management](#secret-management)
- [Testing Strategy](#testing-strategy)
- [Development Guidelines](#development-guidelines)
- [Extension Points](#extension-points)

---

## System Overview

**is-it-up** is a modular Go CLI tool designed for lightweight infrastructure health checking. The architecture emphasizes:

- **Pure configuration parsing** (no network I/O during unmarshaling)
- **Pluggable secret resolution** via dependency injection
- **Testable orchestration** with mockable components
- **Structured logging** for both automation and human consumption

### Deployment Model

The tool is designed to run as a **short-lived process** from a shell, cron,
CI/CD pipeline, container, or the packaged systemd timer:
- Each execution spawns a new process
- Configuration is loaded fresh each time
- No persistent state between runs
- Optimized for 1-5 minute check intervals

---

## Core Packages

### `cmd/is-it-up/` — CLI Layer

**Purpose**: Professional command-line interface and process orchestration

**Key Components**:
- `main.go` — Entry point with argument parsing and exit code handling
- `flags.go` — Enhanced CLI with comprehensive help, version support, and backward compatibility
- `run.go` — Core execution logic with dependency injection support

**Features**:
- Enhanced logging controls (`--log-level`, `--log-format`)
- Backward compatibility (`--debug`, `--pretty`)
- Configuration validation and secret resolution
- Professional help and version display

### `pkg/checks/` — Check Orchestration Layer

**Purpose**: Central execution logic for all health check operations

**Key Components**:
- `checks.go` — Core types, configuration loading, and RunAll orchestration
- `check_*.go` — Individual check implementations (HTTP, TCP, MySQL, Redis, etc.)
- `validation.go` — Comprehensive configuration validation
- `http_client.go` — Enhanced HTTP client with pooling and timeouts

**Supported Check Types**:
- **HTTP**: GET requests with configurable paths and timeout
- **TCP**: Socket connection testing
- **DNS**: Hostname resolution, shipped as a compile-time plugin in `pkg/checkplugins/dns`
- **MySQL**: Connection + SELECT 1 validation
- **PostgreSQL**: Connection + SELECT 1 validation
- **Redis**: PING with optional INFO parsing
- **RabbitMQ**: AMQP connection and channel testing
- **Elasticsearch**: Cluster health endpoint validation
- **SMTP**: Email server connectivity with authentication and encryption support
- **MongoDB/DocumentDB**: MongoDB-compatible primary ping

**Design Patterns**:
- Dependency injection via package-level function variables
- Context-based cancellation and timeout support
- Non-destructive operations by default
- Detailed result reporting with structured metadata

### `pkg/secrets/` — Secret Management Layer

**Purpose**: Abstracted runtime secret resolution with multiple provider support

**Key Features**:
- Parse-only configuration unmarshaling (no network I/O)
- Pluggable `SecretFetcher` interface with an AWS Secrets Manager implementation
- AWS Secrets Manager implementation with region support
- TTL-based caching with `singleflight` deduplication
- Multiple secret reference formats (URI and mapping)

**Security Model**:
- Secrets redacted by default in configuration dumps
- Resolved values remain redacted even when debug logging is enabled
- No secrets stored in logs or persistent state

### `pkg/logger/` — Logging Layer

**Purpose**: Unified structured and human-readable output

**Features**:
- Thin wrapper over `zap` for structured logging
- JSON mode for automation and parsing
- Console mode for human-readable output
- `Loggable` interface to prevent import cycles
- Context-based logger injection

---

## Architecture Principles

### 1. Pure Configuration Parsing
Configuration unmarshaling performs no network operations. Secret references are parsed into typed structures and resolved at runtime.

### 2. Dependency Injection
All external dependencies (network calls, file I/O, secret fetchers) can be injected for testing.

### 3. Context Propagation
All operations respect `context.Context` for cancellation, timeouts, and request tracing.

### 4. Non-Destructive Operations
Checks perform read-only operations (GET, PING, SELECT 1) and avoid side effects.

### 5. Fail-Fast Validation
Configuration errors are caught early with detailed, actionable error messages.

---

## Data Flow

```
┌─────────────────┐
│   CLI Input     │
│  (flags, args)  │
└─────────┬───────┘
          │
          v
┌─────────────────┐    ┌─────────────────┐
│  Configuration  │───▶│   Validation    │
│    Loading      │    │   (Level 2)     │
└─────────┬───────┘    └─────────────────┘
          │
          v
┌─────────────────┐    ┌─────────────────┐
│ Secret Fetcher  │───▶│ Secret Resolution│
│ Initialization  │    │   (Runtime)     │
└─────────┬───────┘    └─────────────────┘
          │
          v
┌─────────────────┐    ┌─────────────────┐
│   Check         │───▶│    Results      │
│ Orchestration   │    │   Logging       │
└─────────────────┘    └─────────────────┘
```

---

## Secret Management

### Configuration Format Support

**Compact URI Format**:
```yaml
password: sm://secret-name#key?region=us-west-2
```

**Mapping Format**:
```yaml
password:
  secret:
    provider: aws-secrets-manager
    secret_id: secret-name
    key: password
    region: us-west-2
```

### Resolution Process

1. **Parse Phase**: Secret references parsed into `SecretValue` structures
2. **Runtime Phase**: `SecretFetcher` resolves actual values
3. **Caching**: TTL cache (5min) with concurrent fetch deduplication
4. **Security**: Values always redacted in configuration dumps

---

## Testing Strategy

### Unit Testing
- **Coverage Target**: at least 85% aggregate Go statement coverage
- **Mocking**: Package-level function variables for network operations
- **Isolation**: Tests use dependency injection to avoid real network calls
- **Determinism**: Mock SecretFetcher for consistent secret resolution

### Integration Testing
- **Docker Compose**: Local service stack for end-to-end testing
- **Example Configs**: Aligned with compose service hostnames
- **Make Targets**: `make test`, `make validate`, `make static-analysis`,
  `make security-scan`, `make test-install`, and `make test-release`

### Test Organization
```
cmd/is-it-up/
├── main_test.go      # Workflow integration tests
├── flags_test.go     # CLI parsing and validation
└── run_test.go       # Core execution logic

pkg/checks/
├── checks_test.go    # Configuration and orchestration
├── check_*_test.go   # Individual check implementations
└── validation_test.go # Configuration validation

pkg/secrets/
├── secrets_test.go   # Secret parsing and resolution
└── fetcher_*_test.go # Provider implementations

pkg/logger/
└── logger_test.go    # Logging functionality
```

---

## Development Guidelines

### Adding New Check Types

Check types are registered in a registry that both `RunAll` and
`ConfigValidator` read, so adding a type needs no edits to core dispatch or to
the supported-type list.

1. **Create a plugin package** whose runner returns `checks.CheckOutcome`:
   ```go
   // pkg/checkplugins/newtype/newtype.go
   func run(ctx context.Context, cfg checks.CheckConfig) checks.CheckOutcome {
       return checks.CheckOutcome{OK: true, Message: "ok"}
   }

   func init() {
       checks.RegisterCheckType("newtype", run)
   }
   ```

   `RunAll` applies the check deadline to `ctx`, recovers panics, and redacts
   secret-backed values from the message and details, so runners do not need to.
   Registering an empty, nil, or already-registered type panics at startup.

2. **Activate the plugin** in each binary that should offer it, with a blank import:
   ```go
   import _ "github.com/LanceSandino/is-it-up/v2/pkg/checkplugins/newtype"
   ```

   This is compile-time activation. There is no runtime plugin discovery or
   dynamic loading, so release binaries stay static.

3. **Add comprehensive tests**:
   ```go
   func TestCheckNewType_Success(t *testing.T) {
       // Test implementation
   }
   ```

4. **Update documentation**:
   - Add to `docs/CHECKS.md`
   - Include example in `example/config.example.yaml`

### Configuration Extensions

- Add new fields to `CheckConfig` struct
- Implement validation in `validation.go`
- Maintain backward compatibility
- Document changes thoroughly

### Logging Guidelines

- Use structured logging for automation compatibility
- Include relevant context in log entries
- Respect debug/verbose flags for output detail
- Avoid logging sensitive information

---

## Extension Points

### Custom Secret Providers
Implement the `SecretFetcher` interface:
```go
type SecretFetcher interface {
    GetSecret(ctx context.Context, ref *SecretRef) (string, error)
}
```

### Custom Check Types
Register new check types from plugin packages (see "Adding New Check Types")
and follow the established patterns:
- Context-based cancellation
- Dependency injection for testing
- Non-destructive operations
- Structured result reporting

### Enhanced Validation
Extend the `ConfigValidator` for new validation rules:
- Field-level validation
- Cross-check validation
- Custom error messages
- Severity-based reporting

---

This architecture supports the tool's primary use cases while maintaining flexibility for future enhancements and ensuring robust operation in production environments.
