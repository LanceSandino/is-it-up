// This file defines health-check configuration, results, and concurrent orchestration.
// It composes the individual check modules and owns their shared public types.
package checks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/LanceSandino/is-it-up/v2/pkg/secrets"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

type Severity int

const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityCritical
)

type CheckConfig struct {
	Name    string              `json:"name" yaml:"name"`
	Type    string              `json:"type" yaml:"type"`       // http, tcp, redis, elasticsearch
	Address secrets.SecretValue `json:"address" yaml:"address"` // host:port or url
	// New fields to support secret-backed host/port/protocol
	Protocol secrets.SecretValue `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	Host     secrets.SecretValue `json:"host,omitempty" yaml:"host,omitempty"`
	Port     secrets.SecretValue `json:"port,omitempty" yaml:"port,omitempty"`
	Path     secrets.SecretValue `json:"path,omitempty" yaml:"path,omitempty"`         // for http/elasticsearch
	Database secrets.SecretValue `json:"database,omitempty" yaml:"database,omitempty"` // for mysql database selection
	Severity string              `json:"severity,omitempty" yaml:"severity,omitempty"` // info|warn|critical
	User     secrets.SecretValue `json:"user,omitempty" yaml:"user,omitempty"`
	Password secrets.SecretValue `json:"password,omitempty" yaml:"password,omitempty"`
	// Method removed: RabbitMQ no longer requires a method field; AMQP is the default behavior.
	Vhost secrets.SecretValue `json:"vhost,omitempty" yaml:"vhost,omitempty"`
	// Per-check timeout in milliseconds (optional)
	TimeoutMS int `json:"timeout_ms,omitempty" yaml:"timeout_ms,omitempty"`
	// For Redis: opt-in to running INFO parsing (non-destructive)
	RedisInfo bool `json:"redis_info,omitempty" yaml:"redis_info,omitempty"`
	// For SMTP: encryption mode for SMTP connections (tls, ssl, none)
	SMTPEncryption secrets.SecretValue `json:"smtp_encryption,omitempty" yaml:"smtp_encryption,omitempty"`
	// TLSMode is the canonical TLS configuration for checks that support TLS.
	// Allowed values vary by check type (see docs). Example values: "skip-verify", "required", "disable", "preferred", "verify-full".
	TLSMode string `json:"tls_mode,omitempty" yaml:"tls_mode,omitempty"`
}

type Config struct {
	Checks []CheckConfig `json:"checks"`
}

type Result struct {
	Name      string        `json:"name"`
	OK        bool          `json:"ok"`
	Message   string        `json:"message"`
	Severity  string        `json:"severity"`
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration_ms"`
	Details   string        `json:"details,omitempty"`
}

func sanitizeResult(config CheckConfig, result Result) Result {
	values := []secrets.SecretValue{
		config.Address, config.Protocol, config.Host, config.Port, config.Path,
		config.Database, config.User, config.Password, config.Vhost,
		config.SMTPEncryption,
	}
	sensitive := make([]string, 0, len(values))
	for _, value := range values {
		if value.Sensitive && value.String() != "" {
			sensitive = append(sensitive, value.String())
		}
	}
	result.Message = redactText(result.Message, sensitive...)
	result.Details = redactText(result.Details, sensitive...)
	return result
}

func runCheckSafely(checkType, checkName string, check func() (bool, string)) (ok bool, message string) {
	defer func() {
		if recover() != nil {
			ok = false
			message = NewCheckError(checkType, checkName, "panic", errors.New("checker panic recovered")).Error()
		}
	}()
	return check()
}

func credentialTransportAddress(config CheckConfig, secureScheme string) string {
	address := config.Address.String()
	mode := strings.ToLower(strings.TrimSpace(config.TLSMode))
	if mode == "disable" || (mode == "" && !config.Password.HasValue()) {
		return address
	}
	securePrefix := secureScheme + "://"
	insecurePrefix := secureScheme + "+insecure://"
	plainScheme := strings.TrimSuffix(secureScheme, "s") + "://"
	if strings.HasPrefix(address, securePrefix) {
		return address
	}
	if strings.HasPrefix(address, insecurePrefix) {
		address = strings.TrimPrefix(address, insecurePrefix)
		if mode == "skip-verify" {
			return insecurePrefix + address
		}
		return securePrefix + address
	}
	address = strings.TrimPrefix(address, plainScheme)
	if strings.Contains(address, "://") {
		return address
	}
	if mode == "skip-verify" {
		return insecurePrefix + address
	}
	return securePrefix + address
}

func (r Result) String() string {
	status := "OK"
	if !r.OK {
		status = "FAIL"
	}
	return fmt.Sprintf("%s - %s - %s (%s)", r.Name, status, r.Message, r.Severity)
}

