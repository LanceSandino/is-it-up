package checks

import (
	"strings"
	"testing"

	"github.com/LanceSandino/is-it-up/pkg/secrets"
)

func TestValidateSMTPEncryption(t *testing.T) {
	tests := []struct {
		name       string
		encryption string
		wantValid  bool
	}{
		{"valid tls", "tls", true},
		{"valid starttls", "starttls", true},
		{"valid ssl", "ssl", true},
		{"valid smtps", "smtps", true},
		{"valid none", "none", true},
		{"valid plain", "plain", true},
		{"invalid encryption", "invalid", false},
		{"empty encryption", "", true}, // Empty is valid (defaults)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := validateSMTPEncryption(tt.encryption)
			if isValid != tt.wantValid {
				t.Errorf("validateSMTPEncryption(%q) = %v, want valid = %v", tt.encryption, isValid, tt.wantValid)
			}
		})
	}
}

func TestConfigValidator_Basic(t *testing.T) {
	validator := NewConfigValidator()
	if validator == nil {
		t.Error("NewConfigValidator() returned nil")
	}

	// Test with empty config should return error (no checks defined)
	config := &Config{}
	err := validator.ValidateConfig(config)
	if err == nil {
		t.Error("ValidateConfig() with empty config should return error")
	}
}

// TestValidationError_Error tests the Error() method of ValidationError
func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      ValidationError
		expected string
	}{
		{
			name: "error with check name",
			err: ValidationError{
				Field:   "checks[0].address",
				Message: "address is required",
				Check:   "web-service",
			},
			expected: "validation error in check 'web-service', field 'checks[0].address': address is required",
		},
		{
			name: "error without check name",
			err: ValidationError{
				Field:   "checks",
				Message: "at least one check must be defined",
				Check:   "",
			},
			expected: "validation error in field 'checks': at least one check must be defined",
		},
		{
			name: "error with value",
			err: ValidationError{
				Field:   "checks[0].type",
				Value:   "invalid",
				Message: "unsupported check type",
				Check:   "test-check",
			},
			expected: "validation error in check 'test-check', field 'checks[0].type': unsupported check type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			if result != tt.expected {
				t.Errorf("ValidationError.Error() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestValidationErrors_Error tests the Error() method of ValidationErrors
func TestValidationErrors_Error(t *testing.T) {
	tests := []struct {
		name     string
		errs     ValidationErrors
		expected string
	}{
		{
			name:     "empty errors",
			errs:     ValidationErrors{},
			expected: "no validation errors",
		},
		{
			name: "single error",
			errs: ValidationErrors{
				{Field: "checks[0].name", Message: "name is required", Check: "test"},
			},
			expected: "validation error in check 'test', field 'checks[0].name': name is required",
		},
		{
			name: "multiple errors",
			errs: ValidationErrors{
				{Field: "checks[0].name", Message: "name is required", Check: "test1"},
				{Field: "checks[1].type", Message: "type is required", Check: "test2"},
			},
			expected: "multiple validation errors: validation error in check 'test1', field 'checks[0].name': name is required; validation error in check 'test2', field 'checks[1].type': type is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.errs.Error()
			if result != tt.expected {
				t.Errorf("ValidationErrors.Error() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestConfigValidator_UnsupportedType tests validation of unsupported check types
func TestConfigValidator_UnsupportedType(t *testing.T) {
	validator := NewConfigValidator()

	config := &Config{
		Checks: []CheckConfig{
			{
				Name: "test-check",
				Type: "unsupported-type",
			},
		},
	}

	err := validator.ValidateConfig(config)
	if err == nil {
		t.Fatal("expected validation error for unsupported type")
	}

	// Check that the error message contains the supported types
	errStr := err.Error()
	if !strings.Contains(errStr, "unsupported check type") {
		t.Errorf("error message should mention unsupported check type, got: %s", errStr)
	}

	// Verify getSupportedTypes was called by checking it contains expected types
	supported := validator.getSupportedTypes()
	expectedTypes := []string{"http", "tcp", "mysql", "postgres", "redis", "rabbitmq", "elasticsearch", "smtp"}
	for _, expected := range expectedTypes {
		if !strings.Contains(supported, expected) {
			t.Errorf("supported types should include %q, got: %s", expected, supported)
		}
	}
}

func TestConfigValidator_AllValidationBranches(t *testing.T) {
	value := func(raw string) secrets.SecretValue { return secrets.SecretValue{Raw: raw} }
	validator := NewConfigValidator()

	if err := validator.ValidateConfig(nil); err == nil {
		t.Fatal("expected nil configuration to fail")
	}

	cfg := &Config{Checks: []CheckConfig{
		{Name: "duplicate", Type: "http", Address: value("https://example.test"), Host: value("host"), TimeoutMS: -1, TLSMode: "invalid"},
		{Name: "duplicate", Type: "mysql", Address: value("db:3306"), TimeoutMS: 999999, TLSMode: "invalid"},
		{Name: "postgres", Type: "postgres", Address: value("db:5432"), TimeoutMS: 10, TLSMode: "invalid"},
		{Name: "missing-address", Type: "tcp"},
		{Address: value("host:1")},
	}}
	err := validator.ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected invalid configuration to fail")
	}
	for _, want := range []string{
		"duplicate check name", "cannot specify both", "timeout cannot be negative",
		"timeout cannot exceed", "timeout cannot be less", "MySQL checks",
		"PostgreSQL checks", "check name is required", "check type is required",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("validation error %q does not contain %q", err, want)
		}
	}

	valid := &Config{Checks: []CheckConfig{
		{Name: "http", Type: "http", Address: value("https://example.test"), TLSMode: "required"},
		{Name: "mysql", Type: "mysql", Address: value("db:3306"), User: value("user"), TLSMode: "preferred"},
		{Name: "postgres", Type: "postgres", Address: value("db:5432"), User: value("user"), TLSMode: "verify-full"},
		{Name: "rabbit", Type: "rabbitmq", Address: value("mq:5672")},
		{Name: "smtp", Type: "smtp", Address: value("mail:25")},
	}}
	if err := validator.ValidateConfig(valid); err != nil {
		t.Fatalf("valid configuration failed: %v", err)
	}
}

func TestValidateResolvedSecrets(t *testing.T) {
	valid := &Config{Checks: []CheckConfig{{
		Name: "mail", Type: "smtp", SMTPEncryption: secrets.SecretValue{Raw: "STARTTLS"},
	}}}
	if err := ValidateResolvedSecrets(valid); err != nil {
		t.Fatalf("valid resolved encryption failed: %v", err)
	}

	invalid := &Config{Checks: []CheckConfig{{
		Name: "mail", Type: "smtp", SMTPEncryption: secrets.SecretValue{Raw: "rot13"},
	}}}
	if err := ValidateResolvedSecrets(invalid); err == nil || !strings.Contains(err.Error(), "smtp_encryption") {
		t.Fatalf("expected smtp encryption error, got %v", err)
	}
}
