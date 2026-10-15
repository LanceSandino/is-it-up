# Secrets Configuration Reference

This document provides comprehensive documentation for all supported secret configuration formats in the is-it-up tool.

## Table of Contents

- [Overview](#overview)
- [Configuration Formats](#configuration-formats)
  - [Compact URI Format](#compact-uri-format)
  - [Mapping Format](#mapping-format)
- [Secret Types and Examples](#secret-types-and-examples)
- [AWS Secrets Manager Integration](#aws-secrets-manager-integration)
- [Advanced Configuration](#advanced-configuration)
- [Security and Best Practices](#security-and-best-practices)
- [Troubleshooting](#troubleshooting)

---

## Overview

The is-it-up tool supports dynamic secret resolution from AWS Secrets Manager for any string configuration field. Secret references are parsed during configuration loading but **resolved at runtime** to ensure fresh credential access and security.

### Key Features
- **Parse-time validation**: Secret references validated during config load
- **Runtime resolution**: Actual values fetched when needed
- **Caching**: 5-minute TTL cache with concurrent fetch deduplication
- **Region support**: Automatic region detection from ARNs or explicit specification
- **Flexible formats**: Both compact URI and structured mapping formats
- **Security**: Resolved values remain redacted in all output, including debug mode

### Supported Fields
Any string field in check configurations supports secret references:
- Database credentials (`user`, `password`)
- Service endpoints (`address`)
- Authentication tokens (`password` for Redis)
- Protocol, host, port, path, database, virtual host, and SMTP encryption fields

---

## Configuration Formats

### Compact URI Format

**Syntax**: `sm://secret-id[#key][?query-params]`

**Components**:
- `sm://` — Required URI scheme prefix
- `secret-id` — AWS Secrets Manager secret name or ARN
- `#key` — Optional fragment for JSON key extraction
- `?query-params` — Optional parameters (region, stage)

**Basic Examples**:

```yaml
# Simple secret (entire payload as value)
address: sm://is-it-up/web-app-address

# Extract specific key from JSON payload
password: sm://is-it-up/config#mysql.password

# Specify region explicitly
password: sm://is-it-up/config#mysql.password?region=us-east-2

# Use full ARN (region auto-detected)
password: sm://arn:aws:secretsmanager:us-east-2:123456789012:secret:is-it-up/config-example#mysql.password

# Complex key extraction (dotted notation)
password: sm://is-it-up/config#database.credentials.password
```

### Mapping Format

**Structure**: YAML object with secret metadata

**Two equivalent forms supported**:

**Form 1** (nested under `secret` key):
```yaml
password:
  secret:
    secret_id: is-it-up/config
    key: mysql.password
    region: us-east-2
    provider: aws-secrets-manager  # optional, defaults to aws-secrets-manager
```

**Form 2** (direct mapping):
```yaml
password:
  secret_id: is-it-up/config
  key: mysql.password
  region: us-east-2
  provider: aws-secrets-manager  # optional, defaults to aws-secrets-manager
```

**Field Descriptions**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `secret_id` | string | ✓ | Secret name or ARN |
| `key` | string | | JSON key to extract (omit for entire payload) |
| `region` | string | | AWS region (auto-detected from ARN if omitted) |
| `provider` | string | | Secret provider (defaults to "aws-secrets-manager") |
| `stage` | string | | Secret version stage (defaults to AWSCURRENT) |

---

## Secret Types and Examples

### 1. Simple String Secrets

**Use Case**: Entire secret payload is the credential value

**AWS Secrets Manager Payload**:
```
MySecretPassword123
```

**Configuration Examples**:
```yaml
# Compact URI (full URL in secret)
address: sm://is-it-up/web-app-address  # Contains: "http://localhost:8080"

# Mapping format
address:
  secret_id: is-it-up/web-app-address
  region: us-east-2
```

### 2. JSON Object Secrets with Key Extraction

**Use Case**: Structured secrets with multiple credentials

**AWS Secrets Manager Payload**:
```json
{
  "username": "app_user",
  "password": "<database-value>",
  "host": "db.example.com",
  "port": 5432
}
```

**Configuration Examples**:
```yaml
# Extract individual keys
user: sm://database-secrets#username
password: sm://database-secrets#password
host: sm://database-secrets#host

# Mapping format equivalent
user:
  secret_id: database-secrets
  key: username
password:
  secret_id: database-secrets
  key: password
```

### 3. Nested JSON Secrets with Dotted Keys

**Use Case**: Hierarchical secret organization

**AWS Secrets Manager Payload**:
```json
{
  "database": {
    "primary": {
      "username": "primary_user",
      "password": "<primary-value>"
    },
    "replica": {
      "username": "replica_user",
      "password": "<replica-value>"
    }
  },
  "cache": {
    "redis": {
      "password": "<redis-value>"
    }
  }
}
```

**Configuration Examples**:
```yaml
# Dotted key extraction
primary_db_user: sm://is-it-up/config#database.primary.username
primary_db_password: sm://is-it-up/config#database.primary.password
replica_db_user: sm://is-it-up/config#database.replica.username
replica_db_password: sm://is-it-up/config#database.replica.password
redis_password: sm://is-it-up/config#cache.redis.password
```

### 4. Literal Keys with Dots

**Use Case**: Top-level keys containing dot characters

**AWS Secrets Manager Payload**:
```json
{
  "mysql.user": "db_user",
  "mysql.password": "<mysql-value>",
  "rabbitmq.user": "queue_user",
  "rabbitmq.pass": "<queue-value>"
}
```

**Configuration Examples**:
```yaml
# Literal key lookup (tries dotted first, then literal)
mysql_user: sm://service-secrets#mysql.user
mysql_password: sm://service-secrets#mysql.password
rabbitmq_user: sm://service-secrets#rabbitmq.user
rabbitmq_password: sm://service-secrets#rabbitmq.pass
```

**Important**: The resolver attempts dotted traversal first, then falls back to literal key lookup for compatibility.

### 5. Regional Secrets

**Use Case**: Region-specific credentials or endpoints

**Configuration Examples**:
```yaml
# Query parameter approach
us_east_db: sm://regional-secrets#us_east_endpoint?region=us-east-1
eu_west_db: sm://regional-secrets#eu_west_endpoint?region=eu-west-1

# ARN approach (region auto-detected)
us_secret: sm://arn:aws:secretsmanager:us-east-2:123456789012:secret:is-it-up/config-example#mysql.password
eu_secret: sm://arn:aws:secretsmanager:eu-west-1:123456789012:secret:is-it-up/config-example#mysql.password

# Mapping approach
api_key:
  secret_id: api-credentials
  key: production_key
  region: us-west-2
```

### 6. Versioned Secrets

**Use Case**: Specific secret versions or staging

**Configuration Examples**:
```yaml
# Current version (default)
current_password: sm://is-it-up/config#mysql.password

# Specific stage
staging_password: sm://is-it-up/config#mysql.password?stage=AWSPENDING

# Mapping format with stage
production_key:
  secret_id: api-secrets
  key: production_key
  stage: AWSCURRENT
```

### 7. Binary Secrets

**Use Case**: Base64-encoded or binary credential data

**AWS Secrets Manager**: Store as binary data

**Configuration Examples**:
```yaml
# Binary secret (converted to string)
tls_cert: sm://certificates#server_cert

# Base64-encoded token
auth_token: sm://tokens#base64_token
```

---

## AWS Secrets Manager Integration

### Authentication Methods

The tool uses AWS SDK v2 default credential chain:

1. **Environment Variables**:
   ```bash
   export AWS_ACCESS_KEY_ID="your-access-key"
   export AWS_SECRET_ACCESS_KEY="your-secret-key"
   export AWS_REGION="us-east-1"
   ```

2. **AWS Profile**:
   ```bash
   export AWS_PROFILE="your-profile"
   ```

3. **IAM Role** (EC2/ECS/Lambda):
   - Automatic credential detection

4. **AWS SSO**:
   ```bash
   aws sso login --profile your-profile
   export AWS_PROFILE="your-profile"
   ```

### Required IAM Permissions

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "secretsmanager:GetSecretValue"
            ],
            "Resource": [
                "arn:aws:secretsmanager:*:*:secret:your-secret-prefix/*"
            ]
        }
    ]
}
```

### Region Handling

**Automatic Detection**:
- ARN format: `arn:aws:secretsmanager:REGION:ACCOUNT:secret:NAME`
- Region extracted from ARN automatically

**Explicit Specification**:
- Query parameter: `?region=us-west-2`
- Mapping field: `region: us-west-2`
- Environment: `AWS_REGION=us-west-2`

**Resolution Priority**:
1. ARN region (if ARN format used)
2. Explicit region parameter
3. AWS SDK default region

### Performance Optimizations

**Caching Strategy**:
- 5-minute TTL per secret, region, and requested version stage
- In-memory cache with automatic expiration
- Concurrent fetch deduplication using the same stage-aware identity

**Regional Client Pooling**:
- Separate AWS clients per region
- Automatic client creation and reuse
- Reduces cross-region API calls

---

## Advanced Configuration

### Complete Multi-Service Example

```yaml
# Production service configuration with secrets
checks:
  # Web application health check
  - name: api-service
    type: http
    address: sm://infrastructure-config#api_base_url
    path: /health
    timeout_ms: 5000
    severity: critical

  # Primary database with regional failover
  - name: primary-database
    type: mysql
    address: sm://db-config#primary_host?region=us-east-1
    user: sm://db-credentials#app_username
    password: sm://db-credentials#app_password
    tls_mode: required
    timeout_ms: 10000
    severity: critical

  # Read replica in different region
  - name: read-replica
    type: mysql
    address: sm://db-config#replica_host?region=us-west-2
    user: sm://db-credentials#readonly_username
    password: sm://db-credentials#readonly_password
    tls_mode: required
    severity: warning

  # Redis cluster with authentication
  - name: redis-sessions
    type: redis
    address: sm://cache-config#sessions_endpoint
    password: sm://cache-credentials#sessions_auth_token
    redis_info: true
    timeout_ms: 3000
    severity: critical

  # ElastiSearch with nested secret structure
  - name: search-cluster
    type: elasticsearch
    address: sm://search-config#cluster.primary.endpoint
    timeout_ms: 6000
    severity: critical

  # RabbitMQ with multi-tenant setup
  - name: tenant-queue
    type: rabbitmq
    address: sm://messaging-config#cluster_endpoint
    user: sm://messaging-credentials#tenant.app.username
    password: sm://messaging-credentials#tenant.app.password
    vhost: sm://messaging-config#tenant.app.vhost
    timeout_ms: 4000
    severity: critical
```

### Mixed Configuration (Secrets + Literals)

```yaml
checks:
  # Public endpoints (no secrets needed)
  - name: public-api
    type: http
    address: https://api.example.com
    path: /status
    severity: warning

  # Internal services with secrets
  - name: internal-db
    type: mysql
    address: internal-db.local:3306  # literal hostname
    user: sm://internal-secrets#db_user  # secret username
    password: sm://internal-secrets#db_password  # secret password
    severity: critical
```

### Environment-Specific Configurations

```yaml
# Development environment
dev_checks: &dev_checks
  - name: dev-database
    type: mysql
    address: localhost:3306
    user: sm://dev-secrets#mysql_user
    password: sm://dev-secrets#mysql_password
    severity: warning

# Staging environment
staging_checks: &staging_checks
  - name: staging-database
    type: mysql
    address: sm://staging-config#db_endpoint
    user: sm://staging-secrets#mysql_user
    password: sm://staging-secrets#mysql_password
    severity: critical

# Production environment
production_checks: &production_checks
  - name: production-database
    type: mysql
    address: sm://prod-config#primary_db_endpoint?region=us-east-1
    user: sm://prod-secrets#mysql_user
    password: sm://prod-secrets#mysql_password
    tls_mode: required
    severity: critical

# Select environment (uncomment one)
checks: *production_checks
```

---

## Security and Best Practices

### Configuration Security

**Never commit secrets to version control**:
```yaml
# ❌ BAD: Hard-coded credentials
password: <literal-development-placeholder>

# ✅ GOOD: Secret reference
password: sm://is-it-up/config#mysql.password
```

**Use specific IAM permissions**:
```json
{
  "Effect": "Allow",
  "Action": "secretsmanager:GetSecretValue",
  "Resource": "arn:aws:secretsmanager:us-east-2:123456789012:secret:is-it-up/*"
}
```

### Secret Organization

**Group related credentials**:
```json
// Single secret: is-it-up/config
{
  "database": {
    "primary": {"user": "...", "password": "..."},
    "replica": {"user": "...", "password": "..."}
  },
  "cache": {
    "redis": {"password": "..."}
  },
  "messaging": {
    "rabbitmq": {"user": "...", "password": "..."}
  }
}
```

**Environment separation**:
- `dev-secrets`, `staging-secrets`, `prod-secrets`
- Different AWS accounts per environment
- Separate IAM roles with environment-scoped access

### Debugging and Monitoring

**Safe configuration validation**:
```bash
# Validate without exposing secrets
is-it-up -c config.yaml --dump-resolved-config

# Debug logging; resolved values remain redacted
is-it-up -c config.yaml --dump-resolved-config --debug
```

**Log monitoring**:
- Monitor for "MissingKeyError" in logs
- Set up alerts for secret fetch failures
- Track secret rotation compatibility

### Rotation and Lifecycle

**Secret rotation compatibility**:
```yaml
# Use AWSCURRENT stage (default)
password: sm://is-it-up/config#mysql.password

# Pin to specific version only when necessary
password: sm://is-it-up/config#mysql.password?stage=AWSCURRENT
```

**Testing secret changes**:
```bash
# Test configuration after secret updates
is-it-up -c config.yaml --dump-resolved-config --debug
```

---

## Troubleshooting

### Common Issues and Solutions

**1. Secret Not Found Error**

Error:
```
SecretFetchError: secret not found: my-secret
```

Solutions:
- Verify secret name spelling
- Check AWS region configuration
- Confirm IAM permissions
- Validate AWS credentials

```bash
# Test AWS access
aws secretsmanager get-secret-value --secret-id my-secret --region us-east-1
```

**2. Key Missing Error**

Error:
```
MissingKeyError: key 'password' not found in secret
```

Solutions:
- Check key name spelling in secret payload
- Verify JSON structure in AWS console
- Try literal key lookup for dotted keys
- Validate secret content format

```yaml
# Debug the secret structure
password: sm://my-secret  # Get entire payload first
```

**3. Region Mismatch**

Error:
```
InvalidRegionError: secret exists in different region
```

Solutions:
- Add region query parameter
- Use full ARN format
- Set AWS_REGION environment variable

```yaml
# Specify region explicitly
password: sm://my-secret#key?region=us-west-2

# Or use ARN format
password: sm://arn:aws:secretsmanager:us-west-2:123:secret:my-secret-AbCdEf#key
```

**4. Permission Denied**

Error:
```
AccessDeniedError: insufficient permissions
```

Solutions:
- Add `secretsmanager:GetSecretValue` permission
- Verify resource ARN in IAM policy
- Check AWS credential configuration

**5. Timeout Issues**

Error:
```
TimeoutError: secret fetch timeout
```

Solutions:
- Check network connectivity to AWS
- Verify security group rules
- Consider caching for high-frequency access

### Debugging Commands

**Validate configuration structure**:
```bash
is-it-up -c config.yaml --dump-resolved-config
```

**Debug secret resolution** (secure environments only):
```bash
is-it-up -c config.yaml --dump-resolved-config --debug
```

**Test AWS connectivity**:
```bash
aws sts get-caller-identity
aws secretsmanager list-secrets --region us-east-1
```

**Verbose logging**:
```bash
is-it-up -c config.yaml --log-level debug --log-format console
```

### Performance Monitoring

**Cache hit/miss tracking**:
- Monitor secret fetch frequency
- Optimize cache TTL if needed
- Consider pre-warming for critical secrets

**Network latency**:
- Use regional secrets for regional deployments
- Monitor cross-region fetch patterns
- Consider VPC endpoints for AWS services

---

For more information, see:
- [Health Check Reference](CHECKS.md)
- [Architecture Overview](ARCHITECTURE.md)
- Main [README](../README.md)
