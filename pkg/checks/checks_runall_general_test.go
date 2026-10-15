package checks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LanceSandino/is-it-up/v2/pkg/secrets"
)

func TestRunAll_AppliesConfiguredTimeout(t *testing.T) {
	original := checkTCPFn
	defer func() { checkTCPFn = original }()

	checkTCPFn = func(ctx context.Context, _ string) (bool, string) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("expected the check context to carry timeout_ms")
		}
		remaining := time.Until(deadline)
		if remaining <= 0 || remaining > 1500*time.Millisecond {
			t.Fatalf("expected a 1250ms check deadline, remaining=%s", remaining)
		}
		return true, "tcp connect ok"
	}

	cfg := &Config{Checks: []CheckConfig{{
		Name:      "short-timeout",
		Type:      "tcp",
		Address:   secrets.SecretValue{Raw: "example.test:443"},
		TimeoutMS: 1250,
	}}}

	results := RunAll(context.Background(), cfg)
	if len(results) != 1 || !results[0].OK {
		t.Fatalf("expected successful result, got %+v", results)
	}
}

func TestConfiguredCheckTimeout_Defaults(t *testing.T) {
	tests := []struct {
		check CheckConfig
		want  time.Duration
	}{
		{check: CheckConfig{Type: "http"}, want: 4 * time.Second},
		{check: CheckConfig{Type: "tcp"}, want: 3 * time.Second},
		{check: CheckConfig{Type: "redis"}, want: 3 * time.Second},
		{check: CheckConfig{Type: "smtp"}, want: 10 * time.Second},
		{check: CheckConfig{Type: "http", TimeoutMS: 1500}, want: 1500 * time.Millisecond},
	}
	for _, test := range tests {
		if got := configuredCheckTimeout(test.check); got != test.want {
			t.Errorf("configuredCheckTimeout(%+v) = %s, want %s", test.check, got, test.want)
		}
	}
}

// TestRunAll_Elasticsearch_DefaultPath tests Elasticsearch check detail formatting with default path.
func TestRunAll_Elasticsearch_DefaultPath(t *testing.T) {
	origES := checkElasticsearchFn
	defer func() { checkElasticsearchFn = origES }()
	checkElasticsearchFn = func(ctx context.Context, address, path string) (bool, string) {
		// ignore path content; return ok
		return true, "green"
	}
	cfg := &Config{Checks: []CheckConfig{{Name: "es", Type: "elasticsearch", Address: secrets.SecretValue{Raw: "http://es"}, Path: secrets.SecretValue{Raw: ""}, Severity: "critical"}}}
	res := RunAll(context.Background(), cfg)
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	// details should include the default cluster health path when Path was empty
	if !strings.Contains(res[0].Details, "_cluster/health") {
		t.Fatalf("expected details to include default path, got %+v", res[0].Details)
	}
}

// TestRunAll_Elasticsearch_URLInDetails tests that Elasticsearch check details contain the full URL.
func TestRunAll_Elasticsearch_URLInDetails(t *testing.T) {
	origES := checkElasticsearchFn
	defer func() { checkElasticsearchFn = origES }()

	checkElasticsearchFn = func(ctx context.Context, address, path string) (bool, string) {
		return true, "cluster green"
	}

	cfg := &Config{Checks: []CheckConfig{{Name: "opensearch", Type: "elasticsearch", Address: secrets.SecretValue{Raw: "http://localhost:9200"}, Path: secrets.SecretValue{Raw: "_cluster/health"}}}}
	res := RunAll(context.Background(), cfg)
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	if res[0].Details == "" || !strings.Contains(res[0].Details, "http://localhost:9200/_cluster/health") {
		t.Fatalf("expected details to contain ES URL; got %q", res[0].Details)
	}
}

// TestRunAll_UnsupportedType tests RunAll behavior with an unknown check type.
func TestRunAll_UnsupportedType(t *testing.T) {
	cfg := &Config{Checks: []CheckConfig{{
		Name:     "mystery",
		Type:     "unknown-type",
		Address:  secrets.SecretValue{Raw: "x"},
		Severity: "warning",
	}}}
	results := RunAll(context.Background(), cfg)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].OK {
		t.Fatalf("unknown type should produce a failing result")
	}
	if results[0].Message != "unknown check type" {
		t.Fatalf("unexpected message: %s", results[0].Message)
	}
}

// TestRunAll_HTTPAndTCP tests HTTP and TCP checks running together in a single configuration.
func TestRunAll_HTTPAndTCP(t *testing.T) {
	// httptest server for http check
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	defer func() { ts.Close() }()

	origTCP := checkTCPFn
	defer func() { checkTCPFn = origTCP }()
	checkTCPFn = func(ctx context.Context, address string) (bool, string) {
		return true, "tcp connect ok"
	}

	cfg := &Config{Checks: []CheckConfig{
		{Name: "web", Type: "http", Address: secrets.SecretValue{Raw: ts.URL}, Path: secrets.SecretValue{Raw: "/"}, Severity: "critical"},
		{Name: "service-tcp", Type: "tcp", Address: secrets.SecretValue{Raw: "svc:1234"}, Severity: "info"},
	}}

	results := RunAll(context.Background(), cfg)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// verify names present
	found := map[string]bool{}
	for _, r := range results {
		found[r.Name] = r.OK
	}
	if ok, okp := found["web"]; !okp || !ok {
		t.Fatalf("expected web check OK, got %+v", results)
	}
	if ok, okp := found["service-tcp"]; !okp || !ok {
		t.Fatalf("expected service-tcp OK, got %+v", results)
	}
}
