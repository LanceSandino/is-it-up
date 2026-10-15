// This file validates configuration structure before any network check runs.
// It depends on shared check types and reports field-specific validation failures.
package checks

import (
	"fmt"
	"strings"
	"time"
)

// ValidationError represents a configuration validation error.
type ValidationError struct {
	Field   string `json:"field"`
	Value   string `json:"value"`
	Message string `json:"message"`
	Check   string `json:"check,omitempty"`
}

// validateSMTPEncryption validates an SMTP encryption value.
func validateSMTPEncryption(value string) bool {
	if value == "" {
		return true // Empty is valid (defaults to STARTTLS)
	}
	enc := strings.ToLower(value)
	validEncryption := []string{"tls", "starttls", "ssl", "smtps", "none", "plain"}
	for _, v := range validEncryption {
		if enc == v {
			return true
		}
	}
	return false
}

// ValidateResolvedSecrets validates configuration fields that were resolved from secrets.
// This should be called after ResolveConfigSecrets to validate the actual resolved values.
func ValidateResolvedSecrets(cfg *Config) error {
	var errors []ValidationError

	for i, check := range cfg.Checks {
		fieldPrefix := fmt.Sprintf("checks[%d]", i)

		// Validate SMTP encryption if it was resolved from a secret
		if check.Type == "smtp" && check.SMTPEncryption.HasValue() {
			encValue := check.SMTPEncryption.String()
			if !validateSMTPEncryption(encValue) {
				value := encValue
				if check.SMTPEncryption.Sensitive {
					value = "REDACTED"
				}
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("%s.smtp_encryption", fieldPrefix),
					Value:   value,
					Message: "smtp_encryption must be one of: tls, starttls, ssl, smtps, none, plain",
					Check:   check.Name,
				})
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation failed after resolving secrets: %v", errors)
	}
	return nil
}

func (e *ValidationError) Error() string {
	if e.Check != "" {
		return fmt.Sprintf("validation error in check '%s', field '%s': %s", e.Check, e.Field, e.Message)
	}
	return fmt.Sprintf("validation error in field '%s': %s", e.Field, e.Message)
}

// ValidationErrors represents multiple validation errors.
type ValidationErrors []ValidationError

func (errs ValidationErrors) Error() string {
	if len(errs) == 0 {
		return "no validation errors"
	}
	if len(errs) == 1 {
		return errs[0].Error()
	}

	var messages []string
	for _, err := range errs {
		messages = append(messages, err.Error())
	}
	return fmt.Sprintf("multiple validation errors: %s", strings.Join(messages, "; "))
}

// ConfigValidator provides comprehensive configuration validation.
type ConfigValidator struct {
	supportedTypes map[string]bool
	maxTimeout     time.Duration
	minTimeout     time.Duration
}

// NewConfigValidator creates a new configuration validator.
func NewConfigValidator() *ConfigValidator {
	return &ConfigValidator{
		supportedTypes: map[string]bool{
			"http":          true,
			"tcp":           true,
			"mysql":         true,
			"postgres":      true,
			"redis":         true,
			"rabbitmq":      true,
			"elasticsearch": true,
			"opensearch":    true,
			"smtp":          true,
			"mongo":         true,
			"documentdb":    true,
		},
		maxTimeout: 5 * time.Minute,
		minTimeout: 1 * time.Second,
	}
}

// ValidateConfig performs comprehensive validation of a configuration.
func (v *ConfigValidator) ValidateConfig(cfg *Config) error {
	var errors ValidationErrors

	if cfg == nil {
		return &ValidationError{Field: "config", Message: "configuration cannot be nil"}
	}

	if len(cfg.Checks) == 0 {
		errors = append(errors, ValidationError{Field: "checks", Message: "at least one check must be defined"})
	}

	checkNames := make(map[string]bool)
	for i, check := range cfg.Checks {
		checkField := fmt.Sprintf("checks[%d]", i)

		if errs := v.validateCheck(check, checkField); len(errs) > 0 {
			errors = append(errors, errs...)
		}

		// Check for duplicate names
		if check.Name != "" {
			if checkNames[check.Name] {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("%s.name", checkField),
					Value:   check.Name,
					Message: "duplicate check name",
					Check:   check.Name,
				})
			}
			checkNames[check.Name] = true
		}
	}

	if len(errors) > 0 {
		return errors
	}
	return nil
}