// Getter methods to implement logger.Loggable without creating an import cycle.
func (r Result) GetName() string            { return r.Name }
func (r Result) IsOK() bool                 { return r.OK }
func (r Result) GetMessage() string         { return r.Message }
func (r Result) GetSeverity() string        { return r.Severity }
func (r Result) GetTimestamp() time.Time    { return r.Timestamp }
func (r Result) GetDuration() time.Duration { return r.Duration }
func (r Result) GetDetails() string         { return r.Details }

// DefaultConfig removed: this application requires an explicit config file
// and will not supply defaults at runtime.

// package-level function variables so tests can stub network behavior.
var (
	checkTCPFn           = checkTCP
	checkMySQLFn         = checkMySQL
	checkPostgresFn      = checkPostgres
	checkRedisFn         = checkRedis
	checkRedisInfoFn     = checkRedisInfo
	checkElasticsearchFn = checkElasticsearch
	checkSMTPFn          = checkSMTP
	checkDocumentDBFn    = checkDocumentDB
)

// LoadConfig loads configuration from path. Returns the loaded config and format type.
// Currently only supports YAML - JSON files are explicitly rejected.
func LoadConfig(path string) (*Config, string, error) {
	// The configuration file path is supplied by the user and intentionally
	// opened as-is. This is expected behavior for a CLI that accepts a
	// config path; callers are responsible for passing a safe path. Suppress
	// gosec warnings about potential path traversal here because the intent
	// is to read a user-provided config file. See documentation for secure
	// deployment practices.
	f, err := os.Open(path) // #nosec G304
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, "", err
	}
	cfg := &Config{}
	lower := strings.ToLower(path)

	// Explicitly reject JSON files
	if strings.HasSuffix(lower, ".json") {
		return nil, "", errors.New("JSON configuration files are not supported; please use YAML format (.yml or .yaml)")
	}

	// Only YAML is supported. Accept .yml or .yaml extensions or attempt YAML
	// parsing for unknown extensions. This keeps configuration focused and
	// unambiguous.
	if strings.HasSuffix(lower, ".yml") || strings.HasSuffix(lower, ".yaml") {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, "yaml", err
		}
		if err := ValidateConfig(cfg); err != nil {
			return nil, "yaml", err
		}
		return cfg, "yaml", nil
	}
	// Try YAML parse as a last resort for files without explicit extension.
	if err := yaml.Unmarshal(data, cfg); err == nil {
		if err := ValidateConfig(cfg); err != nil {
			return nil, "yaml", err
		}
		return cfg, "yaml", nil
	}
	return nil, "", errors.New("failed to parse config as YAML; only YAML format is supported (.yml or .yaml extensions)")
}

// LoadConfigAutoNoValidation loads config without validation (for use when secrets need to be resolved first).
func LoadConfigAutoNoValidation(path string) (*Config, string, error) {
	// The configuration file path is supplied by the user and intentionally
	// opened as-is. This is expected behavior for a CLI that accepts a
	// config path; callers are responsible for passing a safe path. Suppress
	// gosec warnings about potential path traversal here because the intent
	// is to read a user-provided config file. See documentation for secure
	// deployment practices.
	f, err := os.Open(path) // #nosec G304
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, "", err
	}
	cfg := &Config{}
	lower := strings.ToLower(path)

	// Explicitly reject JSON files
	if strings.HasSuffix(lower, ".json") {
		return nil, "json", fmt.Errorf("JSON configuration format not supported; use YAML")
	}

	// For YAML files, parse directly without validation
	if strings.HasSuffix(lower, ".yml") || strings.HasSuffix(lower, ".yaml") {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, "yaml", err
		}
		return cfg, "yaml", nil
	}
	// Try YAML parse as a last resort for files without explicit extension.
	if err := yaml.Unmarshal(data, cfg); err == nil {
		return cfg, "yaml", nil
	} else {
		return nil, "yaml", err
	}
}

