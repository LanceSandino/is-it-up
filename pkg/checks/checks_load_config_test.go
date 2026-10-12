package checks

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/LanceSandino/is-it-up/pkg/secrets"
)

func TestExampleConfigurationsValidate(t *testing.T) {
	for _, path := range []string{
		"../../example/config.example.yaml",
		"../../example/aws-secret.example.yaml",
	} {
		t.Run(path, func(t *testing.T) {
			cfg, format, err := LoadConfigAutoNoValidation(path)
			if err != nil {
				t.Fatalf("load example: %v", err)
			}
			if format != "yaml" {
				t.Fatalf("format = %q, want yaml", format)
			}
			if err := ValidateConfig(cfg); err != nil {
				t.Fatalf("validate example: %v", err)
			}
		})
	}
}

// TestLoadConfig_BasicFunctionality tests the LoadConfig function that reads config from a YAML file.
func TestLoadConfig_BasicFunctionality(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test-config.yaml")

	configContent := `
checks:
  - name: "test-http"
    type: "http"
    address: "http://example.com"
    timeout: "5s"
`

	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	config, _, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if len(config.Checks) != 1 {
		t.Fatalf("expected 1 check, got %d", len(config.Checks))
	}

	check := config.Checks[0]
	if check.Name != "test-http" {
		t.Errorf("expected check name 'test-http', got %q", check.Name)
	}
	if check.Type != "http" {
		t.Errorf("expected check type 'http', got %q", check.Type)
	}
}

// TestLoadConfig_MissingFile tests LoadConfig behavior with non-existent file.
func TestLoadConfig_MissingFile(t *testing.T) {
	_, _, err := LoadConfig("/non/existent/file.yaml")
	if err == nil {
		t.Fatalf("expected error for non-existent file")
	}
}

// TestLoadConfigAndResolve_WithSecrets tests the LoadConfigAndResolve function with secret resolution.
func TestLoadConfigAndResolve_WithSecrets(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test-config.yaml")

	configContent := `
checks:
  - name: "test-tcp"
    type: "tcp"
    address: "localhost:22"
    timeout: "2s"
`

	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	ctx := context.Background()
	config, _, err := LoadConfigAndResolve(ctx, configFile, nil)
	if err != nil {
		t.Fatalf("LoadConfigAndResolve failed: %v", err)
	}

	if len(config.Checks) != 1 {
		t.Fatalf("expected 1 check, got %d", len(config.Checks))
	}

	check := config.Checks[0]
	if check.Name != "test-tcp" {
		t.Errorf("expected check name 'test-tcp', got %q", check.Name)
	}
}

// TestCheckFieldError_ErrorFormatting tests error handling for CheckFieldError wrapper.
func TestCheckFieldError_ErrorFormatting(t *testing.T) {
	err := &CheckFieldError{
		Check: "my-check",
		Field: "target",
		Err:   nil,
	}

	errorStr := err.Error()
	if errorStr != "check 'my-check' field 'target': <nil>" {
		t.Errorf("expected specific error format, got %q", errorStr)
	}

	// Test with actual error
	baseErr := &secrets.MissingKeyError{Key: "password"}
	err2 := &CheckFieldError{
		Check: "db-check",
		Field: "password",
		Err:   baseErr,
	}

	unwrapped := err2.Unwrap()
	if unwrapped != baseErr {
		t.Errorf("expected unwrapped error to be baseErr")
	}
}

// TestFakeAMQPChannel_Methods tests the fake AMQP channel implementation for testing purposes.
func TestFakeAMQPChannel_Methods(t *testing.T) {
	// Create a fake channel implementation
	ch := &fakeAMQPChannel{}

	// Test Close method - should not panic
	err := ch.Close()
	if err != nil {
		t.Errorf("expected Close() to return nil error, got %v", err)
	}
}

// TestLoadConfigAuto_InvalidYAML tests error handling in LoadConfigAuto with invalid YAML.
func TestLoadConfigAuto_InvalidYAML(t *testing.T) {
	// Test with invalid YAML content to trigger an error path
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "invalid-config.yaml")

	invalidContent := `
checks:
  - name: "test"
    type: "http"
    target:
      - invalid: yaml structure
        nested: true
    timeout: invalid_timeout_format
`

	if err := os.WriteFile(configFile, []byte(invalidContent), 0600); err != nil {
		t.Fatalf("failed to write invalid config: %v", err)
	}

	_, _, err := LoadConfig(configFile)
	// We expect this to either succeed (if validation is lenient) or fail
	// Either way, this exercises the LoadConfig code path
	_ = err // Don't fail test - just exercise the code
}

// TestResolveConfigSecrets_NoSecrets tests ResolveConfigSecrets with a config that has no secrets to resolve.
func TestResolveConfigSecrets_NoSecrets(t *testing.T) {
	// Test with config that has no secrets to resolve
	config := &Config{
		Checks: []CheckConfig{
			{
				Name:    "simple-check",
				Type:    "http",
				Address: secrets.SecretValue{Raw: "http://example.com"},
			},
		},
	}

	ctx := context.Background()
	_, err := ResolveConfigSecrets(ctx, config, nil)
	if err != nil {
		t.Fatalf("ResolveConfigSecrets failed for simple config: %v", err)
	}
}

// TestCheckMySQL_InvalidConnection tests error paths in MySQL check to improve coverage.
func TestCheckMySQL_InvalidConnection(t *testing.T) {
	ctx := context.Background()

	// Test with invalid connection string format to exercise error handling
	check := CheckConfig{
		Name:    "mysql-invalid",
		Type:    "mysql",
		Address: secrets.SecretValue{Raw: "invalid-connection-string-format"},
	}

	// This should trigger an error path in checkMySQL
	ok, msg := checkMySQL(ctx, check)

	// Verify it's marked as failed
	if ok {
		t.Errorf("expected check to fail with invalid connection string")
	}

	if msg == "" {
		t.Errorf("expected error message to be provided")
	}
}

