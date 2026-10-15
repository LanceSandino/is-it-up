package checks

import (
	"context"
	"strings"
	"testing"

	"github.com/LanceSandino/is-it-up/v2/pkg/secrets"
)

// TestRunAll_RabbitMQ_AMQP_Success tests successful RabbitMQ AMQP connection with proper authentication.
func TestRunAll_RabbitMQ_AMQP_Success(t *testing.T) {
	ctx := context.Background()
	orig := checkRabbitAMQPFn
	defer func() { checkRabbitAMQPFn = orig }()

	checkRabbitAMQPFn = func(ctx context.Context, address, user, password, vhost string) (bool, string) {
		if user == "u" && password == "p" {
			return true, "amqp ok"
		}
		return false, "auth error"
	}

	cfg := &Config{Checks: []CheckConfig{{
		Name:     "r1",
		Type:     "rabbitmq",
		Address:  secrets.SecretValue{Raw: "rabbit:5672"},
		User:     secrets.SecretValue{Raw: "u"},
		Password: secrets.SecretValue{Raw: "p"},
		Vhost:    secrets.SecretValue{Raw: "/v1"},
		Severity: "info",
	}}}

	res := RunAll(ctx, cfg)
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	if !res[0].OK {
		t.Fatalf("expected ok=true, got false: %s", res[0].Message)
	}
	if !strings.Contains(res[0].Details, "vhost=/v1") {
		t.Fatalf("expected details to mention vhost, got: %s", res[0].Details)
	}
}

// TestRunAll_RabbitMQ_AMQPAndTCPFallback tests both AMQP method and TCP fallback behavior.
func TestRunAll_RabbitMQ_AMQPAndTCPFallback(t *testing.T) {
	origAMQP := checkRabbitAMQPFn
	origTCP := checkTCPFn
	defer func() { checkRabbitAMQPFn = origAMQP; checkTCPFn = origTCP }()

	// AMQP method should call AMQP fn
	calledAMQP := false
	checkRabbitAMQPFn = func(ctx context.Context, address, user, password, vhost string) (bool, string) {
		calledAMQP = true
		return true, "amqp ok"
	}
	cfg := &Config{Checks: []CheckConfig{{Name: "rmq-amqp", Type: "rabbitmq", Address: secrets.SecretValue{Raw: "a:1"}, User: secrets.SecretValue{Raw: "u"}, Password: secrets.SecretValue{Raw: "p"}, Vhost: secrets.SecretValue{Raw: "/"}, Severity: "critical"}}}
	res := RunAll(context.Background(), cfg)
	if len(res) != 1 || !calledAMQP || !res[0].OK {
		t.Fatalf("expected AMQP path to be called and OK; got res=%+v called=%v", res, calledAMQP)
	}

	// Simulate AMQP failure so fallback uses TCP
	checkRabbitAMQPFn = func(ctx context.Context, address, user, password, vhost string) (bool, string) {
		return false, "amqp fail"
	}
	calledTCP := false
	checkTCPFn = func(ctx context.Context, address string) (bool, string) {
		calledTCP = true
		return true, "tcp ok"
	}
	cfg2 := &Config{Checks: []CheckConfig{{Name: "rmq-tcp", Type: "rabbitmq", Address: secrets.SecretValue{Raw: "t:1"}, Severity: "critical"}}}
	res2 := RunAll(context.Background(), cfg2)
	if len(res2) != 1 || !calledTCP || !res2[0].OK {
		t.Fatalf("expected tcp fallback to be called and OK; got res=%+v called=%v", res2, calledTCP)
	}
}

// TestRunAll_RabbitMQ_TCPFallbackTimeout tests RabbitMQ fallback to TCP when AMQP method is not specified.
func TestRunAll_RabbitMQ_TCPFallbackTimeout(t *testing.T) {
	ctx := context.Background()
	origTCP := checkTCPFn
	defer func() { checkTCPFn = origTCP }()

	// Make AMQP fail so TCP fallback is attempted and also fails (timeout)
	checkRabbitAMQPFn = func(ctx context.Context, address, user, password, vhost string) (bool, string) {
		return false, "amqp fail"
	}
	checkTCPFn = func(ctx context.Context, address string) (bool, string) {
		return false, "timeout"
	}

	cfg := &Config{Checks: []CheckConfig{{
		Name:    "r2",
		Type:    "rabbitmq",
		Address: secrets.SecretValue{Raw: "rmq:5672"},
		// Method is not "amqp" -> fallback to tcp
		Severity: "warn",
	}}}

	res := RunAll(ctx, cfg)
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	if res[0].OK {
		t.Fatalf("expected ok=false for tcp fallback failure")
	}
	// When both AMQP and TCP fail the details currently contain AMQP failure info
	if !strings.Contains(res[0].Details, "amqp_error") {
		t.Fatalf("expected amqp_error detail when both AMQP and TCP fail, got: %s", res[0].Details)
	}
}

// TestRunAll_RabbitMQ_AMQP_SuccessAndFailure tests RabbitMQ RunAll integration with both success and failure paths.
func TestRunAll_RabbitMQ_AMQP_SuccessAndFailure(t *testing.T) {
	orig := checkRabbitAMQPFn
	defer func() { checkRabbitAMQPFn = orig }()

	checkRabbitAMQPFn = func(ctx context.Context, address, user, password, vhost string) (bool, string) {
		if address == "amqps://good:5672" {
			return true, "amqp connect ok"
		}
		return false, "access refused"
	}

	cfg := &Config{Checks: []CheckConfig{{Name: "rabbit", Type: "rabbitmq", Address: secrets.SecretValue{Raw: "good:5672"}, User: secrets.SecretValue{Raw: "guest"}, Password: secrets.SecretValue{Raw: "guest"}, Severity: "critical"}}}
	res := RunAll(context.Background(), cfg)
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	if !res[0].OK {
		t.Fatalf("expected ok result, got: %+v", res[0])
	}

	// failure path
	cfg = &Config{Checks: []CheckConfig{{Name: "rabbit", Type: "rabbitmq", Address: secrets.SecretValue{Raw: "bad:5672"}, User: secrets.SecretValue{Raw: "guest"}, Password: secrets.SecretValue{Raw: "guest"}, Severity: "critical"}}}
	res = RunAll(context.Background(), cfg)
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	if res[0].OK {
		t.Fatalf("expected failure result, got: %+v", res[0])
	}
}
