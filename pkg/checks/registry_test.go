package checks

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/LanceSandino/is-it-up/v2/pkg/secrets"
)

func TestRegisterCheckTypeDrivesValidationAndExecution(t *testing.T) {
	const checkType = "registry-test"
	RegisterCheckType(checkType, func(_ context.Context, cfg CheckConfig) CheckOutcome {
		return CheckOutcome{OK: true, Message: "registered check ran", Details: "address=" + cfg.Address.String()}
	})

	cfg := &Config{Checks: []CheckConfig{{
		Name:     "registry",
		Type:     " Registry-Test ",
		Address:  secrets.SecretValue{Raw: "example.test"},
		Severity: "info",
	}}}
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("registered type should validate: %v", err)
	}

	results := RunAll(context.Background(), cfg)
	if len(results) != 1 || !results[0].OK || results[0].Message != "registered check ran" {
		t.Fatalf("registered check did not run: %+v", results)
	}
	if results[0].Details != "address=example.test" || results[0].Severity != "info" {
		t.Fatalf("unexpected result metadata: %+v", results[0])
	}
}

func TestRegisterCheckTypeRejectsInvalidRegistrations(t *testing.T) {
	noop := func(context.Context, CheckConfig) CheckOutcome { return CheckOutcome{} }
	tests := map[string]func(){
		"empty name": func() { RegisterCheckType(" ", noop) },
		"nil runner": func() { RegisterCheckType("nil-runner-test", nil) },
		"duplicate": func() {
			RegisterCheckType("duplicate-registry-test", noop)
			RegisterCheckType("duplicate-registry-test", noop)
		},
		"built-in override": func() { RegisterCheckType("HTTP", noop) },
	}
	for name, register := range tests {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected registration to panic")
				}
			}()
			register()
		})
	}
}

func TestSupportedCheckTypesAreSortedAndIncludeBuiltIns(t *testing.T) {
	types := SupportedCheckTypes()
	if !slices.IsSorted(types) {
		t.Fatalf("supported types should be sorted, got %v", types)
	}
	for _, required := range []string{"documentdb", "elasticsearch", "http", "mongo", "mysql", "opensearch", "postgres", "rabbitmq", "redis", "smtp", "tcp"} {
		if !slices.Contains(types, required) {
			t.Fatalf("supported types should include %q, got %v", required, types)
		}
	}
}

func TestRunAllContainsRegisteredCheckPanic(t *testing.T) {
	RegisterCheckType("panic-plugin-test", func(context.Context, CheckConfig) CheckOutcome {
		panic("plugin failure")
	})
	results := RunAll(context.Background(), &Config{Checks: []CheckConfig{{Name: "panics", Type: "panic-plugin-test"}}})
	if len(results) != 1 || results[0].OK || !strings.Contains(results[0].Message, "checker panic recovered") {
		t.Fatalf("plugin panic was not contained: %+v", results)
	}
}

func TestRunAllAppliesDeadlineAndRedactionToRegisteredChecks(t *testing.T) {
	const sensitiveHost = "registry-secret-host"
	RegisterCheckType("deadline-plugin-test", func(ctx context.Context, cfg CheckConfig) CheckOutcome {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 2*time.Second {
			return CheckOutcome{OK: false, Message: "missing check deadline"}
		}
		return CheckOutcome{OK: false, Message: "failed for " + cfg.Host.String(), Details: "host=" + cfg.Host.String(), Severity: "warning"}
	})
	results := RunAll(context.Background(), &Config{Checks: []CheckConfig{{
		Name:      "deadline",
		Type:      "deadline-plugin-test",
		Host:      secrets.SecretValue{Raw: sensitiveHost, Sensitive: true},
		TimeoutMS: 1500,
		Severity:  "critical",
	}}})
	r := results[0]
	if r.Message == "missing check deadline" {
		t.Fatal("registered check did not receive the configured deadline")
	}
	if strings.Contains(r.Message+r.Details, sensitiveHost) {
		t.Fatalf("secret-backed value leaked from registered check: %+v", r)
	}
	if r.Severity != "warning" {
		t.Fatalf("outcome severity should override configured severity, got %q", r.Severity)
	}
}