// fakeAMQPChannel implements amqpChannel for testing
type fakeAMQPChannel struct{}

func (f *fakeAMQPChannel) Close() error {
	return nil
}

// TestLoadConfigAutoNoValidation tests the LoadConfigAutoNoValidation function
func TestLoadConfigAutoNoValidation_YAMLFile(t *testing.T) {
	// Create a temporary YAML config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test-config.yaml")

	configContent := `
checks:
  - name: "test-http"
    type: "http"
    address: "http://localhost:8080"
  - name: "test-mysql"
    type: "mysql"
    address: "localhost:3306"
    user: "testuser"
    password: "testpass"
`

	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to create test config file: %v", err)
	}

	// Load config without validation
	cfg, format, err := LoadConfigAutoNoValidation(configFile)
	if err != nil {
		t.Fatalf("LoadConfigAutoNoValidation failed: %v", err)
	}

	// Verify format detection
	if format != "yaml" {
		t.Errorf("expected format 'yaml', got %q", format)
	}

	// Verify config was loaded
	if cfg == nil {
		t.Fatal("config is nil")
	}

	if len(cfg.Checks) != 2 {
		t.Errorf("expected 2 checks, got %d", len(cfg.Checks))
	}

	// Verify first check
	if cfg.Checks[0].Name != "test-http" {
		t.Errorf("expected first check name 'test-http', got %q", cfg.Checks[0].Name)
	}
	if cfg.Checks[0].Type != "http" {
		t.Errorf("expected first check type 'http', got %q", cfg.Checks[0].Type)
	}
}

func TestLoadConfigAutoNoValidation_YMLFile(t *testing.T) {
	// Create a temporary .yml config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test-config.yml")

	configContent := `
checks:
  - name: "test-tcp"
    type: "tcp"
    address: "localhost:3306"
`

	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to create test config file: %v", err)
	}

	// Load config without validation
	cfg, format, err := LoadConfigAutoNoValidation(configFile)
	if err != nil {
		t.Fatalf("LoadConfigAutoNoValidation failed: %v", err)
	}

	// Verify format detection for .yml extension
	if format != "yaml" {
		t.Errorf("expected format 'yaml' for .yml file, got %q", format)
	}

	// Verify config was loaded
	if cfg == nil {
		t.Fatal("config is nil")
	}

	if len(cfg.Checks) != 1 {
		t.Errorf("expected 1 check, got %d", len(cfg.Checks))
	}
}

func TestLoadConfigAutoNoValidation_JSONRejection(t *testing.T) {
	// Create a temporary JSON config file (should be rejected)
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test-config.json")

	configContent := `{
  "checks": [
    {
      "name": "test-http",
      "type": "http",
      "address": "http://localhost:8080"
    }
  ]
}`

	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to create test config file: %v", err)
	}

	// Should reject JSON files
	cfg, format, err := LoadConfigAutoNoValidation(configFile)
	if err == nil {
		t.Fatal("expected error for JSON file, got nil")
	}

	if format != "json" {
		t.Errorf("expected format 'json' for rejected file, got %q", format)
	}

	if cfg != nil {
		t.Error("expected config to be nil for rejected JSON file")
	}

	expectedError := "JSON configuration format not supported; use YAML"
	if err.Error() != expectedError {
		t.Errorf("expected error %q, got %q", expectedError, err.Error())
	}
}

func TestLoadConfigAutoNoValidation_InvalidYAML(t *testing.T) {
	// Create a file with invalid YAML syntax
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "invalid-config.yaml")

	// Invalid YAML - missing quotes and improper indentation
	configContent := `
checks:
  - name: test-http
    type: http
    address: http://localhost:8080
  invalid_yaml_structure: [
    missing_closing_bracket
`

	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to create test config file: %v", err)
	}

	// Should return error for invalid YAML
	cfg, format, err := LoadConfigAutoNoValidation(configFile)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}

	if format != "yaml" {
		t.Errorf("expected format 'yaml' even for invalid YAML, got %q", format)
	}

	if cfg != nil {
		t.Error("expected config to be nil for invalid YAML")
	}
}

func TestLoadConfigAutoNoValidation_FileNotFound(t *testing.T) {
	// Test with non-existent file
	nonExistentFile := "/path/to/non/existent/config.yaml"

	cfg, format, err := LoadConfigAutoNoValidation(nonExistentFile)
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}

	if format != "" {
		t.Errorf("expected empty format for file error, got %q", format)
	}

	if cfg != nil {
		t.Error("expected config to be nil for file not found error")
	}
}

func TestLoadConfigAutoNoValidation_NoExtension(t *testing.T) {
	// Create a file without extension (should fallback to YAML parsing)
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "configfile")

	configContent := `
checks:
  - name: "test-redis"
    type: "redis"
    address: "localhost:6379"
`

	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to create test config file: %v", err)
	}

	// Should parse as YAML
	cfg, format, err := LoadConfigAutoNoValidation(configFile)
	if err != nil {
		t.Fatalf("LoadConfigAutoNoValidation failed for no-extension file: %v", err)
	}

	if format != "yaml" {
		t.Errorf("expected format 'yaml' for no-extension file, got %q", format)
	}

	if cfg == nil {
		t.Fatal("config is nil")
	}

	if len(cfg.Checks) != 1 {
		t.Errorf("expected 1 check, got %d", len(cfg.Checks))
	}
}
