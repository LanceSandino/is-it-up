# Logging Best Practices Implementation

## Overview

The `is-it-up` service now implements observability best practices for structured logging that aligns with industry standards for platforms like NewRelic, Splunk, DataDog, and other log aggregation systems.

## Enhanced Features

### 1. **Service Metadata**
All logs now include standardized service metadata:
```json
{
  "service.name": "is-it-up",
  "service.version": "v1.2.3",
  "host.name": "server-01.example.com"
}
```

### 2. **Standardized Field Names**
- `timestamp` (ISO8601 with timezone)
- `level` (debug, info, warn, error)
- `message` (structured message types)
- `check.*` (namespaced health check fields)
- `event.*` (event categorization)

### 3. **Health Check Event Structure**
```json
{
  "level": "info",
  "timestamp": "2025-10-13T15:55:40-04:00",
  "message": "health_check",
  "check.name": "database-primary",
  "check.status": "pass",
  "check.message": "connection successful",
  "check.severity": "critical",
  "check.duration_ms": 45,
  "check.timestamp": "2025-10-13T15:55:40-04:00",
  "event.kind": "metric",
  "event.category": "health_check",
  "event.outcome": "success",
  "service.name": "is-it-up",
  "service.version": "v1.2.3",
  "host.name": "server-01.example.com"
}
```

### 4. **Error Event Structure**
```json
{
  "level": "warn",
  "timestamp": "2025-10-13T15:55:40-04:00",
  "message": "health_check",
  "check.name": "external-api",
  "check.status": "fail",
  "check.message": "connection timeout after 30s",
  "check.severity": "critical",
  "check.duration_ms": 30000,
  "event.kind": "metric",
  "event.category": "health_check",
  "event.outcome": "failure",
  "service.name": "is-it-up",
  "service.version": "v1.2.3",
  "host.name": "server-01.example.com"
}
```

## Environment Configuration

### Service Version Resolution Priority
1. `SERVICE_VERSION` environment variable (highest priority)
2. `GIT_SHA` environment variable (uses first 8 chars for short hash)
3. Build-embedded application version (`dev` for an unversioned source build)

This ensures consistency between the `--version` command output and logged service metadata.

### Example Deployment Configuration
```bash
# Explicit version (recommended for releases)
export SERVICE_VERSION="v1.2.3"
./is-it-up -c config.yaml --log-format json

# Git-based version (common in CI/CD pipelines)
export GIT_SHA="abc123def456789"
./is-it-up -c config.yaml --log-format json

# Development (uses application version from flags.go)
./is-it-up -c config.yaml --log-format json  # Uses the embedded build version
```

> **Note**: Environment classification (production/staging/development) is intentionally omitted from logs, as this is typically handled by infrastructure-level tagging in modern deployments (Kubernetes labels, service mesh metadata, etc.).

## Observability Platform Integration

### NewRelic Integration
The structured logs are compatible with NewRelic's log ingestion:
- Standard field names follow NewRelic conventions
- Event categorization enables automatic dashboards
- Service metadata enables service mapping
- Duration metrics support APM correlation

### Splunk Integration
Structured JSON logs are Splunk-ready:
- Automatic field extraction from JSON
- Standard timestamp format for time-based searches
- Service and environment tags for filtering
- Event outcomes for alerting rules

### Search Examples

#### Find all failed health checks:
```
event.outcome:"failure" AND event.category:"health_check"
```

#### Monitor database check performance:
```
check.name:"database*" AND check.duration_ms:>1000
```

#### Service-specific error tracking:
```
service.name:"is-it-up" AND level:"warn" OR level:"error"
```

## Future Enhancements

- OpenTelemetry trace ID integration for distributed tracing
- Correlation ID support for request tracking
- Metrics export to Prometheus/Grafana integration

## Best Practices Compliance

### ✅ **Implemented Standards**
- **Structured Logging**: JSON format with consistent schema
- **Standard Field Names**: Industry-standard field naming conventions
- **Service Metadata**: Automatic service identification
- **Event Categorization**: Standardized event types and outcomes
- **Performance Metrics**: Duration tracking in milliseconds
- **Error Classification**: Proper log levels for different scenarios
- **Timezone Awareness**: ISO8601 timestamps with timezone info
- **Environment Separation**: Environment-based log tagging

### ✅ **Observability Features**
- **Searchable Fields**: All health check attributes indexed
- **Service Mapping**: Host and service identification
- **Performance Monitoring**: Response time tracking
- **Alert-Ready**: Structured outcomes for alerting rules
- **Debug Support**: Optional detailed information in debug mode

### 🔄 **Future Considerations**
- OpenTelemetry integration for distributed tracing
- Metrics export for Prometheus/Grafana
- Custom field extension support
- Log sampling for high-volume deployments
- Async logging for performance optimization

## Usage Examples

### Basic Health Check Monitoring
```bash
# Production deployment with explicit version
SERVICE_VERSION="v1.2.3" \
./is-it-up -c production-config.yaml \
  --log-format json \
  --log-level info
```

### CI/CD Pipeline Integration
```bash
# Deployment with git commit hash
GIT_SHA="$(git rev-parse HEAD)" \
./is-it-up -c staging-config.yaml \
  --log-format json \
  --log-level debug
```

### Console Mode for Development
```bash
# Development with human-readable output (no version required)
./is-it-up -c dev-config.yaml \
  --log-format console \
  --log-level debug
```

## Log Level Recommendations

**Production Environments:**
- Use `--log-level info` for production monitoring
- Info level includes: health check results, errors, secrets resolution, and critical operational events
- Debug messages (config loading, summaries) are filtered out to reduce noise

**Development/Debug:**
- Use `--log-level debug` for troubleshooting and development
- Debug level includes: config file loading, connection attempts, execution summaries, and detailed operational info

**Log Level Usage:**
- `ERROR`: Service failures, check failures with critical severity
- `WARN`: Check failures with warning/info severity, retryable errors
- `INFO`: Health check results, secrets resolution, service lifecycle events
- `DEBUG`: Configuration loading, connection details, execution summaries

This enhanced logging implementation provides comprehensive observability while maintaining backward compatibility and performance.
