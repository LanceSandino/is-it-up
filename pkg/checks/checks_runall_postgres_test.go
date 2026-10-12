package checks

import (
	"context"
	"strings"
	"testing"

	"github.com/LanceSandino/is-it-up/pkg/secrets"
)

// TestRunAll_Postgres_DirectConnectionFail tests PostgreSQL check behavior when database connection fails.
func TestRunAll_Postgres_DirectConnectionFail(t *testing.T) {
	origPostgres := checkPostgresFn
	defer func() {
		checkPostgresFn = origPostgres
	}()

	// simulate database connection failure
	checkPostgresFn = func(ctx context.Context, c CheckConfig) (bool, string) {
		return false, "connection refused"
	}

	cfg := &Config{Checks: []CheckConfig{{
		Name:     "postgres-main",
		Type:     "postgres",
		Address:  secrets.SecretValue{Raw: "db:5432"},
		User:     secrets.SecretValue{Raw: "u"},
		Password: secrets.SecretValue{Raw: "p"},
		Severity: "critical",
	}}}

	results := RunAll(context.Background(), cfg)
	if len(results) != 1 {
		t.Fatalf("expected 1 result (postgres check), got %d", len(results))
	}
	if results[0].Name != "postgres-main" {
		t.Fatalf("expected result name postgres-main, got %s", results[0].Name)
	}
	if results[0].OK {
		t.Fatalf("expected postgres result to be not OK when connection failed")
	}
}

// TestRunAll_Postgres_DatabaseConnectionRefused tests PostgreSQL check when database connection fails.
func TestRunAll_Postgres_DatabaseConnectionRefused(t *testing.T) {
	ctx := context.Background()
	origPostgres := checkPostgresFn
	defer func() { checkPostgresFn = origPostgres }()

	checkPostgresFn = func(ctx context.Context, c CheckConfig) (bool, string) {
		return false, "connection refused"
	}

	cfg := &Config{Checks: []CheckConfig{{
		Name:     "db1",
		Type:     "postgres",
		Address:  secrets.SecretValue{Raw: "db:5432"},
		Severity: "critical",
	}}}

	res := RunAll(ctx, cfg)
	if len(res) != 1 {
		t.Fatalf("expected 1 result (postgres), got %d", len(res))
	}
	if res[0].OK {
		t.Fatalf("expected postgres check to fail")
	}
	if !strings.Contains(res[0].Message, "connection refused") {
		t.Fatalf("expected connection refused error, got: %s", res[0].Message)
	}
}

// TestRunAll_Postgres_Success tests successful PostgreSQL check with direct database connection.
func TestRunAll_Postgres_Success(t *testing.T) {
	ctx := context.Background()
	origPostgres := checkPostgresFn
	defer func() { checkPostgresFn = origPostgres }()

	checkPostgresFn = func(ctx context.Context, cfg CheckConfig) (bool, string) {
		return true, "select 1 ok"
	}

	cfg := &Config{Checks: []CheckConfig{{
		Name:     "db2",
		Type:     "postgres",
		Address:  secrets.SecretValue{Raw: "db:5432"},
		Severity: "critical",
	}}}

	res := RunAll(ctx, cfg)
	if len(res) != 1 {
		t.Fatalf("expected 1 result (postgres), got %d", len(res))
	}
	if !res[0].OK {
		t.Fatalf("expected postgres check to succeed: %v", res[0])
	}
	if !strings.Contains(res[0].Message, "select 1 ok") {
		t.Fatalf("unexpected postgres message: %s", res[0].Message)
	}
}
