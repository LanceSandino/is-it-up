package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/LanceSandino/is-it-up/v2/pkg/checks"
	"github.com/LanceSandino/is-it-up/v2/pkg/secrets"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "cfg-*.yaml")
	if err != nil {
		t.Fatalf("tempfile: %v", err)
	}
	if _, err := f.Write([]byte(content)); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close temp config: %v", err)
	}
	return f.Name()
}

func TestRun_NoSecrets_Success(t *testing.T) {
	cfg := map[string]interface{}{
		"checks": []map[string]interface{}{
			{"name": "svc", "type": "http", "address": "http://localhost:8080", "severity": "info"},
		},
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal cfg: %v", err)
	}
	path := writeTempConfig(t, string(b))
	defer func() { _ = os.Remove(path) }()

	opts := RunOptions{
		Out:    nil,
		Pretty: false,
		Debug:  false,
		RunAllOverride: func(ctx context.Context, cfg *checks.Config) []checks.Result {
			return []checks.Result{{Name: "svc", OK: true, Message: "ok", Severity: "info"}}
		},
	}

	code := Run(context.Background(), path, opts)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}

func TestRun_SecretsMissing_Fail(t *testing.T) {
	cfg := map[string]interface{}{
		"checks": []map[string]interface{}{
			{"name": "secret-svc", "type": "http", "address": "sm://my-secret#url", "severity": "critical"},
		},
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal cfg: %v", err)
	}
	path := writeTempConfig(t, string(b))
	defer func() { _ = os.Remove(path) }()

	opts := RunOptions{
		Out:    nil,
		Pretty: false,
		Debug:  false,
		// No SecretsFetcher injected; Run should fail when encountering sm:// refs
	}

	code := Run(context.Background(), path, opts)
	if code == 0 {
		t.Fatalf("expected non-zero exit code when secrets missing; got 0")
	}
}

// Provide a tiny mock that implements SecretFetcher for tests. It stores
// secret payloads by secret id and returns them on GetSecret.
type mockFetcher struct {
	store map[string]string
}

func (m *mockFetcher) GetSecret(ctx context.Context, ref *secrets.SecretRef) (string, error) {
	if m == nil || m.store == nil {
		return "", nil
	}
	if v, ok := m.store[ref.SecretID]; ok {
		return v, nil
	}
	return "", nil
}

func TestRun_DumpResolvedConfig_RedactsPasswords(t *testing.T) {
	cfg := map[string]interface{}{
		"checks": []map[string]interface{}{
			{
				"name":     "svc",
				"type":     "http",
				"address":  "http://localhost:8080",
				"password": "sm://pw-secret#value",
				"severity": "info",
			},
		},
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal cfg: %v", err)
	}
	path := writeTempConfig(t, string(b))
	defer func() { _ = os.Remove(path) }()

	// inject a fetcher that returns a JSON payload containing the password
	mf := &mockFetcher{store: map[string]string{"pw-secret": `{"value":"s3cr3t"}`}}
	var out bytes.Buffer
	opts := RunOptions{
		Out:            &out,
		Pretty:         false,
		Debug:          false,
		DumpConfig:     true,
		SecretsFetcher: mf,
	}

	code := Run(context.Background(), path, opts)
	if code != 0 {
		t.Fatalf("expected exit code 0 for dump path; got %d", code)
	}

	// Parse dumped JSON and assert password was redacted. The logger may
	// write structured JSON log entries to the same writer; find the
	// "checks" token and then locate the preceding '{' to extract the
	// configuration JSON object.
	buf := out.Bytes()
	idxChecks := bytes.Index(buf, []byte("\"checks\""))
	if idxChecks == -1 {
		t.Fatalf("no \"checks\" token found in output: %q", string(buf))
	}
	start := bytes.LastIndex(buf[:idxChecks], []byte("{"))
	if start == -1 {
		t.Fatalf("could not find JSON object start before checks token; buf=%q", string(buf))
	}
	var dumped checks.Config
	if err := json.Unmarshal(buf[start:], &dumped); err != nil {
		t.Fatalf("failed to parse dumped json: %v; buf=%q", err, string(buf))
	}
	if len(dumped.Checks) == 0 {
		t.Fatalf("dumped config missing checks")
	}
	pw := dumped.Checks[0].Password.Raw
	if pw != "REDACTED" {
		t.Fatalf("expected password REDACTED in dumped config, got %q", pw)
	}
}