func RunAll(ctx context.Context, cfg *Config) []Result {
	results := make([]Result, 0, len(cfg.Checks))
	for _, c := range cfg.Checks {
		start := time.Now()
		checkCtx, cancel := contextWithCheckTimeout(ctx, c)
		var r Result
		switch strings.ToLower(c.Type) {
		case "http":
			ok, msg := checkHTTP(checkCtx, c)
			detail := fmt.Sprintf("http GET address=%s path=%s", redactAddress(c.Address.String()), c.Path.String())
			// Only show TLS info for HTTPS URLs or when explicitly configured
			if strings.HasPrefix(c.Address.String(), "https://") || c.TLSMode != "" {
				tlsInfo := c.TLSMode
				if tlsInfo == "" && strings.HasPrefix(c.Address.String(), "https://") {
					tlsInfo = "required"
				}
				if tlsInfo != "" {
					detail += fmt.Sprintf(" tls=%s", tlsInfo)
				}
			}
			r = Result{Name: c.Name, OK: ok, Message: msg, Severity: c.Severity, Timestamp: time.Now(), Duration: time.Since(start), Details: detail}
		case "tcp":
			ok, msg := checkTCPFn(checkCtx, c.Address.String())
			detail := fmt.Sprintf("tcp connect to %s", redactAddress(c.Address.String()))
			if !ok {
				detail = detail + "; error: " + msg
			}
			r = Result{Name: c.Name, OK: ok, Message: msg, Severity: c.Severity, Timestamp: time.Now(), Duration: time.Since(start), Details: detail}
		case "mysql":
			// Direct database connection and SELECT 1 query (includes connectivity test)
			ok, msg := runCheckSafely("mysql", c.Name, func() (bool, string) { return checkMySQLFn(checkCtx, c) })
			detail := func() string {
				user := c.User.String()
				if user == "" {
					user = "(not-set)"
				}
				pwd := "(none)"
				if c.Password.HasValue() {
					pwd = "REDACTED"
				}
				action := fmt.Sprintf("mysql SELECT 1 on %s as user=%s password=%s", redactAddress(c.Address.String()), user, pwd)
				if ok {
					return action
				}
				return action + "; error: " + msg
			}()
			r = Result{Name: c.Name, OK: ok, Message: msg, Severity: c.Severity, Timestamp: time.Now(), Duration: time.Since(start), Details: detail}
		case "postgres":
			// Direct database connection and SELECT 1 query (includes connectivity test)
			ok, msg := runCheckSafely("postgres", c.Name, func() (bool, string) { return checkPostgresFn(checkCtx, c) })
			detail := func() string {
				user := c.User.String()
				if user == "" {
					user = "(not-set)"
				}
				pwd := "(none)"
				if c.Password.HasValue() {
					pwd = "REDACTED"
				}
				sslMode := c.TLSMode
				if sslMode == "" {
					sslMode = "verify-full"
				}
				action := fmt.Sprintf("postgres SELECT 1 on %s as user=%s password=%s sslmode=%s", redactAddress(c.Address.String()), user, pwd, sslMode)
				if ok {
					return action
				}
				return action + "; error: " + msg
			}()
			r = Result{Name: c.Name, OK: ok, Message: msg, Severity: c.Severity, Timestamp: time.Now(), Duration: time.Since(start), Details: detail}
		case "redis":
			redisAddress := credentialTransportAddress(c, "rediss")
			ok, msg := checkRedisFn(checkCtx, redisAddress, c.Password.String())
			// build detail including masked password
			pwd := "(none)"
			if c.Password.HasValue() {
				pwd = "REDACTED"
			}
			detail := fmt.Sprintf("redis PING to %s auth=%s", redactAddress(c.Address.String()), pwd)
			// optionally run INFO parsing if requested
			if ok && c.RedisInfo {
				if iok, imsg := checkRedisInfoFn(checkCtx, redisAddress, c.Password.String()); !iok {
					// prefer to mark as warning but keep original ok=false only if INFO fails critically
					r = sanitizeResult(c, Result{Name: c.Name, OK: false, Message: "info: " + imsg, Severity: "warning", Timestamp: time.Now(), Duration: time.Since(start), Details: detail + "; info error: " + imsg})
					results = append(results, r)
					cancel()
					continue
				} else {
					msg = msg + "; info: " + imsg
					detail = detail + "; redis_info=true; info: " + imsg
				}
			}
			// include whether redis_info was enabled in details
			if !c.RedisInfo {
				detail = detail + "; redis_info=false"
			}
			r = Result{Name: c.Name, OK: ok, Message: msg, Severity: c.Severity, Timestamp: time.Now(), Duration: time.Since(start), Details: detail}
		case "elasticsearch", "opensearch":
			ok, msg := checkElasticsearchFn(checkCtx, c.Address.String(), c.Path.String())
			// detail with URL and parsed status
			rawPath := c.Path.String()
			if rawPath == "" {
				rawPath = "_cluster/health"
			}
			esURL := strings.TrimRight(redactAddress(c.Address.String()), "/") + "/" + rawPath
			detail := fmt.Sprintf("elasticsearch/opensearch GET url=%s result=%s", esURL, msg)
			r = Result{Name: c.Name, OK: ok, Message: msg, Severity: c.Severity, Timestamp: time.Now(), Duration: time.Since(start), Details: detail}
		case "rabbitmq":
			// Default behavior: attempt AMQP connect+open-channel. If that fails,
			// fall back to a basic TCP connect to ensure we can still report reachability.
			rabbitAddress := credentialTransportAddress(c, "amqps")
			ok, msg := checkRabbitAMQPFn(checkCtx, rabbitAddress, c.User.String(), c.Password.String(), c.Vhost.String())
			detail := fmt.Sprintf("amqp connect to %s vhost=%s", redactAddress(c.Address.String()), c.Vhost.String())
			if ok {
				r = Result{Name: c.Name, OK: ok, Message: msg, Severity: c.Severity, Timestamp: time.Now(), Duration: time.Since(start), Details: detail}
				break
			}
			// Preserve legacy reachability fallback only for anonymous checks.
			if !c.Password.HasValue() {
				if tock, tmsg := checkTCPFn(checkCtx, c.Address.String()); tock {
					tdetail := fmt.Sprintf("tcp connect to %s", redactAddress(c.Address.String()))
					r = Result{Name: c.Name, OK: true, Message: tmsg, Severity: c.Severity, Timestamp: time.Now(), Duration: time.Since(start), Details: tdetail}
					break
				}
			}
			// Both AMQP and TCP failed — report AMQP failure details (include TCP msg)
			detail = detail + "; amqp_error: " + msg
			r = Result{Name: c.Name, OK: false, Message: msg, Severity: c.Severity, Timestamp: time.Now(), Duration: time.Since(start), Details: detail}

		case "smtp":
			ok, msg := checkSMTPFn(checkCtx, c)
			// build detail including masked credentials and encryption info
			user := c.User.String()
			if user == "" {
				user = "(none)"
			}
			pwd := "(none)"
			if c.Password.HasValue() {
				pwd = "REDACTED"
			}
			enc := c.SMTPEncryption.String()
			if enc == "" {
				enc = "starttls"
			}
			detail := fmt.Sprintf("smtp connect to %s user=%s password=%s encryption=%s", redactAddress(c.Address.String()), user, pwd, enc)
			if !ok {
				detail = detail + "; error: " + msg
			}
			r = Result{Name: c.Name, OK: ok, Message: msg, Severity: c.Severity, Timestamp: time.Now(), Duration: time.Since(start), Details: detail}
		case "mongo", "documentdb":
			ok, msg := checkDocumentDBFn(checkCtx, c)
			detail := fmt.Sprintf("documentdb ping %s", redactAddress(c.Address.String()))
			if !ok {
				detail = detail + "; error: " + msg
			}
			r = Result{Name: c.Name, OK: ok, Message: msg, Severity: c.Severity, Timestamp: time.Now(), Duration: time.Since(start), Details: detail}
		default:
			r = Result{Name: c.Name, OK: false, Message: "unknown check type", Severity: c.Severity, Timestamp: time.Now(), Duration: time.Since(start)}
		}
		results = append(results, sanitizeResult(c, r))
		cancel()
	}
	return results
}

