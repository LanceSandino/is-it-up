package checks

import (
	"context"
	"strings"
	"testing"

	"github.com/LanceSandino/is-it-up/v2/pkg/secrets"
)

// TestRunAll_Redis_InfoBehavior_FailureWarning tests Redis INFO behavior when PING succeeds but INFO fails.
// When INFO parsing fails, the check should be marked as failed with warning severity.
func TestRunAll_Redis_InfoBehavior_FailureWarning(t *testing.T) {
	origRedis := checkRedisFn
	origRedisInfo := checkRedisInfoFn
	defer func() { checkRedisFn = origRedis; checkRedisInfoFn = origRedisInfo }()

	// Case A: PING ok, INFO fails -> expect OK=false and Severity="warning"
	checkRedisFn = func(ctx context.Context, address, password string) (bool, string) {
		return true, "PONG"
	}
	checkRedisInfoFn = func(ctx context.Context, address, password string) (bool, string) {
		return false, "info parse failed"
	}
	cfg := &Config{Checks: []CheckConfig{{Name: "redis-info", Type: "redis", Address: secrets.SecretValue{Raw: "r:1"}, Password: secrets.SecretValue{Raw: "p"}, RedisInfo: true, Severity: "critical"}}}
	res := RunAll(context.Background(), cfg)
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	if res[0].OK {
		t.Fatalf("expected info failure to set OK=false, got ok")
	}
	if strings.ToLower(res[0].Severity) != "warning" {
		t.Fatalf("expected severity warning, got %s", res[0].Severity)
	}

	// Case B: PING ok, RedisInfo disabled -> expect OK=true
	checkRedisInfoFn = func(ctx context.Context, address, password string) (bool, string) {
		t.Fatalf("checkRedisInfoFn should not be called when RedisInfo is false")
		return true, ""
	}
	cfg2 := &Config{Checks: []CheckConfig{{Name: "redis-noinfo", Type: "redis", Address: secrets.SecretValue{Raw: "r:1"}, Password: secrets.SecretValue{Raw: "p"}, RedisInfo: false, Severity: "critical"}}}
	res2 := RunAll(context.Background(), cfg2)
	if len(res2) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res2))
	}
	if !res2[0].OK {
		t.Fatalf("expected ping ok when RedisInfo disabled, got %+v", res2[0])
	}
}

// TestRunAll_Redis_InfoFlagSuccess tests Redis INFO behavior with successful INFO parsing.
func TestRunAll_Redis_InfoFlagSuccess(t *testing.T) {
	ctx := context.Background()
	origR := checkRedisFn
	origI := checkRedisInfoFn
	defer func() { checkRedisFn = origR; checkRedisInfoFn = origI }()

	checkRedisFn = func(ctx context.Context, address, password string) (bool, string) {
		return true, "pong"
	}
	checkRedisInfoFn = func(ctx context.Context, address, password string) (bool, string) {
		return true, "role=master used_memory=123"
	}

	cfg := &Config{Checks: []CheckConfig{{
		Name:      "r3",
		Type:      "redis",
		Address:   secrets.SecretValue{Raw: "redis:6379"},
		RedisInfo: true,
		Severity:  "info",
	}}}

	res := RunAll(ctx, cfg)
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	if !res[0].OK {
		t.Fatalf("expected redis to be ok, got: %s", res[0].Message)
	}
	if !strings.Contains(res[0].Message, "info:") {
		t.Fatalf("expected info to be included in message, got: %s", res[0].Message)
	}
	if !strings.Contains(res[0].Details, "redis_info=true") {
		t.Fatalf("expected details to include redis_info=true, got: %s", res[0].Details)
	}
}

// TestRunAll_Redis_InfoDetailsIncluded tests that Redis INFO flag properly affects result details.
func TestRunAll_Redis_InfoDetailsIncluded(t *testing.T) {
	origRedis := checkRedisFn
	origInfo := checkRedisInfoFn
	defer func() { checkRedisFn = origRedis; checkRedisInfoFn = origInfo }()

	checkRedisFn = func(ctx context.Context, address, password string) (bool, string) {
		return true, "pong"
	}
	checkRedisInfoFn = func(ctx context.Context, address, password string) (bool, string) {
		return true, "role=master used_memory=12345"
	}

	cfg := &Config{Checks: []CheckConfig{{Name: "elasticache", Type: "redis", Address: secrets.SecretValue{Raw: "127.0.0.1:6379"}, RedisInfo: true}}}
	res := RunAll(context.Background(), cfg)
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	if res[0].Details == "" || !strings.Contains(res[0].Details, "redis_info=true") {
		t.Fatalf("expected details to contain redis_info=true; got %q", res[0].Details)
	}
}
