package checks

import (
	"context"
	"strings"
	"testing"

	"github.com/LanceSandino/is-it-up/v2/pkg/secrets"
)

// TestRunAll_MySQL_DirectConnectionFail tests MySQL check behavior when database connection fails.
func TestRunAll_MySQL_DirectConnectionFail(t *testing.T) {
	origMy := checkMySQLFn
	defer func() {
		checkMySQLFn = origMy
	}()

	// simulate database connection failure
	checkMySQLFn = func(ctx context.Context, c CheckConfig) (bool, string) {
		return false, "connection refused"
	}

	cfg := &Config{Checks: []CheckConfig{{
		Name:     "rds-main",
		Type:     "mysql",
		Address:  secrets.SecretValue{Raw: "db:3306"},
		User:     secrets.SecretValue{Raw: "u"},
		Password: secrets.SecretValue{Raw: "p"},
		Severity: "critical",
	}}}

	results := RunAll(context.Background(), cfg)
	if len(results) != 1 {
		t.Fatalf("expected 1 result (direct mysql check), got %d", len(results))
	}
	if results[0].Name != "rds-main" {
		t.Fatalf("expected result name rds-main, got %s", results[0].Name)
	}
	if results[0].OK {
		t.Fatalf("expected mysql result to be not OK when connection failed")
	}
}

// TestRunAll_MySQL_DatabaseConnectionRefused tests MySQL check when database connection fails.
func TestRunAll_MySQL_DatabaseConnectionRefused(t *testing.T) {
	ctx := context.Background()
	origMy := checkMySQLFn
	defer func() { checkMySQLFn = origMy }()

	checkMySQLFn = func(ctx context.Context, c CheckConfig) (bool, string) {
		return false, "connection refused"
	}

	cfg := &Config{Checks: []CheckConfig{{
		Name:     "db1",
		Type:     "mysql",
		Address:  secrets.SecretValue{Raw: "db:3306"},
		Severity: "critical",
	}}}

	res := RunAll(ctx, cfg)
	if len(res) != 1 {
		t.Fatalf("expected 1 result (direct mysql), got %d", len(res))
	}
	if res[0].OK {
		t.Fatalf("expected mysql check to fail")
	}
	if !strings.Contains(res[0].Message, "connection refused") {
		t.Fatalf("expected connection refused error, got: %s", res[0].Message)
	}
}

// TestRunAll_MySQL_Success tests successful MySQL check with direct database connection.
func TestRunAll_MySQL_Success(t *testing.T) {
	ctx := context.Background()
	origMy := checkMySQLFn
	defer func() { checkMySQLFn = origMy }()

	checkMySQLFn = func(ctx context.Context, cfg CheckConfig) (bool, string) {
		return true, "select 1 ok"
	}

	cfg := &Config{Checks: []CheckConfig{{
		Name:     "db2",
		Type:     "mysql",
		Address:  secrets.SecretValue{Raw: "db:3306"},
		Severity: "critical",
	}}}

	res := RunAll(ctx, cfg)
	if len(res) != 1 {
		t.Fatalf("expected 1 result (direct mysql), got %d", len(res))
	}
	if !res[0].OK {
		t.Fatalf("expected mysql check to succeed: %v", res[0])
	}
	if !strings.Contains(res[0].Message, "select 1 ok") {
		t.Fatalf("unexpected mysql message: %s", res[0].Message)
	}
}