// Redis and INFO checks are implemented in check_redis.go

// RabbitMQ management references removed.

// Elasticsearch check implementation moved to check_elasticsearch.go

// RedactedConfig returns a copy of cfg with sensitive fields redacted.
// The legacy reveal argument is ignored so debug logging cannot disclose secrets.
func RedactedConfig(cfg *Config, _ bool) *Config {
	out := &Config{Checks: make([]CheckConfig, 0, len(cfg.Checks))}
	for _, c := range cfg.Checks {
		nc := c
		redactSecretValue := func(value *secrets.SecretValue, always bool) {
			if value.HasValue() && (always || value.Sensitive) {
				*value = secrets.SecretValue{Raw: "REDACTED", Sensitive: true}
			}
		}
		redactSecretValue(&nc.Address, false)
		redactSecretValue(&nc.Protocol, false)
		redactSecretValue(&nc.Host, false)
		redactSecretValue(&nc.Port, false)
		redactSecretValue(&nc.Path, false)
		redactSecretValue(&nc.Database, false)
		redactSecretValue(&nc.User, false)
		redactSecretValue(&nc.Password, true)
		redactSecretValue(&nc.Vhost, false)
		redactSecretValue(&nc.SMTPEncryption, false)
		out.Checks = append(out.Checks, nc)
	}
	return out
}

// ValidateConfig performs comprehensive validation on loaded config entries. It ensures
// there are no conflicting address specifications, validates check types, timeouts,
// and required fields for each check type using the comprehensive validator.
func ValidateConfig(cfg *Config) error {
	// Use the comprehensive validator
	validator := NewConfigValidator()
	return validator.ValidateConfig(cfg)
}