func TestRun_WithInjectedFetcher_ResolvesReferences(t *testing.T) {
	cfg2 := map[string]interface{}{
		"checks": []map[string]interface{}{
			{
				"name":     "secret-svc",
				"type":     "http",
				"address":  "sm://my-secret#url",
				"severity": "info",
			},
		},
	}
	b2, err := yaml.Marshal(cfg2)
	if err != nil {
		t.Fatalf("marshal cfg2: %v", err)
	}
	path := writeTempConfig(t, string(b2))
	defer func() { _ = os.Remove(path) }()

	// inject a fetcher that resolves the URL
	mf := &mockFetcher{store: map[string]string{"my-secret": `{"url":"http://resolved:8080"}`}}
	opts := RunOptions{
		Out:            nil,
		Pretty:         false,
		Debug:          false,
		SecretsFetcher: mf,
		RunAllOverride: func(ctx context.Context, cfg *checks.Config) []checks.Result {
			// assert that the address has been resolved
			if cfg.Checks[0].Address.String() != "http://resolved:8080" {
				t.Fatalf("expected resolved address, got %s", cfg.Checks[0].Address.String())
			}
			return []checks.Result{{Name: "secret-svc", OK: true, Message: "ok", Severity: "info"}}
		},
	}

	code := Run(context.Background(), path, opts)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}

// Test error cases for better coverage
func TestRun_NoConfigFile(t *testing.T) {
	opts := RunOptions{Out: nil, Pretty: false, Debug: false}
	code := Run(context.Background(), "", opts)
	if code != 2 {
		t.Fatalf("expected exit code 2 for missing config, got %d", code)
	}
}

func TestRun_InvalidConfigFile(t *testing.T) {
	opts := RunOptions{Out: nil, Pretty: false, Debug: false}
	code := Run(context.Background(), "/nonexistent/config.yaml", opts)
	if code != 2 {
		t.Fatalf("expected exit code 2 for invalid config path, got %d", code)
	}
}

func TestRun_WithDebugAndPretty(t *testing.T) {
	cfg := map[string]interface{}{
		"checks": []map[string]interface{}{
			{"name": "test", "type": "http", "address": "http://localhost:8080", "severity": "info"},
		},
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal cfg: %v", err)
	}
	path := writeTempConfig(t, string(b))
	defer func() { _ = os.Remove(path) }()

	var out bytes.Buffer
	opts := RunOptions{
		Out:    &out,
		Pretty: true,
		Debug:  true,
		RunAllOverride: func(ctx context.Context, cfg *checks.Config) []checks.Result {
			return []checks.Result{{Name: "test", OK: true, Message: "ok", Severity: "info"}}
		},
	}

	code := Run(context.Background(), path, opts)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}

func TestRun_DumpConfigWithDebugStillRedactsSecrets(t *testing.T) {
	cfg := map[string]interface{}{
		"checks": []map[string]interface{}{
			{
				"name":     "secret-test",
				"type":     "http",
				"address":  "http://localhost:8080",
				"password": "sm://test-secret#pass",
				"severity": "info",
			},
		},
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal cfg: %v", err)
	}
	path := writeTempConfig(t, string(b))
	defer func() { _ = os.Remove(path) }()

	mf := &mockFetcher{store: map[string]string{"test-secret": `{"pass":"secret123"}`}}
	var out bytes.Buffer
	opts := RunOptions{
		Out:            &out,
		Pretty:         false,
		Debug:          true,
		DumpConfig:     true,
		SecretsFetcher: mf,
	}

	code := Run(context.Background(), path, opts)
	if code != 0 {
		t.Fatalf("expected exit code 0 for dump with debug, got %d", code)
	}

	// Debug logging must never turn a safe configuration dump into a secret dump.
	output := out.String()
	if bytes.Contains(out.Bytes(), []byte("secret123")) {
		t.Fatalf("debug dump leaked a resolved secret: %s", output)
	}
	if !bytes.Contains(out.Bytes(), []byte("REDACTED")) {
		t.Fatalf("expected redacted secret in debug dump, but got: %s", output)
	}
}

func TestRun_FailOnUnhealthy(t *testing.T) {
	path := writeTempConfig(t, "checks:\n  - name: api\n    type: http\n    address: https://example.invalid\n")
	defer func() { _ = os.Remove(path) }()

	code := Run(context.Background(), path, RunOptions{
		Out:             &bytes.Buffer{},
		FailOnUnhealthy: true,
		RunAllOverride: func(context.Context, *checks.Config) []checks.Result {
			return []checks.Result{{Name: "api", OK: false, Severity: "critical", Message: "down"}}
		},
	})
	if code != 1 {
		t.Fatalf("expected unhealthy exit code 1, got %d", code)
	}
}

