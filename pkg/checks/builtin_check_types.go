// This file registers the built-in check types as registry runners.
// Each adapter formats redacted details for one protocol module.
package checks

import (
	"context"
	"fmt"
	"strings"
)

func init() {
	RegisterCheckType("http", runHTTPCheck)
	RegisterCheckType("tcp", runTCPCheck)
	RegisterCheckType("mysql", runMySQLCheck)
	RegisterCheckType("postgres", runPostgresCheck)
	RegisterCheckType("redis", runRedisCheck)
	RegisterCheckType("elasticsearch", runElasticsearchCheck)
	RegisterCheckType("opensearch", runElasticsearchCheck)
	RegisterCheckType("rabbitmq", runRabbitMQCheck)
	RegisterCheckType("smtp", runSMTPCheck)
	RegisterCheckType("mongo", runDocumentDBCheck)
	RegisterCheckType("documentdb", runDocumentDBCheck)
}

func passwordLabel(c CheckConfig) string {
	if c.Password.HasValue() {
		return "REDACTED"
	}
	return "(none)"
}

func withError(detail string, ok bool, msg string) string {
	if ok {
		return detail
	}
	return detail + "; error: " + msg
}

func runHTTPCheck(ctx context.Context, c CheckConfig) CheckOutcome {
	ok, msg := checkHTTP(ctx, c)
	detail := fmt.Sprintf("http GET address=%s path=%s", redactAddress(c.Address.String()), c.Path.String())
	// Only show TLS info for HTTPS URLs or when explicitly configured
	if strings.HasPrefix(c.Address.String(), "https://") || c.TLSMode != "" {
		tlsInfo := c.TLSMode
		if tlsInfo == "" {
			tlsInfo = "required"
		}
		detail += fmt.Sprintf(" tls=%s", tlsInfo)
	}
	return CheckOutcome{OK: ok, Message: msg, Details: detail}
}

func runTCPCheck(ctx context.Context, c CheckConfig) CheckOutcome {
	ok, msg := checkTCPFn(ctx, c.Address.String())
	detail := fmt.Sprintf("tcp connect to %s", redactAddress(c.Address.String()))
	return CheckOutcome{OK: ok, Message: msg, Details: withError(detail, ok, msg)}
}

func runMySQLCheck(ctx context.Context, c CheckConfig) CheckOutcome {
	// Direct database connection and SELECT 1 query (includes connectivity test)
	ok, msg := checkMySQLFn(ctx, c)
	user := c.User.String()
	if user == "" {
		user = "(not-set)"
	}
	detail := fmt.Sprintf("mysql SELECT 1 on %s as user=%s password=%s", redactAddress(c.Address.String()), user, passwordLabel(c))
	return CheckOutcome{OK: ok, Message: msg, Details: withError(detail, ok, msg)}
}

func runPostgresCheck(ctx context.Context, c CheckConfig) CheckOutcome {
	// Direct database connection and SELECT 1 query (includes connectivity test)
	ok, msg := checkPostgresFn(ctx, c)
	user := c.User.String()
	if user == "" {
		user = "(not-set)"
	}
	sslMode := c.TLSMode
	if sslMode == "" {
		sslMode = "verify-full"
	}
	detail := fmt.Sprintf("postgres SELECT 1 on %s as user=%s password=%s sslmode=%s", redactAddress(c.Address.String()), user, passwordLabel(c), sslMode)
	return CheckOutcome{OK: ok, Message: msg, Details: withError(detail, ok, msg)}
}

func runRedisCheck(ctx context.Context, c CheckConfig) CheckOutcome {
	redisAddress := credentialTransportAddress(c, "rediss")
	ok, msg := checkRedisFn(ctx, redisAddress, c.Password.String())
	detail := fmt.Sprintf("redis PING to %s auth=%s", redactAddress(c.Address.String()), passwordLabel(c))
	if ok && c.RedisInfo {
		iok, imsg := checkRedisInfoFn(ctx, redisAddress, c.Password.String())
		if !iok {
			// An INFO failure is reported as a warning rather than a critical outage.
			return CheckOutcome{OK: false, Message: "info: " + imsg, Details: detail + "; info error: " + imsg, Severity: "warning"}
		}
		msg += "; info: " + imsg
		detail += "; redis_info=true; info: " + imsg
	}
	if !c.RedisInfo {
		detail += "; redis_info=false"
	}
	return CheckOutcome{OK: ok, Message: msg, Details: detail}
}

func runElasticsearchCheck(ctx context.Context, c CheckConfig) CheckOutcome {
	ok, msg := checkElasticsearchFn(ctx, c.Address.String(), c.Path.String())
	rawPath := c.Path.String()
	if rawPath == "" {
		rawPath = "_cluster/health"
	}
	esURL := strings.TrimRight(redactAddress(c.Address.String()), "/") + "/" + rawPath
	return CheckOutcome{OK: ok, Message: msg, Details: fmt.Sprintf("elasticsearch/opensearch GET url=%s result=%s", esURL, msg)}
}

func runRabbitMQCheck(ctx context.Context, c CheckConfig) CheckOutcome {
	rabbitAddress := credentialTransportAddress(c, "amqps")
	ok, msg := checkRabbitAMQPFn(ctx, rabbitAddress, c.User.String(), c.Password.String(), c.Vhost.String())
	detail := fmt.Sprintf("amqp connect to %s vhost=%s", redactAddress(c.Address.String()), c.Vhost.String())
	if ok {
		return CheckOutcome{OK: true, Message: msg, Details: detail}
	}
	// Preserve legacy reachability fallback only for anonymous checks.
	if !c.Password.HasValue() {
		if tok, tmsg := checkTCPFn(ctx, c.Address.String()); tok {
			return CheckOutcome{OK: true, Message: tmsg, Details: fmt.Sprintf("tcp connect to %s", redactAddress(c.Address.String()))}
		}
	}
	return CheckOutcome{OK: false, Message: msg, Details: detail + "; amqp_error: " + msg}
}

func runSMTPCheck(ctx context.Context, c CheckConfig) CheckOutcome {
	ok, msg := checkSMTPFn(ctx, c)
	user := c.User.String()
	if user == "" {
		user = "(none)"
	}
	enc := c.SMTPEncryption.String()
	if enc == "" {
		enc = "starttls"
	}
	detail := fmt.Sprintf("smtp connect to %s user=%s password=%s encryption=%s", redactAddress(c.Address.String()), user, passwordLabel(c), enc)
	return CheckOutcome{OK: ok, Message: msg, Details: withError(detail, ok, msg)}
}

func runDocumentDBCheck(ctx context.Context, c CheckConfig) CheckOutcome {
	ok, msg := checkDocumentDBFn(ctx, c)
	detail := fmt.Sprintf("documentdb ping %s", redactAddress(c.Address.String()))
	return CheckOutcome{OK: ok, Message: msg, Details: withError(detail, ok, msg)}
}