// validateCheck validates a single check configuration.
func (v *ConfigValidator) validateCheck(check CheckConfig, fieldPrefix string) ValidationErrors {
	var errors ValidationErrors

	// Validate name
	if check.Name == "" {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.name", fieldPrefix),
			Message: "check name is required",
		})
	}

	// Validate type
	if check.Type == "" {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.type", fieldPrefix),
			Message: "check type is required",
			Check:   check.Name,
		})
	} else if !v.supportedTypes[check.Type] {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.type", fieldPrefix),
			Value:   check.Type,
			Message: fmt.Sprintf("unsupported check type, must be one of: %s", v.getSupportedTypes()),
			Check:   check.Name,
		})
	}

	// Validate address requirements
	if !check.Address.HasValue() && !check.Host.HasValue() {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.address", fieldPrefix),
			Message: "either 'address' or 'host' must be specified",
			Check:   check.Name,
		})
	}

	// Validate conflicting address fields
	if check.Address.HasValue() && (check.Host.HasValue() || check.Port.HasValue()) {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.address", fieldPrefix),
			Message: "cannot specify both 'address' and 'host/port' fields",
			Check:   check.Name,
		})
	}

	// Validate timeout
	if check.TimeoutMS < 0 {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.timeout_ms", fieldPrefix),
			Value:   fmt.Sprintf("%d", check.TimeoutMS),
			Message: "timeout cannot be negative",
			Check:   check.Name,
		})
	} else if check.TimeoutMS > 0 {
		timeout := time.Duration(check.TimeoutMS) * time.Millisecond
		if timeout > v.maxTimeout {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.timeout_ms", fieldPrefix),
				Value:   fmt.Sprintf("%d", check.TimeoutMS),
				Message: fmt.Sprintf("timeout cannot exceed %v", v.maxTimeout),
				Check:   check.Name,
			})
		}
		if timeout < v.minTimeout {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.timeout_ms", fieldPrefix),
				Value:   fmt.Sprintf("%d", check.TimeoutMS),
				Message: fmt.Sprintf("timeout cannot be less than %v", v.minTimeout),
				Check:   check.Name,
			})
		}
	}

	// Type-specific validation
	switch check.Type {
	case "http":
		if check.TLSMode != "" {
			if check.TLSMode != "skip-verify" && check.TLSMode != "required" {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("%s.tls_mode", fieldPrefix),
					Value:   check.TLSMode,
					Message: "tls_mode must be 'skip-verify' or 'required' for http checks",
					Check:   check.Name,
				})
			}
		}
		if check.TLSMode == "required" && check.Address.HasValue() && !strings.HasPrefix(strings.ToLower(check.Address.String()), "https://") {
			errors = append(errors, ValidationError{
				Field: fmt.Sprintf("%s.address", fieldPrefix), Message: "https:// address is required when tls_mode is required", Check: check.Name,
			})
		}
	case "mysql":
		if check.TLSMode != "" {
			switch check.TLSMode {
			case "skip-verify", "preferred", "required", "disable":
			default:
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("%s.tls_mode", fieldPrefix),
					Value:   check.TLSMode,
					Message: "tls_mode must be one of: skip-verify, preferred, required, disable for mysql",
					Check:   check.Name,
				})
			}
		}

		if !check.User.HasValue() {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.user", fieldPrefix),
				Message: "user is required for MySQL checks",
				Check:   check.Name,
			})
		}
	case "postgres":
		if check.TLSMode != "" {
			switch check.TLSMode {
			case "disable", "require", "verify-ca", "verify-full":
			default:
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("%s.tls_mode", fieldPrefix),
					Value:   check.TLSMode,
					Message: "tls_mode must be one of: disable, require, verify-ca, verify-full for postgres",
					Check:   check.Name,
				})
			}
		}

		if !check.User.HasValue() {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.user", fieldPrefix),
				Message: "user is required for PostgreSQL checks",
				Check:   check.Name,
			})
		}
	case "rabbitmq":
		if check.TLSMode != "" && check.TLSMode != "required" && check.TLSMode != "skip-verify" && check.TLSMode != "disable" {
			errors = append(errors, ValidationError{Field: fmt.Sprintf("%s.tls_mode", fieldPrefix), Value: check.TLSMode, Message: "tls_mode must be one of: required, skip-verify, disable for rabbitmq", Check: check.Name})
		}
	case "redis":
		if check.TLSMode != "" && check.TLSMode != "required" && check.TLSMode != "skip-verify" && check.TLSMode != "disable" {
			errors = append(errors, ValidationError{Field: fmt.Sprintf("%s.tls_mode", fieldPrefix), Value: check.TLSMode, Message: "tls_mode must be one of: required, skip-verify, disable for redis", Check: check.Name})
		}
	case "smtp":
		// SMTP encryption validation is handled after secret resolution in ValidateResolvedSecrets
		// This is because encryption values may come from secrets and need to be resolved first
	}

	return errors
}

// getSupportedTypes returns a comma-separated list of supported check types.
func (v *ConfigValidator) getSupportedTypes() string {
	var types []string
	for t := range v.supportedTypes {
		types = append(types, t)
	}
	return strings.Join(types, ", ")
}
