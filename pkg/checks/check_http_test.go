package checks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LanceSandino/is-it-up/pkg/secrets"
)

// TestCheckHTTP_StatusCodes tests HTTP check behavior with various HTTP status codes.
// Verifies that 2xx and 3xx codes are considered successful, while 4xx and 5xx codes fail.
func TestCheckHTTP_StatusCodes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			w.WriteHeader(200)
			// In test mocks, write errors are rare but we handle them gracefully
			if _, err := w.Write([]byte("ok")); err != nil {
				// Log but don't fail the test - this is a mock server
				w.WriteHeader(500)
			}
		case "/redir":
			w.Header().Set("Location", "/ok")
			w.WriteHeader(302)
		case "/notfound":
			w.WriteHeader(404)
		case "/err":
			w.WriteHeader(500)
		default:
			w.WriteHeader(200)
		}
	}))
	defer ts.Close()

	cases := []struct {
		name   string
		path   string
		wantOK bool
	}{
		{"200 OK", "/ok", true},
		{"302 Redirect", "/redir", true},
		{"404 NotFound", "/notfound", false},
		{"500 ServerErr", "/err", false},
	}

	for _, c := range cases {
		cfg := CheckConfig{
			Name:    "test",
			Type:    "http",
			Address: secrets.SecretValue{Raw: ts.URL},
			Path:    secrets.SecretValue{Raw: c.path},
		}
		ok, msg := checkHTTP(context.Background(), cfg)
		if ok != c.wantOK {
			t.Fatalf("%s: expected ok=%v got=%v msg=%q", c.name, c.wantOK, ok, msg)
		}
		if !strings.Contains(msg, "status ") {
			t.Fatalf("%s: expected status in message, got %q", c.name, msg)
		}
	}
}

// TestCheckHTTP_PathJoining tests proper URL path construction for HTTP checks.
// Verifies that paths are correctly joined when address lacks trailing slash and path lacks leading slash.
func TestCheckHTTP_PathJoining(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/some/path" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(200)
	}))
	defer ts.Close()

	// address without trailing slash and path without leading slash
	cfg := CheckConfig{
		Name:    "test",
		Type:    "http",
		Address: secrets.SecretValue{Raw: ts.URL},
		Path:    secrets.SecretValue{Raw: "some/path"},
	}
	ok, msg := checkHTTP(context.Background(), cfg)
	if !ok {
		t.Fatalf("expected ok=true got false msg=%q", msg)
	}
}

// TestCheckHTTP_TLSModes tests HTTP check TLS configuration modes.
// Verifies that skip-verify allows self-signed certificates while required fails.
func TestCheckHTTP_TLSModes(t *testing.T) {
	// Create a test server with self-signed certificate
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		// In test mocks, write errors are rare but we handle them gracefully
		if _, err := w.Write([]byte("ok")); err != nil {
			// Log but don't fail the test - this is a mock server
			w.WriteHeader(500)
		}
	}))
	defer ts.Close()

	// Test with skip-verify (should succeed with self-signed cert)
	cfg := CheckConfig{
		Name:    "test-skip-verify",
		Type:    "http",
		Address: secrets.SecretValue{Raw: ts.URL},
		TLSMode: "skip-verify",
	}
	ok, msg := checkHTTP(context.Background(), cfg)
	if !ok {
		t.Fatalf("skip-verify should succeed with self-signed cert, got false msg=%q", msg)
	}

	// Test with required (should fail with self-signed cert)
	cfg = CheckConfig{
		Name:    "test-required",
		Type:    "http",
		Address: secrets.SecretValue{Raw: ts.URL},
		TLSMode: "required",
	}
	ok, msg = checkHTTP(context.Background(), cfg)
	if ok {
		t.Fatalf("required should fail with self-signed cert, got true msg=%q", msg)
	}
	if !strings.Contains(msg, "certificate") {
		t.Fatalf("expected certificate error message, got %q", msg)
	}

	// Test with empty/default (should fail with self-signed cert like required)
	cfg = CheckConfig{
		Name:    "test-default",
		Type:    "http",
		Address: secrets.SecretValue{Raw: ts.URL},
		TLSMode: "",
	}
	ok, msg = checkHTTP(context.Background(), cfg)
	if ok {
		t.Fatalf("default should fail with self-signed cert, got true msg=%q", msg)
	}
	if !strings.Contains(msg, "certificate") {
		t.Fatalf("expected certificate error message, got %q", msg)
	}
}