func TestRun_SecretsResolutionError(t *testing.T) {
	cfg := map[string]interface{}{
		"checks": []map[string]interface{}{
			{
				"name":     "failing-secret",
				"type":     "http",
				"address":  "sm://nonexistent-secret#url",
				"severity": "info",
			},
		},
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal cfg: %v", err)
	}
	path := writeTempConfig(t, string(b))
	defer func() { _ = os.Remove(path) }()

	// Use empty fetcher to cause resolution failure
	mf := &mockFetcher{store: map[string]string{}}
	opts := RunOptions{
		Out:            nil,
		SecretsFetcher: mf,
	}

	code := Run(context.Background(), path, opts)
	if code != 2 {
		t.Fatalf("expected exit code 2 for secrets resolution failure, got %d", code)
	}
}

func TestRun_ConfigValidationError(t *testing.T) {
	// Create config with validation error (duplicate names)
	cfg := map[string]interface{}{
		"checks": []map[string]interface{}{
			{"name": "duplicate", "type": "http", "address": "http://localhost:8080", "severity": "info"},
			{"name": "duplicate", "type": "tcp", "address": "localhost:22", "severity": "info"},
		},
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal cfg: %v", err)
	}
	path := writeTempConfig(t, string(b))
	defer func() { _ = os.Remove(path) }()

	opts := RunOptions{Out: nil}
	code := Run(context.Background(), path, opts)
	if code != 2 {
		t.Fatalf("expected exit code 2 for validation error, got %d", code)
	}
}

func TestRun_DefaultAWSFetcher(t *testing.T) {
	// Test the path where no SecretsFetcher is provided and AWS fetcher creation fails
	cfg := map[string]interface{}{
		"checks": []map[string]interface{}{
			{"name": "test", "type": "http", "address": "http://localhost:8080", "severity": "info"},
		},
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal cfg: %v", err)
	}
	path := writeTempConfig(t, string(b))
	defer func() { _ = os.Remove(path) }()

	opts := RunOptions{
		Out: nil,
		// No SecretsFetcher provided - should use default AWS fetcher
		RunAllOverride: func(ctx context.Context, cfg *checks.Config) []checks.Result {
			return []checks.Result{{Name: "test", OK: true, Message: "ok", Severity: "info"}}
		},
	}

	code := Run(context.Background(), path, opts)
	// Should still work with default fetcher even if AWS isn't configured
	if code != 0 {
		t.Fatalf("expected exit code 0 with default fetcher, got %d", code)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestRun_DumpConfigWriteError(t *testing.T) {
	path := writeTempConfig(t, "checks:\n  - name: api\n    type: http\n    address: https://example.test\n")
	defer func() { _ = os.Remove(path) }()
	code := Run(context.Background(), path, RunOptions{Out: failingWriter{}, DumpConfig: true})
	if code != 1 {
		t.Fatalf("expected output error exit code 1, got %d", code)
	}
}

func TestRun_WarningDoesNotFailByDefault(t *testing.T) {
	path := writeTempConfig(t, "checks:\n  - name: api\n    type: http\n    address: https://example.test\n")
	defer func() { _ = os.Remove(path) }()
	var out bytes.Buffer
	code := Run(context.Background(), path, RunOptions{
		Out: &out,
		RunAllOverride: func(context.Context, *checks.Config) []checks.Result {
			return []checks.Result{{Name: "api", OK: false, Severity: "warning", Message: "slow"}}
		},
	})
	if code != 0 {
		t.Fatalf("expected monitoring-compatible exit code 0, got %d", code)
	}
	if !bytes.Contains(out.Bytes(), []byte("warnings")) {
		t.Fatalf("expected warning summary, got %s", out.String())
	}
}

func TestRun_InvalidResolvedSMTPEncryption(t *testing.T) {
	path := writeTempConfig(t, "checks:\n  - name: mail\n    type: smtp\n    address: mail.example.test:25\n    smtp_encryption: sm://mail-mode\n")
	defer func() { _ = os.Remove(path) }()
	code := Run(context.Background(), path, RunOptions{
		Out:            &bytes.Buffer{},
		SecretsFetcher: &mockFetcher{store: map[string]string{"mail-mode": "rot13"}},
	})
	if code != 2 {
		t.Fatalf("expected resolved validation exit code 2, got %d", code)
	}
}
