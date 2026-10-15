package checks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LanceSandino/is-it-up/v2/pkg/secrets"
)

func TestRunAllVarious(t *testing.T) {
	// stub package-level check functions
	origTCP := checkTCPFn
	origMy := checkMySQLFn
	origRedis := checkRedisFn
	origRedisInfo := checkRedisInfoFn
	origES := checkElasticsearchFn
	origAMQP := checkRabbitAMQPFn
	defer func() {
		checkTCPFn = origTCP
		checkMySQLFn = origMy
		checkRedisFn = origRedis
		checkRedisInfoFn = origRedisInfo
		checkElasticsearchFn = origES
		checkRabbitAMQPFn = origAMQP
	}()
	// start an httptest server to simulate ok and bad endpoints
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bad" {
			w.WriteHeader(500)
			_, _ = w.Write([]byte("internal"))
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	defer func() { ts.Close() }()
	checkTCPFn = func(ctx context.Context, address string) (bool, string) {
		if address == "bad:1" {
			return false, "dial tcp: connection refused"
		}
		return true, "tcp connect ok"
	}
	checkMySQLFn = func(ctx context.Context, c CheckConfig) (bool, string) {
		if c.Address.String() == "db:1" {
			return false, "select failed"
		}
		return true, "select 1 ok"
	}
	checkRedisFn = func(ctx context.Context, address, password string) (bool, string) {
		return true, "PONG"
	}
	checkRedisInfoFn = func(ctx context.Context, address, password string) (bool, string) {
		return false, "info parse failed"
	}
	checkElasticsearchFn = func(ctx context.Context, address, path string) (bool, string) {
		return true, "green"
	}
	checkRabbitAMQPFn = func(ctx context.Context, address, user, password, vhost string) (bool, string) {
		return true, "amqp connect ok"
	}

	cfg := &Config{Checks: []CheckConfig{
		{Name: "ok-http", Type: "http", Address: secrets.SecretValue{Raw: ts.URL}, Path: secrets.SecretValue{Raw: "/health"}, Severity: "critical"},
		{Name: "bad-http", Type: "http", Address: secrets.SecretValue{Raw: ts.URL}, Path: secrets.SecretValue{Raw: "/bad"}, Severity: "warning"},
		{Name: "tcp-good", Type: "tcp", Address: secrets.SecretValue{Raw: "good:1"}, Severity: "info"},
		{Name: "tcp-bad", Type: "tcp", Address: secrets.SecretValue{Raw: "bad:1"}, Severity: "critical"},
		{Name: "mysql-ok", Type: "mysql", Address: secrets.SecretValue{Raw: "db-ok:1"}, User: secrets.SecretValue{Raw: "u"}, Password: secrets.SecretValue{Raw: "p"}, Severity: "critical"},
		{Name: "mysql-bad", Type: "mysql", Address: secrets.SecretValue{Raw: "db:1"}, User: secrets.SecretValue{Raw: "u"}, Password: secrets.SecretValue{Raw: "p"}, Severity: "critical"},
		{Name: "redis-info", Type: "redis", Address: secrets.SecretValue{Raw: "redis:1"}, Password: secrets.SecretValue{Raw: ""}, RedisInfo: true, Severity: "warning"},
		{Name: "es-check", Type: "elasticsearch", Address: secrets.SecretValue{Raw: "http://es"}, Path: secrets.SecretValue{Raw: "_cluster/health"}, Severity: "critical"},
		{Name: "amqp-check", Type: "rabbitmq", Address: secrets.SecretValue{Raw: "amqp:1"}, User: secrets.SecretValue{Raw: "u"}, Password: secrets.SecretValue{Raw: "p"}, Vhost: secrets.SecretValue{Raw: "/"}, Severity: "critical"},
		{Name: "unknown", Type: "foo", Address: secrets.SecretValue{Raw: "x"}, Severity: "warning"},
	}}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	results := RunAll(ctx, cfg)

	// basic assertions
	if len(results) == 0 {
		t.Fatal("expected results, got none")
	}
	// find some specific results
	found := map[string]Result{}
	for _, r := range results {
		found[r.Name] = r
	}
	if r, ok := found["ok-http"]; !ok || !r.OK {
		t.Fatalf("ok-http expected ok, got %+v", r)
	}
	if r, ok := found["bad-http"]; !ok || r.OK {
		t.Fatalf("bad-http expected fail, got %+v", r)
	}
	if _, ok := found["tcp-bad"]; !ok {
		t.Fatal("tcp-bad result missing")
	}
	// mysql produces single direct database connection result
	if _, ok := found["mysql-ok"]; !ok {
		t.Fatal("mysql-ok missing")
	}
	if _, ok := found["mysql-bad"]; !ok {
		t.Fatal("mysql-bad missing")
	}
	if r, ok := found["unknown"]; !ok || r.OK {
		t.Fatalf("unknown expected not ok, got %+v", r)
	}
}
