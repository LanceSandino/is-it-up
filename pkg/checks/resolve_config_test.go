package checks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/LanceSandino/is-it-up/v2/pkg/secrets"
)

// simpleFetcher is a mock SecretFetcher for testing that returns predefined values.
type simpleFetcher struct {
	data map[string]string
}

func (s *simpleFetcher) GetSecret(ctx context.Context, ref *secrets.SecretRef) (string, error) {
	if v, ok := s.data[ref.SecretID]; ok {
		return v, nil
	}
	return "", nil
}

// TestResolveConfigSecrets_ComposeAddress tests that ResolveConfigSecrets properly
// composes addresses from separate host and port fields for HTTP-based checks.
func TestResolveConfigSecrets_ComposeAddress(t *testing.T) {
	cfg := &Config{Checks: []CheckConfig{{
		Name: "svc",
		Type: "http",
		Host: secrets.SecretValue{Raw: "localhost"},
		Port: secrets.SecretValue{Raw: "8080"},
	}}}
	_, err := ResolveConfigSecrets(context.Background(), cfg, &simpleFetcher{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if cfg.Checks[0].Address.Raw != "http://localhost:8080" {
		t.Fatalf("unexpected composed address: %s", cfg.Checks[0].Address.Raw)
	}
}

// failingFetcher is a mock SecretFetcher that always returns errors for testing error paths.
type failingFetcher struct {
	errorMsg string
}

func (f *failingFetcher) GetSecret(ctx context.Context, ref *secrets.SecretRef) (string, error) {
	return "", fmt.Errorf("%s", f.errorMsg)
}

// TestResolveConfigSecrets_SecretFetchError tests that ResolveConfigSecrets returns
// CheckFieldError when secret resolution fails.
func TestResolveConfigSecrets_SecretFetchError(t *testing.T) {
	cfg := &Config{Checks: []CheckConfig{{
		Name:     "test-check",
		Type:     "mysql",
		Password: secrets.SecretValue{Ref: &secrets.SecretRef{SecretID: "missing-secret"}},
	}}}

	fetcher := &failingFetcher{errorMsg: "secret not found"}
	_, err := ResolveConfigSecrets(context.Background(), cfg, fetcher)

	if err == nil {
		t.Fatal("expected error when secret fetch fails")
	}

	// Check that we got a CheckFieldError with correct fields
	var fieldErr *CheckFieldError
	if !errors.As(err, &fieldErr) {
		t.Fatalf("expected CheckFieldError, got %T: %v", err, err)
	}

	if fieldErr.Check != "test-check" {
		t.Errorf("expected check name 'test-check', got %q", fieldErr.Check)
	}
	if fieldErr.Field != "password" {
		t.Errorf("expected field name 'password', got %q", fieldErr.Field)
	}
	if fieldErr.Err == nil {
		t.Error("expected wrapped error to be non-nil")
	}

	// Check error message format
	expectedMsg := "check 'test-check' field 'password': secret not found"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestResolveConfigSecrets_AllFieldsAndAddressComposition(t *testing.T) {
	ref := func(id string) secrets.SecretValue {
		return secrets.SecretValue{Ref: &secrets.SecretRef{SecretID: id}}
	}
	cfg := &Config{Checks: []CheckConfig{
		{
			Name: "all-fields", Type: "smtp", Address: ref("address"), Protocol: ref("protocol"),
			Host: ref("host"), Port: ref("port"), User: ref("user"), Password: ref("password"),
			Path: ref("path"), Database: ref("database"), Vhost: ref("vhost"),
			SMTPEncryption: ref("encryption"),
		},
		{Name: "tcp", Type: "tcp", Host: secrets.SecretValue{Raw: "127.0.0.1"}, Port: secrets.SecretValue{Raw: "80"}},
		{Name: "search", Type: "opensearch", Protocol: secrets.SecretValue{Raw: "https"}, Host: secrets.SecretValue{Raw: "search"}, Port: secrets.SecretValue{Raw: "443"}},
	}}
	fetcher := &simpleFetcher{data: map[string]string{
		"address": "mail:587", "protocol": "smtp", "host": "mail", "port": "587",
		"user": "operator", "password": "value", "path": "/health", "database": "app",
		"vhost": "/", "encryption": "starttls",
	}}
	count, err := ResolveConfigSecrets(context.Background(), cfg, fetcher)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if count != 10 {
		t.Fatalf("resolved %d secrets, want 10", count)
	}
	if cfg.Checks[1].Address.String() != "127.0.0.1:80" {
		t.Fatalf("tcp address = %q", cfg.Checks[1].Address.String())
	}
	if cfg.Checks[2].Address.String() != "https://search:443" {
		t.Fatalf("opensearch address = %q", cfg.Checks[2].Address.String())
	}
	if !cfg.Checks[1].Host.IsEmpty() || !cfg.Checks[1].Port.IsEmpty() {
		t.Fatal("composed host and port were not cleared")
	}

	if count, err := ResolveConfigSecrets(context.Background(), nil, fetcher); count != 0 || err != nil {
		t.Fatalf("nil config = (%d, %v)", count, err)
	}
	if count, err := ResolveConfigSecrets(context.Background(), cfg, nil); count != 0 || err != nil {
		t.Fatalf("nil fetcher = (%d, %v)", count, err)
	}
}

func TestLoadConfigAndResolve_ErrorStages(t *testing.T) {
	if _, _, err := LoadConfigAndResolve(context.Background(), "/not/present.yaml", &simpleFetcher{}); err == nil {
		t.Fatal("expected load error")
	}

	writeConfig := func(t *testing.T, body string) string {
		t.Helper()
		path := t.TempDir() + "/config.yaml"
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}

	resolveFailure := writeConfig(t, "checks:\n  - name: api\n    type: http\n    address: sm://missing\n")
	if _, _, err := LoadConfigAndResolve(context.Background(), resolveFailure, &failingFetcher{errorMsg: "unavailable"}); err == nil {
		t.Fatal("expected secret resolution error")
	}

	postResolveInvalid := writeConfig(t, "checks:\n  - name: api\n    type: http\n    address: sm://empty\n")
	if _, _, err := LoadConfigAndResolve(context.Background(), postResolveInvalid, &simpleFetcher{}); err == nil {
		t.Fatal("expected post-resolution config validation error")
	}

	resolvedValueInvalid := writeConfig(t, "checks:\n  - name: mail\n    type: smtp\n    address: mail:25\n    smtp_encryption: sm://mode\n")
	if _, _, err := LoadConfigAndResolve(context.Background(), resolvedValueInvalid, &simpleFetcher{data: map[string]string{"mode": "rot13"}}); err == nil {
		t.Fatal("expected resolved-value validation error")
	}
}
